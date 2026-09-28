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
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.client.SupportPortableDeploySubstitutionParams;
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
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;

/**
 * Direct Esper 9.0.0 oracle for the pattern-timer-interval-forms-591 unit:
 * PatternObserverTimerInterval ords 1/2/3/5/6 — the five static-spec-resolution
 * forms sharing one external-clock harness (arm-at-deploy under advanceTime(0),
 * or sendCurrentTime("2002-02-01T09:00:00.000") for month-scoped):
 * advanceTime to deadline minus one millisecond stays silent, the milestone(0)
 * savepoint between the probes carries no step, advanceTime to the exact
 * deadline fires exactly one listener batch, undeployAll ends the case.
 * Each case runs on a fresh runtime like the regression suite's fresh
 * per-execution environment.
 *
 * Per-ordinal setup fidelity: interval-spec deploys the literal form
 * `timer:interval(1 minute 2 seconds)`; interval-spec-variables deploys the two
 * `@public create variable double` statements onto an accumulated compiler path
 * (the suite's RegressionPath) before the s0 statement; interval-spec-expression
 * does the same for MOne/SOne; interval-spec-prepared-stmt compiles the
 * `?::int minute ?::int seconds` statement and deploys it with
 * SupportPortableDeploySubstitutionParams().add(1,1).add(2,2) — deploy-time
 * substitution parameters, not variables; month-scoped arms
 * `timer:interval(1 month)` at 2002-02-01T09:00:00.000 so the boundary is
 * 2002-03-01T09:00:00.000 (calendar month recurrence, not a fixed day span).
 * The scenario pins those instants in UTC (`Z` suffix); under the harness's
 * -Duser.timezone=UTC they equal the suite's sendTimer/sendCurrentTime values.
 * select * over a tagless timer match projects one empty row per fire.
 */
