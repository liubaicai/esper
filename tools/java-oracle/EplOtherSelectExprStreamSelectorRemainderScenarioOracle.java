import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventType;
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
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBeanComplexProps;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Properties;
import java.util.Set;

/**
 * Java oracle for the EPLOtherSelectExprStreamSelector remainder unit: the
 * transpose and invalid executions left after the wildcard/alias parity
 * units.  Three cases replay on one fresh runtime each (the regression env
 * disables the internal timer, so timer:within(30 sec) never fires and
 * pattern matches are immediate at t=0):
 *
 * transpose-nested (ordinal 1, EPLOtherInsertTransposeNestedProperty):
 * deploys "@name('l1') @public insert into StreamA select nested.* from
 * SupportBeanComplexProps as s0" and "@name('l2') select nestedValue from
 * StreamA", sends one SupportBeanComplexProps default bean and records both
 * listeners' new-only rows (nestedValue="nestedValue").  "types" steps pin
 * the Java-asserted event-type surface: l1's underlying is the nested bean
 * class and l2's nestedValue property is String.
 *
 * insert-from-pattern (ordinal 2, EPLOtherInsertFromPattern): deploys l1
 * "insert into streamA select a.* from pattern [every a=SupportBean]" and l2
 * with an added timer:within(30 sec) guard; both attach listeners (l1's
 * deliveries are unasserted by the Java source but still recorded).  Two
 * SupportBean sends deliver the sent bean as the row underlying to each
 * listener; the oracle asserts underlying identity and renders the pinned
 * SupportBean mirror surface {intPrimitive, theString}.  l3 "insert into
 * streamB select a.*, 'abc' as abc from pattern [...]" deploys after the
 * sends with no listener and zero deliveries.  "types" steps pin l1's
 * SupportBean underlying and l3's Pair underlying plus the asserted
 * abc/theString String property types.
 *
 * invalid (ordinals 0 and 16, EPLOtherInvalidSelectWildcardProperty and
 * EPLOtherInvalidSelect): five build-error probes record the pinned Java
 * message prefixes — the property-wildcard-with-column-name rejection, the
 * duplicate 'theString'/'abc' column names, the unknown 's1.*' stream
 * selector and the multi-stream stream.* without column name.  All probes
 * compile without the runtime path, mirroring env.tryInvalidCompile's
 * path-less compileWCheckedEx.
 *
 * The "types" record value carries exactly the Java-asserted surface:
 * "underlying" holds the asserted getUnderlyingType() simple name and
 * "properties" holds the asserted property name-to-type pairs; keys absent
 * when the Java source asserts nothing for them.
 */
