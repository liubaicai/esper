import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBeanAtoFBase;
import com.espertech.esper.regressionlib.support.bean.SupportBean_A;
import com.espertech.esper.regressionlib.support.bean.SupportBean_B;
import com.espertech.esper.regressionlib.support.bean.SupportBean_C;
import com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib;
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
import java.util.Arrays;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;

/**
 * Direct Esper 9.0.0 oracle for the pattern-matchuntil-untimed-585 unit:
 * PatternOperatorMatchUntil ords 0/2/3/5 — the untimed until-array quad.
 * Unbounded (and {@code [1:]}) repeated-tag collect followed by
 * projection, correlation and count over SupportBean and the
 * SupportBean_A/B/C letter beans, replayed as four cases through the
 * direct compileDeploy -> sendEventBean -> listener harness. The
 * executions perform NO clock calls: no sendTimer/advanceTime; the
 * milestone() savepoints restore identical state for these
 * non-contextual statements and carry no steps. The internal timer is
 * disabled for determinism like the regression suite's deterministic
 * runtime.
 *
 *   - ord 0 PatternMatchUntilSimple (case simple):
 *     {@code a=SupportBean(intPrimitive=0) until b=SupportBean(intPrimitive=1)}
 *     — A1(0)/A2(0) collect silently, B1(1) fires ONE row
 *     {@code {c0:A1,c1:A2,c2:B1}}, and the completed until stays
 *     permanently false: the trailing A1(0)/B1(1) pair does NOT refire
 *     (single-shot semantics are load-bearing). The EPL pins
 *     {@code @Name} with a CAPITAL N.
 *   - ord 2 PatternSelectArray (case select-array): TWO legs over the
 *     same {@code a=SupportBean_A until b=SupportBean_B} pattern —
 *     leg 1 the explicit {@code select a, b, a[0] as a0, a[0].id as a0Id,
 *     a[1] as a1, a[1].id as a1Id, a[2] as a2, a[2].id as a2Id}
 *     projection ({@code a[2]} and {@code a[2].id} render null, not an
 *     error) and leg 2 the {@code select *} wildcard (fragments: a the
 *     collected bean array in exact order, b the terminator bean). The
 *     Java asserts assertEqualsExactOrder on the a array and
 *     assertSame/assertEquals bean identity; undeployModuleContaining
 *     separates the legs (undeploy-all equivalent). ONE fire per leg.
 *   - ord 3 PatternUseFilter (case use-filter): FIVE select-* legs of
 *     {@code a until b -> c(filter correlating against a[i])} — concat
 *     {@code id = ('C' || a[0].id || a[1].id || b.id)}, equals
 *     {@code theString = a[1].id}, in {@code theString in(a[2].id)}
 *     (NO space after {@code in}), triple {@code !=} not-in, and
 *     {@code intPrimitive between a[0].intPrimitive and a[1].intPrimitive}
 *     (inclusive, over like-'A%'/'B%' filtered SupportBeans). Each leg
 *     asserts the non-correlating probes stay silent then the
 *     correlating event fires ONCE.
 *   - ord 5 PatternArrayFunctionRepeat (case array-function-repeat):
 *     {@code [1:] a=SupportBean_A until SupportBean_B} — the terminator
 *     is UNTAGGED; A1/A2/A3 collect (milestone(0) sits between A2 and A3),
 *     B("A2") fires ONE row {@code {length:3, l2:3}} via
 *     SupportStaticMethodLib.arrayLength(a) and
 *     java.lang.reflect.Array.getLength(a).
 *
 * Listener sequence numbers are per deployment: every leg redeploys s0
 * and the trace record sequence restarts at 1, exactly like the Go
 * runner's per-deploy counter.
 */
