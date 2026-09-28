import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
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
 * Direct Esper 9.0.0 oracle for the pattern-matchuntil-static-586 unit:
 * PatternOperatorMatchUntil ords 7/8 — the static remainder. Ord 7
 * replays the every-bounded and/not repeat through the direct
 * compileDeploy -> sendEventBean -> listener harness; ord 8 replays the
 * four Go-expressible tryInvalidCompile probes as build-error steps that
 * compile without the runtime path and emit compile-error records
 * carrying the pinned Java message prefixes. The executions perform NO
 * clock calls: no sendTimer/advanceTime; the milestone() savepoints
 * restore identical state for these non-contextual statements and carry
 * no steps. The internal timer is disabled for determinism like the
 * regression suite's deterministic runtime.
 *
 *   - ord 7 PatternBoundRepeatWithNot (case bound-repeat-with-not):
 *     {@code every [2] (e = SupportBean(theString='A') and not
 *     SupportBean(theString='B'))} — every pending repeat atom carries
 *     its own not-guard: A(1)/A(2) complete the first pair and deliver
 *     {@code {e:[(A,1),(A,2)]}}; A(3) arms a fresh atom; B(4) cancels
 *     ONLY that pending partial (the completed first fire already
 *     delivered); A(5) arms a new pair which A(6) completes as
 *     {@code {e:[(A,5),(A,6)]}}. TWO listener fires total.
 *   - ord 8 PatternInvalid (case invalid): thirteen tryInvalidPattern
 *     legs wrap each fragment as {@code select * from pattern[<frag>]}
 *     and assert a message prefix. Four legs are Go-expressible and
 *     modeled as build-error steps — the inverted [10:4] and negative
 *     [-1] bounds, the duplicate tag 'c' across the until branch and the
 *     follow-on atom, and the tag 'a' reused across a nested until. The
 *     remaining nine legs are documented Java-only exclusions (zero-
 *     valued [:0]/[0:0]/[0] literals are the legal Go [:M]/[N:] forms,
 *     [4:6]-without-until is an EPL-text-only check, a[0].id own/
 *     follow-on filter references and the non-numeric bound expressions
 *     are compile-text surfaces) and carry NO steps and NO records.
 *
 * Listener sequence numbers are per deployment, like the Go runner's
 * per-deploy counter. Compile-error records carry sequence 0, no time
 * field and the pinned expectError value — identical to the
 * context-key-segmented-invalid oracle convention.
 */
