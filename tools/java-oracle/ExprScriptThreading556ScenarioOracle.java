import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventType;
import com.espertech.esper.common.client.PropertyAccessException;
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
import com.espertech.esper.common.internal.support.SupportEnum;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.lang.reflect.Array;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.math.BigDecimal;
import java.math.BigInteger;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the expr-script-threading-556 parity
 * scenario. Mirrors two executions:
 *
 * <p>case script-probes bundles EPLScriptExpression ord 1 EPLScriptQuoteEscape
 * (java-runtime-950fed16f3830d3a9926) and ord 4
 * EPLScriptInvalidRegardlessDialect (java-runtime-416f111d2368882ea644) on one
 * scenario case because both are compile-only probes with no listener
 * surface; the pinned per-step runtimeId keeps each probe on the runtime the
 * Java execution would own. Ord 1 runs the two compileDeploy quote-escape
 * probes (single-line and multi-line comment bodies containing I'am) and
 * records "compile-ok" markers. Ord 4 runs the eight tryInvalidCompile
 * probes in source order and records the classification the Go runner
 * mirrors: "compile-error" where the typed Go surface rejects
 * (unknown-script, arity-mismatch, same-arity-overload, param-name-overlap),
 * "unrepresentable" where the Java surface has no Go counterpart
 * (param-defined-twice, unresolvable-return-type), and
 * "intentionally-different" where Go accepts the equivalent registration
 * (invalid-dialect, script-expression-overlap).
 *
 * <p>case large-threading mirrors ExprFilterLargeThreading ord 0
 * (java-runtime-17b5c153a73414b75e24): deploy
 * "@name('s0') select * from pattern[a=SupportBean -&gt; every
 * event1=SupportTradeEvent(userId like '123%')]", arm with SupportBean(),
 * send TradeEvent(1, null, 1001) with no fire, then TradeEvent(2, '1234',
 * 1001) which fires event1.id=2.
 *
 * <p>Records: script probes carry {case,operation,statement,sequence,value}
 * (no time field, matching the epl-other-invalid convention); the threading
 * case carries deployed/types/listener records. Time is the
 * externally-advanced engine clock so the Go replay matches byte-for-byte.
 */
