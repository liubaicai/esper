import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
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
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.Collection;
import java.util.HashSet;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the Draft 4.438 'resultset-aggregate-remainder'
 * unit: ResultSetAggregateFiltered ordinal 3 (ResultSetAggregateFirstLastEver),
 * ResultSetAggregateSortedMinMaxBy ordinal 5 (ResultSetAggregateMultipleCriteria)
 * and ResultSetAggregateFilterNamedParameter ordinal 19
 * (ResultSetAggregateAuditAndReuse).  Each Java execution is one scenario case
 * inside its own runtime; multiple-criteria runs two sequential
 * deploy/undeploy cycles (phase A sorted multi-criteria, phase B
 * minby/maxby/minbyever/maxbyever multi-key) matching the single Java
 * execution.  ResultSetAggregateFirstLastEver runs its assertion twice under
 * soda=true/false; the scenario replays the assertion once because soda only
 * changes compile-time serialization, not listener output.
 *
 * Two normalization rules pin the language-neutral trace shape.  First,
 * Esper's sorted()/window(*) accessors return the stream's underlying bean
 * array (SupportBean[]), so bean-array columns are projected to
 * {"kind":"row","fields":{theString,intPrimitive}} element maps; the Go runner
 * emits the identical two-field projection for its []esper.Event columns.
 * Second, Esper's access-aggregate getValue returns null when the retained
 * state is empty (the audit-reuse window(*, filter:intPrimitive=1) columns see
 * zero matching events), so a null value on an array-typed column normalizes
 * to []; the Go runner applies the same rule to null slice-typed fields.
 */
public final class ResultSetAggregateRemainderScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-remainder";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFiltered.java";
    private static final String JAVA_SOURCE2 =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateSortedMinMaxBy.java";
    private static final String JAVA_SOURCE3 =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFilterNamedParameter.java";
    private static final String DESCRIPTION =
            "ResultSetAggregateFiltered ordinal 3 (firstever/lastever/countever positional filters), ResultSetAggregateSortedMinMaxBy ordinal 5 (multi-criteria sorted plus multi-key minby/maxby/minbyever/maxbyever) and ResultSetAggregateFilterNamedParameter ordinal 19 (audit-and-reuse of identical filtered aggregates)";

    private static final String[] CASES = {
            "first-last-ever", "multiple-criteria", "audit-reuse"};
    private static final int[] ORDINALS = {3, 5, 19};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-47dee40e6fe320005f49",
            "java-runtime-dd3414775a8421e08a52",
            "java-runtime-c95393b13d253135c833"
    };
    private static final String[] EXECUTIONS = {
            "ResultSetAggregateFirstLastEver",
            "ResultSetAggregateMultipleCriteria",
            "ResultSetAggregateAuditAndReuse"
    };
    private static final String[] STATIC_IDS = {
            "java-3d2c17d4a26d35a0019b",
            "java-553516b9d01c12a13172",
            "java-0c29efb6d43971aba5c4"
    };
    private static final String[] OBSERVATIONS = {
            "listener; Java runs the assertion twice (soda=true/false), one scenario pass covers both",
            "listener",
            "listener"
    };

    // ResultSetAggregateFirstLastEver EPL, byte-exact from
    // ResultSetAggregateFiltered (single space after each "as cN,").
    private static final String FIRST_LAST_EVER_EPL =
            "@name('s0') select " +
            "firstever(intBoxed,boolPrimitive) as c1, " +
            "lastever(intBoxed,boolPrimitive) as c2, " +
            "countever(*,boolPrimitive) as c3 " +
            "from SupportBean#length(3)";

    // ResultSetAggregateMultipleCriteria phase A, byte-exact from
    // ResultSetAggregateSortedMinMaxBy: no space after the "as cN," commas.
    private static final String MULTIPLE_CRITERIA_EPL_A =
            "@name('s0') select " +
            "sorted(theString desc, intPrimitive desc) as c0," +
            "sorted(theString, intPrimitive) as c1," +
            "sorted(theString asc, intPrimitive asc) as c2," +
            "sorted(theString desc, intPrimitive asc) as c3 " +
            "from SupportBean#keepall";

    // ResultSetAggregateMultipleCriteria phase B, byte-exact: no space after
    // the "as cN," commas.
    private static final String MULTIPLE_CRITERIA_EPL_B =
            "@name('s0') select " +
            "maxbyever(intPrimitive, theString).longPrimitive as c0," +
            "minbyever(intPrimitive, theString).longPrimitive as c1," +
            "maxbyever(theString, intPrimitive).longPrimitive as c2," +
            "minbyever(theString, intPrimitive).longPrimitive as c3," +
            "maxby(intPrimitive, theString).longPrimitive as c4," +
            "minby(intPrimitive, theString).longPrimitive as c5," +
            "maxby(theString, intPrimitive).longPrimitive as c6," +
            "minby(theString, intPrimitive).longPrimitive as c7 " +
            "from SupportBean#keepall";

    // ResultSetAggregateAuditAndReuse EPL, byte-exact from
    // ResultSetAggregateFilterNamedParameter including the double space
    // before "from".
    private static final String AUDIT_REUSE_EPL =
            "@name('s0') select " +
            "sum(intPrimitive, filter:intPrimitive=1) as c0, sum(intPrimitive, filter:intPrimitive=1) as c1, " +
            "window(*, filter:intPrimitive=1) as c2, window(*, filter:intPrimitive=1) as c3 " +
            " from SupportBean#length(3)";

    private ResultSetAggregateRemainderScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetAggregateRemainderScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBean.class);

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
        switch (caseName) {
            case "first-last-ever":
                if (deployIndex != 0) {
                    throw new IllegalArgumentException("unexpected deploy index " + deployIndex);
                }
                return FIRST_LAST_EVER_EPL;
            case "multiple-criteria":
                if (deployIndex == 0) {
                    return MULTIPLE_CRITERIA_EPL_A;
                }
                if (deployIndex == 1) {
                    return MULTIPLE_CRITERIA_EPL_B;
                }
                throw new IllegalArgumentException("unexpected deploy index " + deployIndex);
            case "audit-reuse":
                if (deployIndex != 0) {
                    throw new IllegalArgumentException("unexpected deploy index " + deployIndex);
                }
                return AUDIT_REUSE_EPL;
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
        if ("SupportBean".equals(eventType)) {
            SupportBean bean = new SupportBean();
            bean.setTheString(payload.getString("theString", null));
            bean.setIntPrimitive(payload.getInt("intPrimitive", 0));
            bean.setLongPrimitive(payload.getLong("longPrimitive", 0));
            bean.setBoolPrimitive(payload.getBoolean("boolPrimitive", false));
            JsonValue intBoxed = payload.get("intBoxed");
            if (intBoxed != null && intBoxed.isNumber()) {
                bean.setIntBoxed(intBoxed.asInt());
            }
            runtime.getEventService().sendEventBean(bean, "SupportBean");
        } else {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaSource2", "javaSource3", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags",
                "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))
                || !JAVA_SOURCE2.equals(string(scenario, "javaSource2"))
                || !JAVA_SOURCE3.equals(string(scenario, "javaSource3"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTIONS, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly three cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            boolean twoPhase = "multiple-criteria".equals(CASES[index]);
            if (twoPhase) {
                requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                        "observation", "epl", "eplB");
            } else {
                requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                        "observation", "epl");
            }
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !s0Epl(CASES[index], 0).equals(string(definition, "epl"))
                    || (twoPhase && !s0Epl(CASES[index], 1).equals(string(definition, "eplB")))) {
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
        if (value instanceof EventBean[] events) {
            JsonArray array = new JsonArray();
            for (EventBean event : events) {
                array.add(normalize(event));
            }
            return array;
        }
        if (value instanceof EventBean event) {
            JsonObject fields = new JsonObject();
            String[] names = event.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            for (String name : names) {
                fields.add(name, normalize(event.get(name)));
            }
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof Object[] objects) {
            JsonArray array = new JsonArray();
            for (Object object : objects) {
                array.add(normalize(object));
            }
            return array;
        }
        if (value instanceof Collection<?> collection) {
            JsonArray array = new JsonArray();
            for (Object item : collection) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof SupportBean bean) {
            // sorted()/window(*) columns carry the stream's underlying beans;
            // project the two sort-key fields so the Go runner can compare
            // element-wise against its own two-field event projection.
            JsonObject fields = new JsonObject()
                    .add("theString", normalize(bean.getTheString()))
                    .add("intPrimitive", normalize(bean.getIntPrimitive()));
            return new JsonObject().add("kind", "row").add("fields", fields);
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
                    Object value = event.get(name);
                    if (value == null && isArrayColumn(event, name)) {
                        // Access aggregates (sorted, window) return null for an
                        // empty retained state; the trace pins the empty
                        // collection shape the Go runner emits for the same
                        // slice-typed column.
                        fields.add(name, new JsonArray());
                        continue;
                    }
                    fields.add(name, normalize(value));
                }
                output.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return output;
        }

        private boolean isArrayColumn(EventBean event, String name) {
            Class<?> propertyType = event.getEventType().getPropertyType(name);
            return propertyType != null && (propertyType.isArray() || Collection.class.isAssignableFrom(propertyType));
        }
    }
}