public final class PatternMatchUntilStatic586ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "pattern-matchuntil-static-586";
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
            "PatternOperatorMatchUntil ords 7/8 static remainder — ord 7 "
                    + "PatternBoundRepeatWithNot `@name('s0') select * from pattern "
                    + "[every [2] (e = SupportBean(theString='A') and not "
                    + "SupportBean(theString='B'))]` (A(1)/A(2) fire e={1,2}, A(3) arms "
                    + "a fresh atom, B(4) cancels only that pending partial, A(5)/A(6) "
                    + "fire e={5,6} — each repeat atom carries its own not-guard and "
                    + "the completed first delivery survives); ord 8 PatternInvalid "
                    + "pins the four Go-expressible tryInvalidCompile legs (inverted "
                    + "[10:4], negative [-1], duplicate tag across until/follow-on, "
                    + "nested-until tag reuse) as compile-error records carrying the "
                    + "Java message prefixes — the other nine legs are documented "
                    + "Java-only exclusions (zero-valued literals are the legal Go "
                    + "[:M]/[N:] forms, [4:6]-without-until has no EPL-text surface, "
                    + "a[0] own/follow-on filter refs and non-numeric bound "
                    + "expressions are compile-text concerns) and carry no steps. "
                    + "No clock ops; milestone() savepoints carry no steps.";

    private static final String[] CASES = {"bound-repeat-with-not", "invalid"};
    private static final int[] CASE_ORDINALS = {7, 8};
    private static final int[] CASE_RUNTIME_INDEX = {0, 1};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-f10e941aae4f3b003593",
            "java-runtime-50cec2b441fd878bf1f1"};
    private static final String[] EXECUTIONS = {
            "PatternBoundRepeatWithNot",
            "PatternInvalid"};
    // Static-manifest ids per execution (discovery static-candidate).
    private static final String[] STATIC_IDS = {
            "java-ada27e5f94fecc9ffce2",
            "java-256196856cfafb3c17b7"};
    private static final String[] OBSERVATIONS = {
            "listener x2; no clock ops (milestone 0-3 savepoints carry no steps); "
                    + "every [2] (e=SupportBean(theString='A') and not "
                    + "SupportBean(theString='B')): A(1)/A(2) complete the first pair "
                    + "and fire {e:[(A,1),(A,2)]}; A(3) arms a fresh repeat atom; B(4) "
                    + "cancels ONLY that pending partial — the completed first "
                    + "delivery already left the engine; A(5) arms a new pair which "
                    + "A(6) completes, firing {e:[(A,5),(A,6)]}; each pending repeat "
                    + "atom carries its own not-guard",
            "compile-error; four tryInvalidCompile probes pin the Go-expressible legs — "
                    + "inverted [10:4] (lower bound higher than upper), negative [-1] "
                    + "bound, duplicate tag 'c' redeclared across until and follow-on, "
                    + "tag 'a' reused inside a nested until — recording the pinned "
                    + "Java message prefixes; the remaining nine legs are documented "
                    + "Java-only exclusions (zero-valued [:0]/[0:0]/[0] literals are "
                    + "legal Go [:M]/[N:] forms, [4:6]-without-until is EPL-text-only, "
                    + "a[0].id own/follow-on filter refs and the non-numeric bound "
                    + "expressions are compile-text surfaces) with no steps and no "
                    + "records"};

    // Byte-exact EPL pins (PatternOperatorMatchUntil.java verbatim).
    private static final String EPL_BOUND_REPEAT =
            "@name('s0') select * from pattern [every [2] (e = "
                    + "SupportBean(theString='A') and not SupportBean(theString='B'))]";
    private static final String PROBE_INVERTED_EPL =
            "select * from pattern[[10:4] SupportBean_A]";
    private static final String PROBE_NEGATIVE_EPL =
            "select * from pattern[[-1] SupportBean_A]";
    private static final String PROBE_ACROSS_UNTIL_EPL =
            "select * from pattern[(a=SupportBean_A until c=SupportBean_B) -> "
                    + "c=SupportBean_C]";
    private static final String PROBE_NESTED_UNTIL_EPL =
            "select * from pattern[((a=SupportBean_A until b=SupportBean_B) until "
                    + "a=SupportBean_A)]";

    // Pinned Java message prefixes (assertMessage is a startsWith check).
    private static final String ERR_INVERTED =
            "Incorrect range specification, lower bounds value '10' is higher then "
                    + "higher bounds '4'";
    private static final String ERR_NEGATIVE =
            "Incorrect range specification, a bounds value of zero or negative value "
                    + "is not allowed";
    private static final String ERR_ACROSS_UNTIL =
            "Tag 'c' for event 'SupportBean_C' has already been declared for events of "
                    + "type " + SupportBean_B.class.getName();
    private static final String ERR_NESTED_UNTIL =
            "Tag 'a' for event 'SupportBean_A' used in the repeat-until operator cannot "
                    + "also appear in other filter expressions";

    // Per-case representative EPLs (the case metadata "epl" pins the
    // first script as the representative — ord 8's first modeled probe).
    private static final String[] CASE_EPLS = {
            EPL_BOUND_REPEAT,
            PROBE_INVERTED_EPL
    };

    // Pinned op sequence (after each case marker), in Java source order.
    // No clock ops — the executions perform no sendTimer/advanceTime
    // calls and the milestone() savepoints carry no steps.
    private static final String[][] CASE_OPS = {
            {"deploy", "send", "send", "send", "send", "send", "send", "undeploy-all"},
            {"build-error", "build-error", "build-error", "build-error"}
    };

    // Pinned send payloads, in send order per case (SupportBean renders
    // "SupportBean|theString|int").
    private static final String[][] CASE_SENDS = {
            {"SupportBean|A|1", "SupportBean|A|2", "SupportBean|A|3",
                    "SupportBean|B|4", "SupportBean|A|5", "SupportBean|A|6"},
            {}
    };

    // Pinned build-error probes per case: label, wrapped EPL, expected
    // Java message prefix (in Java source order of the modeled legs).
    private static final String[][][] CASE_PROBES = {
            {},
            {
                    {"inverted-bounds", PROBE_INVERTED_EPL, ERR_INVERTED},
                    {"negative-bounds", PROBE_NEGATIVE_EPL, ERR_NEGATIVE},
                    {"tag-redeclared-across-until", PROBE_ACROSS_UNTIL_EPL, ERR_ACROSS_UNTIL},
                    {"tag-reused-inside-nested-until", PROBE_NESTED_UNTIL_EPL, ERR_NESTED_UNTIL}
            }
    };

    private static final int EXPECTED_STEPS = 14;
    private static final int EXPECTED_RECORDS = 6;

    private PatternMatchUntilStatic586ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: PatternMatchUntilStatic586ScenarioOracle <scenario.json>");
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
        if ("SupportBean".equals(eventType)) {
            runtime.getEventService().sendEventBean(
                    new SupportBean(payload.getString("theString", null),
                            integer(payload, "intPrimitive")),
                    "SupportBean");
            return;
        }
        throw new IllegalStateException("unknown eventType: " + eventType);
    }

    /**
     * Compiles an expected-invalid probe WITHOUT the runtime path,
     * mirroring env.tryInvalidCompile(epl, message)'s path-less
     * compileWCheckedEx, and emits {"operation":"compile-error"}
     * carrying the pinned expectError prefix after verifying the caught
     * message starts with it (SupportMessageAssertUtil.assertMessage
     * semantics).
     */
    private static void buildErrorStep(Configuration configuration, String caseName,
                                       JsonObject step, JsonArray records) {
        String label = step.getString("statement", "");
        String expected = step.getString("expectError", "");
        String epl = step.getString("epl", "");
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
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
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "compile-error");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", expected);
        records.add(record);
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
        for (int index = 0; index < cases.size(); index++) {
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
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: each case marker is followed by the
     * pinned ops — ord 7's s0 deploy carrying the verbatim EPL, the six
     * pinned sends and undeploy-all; ord 8's four build-error probes
     * carrying the wrapped EPL and the asserted Java message prefix.
     * Unknown step fields are rejected. There are no advance-time or
     * milestone steps because the Java executions perform no clock calls.
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
            int probes = 0;
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
                                || !EPL_BOUND_REPEAT.equals(string(step, "epl"))) {
                            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
                        }
                        deploys++;
                        break;
                    case "send": {
                        requireFields(step, "op", "case", "eventType", "payload");
                        String expected = CASE_SENDS[caseIndex][sends++];
                        JsonObject payload = object(step.get("payload"), "send payload " + cursor);
                        String eventType = string(step, "eventType");
                        if (!"SupportBean".equals(eventType)) {
                            throw new IllegalArgumentException("send step " + cursor
                                    + " carries unknown eventType " + eventType);
                        }
                        requireFields(payload, "theString", "intPrimitive");
                        String actual = "SupportBean|" + string(payload, "theString") + "|"
                                + longInteger(payload, "intPrimitive");
                        if (!expected.equals(actual)) {
                            throw new IllegalArgumentException("send payload " + cursor
                                    + " is not pinned: expected " + expected + " got " + actual);
                        }
                        break;
                    }
                    case "build-error": {
                        requireFields(step, "op", "case", "statement", "epl", "expectError");
                        String[] probe = CASE_PROBES[caseIndex][probes++];
                        if (!probe[0].equals(string(step, "statement"))
                                || !probe[1].equals(string(step, "epl"))
                                || !probe[2].equals(string(step, "expectError"))) {
                            throw new IllegalArgumentException("build-error step " + cursor
                                    + " is not pinned");
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
            if (sends != CASE_SENDS[caseIndex].length
                    || probes != CASE_PROBES[caseIndex].length) {
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
        return longInteger(value, name);
    }

    private static long longInteger(JsonValue value, String label) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(label + " must be an integer JSON number");
        }
        String text = value.toString();
        if (!text.matches("-?(0|[1-9][0-9]*)")) {
            throw new IllegalArgumentException(label + " must be an integer JSON number");
        }
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(label + " is outside the Java long range", ex);
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
