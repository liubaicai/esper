import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.compiler.client.CompilerArguments;
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
import java.util.HashSet;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the Draft 4.443 'epl-other-pattern-queries'
 * unit: EPLOtherPatternQueries ordinals 0/1/2 (EPLOtherWhereOM,
 * EPLOtherWhereCompile, EPLOtherWhere) and ordinal 3 (EPLOtherAggregation).
 * Ordinals 0 and 1 are SODA-OM compile-path variants of ordinal 2's
 * semantics: the object-model and eplToModel executions deploy the same
 * every-or pattern with the same where predicate and assert the same
 * listener output, so one scenario pass of ordinal 2's byte-exact EPL covers
 * all three executions.  Each scenario case replays inside its own runtime.
 *
 * Two shape notes pin the trace.  First, the or-pattern binds exactly one
 * tag per match, so the unbound tag's column is null: pattern-where emits
 * {idS0,idS1} with one side null and pattern-aggregation's
 * sum(s0.id + s1.id) stays null because the addition sees a null operand.
 * Second, the where clause suppresses non-matching sends (S0 id 101 and S1
 * id 1), so those sends emit no trace record.
 */
public final class EPLOtherPatternQueriesScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "epl-other-pattern-queries";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherPatternQueries.java";
    private static final String DESCRIPTION =
            "EPLOtherPatternQueries ordinals 0/1/2 (pattern where over every-or tags; ords 0/1 are SODA-OM compile-path variants of ord 2) and ordinal 3 (sum aggregation over every-or tags)";

    private static final String[] CASES = {"pattern-where", "pattern-aggregation"};
    private static final int[] ORDINALS = {2, 3};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-5b43884f1025d0953a3c",
            "java-runtime-ba57386eec69d7a3e1f4"
    };
    private static final String[] EXECUTIONS = {
            "EPLOtherWhere",
            "EPLOtherAggregation"
    };
    private static final String[] OBSERVATIONS = {
            "listener; Java ordinals 0/1 are SODA-OM compile-path variants of ordinal 2, one scenario pass covers all three",
            "listener"
    };

    private static final String[] JAVA_RUNTIMES = {
            "java-runtime-00e2f9b1ff1f7568065e",
            "java-runtime-2d9518b6894d4c46972d",
            "java-runtime-5b43884f1025d0953a3c",
            "java-runtime-ba57386eec69d7a3e1f4"
    };
    private static final String[] JAVA_NAMES = {
            "EPLOtherWhereOM",
            "EPLOtherWhereCompile",
            "EPLOtherWhere",
            "EPLOtherAggregation"
    };
    private static final String[] JAVA_STATIC_IDS = {
            "java-08ff402bdcbb7f821aa4",
            "java-08ff402bdcbb7f821aa4",
            "java-08ff402bdcbb7f821aa4",
            "java-08ff402bdcbb7f821aa4"
    };

    // EPLOtherWhere (ordinal 2) EPL, byte-exact from EPLOtherPatternQueries:
    // the parenthesized where form; ordinals 0/1 compile the unparenthesized
    // equivalent through the SODA object model.
    private static final String PATTERN_WHERE_EPL =
            "@name('s0') select s0.id as idS0, s1.id as idS1 " +
            "from pattern [every s0=SupportBean_S0" +
            " or every s1=SupportBean_S1] " +
            "where (s0.id is not null and s0.id < 100) or (s1.id is not null and s1.id >= 100)";

    // EPLOtherAggregation (ordinal 3) EPL, byte-exact.
    private static final String PATTERN_AGGREGATION_EPL =
            "@name('s0') select sum(s0.id) as sumS0, sum(s1.id) as sumS1, sum(s0.id + s1.id) as sumS0S1 " +
            "from pattern [every s0=SupportBean_S0" +
            " or every s1=SupportBean_S1]";

    private EPLOtherPatternQueriesScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: EPLOtherPatternQueriesScenarioOracle <scenario.json>");
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
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCommon().addEventType(SupportBean_S1.class);

        String runtimeURI = "parity-" + ID + "-" + runtimeId;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            boolean active = false;
            int deployIndex = 0;
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
                    String epl = s0Epl(caseName, phase);
                    EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                            new CompilerArguments(configuration));
                    EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                            new DeploymentOptions().setDeploymentId(ID + "-" + caseIndex + "-" + deployIndex));
                    EPStatement s0 = findStatement(deployment);
                    if (s0 == null) {
                        throw new IllegalStateException("deployment has no statement named s0");
                    }
                    s0.addListener(writer);
                } else if ("undeploy-all".equals(operation)) {
                    runtime.getDeploymentService().undeployAll();
                } else if ("send".equals(operation)) {
                    sendEvent(runtime, step);
                } else {
                    throw new IllegalArgumentException("unsupported step op " + operation);
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static String s0Epl(String caseName, int deployIndex) {
        if (deployIndex != 0) {
            throw new IllegalArgumentException("unexpected deploy index " + deployIndex);
        }
        switch (caseName) {
            case "pattern-where":
                return PATTERN_WHERE_EPL;
            case "pattern-aggregation":
                return PATTERN_AGGREGATION_EPL;
            default:
                throw new IllegalArgumentException("unexpected case " + caseName);
        }
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
        if ("SupportBean_S0".equals(eventType)) {
            runtime.getEventService().sendEventBean(
                    new SupportBean_S0(payload.getInt("id", 0)), "SupportBean_S0");
        } else if ("SupportBean_S1".equals(eventType)) {
            runtime.getEventService().sendEventBean(
                    new SupportBean_S1(payload.getInt("id", 0)), "SupportBean_S1");
        } else {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
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
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly two cases");
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
                    || !s0Epl(CASES[index], 0).equals(string(definition, "epl"))) {
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

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.runtime = runtime;
        }

        private long nextSequence() {
            return ++sequence;
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
                    .add("sequence", nextSequence())
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", rows(oldEvents));
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
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
    }
}