public final class EplOtherSelectExprStreamSelectorRemainderScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "epl-other-select-expr-stream-selector-remainder";
    private static final String DESCRIPTION =
            "EPLOtherSelectExprStreamSelector remainder executions: transpose-nested (ord 1) routes "
                    + "nested.* of a SupportBeanComplexProps default bean into @public StreamA whose "
                    + "consumer selects nestedValue, both listeners delivering nestedValue=\"nestedValue\"; "
                    + "insert-from-pattern (ord 2) transposes the tagged SupportBean underlying out of "
                    + "every-pattern matches into streamA (l1 unasserted but recorded, l2 asserted by "
                    + "underlying identity) and, after the sends, into streamB with an added 'abc' constant "
                    + "(Pair underlying, no listener, zero deliveries); invalid (ords 0+16) records the "
                    + "five pinned compile-error prefixes for property-wildcard-with-column-name, duplicate "
                    + "column names, an unknown stream selector and multi-stream stream.* without a "
                    + "column name. Ord 3 (EPLOtherObjectModelJoinAlias) is intentionally-different "
                    + "(SODA/eplToModel JVM-only) and carries no steps.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/"
                    + "EPLOtherSelectExprStreamSelector.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-784378ea15f390e587b2",
            "java-runtime-68b49a5d4a1630d85f75",
            "java-runtime-0fe17586d141b87f3cff",
            "java-runtime-fb00a9a05a09d998736d"
    };
    private static final String[] EXECUTION_NAMES = {
            "EPLOtherInsertTransposeNestedProperty",
            "EPLOtherInsertFromPattern",
            "EPLOtherInvalidSelectWildcardProperty",
            "EPLOtherInvalidSelect"
    };
    private static final String[] STATIC_IDS = {
            "java-5dcc72bb024241a1c003",
            "java-587f1eac414f59c3f257",
            "java-53aaecf8c98ebdd1c538",
            "java-0b8c93bdcb738bd64543"
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String[] CASES = {
            "transpose-nested", "insert-from-pattern", "invalid"
    };
    private static final int[] ORDINALS = {1, 2, 0};
    private static final String[] CASE_OBSERVATIONS = {
            "listener+types; @public insert-into transposes the nested fragment into StreamA "
                    + "(underlying SupportBeanSpecialGetterNested) and the consumer selects "
                    + "nestedValue; one default bean delivers nestedValue=\"nestedValue\" to both "
                    + "listeners; the second undeployAll in the Java source is a no-op and carries "
                    + "no step",
            "listener+types; two pattern-source transposes insert the sent SupportBean underlying "
                    + "into streamA per send (l1 unasserted but recorded, l2 asserted); l3 deploys "
                    + "after the sends into streamB (underlying Pair, abc/theString String) with no "
                    + "listener and zero deliveries",
            "compile-error; ord-0 property-wildcard-with-column-name probe plus ord-16's four "
                    + "stream-selector probes record the pinned Java message prefixes in one pass "
                    + "(ord 16's runtime java-runtime-fb00a9a05a09d998736d is covered by this pass)"
    };

    // Verbatim transcriptions of EPLOtherSelectExprStreamSelector lines 57,
    // 65, 69, 85, 88, 102, 465, 468, 471, 474.
    private static final String EPL_TRANSPOSE_L1 =
            "@name('l1') @public insert into StreamA select nested.* "
                    + "from SupportBeanComplexProps as s0";
    private static final String EPL_TRANSPOSE_L2 =
            "@name('l2') select nestedValue from StreamA";
    private static final String EPL_PATTERN_L1 =
            "@name('l1') insert into streamA select a.* from pattern [every a=SupportBean]";
    private static final String EPL_PATTERN_L2 =
            "@name('l2') insert into streamA select a.* from pattern "
                    + "[every a=SupportBean where timer:within(30 sec)]";
    private static final String EPL_PATTERN_L3 =
            "@name('l3') insert into streamB select a.*, 'abc' as abc from pattern "
                    + "[every a=SupportBean where timer:within(30 sec)]";
    private static final String EPL_INVALID_WILDCARD_PROPERTY =
            "select simpleProperty.* as a from SupportBeanComplexProps as s0";
    private static final String EPL_INVALID_DUPLICATE_COLUMN =
            "select theString.* as theString, theString from SupportBean#length(3) as theString";
    private static final String EPL_INVALID_UNKNOWN_SELECTOR =
            "select s1.* as abc from SupportBean#length(3) as s0";
    private static final String EPL_INVALID_DUPLICATE_STREAM_ALIAS =
            "select s0.* as abc, s0.* as abc from SupportBean#length(3) as s0";
    private static final String EPL_INVALID_MULTI_STREAM =
            "select s0.*, s1.* from SupportBean#keepall as s0, SupportBean#keepall as s1";

    private static final String ERR_WILDCARD_PROPERTY =
            "The property wildcard syntax must be used without column name";
    private static final String ERR_DUPLICATE_COLUMN =
            "Column name 'theString' appears more then once in select clause";
    private static final String ERR_UNKNOWN_SELECTOR =
            "Stream selector 's1.*' does not match any stream name in the from clause [";
    private static final String ERR_DUPLICATE_STREAM_ALIAS =
            "Column name 'abc' appears more then once in select clause";
    private static final String ERR_MULTI_STREAM =
            "A column name must be supplied for all but one stream if multiple streams are "
                    + "selected via the stream.* notation";

    // SupportBeanComplexProps.makeDefaultBean() rendered as the pinned send
    // payload: simpleProperty "simple", mapped {keyOne,keyTwo}, indexed [1,2],
    // mapProperty {xOne,xTwo}, arrayProperty [10,20,30] and the nested fragment
    // chain nestedValue/nestedNestedValue.
    private static final String COMPLEX_PROPS_PAYLOAD =
            "{\"simpleProperty\":\"simple\",\"mapped\":{\"keyOne\":\"valueOne\","
                    + "\"keyTwo\":\"valueTwo\"},\"indexed\":[1,2],"
                    + "\"mapProperty\":{\"xOne\":\"yOne\",\"xTwo\":\"yTwo\"},"
                    + "\"arrayProperty\":[10,20,30],\"nested\":{\"nestedValue\":\"nestedValue\","
                    + "\"nestedNestedValue\":\"nestedNestedValue\"}}";

    private static final String[] CASE_EPLS = {
            EPL_TRANSPOSE_L1, EPL_PATTERN_L1, EPL_INVALID_WILDCARD_PROPERTY
    };

    private static final Set<String> LISTENED_STATEMENTS = new HashSet<>(
            Arrays.asList("l1", "l2"));

    // Java-asserted event-type surface per case/statement: the underlying
    // simple name where the source asserts getUnderlyingType() and the
    // asserted property name-to-type pairs.
    private static final Map<String, String> EXPECTED_UNDERLYING = new HashMap<>();
    private static final Map<String, String[][]> EXPECTED_PROPERTIES = new HashMap<>();
    static {
        EXPECTED_UNDERLYING.put("transpose-nested/l1", "SupportBeanSpecialGetterNested");
        EXPECTED_UNDERLYING.put("insert-from-pattern/l1", "SupportBean");
        EXPECTED_UNDERLYING.put("insert-from-pattern/l3", "Pair");
        EXPECTED_PROPERTIES.put("transpose-nested/l2", new String[][]{{"nestedValue", "String"}});
        EXPECTED_PROPERTIES.put("insert-from-pattern/l3",
                new String[][]{{"abc", "String"}, {"theString", "String"}});
    }

    private static final int EXPECTED_STEPS = 22;
    private static final int EXPECTED_RECORDS = 15;

    /**
     * Pinned per-case step keys rendered as
     * op|case|statement|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|selector|ids|fields.  Deploy steps carry the byte-exact EPL text;
     * send payloads render as their compact JSON.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("transpose-nested", new String[]{
                "deploy|transpose-nested|l1||" + EPL_TRANSPOSE_L1 + "|||||||",
                "types|transpose-nested|l1|||||||||",
                "deploy|transpose-nested|l2||" + EPL_TRANSPOSE_L2 + "|||||||",
                "types|transpose-nested|l2|||||||||",
                "send|transpose-nested||SupportBeanComplexProps||" + COMPLEX_PROPS_PAYLOAD + "||||||",
                "undeploy-all|transpose-nested||||||||||",
        });
        CASE_STEPS.put("insert-from-pattern", new String[]{
                "deploy|insert-from-pattern|l1||" + EPL_PATTERN_L1 + "|||||||",
                "deploy|insert-from-pattern|l2||" + EPL_PATTERN_L2 + "|||||||",
                "types|insert-from-pattern|l1|||||||||",
                "send|insert-from-pattern||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":10}||||||",
                "send|insert-from-pattern||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":10}||||||",
                "deploy|insert-from-pattern|l3||" + EPL_PATTERN_L3 + "|||||||",
                "types|insert-from-pattern|l3|||||||||",
                "undeploy-all|insert-from-pattern||||||||||",
        });
        CASE_STEPS.put("invalid", new String[]{
                "build-error|invalid|wildcard-property-column-name||" + EPL_INVALID_WILDCARD_PROPERTY
                        + "||" + ERR_WILDCARD_PROPERTY + "|1||||",
                "build-error|invalid|duplicate-column-name||" + EPL_INVALID_DUPLICATE_COLUMN
                        + "||" + ERR_DUPLICATE_COLUMN + "|1||||",
                "build-error|invalid|unknown-stream-selector||" + EPL_INVALID_UNKNOWN_SELECTOR
                        + "||" + ERR_UNKNOWN_SELECTOR + "|1||||",
                "build-error|invalid|duplicate-stream-alias||" + EPL_INVALID_DUPLICATE_STREAM_ALIAS
                        + "||" + ERR_DUPLICATE_STREAM_ALIAS + "|1||||",
                "build-error|invalid|multi-stream-wildcard||" + EPL_INVALID_MULTI_STREAM
                        + "||" + ERR_MULTI_STREAM + "|1||||",
        });
    }

    private EplOtherSelectExprStreamSelectorRemainderScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: EplOtherSelectExprStreamSelectorRemainderScenarioOracle <scenario.json>");
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
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(caseIndex, allSteps, records);
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

    /**
     * Replays one case's steps on a fresh runtime; sequences restart per
     * case.  Deploys register their statement under the step label so types
     * steps can resolve it; listeners attach to statements named l1/l2 (l3
     * deploys without a listener, mirroring the Java source).
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBeanComplexProps.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-" + ID + "-" + RUNTIME_IDS[caseIndex], configuration);
        runtime.getEventService().advanceTime(0);
        try {
            Map<String, Integer> sequences = new HashMap<>();
            Map<String, EPStatement> statements = new HashMap<>();
            CaseContext context = new CaseContext(caseName);
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
                        String label = string(step, "statement");
                        String epl = string(step, "epl");
                        EPCompiled compiled = compileModule(epl, configuration, runtime);
                        EPDeployment deployment = runtime.getDeploymentService()
                                .deploy(compiled, new DeploymentOptions());
                        EPStatement[] deployed = deployment.getStatements();
                        if (deployed.length != 1) {
                            throw new IllegalStateException("deployment of " + label + " has "
                                    + deployed.length + " statements, want 1");
                        }
                        EPStatement statement = deployed[0];
                        if (!label.equals(statement.getName())) {
                            throw new IllegalStateException("deployment of " + label
                                    + " produced statement named " + statement.getName());
                        }
                        if (LISTENED_STATEMENTS.contains(statement.getName())) {
                            statement.addListener(
                                    listener(context, sequences, records, runtime, statement));
                        }
                        statements.put(label, statement);
                        break;
                    }
                    case "send":
                        sendEvent(runtime, context, string(step, "eventType"),
                                object(step.get("payload"), "payload"));
                        break;
                    case "types":
                        typesStep(caseName, statements, string(step, "statement"), records,
                                runtime);
                        break;
                    case "build-error":
                        buildErrorStep(runtime, configuration, caseName, step, records);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        statements.clear();
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
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

    /**
     * Module compile with the runtime path, mirroring
     * RegressionEnvironmentBase.compileDeploy(epl, path): the compiler sees
     * the configuration plus the already-deployed module path so l2 resolves
     * the @public StreamA insert-into type.
     */
    private static EPCompiled compileModule(String epl, Configuration configuration,
                                            EPRuntime runtime) throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        return EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
    }

    /**
     * Emits a {"operation":"types"} record carrying exactly the
     * Java-asserted event-type surface for the statement: the underlying
     * simple name where the source asserts getUnderlyingType() and the
     * asserted property name-to-type pairs.  The actual event type is
     * verified against the pinned surface before recording so a drift fails
     * the oracle (env.assertStatement semantics).
     */
    private static void typesStep(String caseName, Map<String, EPStatement> statements,
                                  String label, JsonArray records, EPRuntime runtime) {
        EPStatement statement = statements.get(label);
        if (statement == null) {
            throw new IllegalStateException("types statement " + label
                    + " was not deployed in case " + caseName);
        }
        String key = caseName + "/" + label;
        EventType eventType = statement.getEventType();
        JsonObject value = new JsonObject();
        String underlying = EXPECTED_UNDERLYING.get(key);
        if (underlying != null) {
            String actual = eventType.getUnderlyingType() == null
                    ? "null" : eventType.getUnderlyingType().getSimpleName();
            if (!underlying.equals(actual)) {
                throw new IllegalStateException("underlying type drift for " + key
                        + ": expected " + underlying + " got " + actual);
            }
            value.add("underlying", underlying);
        }
        String[][] properties = EXPECTED_PROPERTIES.get(key);
        if (properties != null) {
            JsonObject pinned = new JsonObject();
            for (String[] pair : properties) {
                Class<?> propertyType = eventType.getPropertyType(pair[0]);
                String actual = propertyType == null ? "null" : propertyType.getSimpleName();
                if (!pair[1].equals(actual)) {
                    throw new IllegalStateException("property type drift for " + key + "."
                            + pair[0] + ": expected " + pair[1] + " got " + actual);
                }
                pinned.add(pair[0], pair[1]);
            }
            value.add("properties", pinned);
        }
        if (value.size() == 0) {
            throw new IllegalStateException("types step for " + key + " pins no asserted surface");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "types");
        record.add("statement", statement.getName());
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(
                runtime.getEventService().getCurrentTime()).toString());
        record.add("value", value);
        records.add(record);
    }

    /**
     * Compiles an expected-invalid probe and emits {"operation":"compile-error"}
     * carrying the pinned expectError prefix after verifying the caught
     * message starts with it (SupportMessageAssertUtil.assertMessage
     * semantics).  All five probes carry compileWithoutPath mirroring
     * env.tryInvalidCompile's path-less compileWCheckedEx.
     */
    private static void buildErrorStep(EPRuntime runtime, Configuration configuration,
                                       String caseName, JsonObject step, JsonArray records)
            throws Exception {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        String epl = string(step, "epl");
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            if (!step.getBoolean("compileWithoutPath", false)) {
                compilerArgs.getPath().add(runtime.getRuntimePath());
            }
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught == null || caught.equals("<no-error>")) {
            throw new IllegalStateException("build-error probe " + label
                    + " unexpectedly succeeded");
        }
        if (!expected.isEmpty() && !caught.startsWith(expected)) {
            throw new IllegalStateException("compile-error message drift for " + label
                    + ": expected prefix [" + expected + "] got [" + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "compile-error");
        record.add("statement", label);
        record.add("sequence", 0);
        if (!expected.isEmpty()) {
            record.add("value", expected);
        }
        records.add(record);
    }

    /**
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; new and old arrays render only when non-empty.  Each
     * delivery must be a single new-only row.  insert-from-pattern rows
     * assert the delivered underlying is the exact sent SupportBean instance
     * (the Java source's assertSame) and render the pinned bean mirror
     * surface {intPrimitive, theString}; transpose-nested rows render all
     * sorted event-type properties.
     */
    private static UpdateListener listener(CaseContext context, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime,
                                           EPStatement statement) {
        return (newEvents, oldEvents, ignoredStatement, ignoredRuntime) -> {
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if (!hasNew || hasOld || newEvents.length != 1) {
                throw new IllegalStateException("case " + context.caseName
                        + " listener callback must contain one new-only row");
            }
            EventBean event = newEvents[0];
            JsonObject row;
            if ("insert-from-pattern".equals(context.caseName)) {
                if (event.getUnderlying() != context.lastBean) {
                    throw new IllegalStateException("case " + context.caseName
                            + " row underlying is not the sent SupportBean instance");
                }
                JsonObject fields = new JsonObject();
                fields.add("intPrimitive", normalize(context.lastBean.getIntPrimitive()));
                fields.add("theString", normalize(context.lastBean.getTheString()));
                row = new JsonObject().add("kind", "row").add("fields", fields);
            } else {
                if ("transpose-nested".equals(context.caseName) && "l1".equals(statement.getName())
                        && !(event.getUnderlying()
                        instanceof SupportBeanComplexProps.SupportBeanSpecialGetterNested)) {
                    throw new IllegalStateException("case " + context.caseName
                            + " l1 row underlying is not the nested bean: "
                            + event.getUnderlying());
                }
                row = fullRow(event);
            }
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", context.caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time",
                    Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            record.add("new", new JsonArray().add(row));
            records.add(record);
        };
    }

    /** Canonical row rendering with sorted property names for a stable field order. */
    private static JsonObject fullRow(EventBean event) {
        JsonObject fields = new JsonObject();
        String[] names = event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        for (String name : names) {
            fields.add(name, normalize(event.get(name)));
        }
        return new JsonObject().add("kind", "row").add("fields", fields);
    }

    /**
     * Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, and null as the tagged
     * {"state":"null"} object.  EventBean values and the nested fragment
     * beans render as {"kind":"row","fields":{...}} over their sorted
     * properties so the transposed nested fragment compares field-for-field
     * with the Go rendering.
     */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof EventBean eventBean) {
            return fullRow(eventBean);
        }
        if (value instanceof SupportBeanComplexProps.SupportBeanSpecialGetterNested nested) {
            JsonObject fields = new JsonObject();
            fields.add("nestedNested", normalize(nested.getNestedNested()));
            fields.add("nestedValue", normalize(nested.getNestedValue()));
            JsonObject row = new JsonObject();
            row.add("kind", "row");
            row.add("fields", fields);
            return row;
        }
        if (value instanceof SupportBeanComplexProps.SupportBeanSpecialGetterNestedNested nested) {
            JsonObject fields = new JsonObject();
            fields.add("nestedNestedValue", normalize(nested.getNestedNestedValue()));
            JsonObject row = new JsonObject();
            row.add("kind", "row");
            row.add("fields", fields);
            return row;
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

    private static void sendEvent(EPRuntime runtime, CaseContext context, String type,
                                  JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                requireFields(payload, "theString", "intPrimitive");
                SupportBean bean = new SupportBean(
                        nullableString(payload, "theString"),
                        integer(payload, "intPrimitive"));
                context.lastBean = bean;
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBeanComplexProps": {
                requireFields(payload, "simpleProperty", "mapped", "indexed", "mapProperty",
                        "arrayProperty", "nested");
                Properties mapped = new Properties();
                JsonObject mappedObject = object(payload.get("mapped"), "mapped");
                for (String key : mappedObject.names()) {
                    mapped.put(key, string(mappedObject, key));
                }
                Map<String, String> mapProperty = new HashMap<>();
                JsonObject mapObject = object(payload.get("mapProperty"), "mapProperty");
                for (String key : mapObject.names()) {
                    mapProperty.put(key, string(mapObject, key));
                }
                JsonObject nested = object(payload.get("nested"), "nested");
                requireFields(nested, "nestedValue", "nestedNestedValue");
                SupportBeanComplexProps bean = new SupportBeanComplexProps(
                        string(payload, "simpleProperty"),
                        mapped,
                        intArray(payload.get("indexed"), "indexed"),
                        mapProperty,
                        intArray(payload.get("arrayProperty"), "arrayProperty"),
                        string(nested, "nestedValue"),
                        string(nested, "nestedNestedValue"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
    }

    /** Per-case send state: the last sent SupportBean for the underlying-identity check. */
    private static final class CaseContext {
        private final String caseName;
        private SupportBean lastBean;

        private CaseContext(String caseName) {
            this.caseName = caseName;
        }
    }

    private static String nullableString(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        if (value == null || value.isNull()) {
            return null;
        }
        return value.asString();
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
        validateStringArray(scenario.get("javaFlags"), JAVA_FLAGS, "javaFlags");

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
        for (String caseName : CASES) {
            validateCaseMarker(steps.get(offset++), caseName);
            String[] expected = CASE_STEPS.get(caseName);
            for (String key : expected) {
                JsonObject step = object(steps.get(offset++), "step");
                String actual = stepKey(step);
                if (!key.equals(actual)) {
                    throw new IllegalArgumentException("step is not pinned for " + caseName
                            + ": expected [" + key + "] got [" + actual + "]");
                }
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    /**
     * Renders one step as its pinned key:
     * op|case|statement|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|selector|ids|fields with the payload compacted and ids/fields
     * rendered as JSON arrays.  Unknown fields are rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
                "op", "case", "statement", "eventType", "epl", "payload",
                "expectError", "compileWithoutPath", "mode", "selector", "ids", "fields"));
        for (String field : step.names()) {
            if (!allowed.contains(field)) {
                throw new IllegalArgumentException("step has unexpected field " + field);
            }
        }
        JsonValue payload = step.get("payload");
        String payloadText = payload == null ? "" : payload.toString();
        String cwp = step.getBoolean("compileWithoutPath", false) ? "1" : "";
        JsonValue ids = step.get("ids");
        String idsText = ids == null ? "" : ids.toString();
        JsonValue fields = step.get("fields");
        String fieldsText = fields == null ? "" : joinStrings(fields);
        return string(step, "op") + "|" + string(step, "case") + "|" + string(step, "statement")
                + "|" + string(step, "eventType") + "|" + string(step, "epl") + "|" + payloadText
                + "|" + string(step, "expectError") + "|" + cwp
                + "|" + string(step, "mode") + "|" + string(step, "selector") + "|" + idsText
                + "|" + fieldsText;
    }

    private static String joinStrings(JsonValue value) {
        JsonArray items = array(value, "fields");
        StringBuilder text = new StringBuilder();
        for (int index = 0; index < items.size(); index++) {
            if (index > 0) {
                text.append(',');
            }
            JsonValue item = items.get(index);
            if (!(item instanceof JsonString)) {
                throw new IllegalArgumentException("fields must be a string array");
            }
            text.append(item.asString());
        }
        return text.toString();
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
        if (value == null) {
            return "";
        }
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

    private static int[] intArray(JsonValue value, String label) {
        JsonArray items = array(value, label);
        int[] result = new int[items.size()];
        for (int index = 0; index < items.size(); index++) {
            result[index] = (int) longInteger(items.get(index), label);
        }
        return result;
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