public final class PatternMatchUntilUntimed585ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "pattern-matchuntil-untimed-585";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorMatchUntil.java";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorMatchUntil.java",
            "common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_A.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_B.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_C.java"};

    private static final String DESCRIPTION =
            "PatternOperatorMatchUntil ords 0/2/3/5 untimed until-array quad — ord 0 "
                    + "PatternMatchUntilSimple `@Name('s0') select a[0].theString as c0, "
                    + "a[1].theString as c1, b.theString as c2 from pattern "
                    + "[a=SupportBean(intPrimitive=0) until b=SupportBean(intPrimitive=1)]` "
                    + "(capital @Name; A1/A2 silent, B1 fires {A1,A2,B1}, trailing pair "
                    + "permanently silent); ord 2 PatternSelectArray two legs (explicit "
                    + "a/b/a[0..2]+id projection with null a[2], then select *) over "
                    + "a=SupportBean_A until b=SupportBean_B, ONE fire each with exact-order "
                    + "a array; ord 3 PatternUseFilter five select-* legs correlating a "
                    + "follow-on c against a[i] (concat/equals/in/not-in/between); ord 5 "
                    + "PatternArrayFunctionRepeat `[1:] a=SupportBean_A until SupportBean_B` "
                    + "firing {length:3,l2:3} via TagCount (registered mapping for "
                    + "arrayLength/Array.getLength). No clock ops; milestone() savepoints "
                    + "carry no steps.";

    private static final String[] CASES = {"simple", "select-array", "use-filter",
            "array-function-repeat"};
    private static final int[] CASE_ORDINALS = {0, 2, 3, 5};
    private static final int[] CASE_RUNTIME_INDEX = {0, 1, 2, 3};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-93ea4ae0a2ca85d18a0d",
            "java-runtime-bd06e4f21e0083fb261d",
            "java-runtime-4b8c99341f4a3af06ea9",
            "java-runtime-602d438d756709deb50e"};
    private static final String[] EXECUTIONS = {
            "PatternMatchUntilSimple",
            "PatternSelectArray",
            "PatternUseFilter",
            "PatternArrayFunctionRepeat"};
    // Static-manifest ids per execution (discovery static-candidate).
    // Ord 0 carries the file-level id java-2169accb9d43755e87e4 shared
    // with the deduplicated inventory row; ords 2/3/5 carry their
    // per-execution static ids.
    private static final String[] STATIC_IDS = {
            "java-2169accb9d43755e87e4",
            "java-c393b406de406e3ffd1f",
            "java-bf3211b9b2822d0cbbf4",
            "java-7a5adcaeaa460c4641cb"};
    private static final String[] OBSERVATIONS = {
            "listener; no clock ops (milestone 0-3 savepoints carry no steps); "
                    + "a=SupportBean(intPrimitive=0) until b=SupportBean(intPrimitive=1): "
                    + "A1(0)/A2(0) collect silently, B1(1) fires ONE row {c0:A1,c1:A2,c2:B1}, "
                    + "then the trailing A1(0)/B1(1) pair stays silent — the completed until "
                    + "is permanently false (single-shot); EPL pins capital @Name('s0')",
            "listener x2; no clock ops (milestone 0-2 carry no steps); a=SupportBean_A "
                    + "until b=SupportBean_B over A1/A2 then B1 — leg 1 explicit select "
                    + "a,b,a[0..2]+id fires {a:[A1,A2] exact order, a0/a1 beans, a2+a2Id "
                    + "null, b:B1}; undeployModuleContaining (undeploy-all), then leg 2 "
                    + "select * fires {a:[A1,A2],b:B1} fragments (Java pins assertSame bean "
                    + "identity + exact order)",
            "listener x5; no clock ops (milestone 0-7 carry no steps); five select-* legs "
                    + "of a until b -> c correlating against a[i]: concat "
                    + "(C('C'||a0||a1||b.id) silent then CA1A2B1 fires), equals (A3/20 "
                    + "silent, A2/10 fires c.intPrimitive=10), in (a[2]=A3: A2/20 silent, "
                    + "A3/5 fires), not-in (A2/20 + A1/20 silent, A6/5 fires), between over "
                    + "like-'A%'/'B%' SupportBeans (E1/20 + E2/3 silent, E3/5 fires "
                    + "inclusively); undeploy-all between legs",
            "listener; no clock ops (milestone 0 carries no step); [1:] a=SupportBean_A "
                    + "until UNTAGGED SupportBean_B: A1/A2/A3 collect, B(A2) fires ONE row "
                    + "{length:3,l2:3} — SupportStaticMethodLib.arrayLength(a) and "
                    + "java.lang.reflect.Array.getLength(a) both map to TagCount"};

    private static final String EPL_SIMPLE =
            "@Name('s0') select a[0].theString as c0, a[1].theString as c1, "
                    + "b.theString as c2 from pattern [a=SupportBean(intPrimitive=0) "
                    + "until b=SupportBean(intPrimitive=1)]";
    private static final String EPL_SELECT_ARRAY =
            "@name('s0') select a, b, a[0] as a0, a[0].id as a0Id, a[1] as a1, "
                    + "a[1].id as a1Id, a[2] as a2, a[2].id as a2Id from pattern "
                    + "[a=SupportBean_A until b=SupportBean_B]";
    private static final String EPL_SELECT_WILDCARD =
            "@name('s0') select * from pattern [a=SupportBean_A until b=SupportBean_B]";
    private static final String EPL_USE_FILTER_CONCAT =
            "@name('s0') select * from pattern [a=SupportBean_A until b=SupportBean_B -> "
                    + "c=SupportBean_C(id = ('C' || a[0].id || a[1].id || b.id))]";
    private static final String EPL_USE_FILTER_EQUALS =
            "@name('s0') select * from pattern [a=SupportBean_A until b=SupportBean_B -> "
                    + "c=SupportBean(theString = a[1].id)]";
    private static final String EPL_USE_FILTER_IN =
            "@name('s0') select * from pattern [a=SupportBean_A until b=SupportBean_B -> "
                    + "c=SupportBean(theString in(a[2].id))]";
    private static final String EPL_USE_FILTER_NOT_IN =
            "@name('s0') select * from pattern [a=SupportBean_A until b=SupportBean_B -> "
                    + "c=SupportBean(theString!=a[0].id and theString!=a[1].id and "
                    + "theString!=a[2].id)]";
    private static final String EPL_USE_FILTER_BETWEEN =
            "@name('s0') select * from pattern [a=SupportBean(theString like 'A%') until "
                    + "b=SupportBean(theString like 'B%') -> c=SupportBean(intPrimitive "
                    + "between a[0].intPrimitive and a[1].intPrimitive)]";
    private static final String EPL_ARRAY_FUNCTION_REPEAT =
            "@name('s0') select SupportStaticMethodLib.arrayLength(a) as length, "
                    + "java.lang.reflect.Array.getLength(a) as l2 from pattern "
                    + "[[1:] a=SupportBean_A until SupportBean_B]";

    // Per-case leg EPLs in deploy order; the case metadata "epl" pins the
    // first leg as the representative.
    private static final String[][] CASE_EPLS = {
            {EPL_SIMPLE},
            {EPL_SELECT_ARRAY, EPL_SELECT_WILDCARD},
            {EPL_USE_FILTER_CONCAT, EPL_USE_FILTER_EQUALS, EPL_USE_FILTER_IN,
                    EPL_USE_FILTER_NOT_IN, EPL_USE_FILTER_BETWEEN},
            {EPL_ARRAY_FUNCTION_REPEAT}
    };

    // Pinned op sequence (after each case marker), in Java source order:
    // per leg one s0 deploy, the pinned sends and undeploy-all. No clock
    // ops — the executions perform no sendTimer/advanceTime calls and the
    // milestone() savepoints carry no steps.
    private static final String[][] CASE_OPS = {
            {"deploy", "send", "send", "send", "send", "send", "undeploy-all"},
            {"deploy", "send", "send", "send", "undeploy-all",
                    "deploy", "send", "send", "send", "undeploy-all"},
            {"deploy", "send", "send", "send", "send", "send", "undeploy-all",
                    "deploy", "send", "send", "send", "send", "send", "undeploy-all",
                    "deploy", "send", "send", "send", "send", "send", "send", "undeploy-all",
                    "deploy", "send", "send", "send", "send", "send", "send", "send",
                    "undeploy-all",
                    "deploy", "send", "send", "send", "send", "send", "send", "undeploy-all"},
            {"deploy", "send", "send", "send", "send", "undeploy-all"}
    };

    // Pinned send payloads, in send order per case. Letter beans render
    // "eventType|id"; SupportBean renders "SupportBean|theString|int".
    private static final String[][] CASE_SENDS = {
            {"SupportBean|A1|0", "SupportBean|A2|0", "SupportBean|B1|1",
                    "SupportBean|A1|0", "SupportBean|B1|1"},
            {"SupportBean_A|A1", "SupportBean_A|A2", "SupportBean_B|B1",
                    "SupportBean_A|A1", "SupportBean_A|A2", "SupportBean_B|B1"},
            {"SupportBean_A|A1", "SupportBean_A|A2", "SupportBean_B|B1",
                    "SupportBean_C|C1", "SupportBean_C|CA1A2B1",
                    "SupportBean_A|A1", "SupportBean_A|A2", "SupportBean_B|B1",
                    "SupportBean|A3|20", "SupportBean|A2|10",
                    "SupportBean_A|A1", "SupportBean_A|A2", "SupportBean_A|A3",
                    "SupportBean_B|B1", "SupportBean|A2|20", "SupportBean|A3|5",
                    "SupportBean_A|A1", "SupportBean_A|A2", "SupportBean_A|A3",
                    "SupportBean_B|B1", "SupportBean|A2|20", "SupportBean|A1|20",
                    "SupportBean|A6|5",
                    "SupportBean|A1|5", "SupportBean|A2|8", "SupportBean|B1|-1",
                    "SupportBean|E1|20", "SupportBean|E2|3", "SupportBean|E3|5"},
            {"SupportBean_A|A1", "SupportBean_A|A2", "SupportBean_A|A3",
                    "SupportBean_B|A2"}
    };

    private static final int EXPECTED_STEPS = 66;
    private static final int EXPECTED_RECORDS = 9;

    private PatternMatchUntilUntimed585ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: PatternMatchUntilUntimed585ScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBean_B.class);
        configuration.getCommon().addEventType(SupportBean_C.class);
        // The regression environment's default imports resolve
        // SupportStaticMethodLib for the ord 5 EPL; pin the same import.
        configuration.getCommon().addImport(SupportStaticMethodLib.class.getName());

        String runtimeURI = SCENARIO_ID + "-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = null;
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
                        // The compiler arguments carry the runtime
                        // configuration so the SupportStaticMethodLib
                        // import resolves for the ord 5 EPL.
                        CompilerArguments compilerArgs = new CompilerArguments(configuration);
                        compilerArgs.getPath().add(runtime.getRuntimePath());
                        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                                compilerArgs);
                        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                                new DeploymentOptions()
                                        .setDeploymentId(SCENARIO_ID + "-" + caseIndex));
                        writer = new TraceWriter(records, caseName, findStatement(deployment), runtime);
                        writer.statement.addListener(writer);
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        writer = null;
                        break;
                    case "send":
                        sendEvent(runtime, step);
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
        switch (eventType) {
            case "SupportBean":
                runtime.getEventService().sendEventBean(
                        new SupportBean(payload.getString("theString", null),
                                integer(payload, "intPrimitive")),
                        "SupportBean");
                return;
            case "SupportBean_A":
                runtime.getEventService().sendEventBean(
                        new SupportBean_A(payload.getString("id", null)), "SupportBean_A");
                return;
            case "SupportBean_B":
                runtime.getEventService().sendEventBean(
                        new SupportBean_B(payload.getString("id", null)), "SupportBean_B");
                return;
            case "SupportBean_C":
                runtime.getEventService().sendEventBean(
                        new SupportBean_C(payload.getString("id", null)), "SupportBean_C");
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
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != CASE_ORDINALS[index]
                    || !RUNTIME_IDS[CASE_RUNTIME_INDEX[index]].equals(
                            string(definition, "runtimeId"))
                    || !EXECUTIONS[CASE_RUNTIME_INDEX[index]].equals(
                            string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index][0].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: each case marker is followed by the
     * pinned ops — per leg one s0 deploy carrying the verbatim leg EPL,
     * the pinned sends and undeploy-all. Unknown step fields are
     * rejected. There are no advance-time or milestone steps because the
     * Java executions perform no clock calls.
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
            for (String operation : CASE_OPS[caseIndex]) {
                JsonObject step = object(steps.get(cursor), "step " + cursor);
                if (!operation.equals(string(step, "op"))
                        || !CASES[caseIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("step " + cursor + " is not pinned");
                }
                switch (operation) {
                    case "deploy":
                        requireFields(step, "op", "case", "statement", "epl");
                        if (!"s0".equals(string(step, "statement"))
                                || !CASE_EPLS[caseIndex][deploys].equals(string(step, "epl"))) {
                            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
                        }
                        deploys++;
                        break;
                    case "send": {
                        requireFields(step, "op", "case", "eventType", "payload");
                        String expected = CASE_SENDS[caseIndex][sends++];
                        JsonObject payload = object(step.get("payload"), "send payload " + cursor);
                        String eventType = string(step, "eventType");
                        String actual;
                        if ("SupportBean".equals(eventType)) {
                            requireFields(payload, "theString", "intPrimitive");
                            actual = "SupportBean|" + string(payload, "theString") + "|"
                                    + longInteger(payload, "intPrimitive");
                        } else if ("SupportBean_A".equals(eventType)
                                || "SupportBean_B".equals(eventType)
                                || "SupportBean_C".equals(eventType)) {
                            requireFields(payload, "id");
                            actual = eventType + "|" + string(payload, "id");
                        } else {
                            throw new IllegalArgumentException("send step " + cursor
                                    + " carries unknown eventType " + eventType);
                        }
                        if (!expected.equals(actual)) {
                            throw new IllegalArgumentException("send payload " + cursor
                                    + " is not pinned: expected " + expected + " got " + actual);
                        }
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
            if (sends != CASE_SENDS[caseIndex].length || deploys != CASE_EPLS[caseIndex].length) {
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
        long parsed = longInteger(object, name);
        if (parsed < Integer.MIN_VALUE || parsed > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " is outside the Java int range");
        }
        return (int) parsed;
    }

    private static long longInteger(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (!(value instanceof com.espertech.esper.common.client.json.minimaljson.JsonNumber)) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        String text = value.toString();
        if (!text.matches("-?(0|[1-9][0-9]*)")) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        try {
            return Long.parseLong(text, 10);
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
            // Render the asserted-field projection {theString,intPrimitive}
            // so the trace carries no unasserted engine defaults (the
            // EplInsertIntoFromPattern precedent).
            SupportBean bean = (SupportBean) value;
            JsonObject fields = new JsonObject();
            fields.add("intPrimitive", normalize(bean.getIntPrimitive()));
            fields.add("theString", normalize(bean.getTheString()));
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof SupportBeanAtoFBase) {
            // SupportBean_A/B/C share the id-only SupportBeanAtoFBase:
            // tagged-event columns arrive as the underlying bean and
            // render the single {id} property like the Go NormalizeResults
            // event rendering.
            SupportBeanAtoFBase bean = (SupportBeanAtoFBase) value;
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(bean.getId()));
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
}