public final class ExprScriptThreading556ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "expr-script-threading-556";
    private static final String DESCRIPTION =
            "EPLScriptExpression compile probes + ExprFilterLargeThreading (pinned "
                    + "9e1b9f1cc9117fea4bf33ab043762c045d73839c): script-probes bundles "
                    + "ord 1 EPLScriptQuoteEscape (two compileDeploy quote-escape "
                    + "probes over single-line and multi-line I'am comments) and ord 4 "
                    + "EPLScriptInvalidRegardlessDialect (eight tryInvalidCompile "
                    + "probes — parameter defined twice, invalid dialect, unknown "
                    + "script, arity mismatch, same-arity overload, parameter-name "
                    + "overlap, script/expression overlap and unresolvable return "
                    + "type); large-threading (ord 0) deploys pattern[a=SupportBean "
                    + "-> every event1=SupportTradeEvent(userId like '123%')] and one "
                    + "SupportBean arm + TradeEvent(1,null,1001) no-fire + "
                    + "TradeEvent(2,'1234',1001) send emits event1.id=2";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/script/"
                    + "EPLScriptExpression.java";
    private static final String JAVA_SOURCE_SCRIPT = JAVA_SOURCE;
    private static final String JAVA_SOURCE_THREADING =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/filter/"
                    + "ExprFilterLargeThreading.java";

    private static final String CASE_PROBES = "script-probes";
    private static final String CASE_THREADING = "large-threading";
    private static final String[] CASES = {CASE_PROBES, CASE_THREADING};
    private static final int[][] ORDINALS = {{1, 4}, {0}};
    private static final String[][] RUNTIME_IDS = {
            {"java-runtime-950fed16f3830d3a9926", "java-runtime-416f111d2368882ea644"},
            {"java-runtime-17b5c153a73414b75e24"}};
    private static final String[][] EXECUTION_NAMES = {
            {"EPLScriptQuoteEscape", "EPLScriptInvalidRegardlessDialect"},
            {"ExprFilterLargeThreading"}};
    private static final String[][] STATIC_IDS = {
            {"java-25975b631283eca66de6", "java-25975b631283eca66de6"},
            {"java-a977dd316de9d77c011c"}};
    private static final String[] CASE_SOURCES = {JAVA_SOURCE_SCRIPT, JAVA_SOURCE_THREADING};

    // Byte-exact transcription of EPLScriptExpression.java lines 139-150
    // (ord 1 EPLScriptQuoteEscape): both modules compile and deploy; the
    // apostrophes inside the comments exercise the lexer quote handling.
    private static final String EPL_SL_COMMENT = "create expression f(params)[\n"
            + "  // I'am...\n"
            + "];";
    private static final String EPL_ML_COMMENT = "create expression g(params)[\n"
            + "  /* I'am... */"
            + "];";

    // Byte-exact transcription of EPLScriptExpression.java lines 229-261
    // (ord 4 EPLScriptInvalidRegardlessDialect) in source order.
    private static final String[][] INVALID_PROBES = {
            {"param-defined-twice",
                    "expression js:abc(p1, p1) [/* text */] select * from SupportBean",
                    "Invalid script parameters for script 'abc', parameter 'p1' is defined"
                            + " more then once [expression js:abc(p1, p1) [/* text */]"
                            + " select * from SupportBean]",
                    "unrepresentable"},
            {"invalid-dialect",
                    "expression dummy:abc() [10] select * from SupportBean",
                    "Failed to obtain script runtime for dialect 'dummy' for script 'abc'"
                            + " [expression dummy:abc() [10] select * from SupportBean]",
                    "intentionally-different"},
            {"unknown-script",
                    "select abc() from SupportBean",
                    "Failed to validate select-clause expression 'abc()': Unknown"
                            + " single-row function, expression declaration, script or"
                            + " aggregation function named 'abc' could not be resolved"
                            + " [select abc() from SupportBean]",
                    "compile-error"},
            {"arity-mismatch",
                    "expression js:abc() [10] select abc(1) from SupportBean",
                    "Failed to validate select-clause expression 'abc(1)': Invalid number"
                            + " of parameters for script 'abc', expected 0 parameters but"
                            + " received 1 parameters [expression js:abc() [10] select"
                            + " abc(1) from SupportBean]",
                    "compile-error"},
            {"same-arity-overload",
                    "expression js:abc() [10] expression js:abc() [10] select abc() from"
                            + " SupportBean",
                    "Script name 'abc' has already been defined with the same number of"
                            + " parameters [expression js:abc() [10] expression js:abc()"
                            + " [10] select abc() from SupportBean]",
                    "compile-error"},
            {"param-name-overlap",
                    "expression js:abc(p1) [10] expression js:abc(p2) [10] select abc()"
                            + " from SupportBean",
                    "Script name 'abc' has already been defined with the same number of"
                            + " parameters [expression js:abc(p1) [10] expression"
                            + " js:abc(p2) [10] select abc() from SupportBean]",
                    "compile-error"},
            {"script-expression-overlap",
                    "expression js:abc() [10] expression abc {10} select abc() from"
                            + " SupportBean",
                    "Script name 'abc' overlaps with another expression of the same name"
                            + " [expression js:abc() [10] expression abc {10} select"
                            + " abc() from SupportBean]",
                    "intentionally-different"},
            {"unresolvable-return-type",
                    "expression dummy js:abc() [10] select abc() from SupportBean",
                    "Failed to validate select-clause expression 'abc()': Failed to"
                            + " resolve return type 'dummy' specified for script 'abc'"
                            + " [expression dummy js:abc() [10] select abc() from"
                            + " SupportBean]",
                    "unrepresentable"},
    };

    // The threading EPL, byte-exact from ExprFilterLargeThreading.java line 28.
    private static final String EPL_THREADING =
            "@name('s0') select * from pattern[a=SupportBean -> every"
                    + " event1=SupportTradeEvent(userId like '123%')]";

    // The compile-ok notes the quote-escape records pin; identical wording on
    // both sides of the diff.
    private static final String NOTE_SL_COMMENT =
            "compile-ok: Java compileDeploy accepts the script body whose single-line"
                    + " comment text contains an apostrophe (// I'am...); the Go script"
                    + " surface registers provider callbacks rather than EPL bodies, so"
                    + " the lexing detail has no Go boundary";
    private static final String NOTE_ML_COMMENT =
            "compile-ok: Java compileDeploy accepts the script body whose multi-line"
                    + " comment text contains an apostrophe (/* I'am... */); the Go"
                    + " script surface registers provider callbacks rather than EPL"
                    + " bodies, so the lexing detail has no Go boundary";

    private static final String[] CASE_OBSERVATIONS = {
            "compile-ok+compile-error+unrepresentable+intentionally-different; the two"
                    + " quote-escape compileDeploy probes (ord 1) and the eight"
                    + " tryInvalidCompile probes (ord 4) pin the asserted Java messages"
                    + " — Go rejects the unregistered/arity/duplicate-name probes at"
                    + " Build, the named-parameter and named-return-type probes have no"
                    + " Go surface, and Go accepts the descriptive 'dummy' dialect plus"
                    + " the separate script/expression name spaces",
            "deployed+types+listener; pattern[a=SupportBean -> every"
                    + " event1=SupportTradeEvent(userId like '123%')] arms on"
                    + " SupportBean(), ignores TradeEvent(1,null,1001) (null userId"
                    + " like-fails) and fires once on TradeEvent(2,'1234',1001) with"
                    + " event1.id=2"};

    private static final String[] CASE_EPLS = {
            EPL_SL_COMMENT + "\n" + EPL_ML_COMMENT + "\n"
                    + "expression js:abc(p1, p1) [/* text */] select * from SupportBean\n"
                    + "expression dummy:abc() [10] select * from SupportBean\n"
                    + "select abc() from SupportBean\n"
                    + "expression js:abc() [10] select abc(1) from SupportBean\n"
                    + "expression js:abc() [10] expression js:abc() [10] select abc() from SupportBean\n"
                    + "expression js:abc(p1) [10] expression js:abc(p2) [10] select abc() from SupportBean\n"
                    + "expression js:abc() [10] expression abc {10} select abc() from SupportBean\n"
                    + "expression dummy js:abc() [10] select abc() from SupportBean",
            EPL_THREADING};

    private static final int EXPECTED_RECORDS = 13;
    private static final int EXPECTED_STEPS = 20;

    /** Deployed statements keyed by case/label for the types step. */
    private static final Map<String, EPStatement[]> statements = new HashMap<>();

    private ExprScriptThreading556ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ExprScriptThreading556ScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        Configuration configuration = new Configuration();
        registerEventTypes(configuration);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);

        JsonArray records = new JsonArray();
        Map<String, EPRuntime> runtimes = new LinkedHashMap<>();
        try {
            for (JsonValue stepValue : allSteps) {
                JsonObject step = stepValue.asObject();
                String operation = string(step, "op");
                String caseName = string(step, "case");
                if ("case".equals(operation)) {
                    continue;
                }
                EPRuntime runtime = runtimeFor(runtimes, string(step, "runtimeId"),
                        configuration);
                switch (operation) {
                    case "deploy":
                        deployStep(configuration, runtime, caseName, step, records);
                        break;
                    case "deployed":
                        deployedStep(runtime, caseName, step, records);
                        break;
                    case "types":
                        typesStep(runtime, caseName, step, records);
                        break;
                    case "send":
                        sendStep(runtime, caseName, step, records);
                        break;
                    case "build-error":
                    case "unrepresentable":
                        invalidProbeStep(configuration, caseName, step, records);
                        break;
                    case "undeploy-all":
                        for (EPRuntime each : runtimes.values()) {
                            each.getDeploymentService().undeployAll();
                        }
                        break;
                    default:
                        throw new IllegalArgumentException("unknown op: " + operation);
                }
            }
        } finally {
            for (EPRuntime runtime : runtimes.values()) {
                try {
                    runtime.getDeploymentService().undeployAll();
                } finally {
                    runtime.destroy();
                }
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

    /** Lazily creates the runtime for the step's pinned runtimeId, mirroring
     *  the per-execution runtime the regression harness assigns. */
    private static EPRuntime runtimeFor(Map<String, EPRuntime> runtimes, String runtimeId,
                                        Configuration configuration) {
        EPRuntime runtime = runtimes.get(runtimeId);
        if (runtime == null) {
            runtime = EPRuntimeProvider.getRuntime(runtimeId, configuration);
            runtime.getEventService().advanceTime(0);
            runtimes.put(runtimeId, runtime);
        }
        return runtime;
    }

    /**
     * Deploy steps carry two shapes: in the script-probes case a deploy step
     * is one ord-1 compileDeploy probe whose record is the pinned compile-ok
     * note (no listener exists); in the threading case the deploy step builds
     * s0 and attaches the trace listener like
     * env.compileDeploy(epl).addListener("s0").
     */
    private static void deployStep(Configuration configuration, EPRuntime runtime,
                                   String caseName, JsonObject step, JsonArray records)
            throws Exception {
        String label = string(step, "statement");
        String epl = string(step, "epl");
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                new DeploymentOptions());
        if (CASE_PROBES.equals(caseName)) {
            String note;
            if ("sl-comment".equals(label)) {
                note = NOTE_SL_COMMENT;
            } else if ("ml-comment".equals(label)) {
                note = NOTE_ML_COMMENT;
            } else {
                throw new IllegalStateException("script-probes deploy " + label
                        + " is not pinned");
            }
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "compile-ok");
            record.add("statement", label);
            record.add("sequence", 0);
            record.add("value", note);
            records.add(record);
            return;
        }
        if (!CASE_THREADING.equals(caseName) || !"s0".equals(label)
                || !EPL_THREADING.equals(epl)) {
            throw new IllegalStateException("deploy step " + label
                    + " is not pinned for case " + caseName);
        }
        for (EPStatement statement : deployment.getStatements()) {
            if ("s0".equals(statement.getName())) {
                statement.addListener(listener(caseName, records, runtime));
            }
        }
        statements.put(caseName + "/" + label, deployment.getStatements());
    }

    /** The deployed marker for the threading s0 statement, mirroring the
     *  epl-other-invalid per-statement marker convention. */
    private static void deployedStep(EPRuntime runtime, String caseName, JsonObject step,
                                     JsonArray records) {
        String label = string(step, "statement");
        if (!CASE_THREADING.equals(caseName) || !"s0".equals(label)) {
            throw new IllegalStateException("deployed marker " + label
                    + " is not pinned for case " + caseName);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "deployed");
        record.add("statement", label);
        record.add("sequence", 1);
        record.add("time", Instant.ofEpochMilli(
                runtime.getEventService().getCurrentTime()).toString());
        records.add(record);
    }

    /**
     * Pins the asserted select-clause surface of the threading s0 statement:
     * both pattern tags surface as Map-typed properties under select *.
     */
    private static void typesStep(EPRuntime runtime, String caseName, JsonObject step,
                                  JsonArray records) {
        String label = string(step, "statement");
        if (!CASE_THREADING.equals(caseName) || !"s0".equals(label)) {
            throw new IllegalStateException("types step " + label
                    + " is not pinned for case " + caseName);
        }
        EPStatement[] statements = ExprScriptThreading556ScenarioOracle.statements
                .get(caseName + "/" + label);
        if (statements == null || statements.length == 0) {
            throw new IllegalStateException("types statement " + label
                    + " was not deployed in case " + caseName);
        }
        EventType eventType = statements[0].getEventType();
        String[] pinnedNames = {"a", "event1"};
        String[] pinnedTypes = {"Map", "Map"};
        JsonObject pinned = new JsonObject();
        for (int index = 0; index < pinnedNames.length; index++) {
            Class<?> propertyType = eventType.getPropertyType(pinnedNames[index]);
            String actual = propertyType == null ? "null" : propertyType.getSimpleName();
            if (!pinnedTypes[index].equals(actual)) {
                throw new IllegalStateException("property type drift for " + caseName
                        + "/s0." + pinnedNames[index] + ": expected " + pinnedTypes[index]
                        + " got " + actual);
            }
            pinned.add(pinnedNames[index], pinnedTypes[index]);
        }
        JsonObject value = new JsonObject();
        value.add("properties", pinned);
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "types");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(
                runtime.getEventService().getCurrentTime()).toString());
        record.add("value", value);
        records.add(record);
    }

    /**
     * Sends one pinned event via sendEventMap (the in-and-between
     * precedent: map events surface pattern tags as MapEventBeans, so the
     * listener rows carry the same plain property maps the Go tag-map
     * projection emits; absent keys render the bean-style defaults). The
     * payload's expectedFire flag mirrors the execution's
     * assertListenerNotInvoked/assertEqualsNew assertions: a send whose
     * fire flag contradicts the pin is an oracle failure, not a recorded
     * difference.
     */
    private static void sendStep(EPRuntime runtime, String caseName, JsonObject step,
                                 JsonArray records) {
        if (!CASE_THREADING.equals(caseName)) {
            throw new IllegalStateException("send step is not pinned for case " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "payload");
        boolean expectedFire = payload.get("expectedFire").asBoolean();
        String eventType = string(step, "eventType");
        int before = records.size();
        Map<String, Object> event;
        switch (eventType) {
            case "SupportBean":
                // sendEventMap mirrors the expr-filter precedents: full
                // bean-default maps surface pattern tags as plain property
                // maps, the same shape the Go tag-map projection emits.
                event = new LinkedHashMap<>();
                event.put("theString", null);
                event.put("boolPrimitive", false);
                event.put("intPrimitive", 0);
                event.put("longPrimitive", 0L);
                event.put("charPrimitive", '\0');
                event.put("shortPrimitive", (short) 0);
                event.put("bytePrimitive", (byte) 0);
                event.put("floatPrimitive", 0f);
                event.put("doublePrimitive", 0d);
                event.put("boolBoxed", null);
                event.put("intBoxed", null);
                event.put("longBoxed", null);
                event.put("charBoxed", null);
                event.put("shortBoxed", null);
                event.put("byteBoxed", null);
                event.put("floatBoxed", null);
                event.put("doubleBoxed", null);
                event.put("bigDecimal", null);
                event.put("bigInteger", null);
                event.put("enumValue", null);
                break;
            case "SupportTradeEvent":
                // the 3-argument constructor leaves ccypair/direction null.
                event = new LinkedHashMap<>();
                event.put("id", 0);
                event.put("userId", null);
                event.put("ccypair", null);
                event.put("direction", null);
                event.put("amount", 0);
                event.put("id", integer(payload, "id"));
                JsonValue userId = payload.get("userId");
                if (userId != null && !userId.isNull()) {
                    event.put("userId", userId.asString());
                }
                event.put("amount", integer(payload, "amount"));
                break;
            default:
                throw new IllegalArgumentException("unknown event type: " + eventType);
        }
        runtime.getEventService().sendEventMap(event, eventType);
        boolean fired = records.size() > before;
        if (fired != expectedFire) {
            throw new IllegalStateException("send " + eventType
                    + " fired=" + fired + " contradicts expectedFire=" + expectedFire);
        }
    }
    /**
     * Declares SupportBean and SupportTradeEvent as Map event types (the
     * expr-filter oracle convention) so pattern tags surface as plain
     * property maps; the Go side sends the equivalent tag-map
     * projections. SupportBean mirrors the 20-property SupportBean shape
     * pinned by ExprFilterInAndBetween; SupportTradeEvent mirrors
     * ExprFilterExpressions.
     */
    private static void registerEventTypes(Configuration configuration) {
        Map<String, Object> supportBean = new LinkedHashMap<>();
        supportBean.put("theString", String.class);
        supportBean.put("boolPrimitive", boolean.class);
        supportBean.put("intPrimitive", int.class);
        supportBean.put("longPrimitive", long.class);
        supportBean.put("charPrimitive", char.class);
        supportBean.put("shortPrimitive", short.class);
        supportBean.put("bytePrimitive", byte.class);
        supportBean.put("floatPrimitive", float.class);
        supportBean.put("doublePrimitive", double.class);
        supportBean.put("boolBoxed", Boolean.class);
        supportBean.put("intBoxed", Integer.class);
        supportBean.put("longBoxed", Long.class);
        supportBean.put("charBoxed", Character.class);
        supportBean.put("shortBoxed", Short.class);
        supportBean.put("byteBoxed", Byte.class);
        supportBean.put("floatBoxed", Float.class);
        supportBean.put("doubleBoxed", Double.class);
        supportBean.put("bigDecimal", BigDecimal.class);
        supportBean.put("bigInteger", BigInteger.class);
        supportBean.put("enumValue", SupportEnum.class);
        configuration.getCommon().addEventType("SupportBean", supportBean);

        Map<String, Object> trade = new LinkedHashMap<>();
        trade.put("id", int.class);
        trade.put("userId", String.class);
        trade.put("ccypair", String.class);
        trade.put("direction", String.class);
        trade.put("amount", int.class);
        configuration.getCommon().addEventType("SupportTradeEvent", trade);
    }

    /**
     * Replays one ord-4 tryInvalidCompile probe: the compile must fail with a
     * message starting with the pinned Java prefix (the regression
     * assertion is SupportMessageAssertUtil.startsWith); the recorded
     * operation carries the Go-boundary classification pinned per label.
     */
    private static void invalidProbeStep(Configuration configuration, String caseName,
                                         JsonObject step, JsonArray records) {
        if (!CASE_PROBES.equals(caseName)) {
            throw new IllegalStateException("invalid-probe step is not pinned for case "
                    + caseName);
        }
        String label = string(step, "statement");
        String epl = string(step, "epl");
        String expected = string(step, "expectError");
        String[] pinned = null;
        for (String[] probe : INVALID_PROBES) {
            if (probe[0].equals(label)) {
                pinned = probe;
                break;
            }
        }
        if (pinned == null || !pinned[1].equals(epl) || !pinned[2].equals(expected)) {
            throw new IllegalStateException("invalid-probe step " + label
                    + " is not pinned");
        }
        String caught = compileError(configuration, epl);
        if (caught == null) {
            throw new IllegalStateException("invalid-probe " + label
                    + " unexpectedly compiled");
        }
        if (!caught.startsWith(expected)) {
            throw new IllegalStateException("invalid-probe " + label
                    + " message drift: expected prefix [" + expected + "] got ["
                    + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", pinned[3]);
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", expected);
        records.add(record);
    }

    /** Compiles the EPL against the shared configuration and returns the
     *  caught message, or null when the compile unexpectedly succeeds —
     *  mirrors env.tryInvalidCompile without a path. */
    private static String compileError(Configuration configuration, String epl) {
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            return null;
        } catch (Exception ex) {
            return ex.getMessage() == null ? ex.getClass().getName() : ex.getMessage();
        }
    }

    /**
     * Listener emitting one record per invocation; a listener invocation
     * without a stream is a contract violation. The firing row is verified
     * in-process against the execution's assertEqualsNew pin
     * (event1.id == 2) before recording.
     */
    private static UpdateListener listener(String caseName, JsonArray records,
                                           EPRuntime runtime) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            JsonArray newRows = rows(newEvents);
            JsonArray oldRows = rows(oldEvents);
            if (newRows.size() == 0 && oldRows.size() == 0) {
                throw new IllegalStateException("listener for statement "
                        + statement.getName() + " was invoked without a stream in case "
                        + caseName);
            }
            if (newEvents != null && newEvents.length == 1) {
                Object event1 = newEvents[0].get("event1");
                Object id = event1 instanceof Map ? ((Map<?, ?>) event1).get("id")
                        : newEvents[0].get("event1.id");
                if (!Integer.valueOf(2).equals(id)) {
                    throw new IllegalStateException("event1.id = " + id + ", want 2");
                }
            }
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", 1);
            record.add("time", Instant.ofEpochMilli(
                    runtime.getEventService().getCurrentTime()).toString());
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
            Object value;
            try {
                value = event.get(name);
            } catch (PropertyAccessException unreadable) {
                continue;
            }
            fields.add(name, normalize(value));
        }
        item.add("fields", fields);
        return item;
    }

    /**
     * Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, null as the tagged
     * {"state":"null"} object; maps (pattern tags surface as Maps under
     * select *) render with sorted keys and collections/arrays recurse —
     * the same shapes the Go normalizer emits for the Go fixture structs.
     */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof EventBean eventBean) {
            JsonObject object = new JsonObject();
            String[] names = eventBean.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            for (String name : names) {
                Object member;
                try {
                    member = eventBean.get(name);
                } catch (PropertyAccessException unreadable) {
                    continue;
                }
                object.add(name, normalize(member));
            }
            return object;
        }
        if (value instanceof Map<?, ?> map) {
            JsonObject object = new JsonObject();
            List<String> keys = new ArrayList<>();
            for (Object key : map.keySet()) {
                keys.add(String.valueOf(key));
            }
            Collections.sort(keys);
            for (String key : keys) {
                object.add(key, normalize(map.get(key)));
            }
            return object;
        }
        if (value instanceof Collection<?> collection) {
            JsonArray array = new JsonArray();
            for (Object element : collection) {
                array.add(normalize(element));
            }
            return array;
        }
        if (value.getClass().isArray()) {
            JsonArray array = new JsonArray();
            int length = Array.getLength(value);
            for (int index = 0; index < length; index++) {
                array.add(normalize(Array.get(value, index)));
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
        if (value instanceof Character) {
            return Json.value(String.valueOf(value));
        }
        return Json.value(String.valueOf(value));
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit",
                "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags",
                "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), flatten(RUNTIME_IDS),
                "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), flatten(EXECUTION_NAMES),
                "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), flatten(STATIC_IDS),
                "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[]{}, "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + CASES.length + " cases");
        }
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinals", "runtimeIds", "executionNames",
                    "staticIds", "source", "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || !CASE_OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))
                    || !CASE_SOURCES[index].equals(string(definition, "source"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index "
                        + index);
            }
            validateIntArray(definition.get("ordinals"), ORDINALS[index], "ordinals");
            validateStringArray(definition.get("runtimeIds"), RUNTIME_IDS[index],
                    "runtimeIds");
            validateStringArray(definition.get("executionNames"), EXECUTION_NAMES[index],
                    "executionNames");
            validateStringArray(definition.get("staticIds"), STATIC_IDS[index],
                    "staticIds");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + EXPECTED_STEPS + " steps, got " + steps.size());
        }
        int offset = 0;
        offset = validateCaseMarker(steps, offset, CASE_PROBES);
        offset = validateProbeDeploy(steps, offset, "sl-comment",
                RUNTIME_IDS[0][0], EPL_SL_COMMENT);
        offset = validateProbeDeploy(steps, offset, "ml-comment",
                RUNTIME_IDS[0][0], EPL_ML_COMMENT);
        for (String[] probe : INVALID_PROBES) {
            offset = validateInvalidProbe(steps, offset, probe[0],
                    RUNTIME_IDS[0][1], probe[1], probe[2],
                    "compile-error".equals(probe[3]) ? "build-error" : "unrepresentable");
        }
        offset = validateUndeployAll(steps, offset, CASE_PROBES, RUNTIME_IDS[0][0]);
        offset = validateCaseMarker(steps, offset, CASE_THREADING);
        offset = validateThreadingDeploy(steps, offset);
        offset = validateMarker(steps, offset, "deployed");
        offset = validateMarker(steps, offset, "types");
        offset = validateSend(steps, offset, "SupportBean",
                "{\"expectedFire\":false}");
        offset = validateSend(steps, offset, "SupportTradeEvent",
                "{\"id\":1,\"userId\":null,\"amount\":1001,\"expectedFire\":false}");
        offset = validateSend(steps, offset, "SupportTradeEvent",
                "{\"id\":2,\"userId\":\"1234\",\"amount\":1001,\"expectedFire\":true}");
        offset = validateUndeployAll(steps, offset, CASE_THREADING, RUNTIME_IDS[1][0]);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static int validateCaseMarker(JsonArray steps, int offset, String caseName) {
        JsonObject step = object(steps.get(offset), "case marker");
        requireFields(step, "op", "case");
        if (!"case".equals(string(step, "op")) || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("step " + offset
                    + " is not the pinned case marker for " + caseName);
        }
        return offset + 1;
    }

    private static int validateProbeDeploy(JsonArray steps, int offset, String label,
                                           String runtimeId, String epl) {
        JsonObject step = object(steps.get(offset), "probe deploy step");
        requireFields(step, "op", "case", "statement", "runtimeId", "epl");
        if (!"deploy".equals(string(step, "op"))
                || !CASE_PROBES.equals(string(step, "case"))
                || !label.equals(string(step, "statement"))
                || !runtimeId.equals(string(step, "runtimeId"))
                || !epl.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("step " + offset
                    + " is not the pinned quote-escape probe " + label);
        }
        return offset + 1;
    }

    private static int validateInvalidProbe(JsonArray steps, int offset, String label,
                                            String runtimeId, String epl, String message,
                                            String expectedOp) {
        JsonObject step = object(steps.get(offset), "invalid-probe step");
        requireFields(step, "op", "case", "statement", "runtimeId", "epl", "expectError");
        if (!expectedOp.equals(string(step, "op"))
                || !CASE_PROBES.equals(string(step, "case"))
                || !label.equals(string(step, "statement"))
                || !runtimeId.equals(string(step, "runtimeId"))
                || !epl.equals(string(step, "epl"))
                || !message.equals(string(step, "expectError"))) {
            throw new IllegalArgumentException("step " + offset
                    + " is not the pinned invalid probe " + label);
        }
        return offset + 1;
    }

    private static int validateThreadingDeploy(JsonArray steps, int offset) {
        JsonObject step = object(steps.get(offset), "threading deploy step");
        requireFields(step, "op", "case", "statement", "runtimeId", "epl");
        if (!"deploy".equals(string(step, "op"))
                || !CASE_THREADING.equals(string(step, "case"))
                || !"s0".equals(string(step, "statement"))
                || !RUNTIME_IDS[1][0].equals(string(step, "runtimeId"))
                || !EPL_THREADING.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("step " + offset
                    + " is not the pinned threading deploy");
        }
        return offset + 1;
    }

    private static int validateMarker(JsonArray steps, int offset, String operation) {
        JsonObject step = object(steps.get(offset), operation + " step");
        requireFields(step, "op", "case", "statement", "runtimeId");
        if (!operation.equals(string(step, "op"))
                || !CASE_THREADING.equals(string(step, "case"))
                || !"s0".equals(string(step, "statement"))
                || !RUNTIME_IDS[1][0].equals(string(step, "runtimeId"))) {
            throw new IllegalArgumentException("step " + offset + " is not the pinned "
                    + operation + " step");
        }
        return offset + 1;
    }

    private static int validateSend(JsonArray steps, int offset, String eventType,
                                    String expectedPayload) {
        JsonObject step = object(steps.get(offset), "send step");
        requireFields(step, "op", "case", "statement", "runtimeId", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !CASE_THREADING.equals(string(step, "case"))
                || !"s0".equals(string(step, "statement"))
                || !RUNTIME_IDS[1][0].equals(string(step, "runtimeId"))
                || !eventType.equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("step " + offset
                    + " is not the pinned send " + eventType);
        }
        JsonObject expected = Json.parse(expectedPayload).asObject();
        if (!expected.equals(object(step.get("payload"), "payload"))) {
            throw new IllegalArgumentException("step " + offset
                    + " send payload is not pinned");
        }
        return offset + 1;
    }

    private static int validateUndeployAll(JsonArray steps, int offset, String caseName,
                                           String runtimeId) {
        JsonObject step = object(steps.get(offset), "undeploy-all step");
        requireFields(step, "op", "case", "runtimeId");
        if (!"undeploy-all".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !runtimeId.equals(string(step, "runtimeId"))) {
            throw new IllegalArgumentException("step " + offset
                    + " is not the pinned undeploy-all for " + caseName);
        }
        return offset + 1;
    }

    private static String[] flatten(String[][] matrix) {
        List<String> flat = new ArrayList<>();
        for (String[] row : matrix) {
            flat.addAll(Arrays.asList(row));
        }
        return flat.toArray(new String[0]);
    }

    private static void validateIntArray(JsonValue value, int[] expected, String label) {
        JsonArray actual = array(value, label);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(label + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (!(item instanceof JsonNumber) || item.asInt() != expected[index]) {
                throw new IllegalArgumentException(label + " mismatch at index " + index);
            }
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
        JsonValue value = object.get(name);
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be a JSON integer");
        }
        return value.asInt();
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
