import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.internal.support.SupportEnum;
import com.espertech.esper.common.internal.util.SerializableObjectCopier;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the Draft 4.444 'epl-other-for-group-delivery'
 * unit: EPLOtherForGroupDelivery ordinals 0-5.  Each scenario case replays
 * inside its own runtime.
 *
 * Ordinal 0 (EPLOtherInvalid) is compile-only: seven tryInvalidCompile probes
 * replayed through the compiler with the suite's assertMessage prefix
 * semantics (startsWith), each emitted as a compile-rejected record carrying
 * the pinned errorPrefix.
 *
 * Ordinal 1 (EPLOtherSubscriberOnly) is the suite's only subscriber surface:
 * the scenario attaches a SupportSubscriberMRD-shaped subscriber
 * (update(Object[][] insertStream, Object[][] removeStream)) instead of a
 * listener.  Discrete delivery produces one update call per row; grouped
 * delivery produces one call per group.  Records carry operation "listener"
 * per the frozen contract (no subscriber step kind exists in the replay
 * protocol); the subscriber-only nature is pinned in the case observation.
 * Note the suite's SupportSubscriberMRD mirrors insert rows into its remove
 * list; this oracle records the true remove stream, which is empty at the
 * first (and only) flush of each deployment because time_batch releases the
 * prior interval's batch as old data.
 *
 * Ordinals 2-5 are listener cases.  Ordinal 2 (EPLOtherDiscreteDelivery)
 * covers discrete delivery over #time_batch(1) plus the empty-batch
 * suppression of 'OUTPUT ALL EVERY 1 seconds' over ObjectEvent (the second
 * quiet boundary delivers no callback).  Ordinal 3 (EPLOtherGroupDelivery)
 * covers grouped delivery: unsorted, order-by-desc sorted, multi-key
 * (doubleBoxed, enumValue) sorted, and a fourth deploy that replays the
 * suite's SODA phase through eplToModel + compile(model) with the same
 * byte-exact statement text.  Ordinals 4 and 5 deploy a two-statement module
 * (create context + named select) whose 'output snapshot when terminated'
 * fires the grouped delivery at context termination; ordinal 4 keys on the
 * int[] intOne property (deep equality) and ordinal 5 on the
 * (intPrimitive, longPrimitive) pair.
 *
 * Event types mirror the pinned harness with reduced surfaces: SupportBean is
 * a local bean exposing only the five properties the executions send or
 * select (theString, intPrimitive, longPrimitive, doubleBoxed, enumValue);
 * SupportEventWithManyArray is a map type {id string, intOne int[]} exactly
 * like the epl-other-distinct oracle; ObjectEvent is java.lang.Object
 * registered under the suite's explicit name.  The internal timer is
 * disabled and every timed case begins with an advance-time step to the
 * epoch, matching the suite's advanceTime(0) before deploy.
 */
public final class EPLOtherForGroupDeliveryScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "epl-other-for-group-delivery";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherForGroupDelivery.java";
    private static final String DESCRIPTION =
            "EPLOtherForGroupDelivery ordinals 0-5: seven tryInvalidCompile probes (ord 0), " +
            "subscriber-only discrete and grouped delivery over #time_batch(1) (ord 1), " +
            "listener discrete delivery plus empty-batch suppression (ord 2), grouped listener " +
            "delivery over order-by and multi-key with a SODA phase (ord 3), and " +
            "context-terminated grouped snapshots keyed on int[] (ord 4) and " +
            "(intPrimitive,longPrimitive) (ord 5)";

    private static final String[] CASES = {
            "invalid",
            "subscriber-only",
            "discrete-delivery",
            "group-delivery",
            "group-delivery-array-key",
            "group-delivery-two-field-key"
    };
    private static final int[] ORDINALS = {0, 1, 2, 3, 4, 5};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-739ab323159d5359d1d4",
            "java-runtime-a89ce4feaab75c0bebc5",
            "java-runtime-1b0fc79aaff7449c2c6e",
            "java-runtime-6238688d054ca4ece96e",
            "java-runtime-8cf7563abbca0f451537",
            "java-runtime-1a7765c6b98eb79db7ce"
    };
    private static final String[] EXECUTIONS = {
            "EPLOtherInvalid",
            "EPLOtherSubscriberOnly",
            "EPLOtherDiscreteDelivery",
            "EPLOtherGroupDelivery",
            "EPLOtherGroupDeliveryMultikeyWArraySingleArray",
            "EPLOtherGroupDeliveryMultikeyWArrayTwoField"
    };
    private static final String[] OBSERVATIONS = {
            "compile-only; seven tryInvalidCompile probes with prefix-matched messages",
            "subscriber; discrete then grouped delivery observed through a " +
                    "SupportSubscriberMRD-shaped update(Object[][],Object[][]) subscriber, " +
                    "emitted as listener records",
            "listener; discrete delivery plus empty-batch suppression under " +
                    "output all every 1 seconds",
            "listener; grouped delivery over order-by and multi-key; the fourth deploy is " +
                    "the SODA eplToModel compile-path variant of the third",
            "listener; grouped delivery keyed on int[] deep equality under a " +
                    "start-now/end-after-1-second context snapshot",
            "listener; grouped delivery keyed on (intPrimitive,longPrimitive) under a " +
                    "start-now/end-after-1-second context snapshot"
    };

    private static final String[] JAVA_RUNTIMES = RUNTIME_IDS;
    private static final String[] JAVA_NAMES = EXECUTIONS;
    private static final String[] JAVA_STATIC_IDS = {
            "java-49b703a73b938557e3c1",
            "java-49b703a73b938557e3c1",
            "java-49b703a73b938557e3c1",
            "java-49b703a73b938557e3c1",
            "java-49b703a73b938557e3c1",
            "java-49b703a73b938557e3c1"
    };
    private static final String[] JAVA_FLAGS = {"OBSERVEROPS", "SERDEREQUIRED"};

    // Subscriber row columns in select-clause order (the Object[][] row shape
    // Esper delivers to update(Object[][],Object[][])); both subscriber
    // statements project theString then intPrimitive.
    private static final String[] SUBSCRIBER_COLUMNS = {"theString", "intPrimitive"};

    // EPLOtherInvalid (ordinal 0) probes, byte-exact from the suite including
    // the trailing space of the first EPL and of the last expected prefix.
    private static final String[] INVALID_STATEMENTS = {
            "for-eof",
            "for-other-keyword",
            "grouped-no-expr",
            "grouped-empty-expr",
            "grouped-unknown-prop",
            "discrete-with-expr",
            "discrete-then-grouped"
    };
    private static final String[] INVALID_EPLS = {
            "select * from SupportBean for ",
            "select * from SupportBean for other_keyword",
            "select * from SupportBean for grouped_delivery",
            "select * from SupportBean for grouped_delivery()",
            "select * from SupportBean for grouped_delivery(dummy)",
            "select * from SupportBean for discrete_delivery(dummy)",
            "select * from SupportBean for discrete_delivery for grouped_delivery(intPrimitive)"
    };
    private static final String[] INVALID_PREFIXES = {
            "Incorrect syntax near end-of-input ('for' is a reserved keyword) expecting an " +
                    "identifier but found end-of-input at line 1 column 29",
            "Expected any of the [grouped_delivery, discrete_delivery] for-clause keywords " +
                    "after reserved keyword 'for'",
            "The for-clause with the grouped_delivery keyword requires one or more grouping expressions",
            "The for-clause with the grouped_delivery keyword requires one or more grouping expressions",
            "Failed to validate for-clause expression 'dummy': Property named 'dummy' is not " +
                    "valid in any stream",
            "The for-clause with the discrete_delivery keyword does not allow grouping expressions",
            "Incorrect syntax near 'for' (a reserved keyword) at line 1 column 48 "
    };

    // EPLOtherSubscriberOnly (ordinal 1) EPLs, byte-exact.
    private static final String SUBSCRIBER_DISCRETE_EPL =
            "@name('s0') select irstream theString,intPrimitive from SupportBean#time_batch(1) " +
            "for discrete_delivery";
    private static final String SUBSCRIBER_GROUPED_EPL =
            "@name('s0') select irstream theString,intPrimitive from SupportBean#time_batch(1) " +
            "for grouped_delivery(intPrimitive)";

    // EPLOtherDiscreteDelivery (ordinal 2) EPLs, byte-exact including the
    // double space after SELECT * in the ObjectEvent statement.
    private static final String DISCRETE_EPL =
            "@name('s0') select * from SupportBean#time_batch(1) for discrete_delivery";
    private static final String DISCRETE_EMPTY_EPL =
            "@name('s0') SELECT *  FROM ObjectEvent OUTPUT ALL EVERY 1 seconds for discrete_delivery";

    // EPLOtherGroupDelivery (ordinal 3) EPLs, byte-exact including the space
    // before the grouped_delivery parenthesis in the first two statements.
    private static final String GROUPED_EPL =
            "@name('s0') select * from SupportBean#time_batch(1) for grouped_delivery (intPrimitive)";
    private static final String GROUPED_SORTED_EPL =
            "@name('s0') select * from SupportBean#time_batch(1) order by intPrimitive desc " +
            "for grouped_delivery (intPrimitive)";
    private static final String GROUPED_MULTIKEY_EPL =
            "@name('s0') select theString, doubleBoxed, enumValue from SupportBean#time_batch(1) " +
            "order by theString, doubleBoxed, enumValue for grouped_delivery(doubleBoxed, enumValue)";

    // Ordinals 4/5 two-statement modules, byte-exact including the newline.
    private static final String ARRAY_KEY_EPL =
            "create context MyContext start @now end after 1 second;\n" +
            "@name('s0') context MyContext select * from SupportEventWithManyArray#keepall " +
            "output snapshot when terminated for grouped_delivery (intOne)";
    private static final String TWO_FIELD_KEY_EPL =
            "create context MyContext start @now end after 1 second;\n" +
            "@name('s0') context MyContext select * from SupportBean#keepall " +
            "output snapshot when terminated for grouped_delivery (intPrimitive, longPrimitive)";

    private EPLOtherForGroupDeliveryScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: EPLOtherForGroupDeliveryScenarioOracle <scenario.json>");
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
            runCase(steps, CASES[index], RUNTIME_IDS[index], index, records);
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, String caseName, String runtimeId,
                                int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getCompiler().getByteCode().setAllowSubscriber(true);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType("SupportBean", LocalSupportBean.class);
        configuration.getCommon().addEventType("ObjectEvent", Object.class);
        Map<String, Object> manyArrayType = new HashMap<>();
        manyArrayType.put("id", String.class);
        manyArrayType.put("intOne", int[].class);
        configuration.getCommon().addEventType("SupportEventWithManyArray", manyArrayType);

        String runtimeURI = "parity-" + ID + "-" + runtimeId;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            long[] sequence = {0};
            TraceWriter writer = new TraceWriter(records, caseName, runtime, sequence);
            boolean active = false;
            int deployIndex = 0;
            int buildErrorIndex = 0;
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
                if ("deploy".equals(operation)) {
                    String statement = step.getString("statement", "");
                    if (!"s0".equals(statement)) {
                        throw new IllegalArgumentException("unexpected deploy statement " + statement);
                    }
                    int phase = deployIndex;
                    deployIndex++;
                    String epl = deployEpl(caseName, phase);
                    if (!epl.equals(step.getString("epl", ""))) {
                        throw new IllegalArgumentException("deploy epl is not pinned for case "
                                + caseName + " phase " + phase);
                    }
                    EPCompiled compiled = compile(configuration, caseName, phase, epl);
                    EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                            new DeploymentOptions().setDeploymentId(ID + "-" + caseIndex + "-" + deployIndex));
                    EPStatement s0 = findStatement(deployment);
                    if (s0 == null) {
                        throw new IllegalStateException("deployment has no statement named s0");
                    }
                    if ("subscriber-only".equals(caseName)) {
                        s0.setSubscriber(new TraceSubscriber(records, caseName, runtime,
                                s0.getName(), SUBSCRIBER_COLUMNS, sequence));
                    } else {
                        s0.addListener(writer);
                    }
                } else if ("undeploy-all".equals(operation)) {
                    runtime.getDeploymentService().undeployAll();
                } else if ("send".equals(operation)) {
                    sendEvent(runtime, step);
                } else if ("advance-time".equals(operation)) {
                    runtime.getEventService().advanceTime(
                            Instant.parse(step.getString("at", "")).toEpochMilli());
                } else if ("build-error".equals(operation)) {
                    String label = step.getString("statement", "");
                    if (buildErrorIndex >= INVALID_STATEMENTS.length
                            || !INVALID_STATEMENTS[buildErrorIndex].equals(label)
                            || !INVALID_EPLS[buildErrorIndex].equals(step.getString("epl", ""))
                            || !INVALID_PREFIXES[buildErrorIndex].equals(step.getString("expectError", ""))) {
                        throw new IllegalArgumentException("build-error probe is not pinned: " + label);
                    }
                    String epl = INVALID_EPLS[buildErrorIndex];
                    String prefix = INVALID_PREFIXES[buildErrorIndex];
                    buildErrorIndex++;
                    tryInvalidCompile(configuration, epl, prefix);
                    records.add(new JsonObject()
                            .add("case", caseName)
                            .add("operation", "compile-rejected")
                            .add("statement", label)
                            .add("sequence", ++sequence[0])
                            .add("time", Instant.ofEpochMilli(
                                    runtime.getEventService().getCurrentTime()).toString())
                            .add("errorPrefix", prefix));
                } else {
                    throw new IllegalArgumentException("unsupported step op " + operation);
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static String deployEpl(String caseName, int deployIndex) {
        switch (caseName) {
            case "subscriber-only":
                if (deployIndex == 0) {
                    return SUBSCRIBER_DISCRETE_EPL;
                }
                if (deployIndex == 1) {
                    return SUBSCRIBER_GROUPED_EPL;
                }
                break;
            case "discrete-delivery":
                if (deployIndex == 0) {
                    return DISCRETE_EPL;
                }
                if (deployIndex == 1) {
                    return DISCRETE_EMPTY_EPL;
                }
                break;
            case "group-delivery":
                if (deployIndex == 0) {
                    return GROUPED_EPL;
                }
                if (deployIndex == 1) {
                    return GROUPED_SORTED_EPL;
                }
                if (deployIndex == 2 || deployIndex == 3) {
                    // Phase 3 is the suite's SODA leg: eplToModel of the same
                    // text, asserted to round-trip, then compileDeploy(model).
                    return GROUPED_MULTIKEY_EPL;
                }
                break;
            case "group-delivery-array-key":
                if (deployIndex == 0) {
                    return ARRAY_KEY_EPL;
                }
                break;
            case "group-delivery-two-field-key":
                if (deployIndex == 0) {
                    return TWO_FIELD_KEY_EPL;
                }
                break;
            default:
                throw new IllegalArgumentException("unexpected case " + caseName);
        }
        throw new IllegalArgumentException("unexpected deploy index " + deployIndex
                + " for case " + caseName);
    }

    /** EPL pinned in the cases[] metadata: the first deploy text per case. */
    private static String caseEpl(String caseName) {
        if ("invalid".equals(caseName)) {
            return INVALID_EPLS[0];
        }
        return deployEpl(caseName, 0);
    }

    private static EPCompiled compile(Configuration configuration, String caseName,
                                      int deployIndex, String epl) throws EPCompileException {
        if ("group-delivery".equals(caseName) && deployIndex == 3) {
            // SODA phase: env.eplToModel(stmtText) asserts model.toEPL() equals
            // the text, then compileDeploy(model) deploys the object model.
            EPStatementObjectModel model =
                    EPCompilerProvider.getCompiler().eplToModel(epl, configuration);
            model = SerializableObjectCopier.copyMayFail(model);
            if (!epl.equals(model.toEPL())) {
                throw new IllegalStateException("SODA round-trip drift: " + model.toEPL());
            }
            Module module = new Module();
            module.getItems().add(new ModuleItem(model));
            module.setModuleText(model.toEPL());
            return EPCompilerProvider.getCompiler().compile(module,
                    new CompilerArguments(configuration));
        }
        return EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(configuration));
    }

    /**
     * Mirrors env.tryInvalidCompile(epl, message): rejects unless the
     * EPCompileException message starts with {@code prefix} (exact
     * assertMessage semantics; the tail may carry the rendered module text).
     */
    private static void tryInvalidCompile(Configuration configuration, String epl, String prefix) {
        try {
            EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(configuration));
        } catch (EPCompileException ex) {
            String message = ex.getMessage();
            if (message == null || !message.startsWith(prefix)) {
                throw new IllegalStateException("expected error prefix <" + prefix + "> for <"
                        + epl + "> got <" + message + ">", ex);
            }
            return;
        }
        throw new IllegalStateException("expected compile failure for <" + epl + ">");
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        for (EPStatement candidate : deployment.getStatements()) {
            if ("s0".equals(candidate.getName())) {
                return candidate;
            }
        }
        return null;
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (eventType) {
            case "SupportBean": {
                LocalSupportBean bean = new LocalSupportBean();
                bean.setTheString(payload.getString("theString", ""));
                bean.setIntPrimitive(payload.getInt("intPrimitive", 0));
                bean.setLongPrimitive(longField(payload, "longPrimitive"));
                bean.setDoubleBoxed(doubleField(payload, "doubleBoxed"));
                bean.setEnumValue(supportEnum(payload.getString("enumValue", null)));
                runtime.getEventService().sendEventBean(bean, eventType);
                break;
            }
            case "ObjectEvent":
                runtime.getEventService().sendEventBean(new Object(), eventType);
                break;
            case "SupportEventWithManyArray": {
                Map<String, Object> event = new HashMap<>();
                event.put("id", payload.getString("id", ""));
                JsonValue one = payload.get("intOne");
                event.put("intOne", (one == null || !one.isArray()) ? null : intArray(one));
                runtime.getEventService().sendEventMap(event, eventType);
                break;
            }
            default:
                throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
    }

    private static long longField(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        return value == null ? 0L : value.asLong();
    }

    private static Double doubleField(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        return value == null ? null : value.asDouble();
    }

    private static SupportEnum supportEnum(String name) {
        return name == null ? null : SupportEnum.valueOf(name);
    }

    private static int[] intArray(JsonValue value) {
        JsonArray array = value.asArray();
        int[] values = new int[array.size()];
        for (int index = 0; index < array.size(); index++) {
            values[index] = array.get(index).asInt();
        }
        return values;
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags",
                "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), JAVA_RUNTIMES, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), JAVA_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), JAVA_STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), JAVA_FLAGS, "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly six cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !caseEpl(CASES[index]).equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
        }
    }

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (com.espertech.esper.common.client.json.minimaljson.Member member : value.asObject()) {
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
        long value = longNumber(object, name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        return (int) value;
    }

    private static long longNumber(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (!(value instanceof JsonNumber)) {
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

    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value instanceof int[] array) {
            JsonArray output = new JsonArray();
            for (int element : array) {
                output.add(element);
            }
            return output;
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
        if (value instanceof Character character) {
            return Json.value(String.valueOf(character));
        }
        return Json.value(String.valueOf(value));
    }

    private static JsonArray rowsOf(EventBean[] events) {
        JsonArray output = new JsonArray();
        if (events == null) {
            return output;
        }
        for (EventBean event : events) {
            JsonObject fields = new JsonObject();
            String[] names = event.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            for (String name : names) {
                fields.add(name, normalize(event.get(name)));
            }
            output.add(new JsonObject().add("kind", "row").add("fields", fields));
        }
        return output;
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPRuntime runtime;
        private final long[] sequence;

        private TraceWriter(JsonArray records, String caseName, EPRuntime runtime, long[] sequence) {
            this.records = records;
            this.caseName = caseName;
            this.runtime = runtime;
            this.sequence = sequence;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement,
                           EPRuntime ignoredRuntime) {
            if (newEvents == null && oldEvents == null) {
                return;
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence[0])
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rowsOf(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", rowsOf(oldEvents));
            }
            records.add(record);
        }
    }

    /**
     * SupportSubscriberMRD-shaped subscriber: Esper binds the reflective
     * update(Object[][], Object[][]) signature for multi-row delivery.  One
     * call per delivered row under discrete delivery, one call per group under
     * grouped delivery; records carry operation "listener" per the frozen
     * contract.  Unlike SupportSubscriberMRD this records the true remove
     * stream (the suite's helper mirrors insert rows into its remove list).
     */
    public static final class TraceSubscriber {
        private final JsonArray records;
        private final String caseName;
        private final EPRuntime runtime;
        private final String statementName;
        private final String[] propertyNames;
        private final int[] sortedIndex;
        private final long[] sequence;

        private TraceSubscriber(JsonArray records, String caseName, EPRuntime runtime,
                                String statementName, String[] propertyNames, long[] sequence) {
            this.records = records;
            this.caseName = caseName;
            this.runtime = runtime;
            this.statementName = statementName;
            this.propertyNames = propertyNames.clone();
            Integer[] order = new Integer[this.propertyNames.length];
            for (int index = 0; index < order.length; index++) {
                order[index] = index;
            }
            Arrays.sort(order, Comparator.comparing(index -> this.propertyNames[index]));
            this.sortedIndex = new int[order.length];
            for (int index = 0; index < order.length; index++) {
                this.sortedIndex[index] = order[index];
            }
            this.sequence = sequence;
        }

        public void update(Object[][] insertStream, Object[][] removeStream) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statementName)
                    .add("sequence", ++sequence[0])
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(insertStream);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            JsonArray oldRows = rows(removeStream);
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        }

        private JsonArray rows(Object[][] stream) {
            JsonArray output = new JsonArray();
            if (stream == null) {
                return output;
            }
            for (Object[] row : stream) {
                JsonObject fields = new JsonObject();
                for (int index : sortedIndex) {
                    fields.add(propertyNames[index], normalize(row[index]));
                }
                output.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return output;
        }
    }

    /** Local mirror of the pinned SupportBean members the executions use. */
    public static class LocalSupportBean {
        private String theString;
        private int intPrimitive;
        private long longPrimitive;
        private Double doubleBoxed;
        private SupportEnum enumValue;

        public String getTheString() {
            return theString;
        }

        public void setTheString(String theString) {
            this.theString = theString;
        }

        public int getIntPrimitive() {
            return intPrimitive;
        }

        public void setIntPrimitive(int intPrimitive) {
            this.intPrimitive = intPrimitive;
        }

        public long getLongPrimitive() {
            return longPrimitive;
        }

        public void setLongPrimitive(long longPrimitive) {
            this.longPrimitive = longPrimitive;
        }

        public Double getDoubleBoxed() {
            return doubleBoxed;
        }

        public void setDoubleBoxed(Double doubleBoxed) {
            this.doubleBoxed = doubleBoxed;
        }

        public SupportEnum getEnumValue() {
            return enumValue;
        }

        public void setEnumValue(SupportEnum enumValue) {
            this.enumValue = enumValue;
        }
    }
}