public final class PatternTimerIntervalForms591ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "pattern-timer-interval-forms-591";
    private static final String DESCRIPTION =
            "PatternObserverTimerInterval ords 1/2/3/5/6 — the five static-spec-resolution "
                    + "forms sharing one external-clock harness, replayed as five cases on "
                    + "fresh runtimes: advanceTime(0) (sendCurrentTime(2002-02-01T09:00:00.000) "
                    + "for month-scoped) before deploy, then deploy the pinned s0 pattern "
                    + "statement and listener, advanceTime to one millisecond before the "
                    + "deadline (silent), milestone(0) (a savepoint carrying no step), "
                    + "advanceTime to the exact deadline (ONE fire) and undeployAll. "
                    + "(interval-spec) PatternIntervalSpec deploys `@name('s0') select * "
                    + "from pattern [timer:interval(1 minute 2 seconds)]` — a fixed 62000ms "
                    + "deadline. (interval-spec-variables) PatternIntervalSpecVariables "
                    + "deploys `@public create variable double M_isv=1` and `@public "
                    + "create variable double S_isv=2` (RegressionPath-scoped) then "
                    + "`@name('s0') select * from pattern [timer:interval(M_isv minute "
                    + "S_isv seconds)]` — the double variables are read once at arming. "
                    + "(interval-spec-expression) PatternIntervalSpecExpression deploys "
                    + "create-variable doubles MOne=1/SOne=2 then `@name('s0') select * "
                    + "from pattern [timer:interval(MOne*60+SOne seconds)]` — the same "
                    + "62000ms deadline via double arithmetic. (interval-spec-prepared-stmt) "
                    + "PatternIntervalSpecPreparedStmt compiles `@name('s0') select * "
                    + "from pattern [timer:interval(?::int minute ?::int seconds)]` and "
                    + "deploys it with statement substitution parameters add(1,1).add(2,2) "
                    + "— deploy-time (not variable) binding. (month-scoped) PatternMonthScoped "
                    + "deploys `@name('s0') select * from pattern [timer:interval(1 month)]` "
                    + "at sendCurrentTime(2002-02-01T09:00:00.000): `1 month` is a calendar "
                    + "recurrence so the boundary is 2002-03-01T09:00:00.000 (Feb 1 + 1 "
                    + "month), not a fixed day span. The deadline contract is strictly-after "
                    + "arming: advancing to deadline minus one millisecond must not fire. "
                    + "select * over a tagless timer match projects one empty row per fire. "
                    + "env.milestone(0) savepoints carry no steps.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/"
                    + "PatternObserverTimerInterval.java";
    private static final String[] JAVA_SOURCE_FILES = {
            JAVA_SOURCE,
            "common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java"
    };
    private static final String[] RUNTIME_IDS = {
            "java-runtime-d5ad6ad9d226628f8383",
            "java-runtime-bbb75d6bcc54f29c7652",
            "java-runtime-97c5e6b5e4c46cc0adb4",
            "java-runtime-ea394f5795b88ddd71c9",
            "java-runtime-28fc7f508485cbc765a2"
    };
    private static final String[] EXECUTIONS = {
            "PatternIntervalSpec",
            "PatternIntervalSpecVariables",
            "PatternIntervalSpecExpression",
            "PatternIntervalSpecPreparedStmt",
            "PatternMonthScoped"
    };
    private static final String[] STATIC_IDS = {
            "java-1422565b568236b2bfea",
            "java-1422565b568236b2bfea",
            "java-1422565b568236b2bfea",
            "java-1422565b568236b2bfea",
            "java-1422565b568236b2bfea"
    };

    private static final String[] CASES = {
            "interval-spec", "interval-spec-variables", "interval-spec-expression",
            "interval-spec-prepared-stmt", "month-scoped"
    };
    private static final int[] CASE_ORDINALS = {1, 2, 3, 5, 6};

    // Byte-exact statement texts (PatternObserverTimerInterval.java verbatim):
    // the five s0 pattern statements plus the four RegressionPath-scoped
    // `@public create variable double` deployments of the variables and
    // expression cases. The prepared-stmt EPL keeps its `?::int` casts.
    private static final String EPL_INTERVAL_SPEC =
            "@name('s0') select * from pattern [timer:interval(1 minute 2 seconds)]";
    private static final String EPL_VAR_M_ISV =
            "@public create variable double M_isv=1";
    private static final String EPL_VAR_S_ISV =
            "@public create variable double S_isv=2";
    private static final String EPL_INTERVAL_VARS =
            "@name('s0') select * from pattern [timer:interval(M_isv minute S_isv seconds)]";
    private static final String EPL_VAR_MONE =
            "@public create variable double MOne=1";
    private static final String EPL_VAR_SONE =
            "@public create variable double SOne=2";
    private static final String EPL_INTERVAL_EXPR =
            "@name('s0') select * from pattern [timer:interval(MOne*60+SOne seconds)]";
    private static final String EPL_INTERVAL_PREPARED =
            "@name('s0') select * from pattern [timer:interval(?::int minute ?::int seconds)]";
    private static final String EPL_INTERVAL_MONTH =
            "@name('s0') select * from pattern [timer:interval(1 month)]";

    private static final String[][] CASE_EPLS = {
            {EPL_INTERVAL_SPEC},
            {EPL_VAR_M_ISV, EPL_VAR_S_ISV, EPL_INTERVAL_VARS},
            {EPL_VAR_MONE, EPL_VAR_SONE, EPL_INTERVAL_EXPR},
            {EPL_INTERVAL_PREPARED},
            {EPL_INTERVAL_MONTH}
    };

    private static final String[] OBSERVATIONS = {
            "listener x1; advanceTime(0) arms, deploy s0 `timer:interval(1 minute 2 "
                    + "seconds)`, advanceTime(61999) silent, milestone(0) no-step, "
                    + "advanceTime(62000) fires ONE row (select * over a tagless timer "
                    + "match yields the empty row), undeployAll",
            "listener x1; advanceTime(0) arms, deploy `@public create variable double "
                    + "M_isv=1` + `@public create variable double S_isv=2` on the shared "
                    + "path then s0 `timer:interval(M_isv minute S_isv seconds)` — "
                    + "variables read once at arming give the same 62000ms deadline; "
                    + "advanceTime(61999) silent, milestone(0) no-step, "
                    + "advanceTime(62000) fires ONE row, undeployAll",
            "listener x1; advanceTime(0) arms, deploy `@public create variable double "
                    + "MOne=1` + `@public create variable double SOne=2` then s0 "
                    + "`timer:interval(MOne*60+SOne seconds)` — double arithmetic yields "
                    + "the same 62000ms deadline; advanceTime(61999) silent, milestone(0) "
                    + "no-step, advanceTime(62000) fires ONE row, undeployAll",
            "listener x1; advanceTime(0) arms, compile `@name('s0') select * from pattern "
                    + "[timer:interval(?::int minute ?::int seconds)]` and deploy with "
                    + "statement substitution parameters add(1,1).add(2,2) (deploy-time "
                    + "binding, not variables); advanceTime(61999) silent, milestone(0) "
                    + "no-step, advanceTime(62000) fires ONE row, undeployAll",
            "listener x1; sendCurrentTime(2002-02-01T09:00:00.000) arms, deploy s0 "
                    + "`timer:interval(1 month)` — calendar recurrence: the boundary is "
                    + "2002-03-01T09:00:00.000 (Feb 1 + 1 calendar month, not a fixed day "
                    + "span); advanceTime(boundary-1ms) silent, milestone(0) no-step, "
                    + "advanceTime(boundary) fires ONE row, undeployAll"
    };

    // Pinned op sequence per case (after the case marker). The milestone(0)
    // savepoint between the silent probe and the firing advance is harness
    // machinery and carries no step.
    private static final String[][] CASE_OPS = {
            {"advance-time", "deploy", "advance-time", "advance-time", "undeploy-all"},
            {"advance-time", "deploy", "deploy", "deploy",
                    "advance-time", "advance-time", "undeploy-all"},
            {"advance-time", "deploy", "deploy", "deploy",
                    "advance-time", "advance-time", "undeploy-all"},
            {"advance-time", "deploy", "advance-time", "advance-time", "undeploy-all"},
            {"advance-time", "deploy", "advance-time", "advance-time", "undeploy-all"}
    };

    // Pinned deploy statement labels and EPLs per case; the create-* labels are
    // the `@public create variable` deployments and carry no listener.
    private static final String[][] CASE_DEPLOYS = {
            {"s0"},
            {"create-m_isv", "create-s_isv", "s0"},
            {"create-mone", "create-sone", "s0"},
            {"s0"},
            {"s0"}
    };

    // Pinned advance instants per case (UTC `Z` — sendTimer(0/61999/62000)
    // for the fixed forms, sendCurrentTime for month-scoped).
    private static final String[][] CASE_ADVANCES = {
            {"1970-01-01T00:00:00Z", "1970-01-01T00:01:01.999Z", "1970-01-01T00:01:02Z"},
            {"1970-01-01T00:00:00Z", "1970-01-01T00:01:01.999Z", "1970-01-01T00:01:02Z"},
            {"1970-01-01T00:00:00Z", "1970-01-01T00:01:01.999Z", "1970-01-01T00:01:02Z"},
            {"1970-01-01T00:00:00Z", "1970-01-01T00:01:01.999Z", "1970-01-01T00:01:02Z"},
            {"2002-02-01T09:00:00Z", "2002-03-01T08:59:59.999Z", "2002-03-01T09:00:00Z"}
    };

    // Pinned deploy payload: only the prepared-stmt deploy carries positional
    // substitution parameters, JSON-pinned as [1, 2] (add(1,1).add(2,2)).
    private static final int[] PREPARED_PARAM_VALUES = {1, 2};

    private static final int EXPECTED_STEPS = 34;
    private static final int EXPECTED_RECORDS = 5;

    private PatternTimerIntervalForms591ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: PatternTimerIntervalForms591ScenarioOracle <scenario.json>");
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
            // The accumulated compiler path mirrors the regression suite's
            // RegressionPath: the create-variable deployments' public variables
            // resolve in the s0 compile.
            List<EPCompiled> path = new ArrayList<>();
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
                    case "advance-time": {
                        // sendTimer/sendCurrentTime: the pinned `at` instant in
                        // UTC equals the suite's advanceTime millisecond target.
                        runtime.getEventService().advanceTime(
                                Instant.parse(step.getString("at", "")).toEpochMilli());
                        break;
                    }
                    case "deploy": {
                        String label = step.getString("statement", "");
                        String epl = step.getString("epl", "");
                        EPCompiled compiled = compileWithPath(configuration, path, epl);
                        path.add(compiled);
                        DeploymentOptions options = new DeploymentOptions()
                                .setDeploymentId(SCENARIO_ID + "-" + caseIndex
                                        + "-" + (deployIndex++));
                        JsonValue payload = step.get("payload");
                        if (payload != null) {
                            // The prepared-stmt deploy carries the pinned
                            // positional substitution parameters [1, 2] —
                            // SupportPortableDeploySubstitutionParams()
                            // .add(1,1).add(2,2).
                            JsonArray values = payload.asArray();
                            SupportPortableDeploySubstitutionParams params =
                                    new SupportPortableDeploySubstitutionParams();
                            for (int position = 0; position < values.size(); position++) {
                                params.add(position + 1, values.get(position).asInt());
                            }
                            options.setStatementSubstitutionParameter(params);
                        }
                        EPDeployment deployment = runtime.getDeploymentService()
                                .deploy(compiled, options);
                        // Only the s0 statement carries a listener
                        // (env.addListener("s0")); the create-variable
                        // deployments declare public variables and have no
                        // subscriber surface.
                        if ("s0".equals(label)) {
                            EPStatement statement = requireStatement(deployment, label);
                            statement.addListener(new TraceWriter(
                                    records, caseName, statement, runtime));
                        }
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
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

    // compileWithPath mirrors RegressionEnvironmentBase.compileWCheckedEx(epl,
    // path): the compiler sees the configuration plus the accumulated compiled
    // modules (the RegressionPath) so the variables/expression s0 deployments
    // resolve the public variables of the earlier create-variable deployments.
    private static EPCompiled compileWithPath(Configuration configuration,
                                              List<EPCompiled> path, String epl) throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().addAll(path);
        return EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
    }

    private static EPStatement requireStatement(EPDeployment deployment, String name) {
        for (EPStatement candidate : deployment.getStatements()) {
            if (name.equals(candidate.getName())) {
                return candidate;
            }
        }
        throw new IllegalStateException("statement " + name + " was not deployed");
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
     * Pins the full step sequence: each case marker is followed by the pinned
     * ops — arm advance-time, the deploys (byte-exact EPL per step, payload
     * [1, 2] only on the prepared-stmt s0 deploy), the silent and firing
     * advance-time probes and undeploy-all. The milestone(0) savepoints between
     * the probes carry no steps. Unknown step fields are rejected.
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
            int advances = 0;
            for (String operation : CASE_OPS[caseIndex]) {
                JsonObject step = object(steps.get(cursor), "step " + cursor);
                if (!operation.equals(string(step, "op"))
                        || !CASES[caseIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("step " + cursor + " is not pinned");
                }
                switch (operation) {
                    case "advance-time":
                        requireFields(step, "op", "case", "at");
                        if (!CASE_ADVANCES[caseIndex][advances]
                                .equals(string(step, "at"))) {
                            throw new IllegalArgumentException("advance-time step " + cursor
                                    + " instant is not pinned: expected "
                                    + CASE_ADVANCES[caseIndex][advances]
                                    + " got " + string(step, "at"));
                        }
                        advances++;
                        break;
                    case "deploy": {
                        String label = CASE_DEPLOYS[caseIndex][deploys];
                        if (!label.equals(string(step, "statement"))
                                || !CASE_EPLS[caseIndex][deploys]
                                        .equals(string(step, "epl"))) {
                            throw new IllegalArgumentException(
                                    "deploy step " + cursor + " is not pinned");
                        }
                        if (caseIndex == 3 && "s0".equals(label)) {
                            // Prepared-stmt deploy pins the positional
                            // substitution parameters [1, 2].
                            requireFields(step, "op", "case", "statement", "epl",
                                    "payload");
                            JsonArray values = array(step.get("payload"),
                                    "deploy payload " + cursor);
                            if (values.size() != PREPARED_PARAM_VALUES.length) {
                                throw new IllegalArgumentException("deploy payload "
                                        + cursor + " is not pinned");
                            }
                            for (int position = 0; position < values.size(); position++) {
                                if (values.get(position).asInt()
                                        != PREPARED_PARAM_VALUES[position]) {
                                    throw new IllegalArgumentException("deploy payload "
                                            + cursor + " is not pinned");
                                }
                            }
                        } else {
                            requireFields(step, "op", "case", "statement", "epl");
                        }
                        deploys++;
                        break;
                    }
                    case "undeploy-all":
                        requireFields(step, "op", "case");
                        break;
                    default:
                        throw new IllegalArgumentException(
                                "unsupported operation at step " + cursor);
                }
                cursor++;
            }
            if (deploys != CASE_DEPLOYS[caseIndex].length
                    || advances != CASE_ADVANCES[caseIndex].length) {
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
                    throw new IllegalArgumentException(
                            "duplicate JSON object key: " + member.getName());
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
                    .add("time", Instant.ofEpochMilli(
                            runtime.getEventService().getCurrentTime()).toString());
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
