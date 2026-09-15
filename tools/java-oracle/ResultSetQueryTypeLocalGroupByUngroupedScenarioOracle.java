import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventPropertyDescriptor;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.util.StatementProperty;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
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
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for ResultSetQueryTypeLocalGroupBy ordinals 3-6,
 * the ungrouped local-group executions: ResultSetLocalUngroupedAggIterator
 * (iterator snapshots), ResultSetLocalUngroupedParenSODA with soda=false and
 * soda=true (per-send listener rows, the second built through the SODA model)
 * and ResultSetLocalUngroupedColNameRendering (deployment acknowledgement plus
 * the rendered auto-named output columns).  Each case replays in its own
 * runtime; the iterator case attaches no listener because the Java execution's
 * listener is never asserted, while the two statement-metadata records carry no
 * time field, matching the shared trace shape for "deployed" and "types".
 */
public final class ResultSetQueryTypeLocalGroupByUngroupedScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-querytype-local-group-ungrouped";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java";
    private static final String DESCRIPTION =
            "ResultSetQueryTypeLocalGroupBy ordinals 3-6: ungrouped local-group aggregates across"
                    + " iterator, listener and statement-metadata observations.";
    private static final String[] CASES = {
            "ungrouped-agg-iterator", "ungrouped-paren-soda-text", "ungrouped-paren-soda-model",
            "ungrouped-colname-rendering"
    };
    private static final int[] ORDINALS = {3, 4, 5, 6};
    private static final String[] RUNTIMES = {
            "java-runtime-2116fa46dfb43cd6ec83", "java-runtime-1309ac2ab21826013abf",
            "java-runtime-e9dbed674201a7324216", "java-runtime-748fbcf754462901d0e0"
    };
    private static final String[] NAMES = {
            "ResultSetLocalUngroupedAggIterator", "ResultSetLocalUngroupedParenSODA{soda=false}",
            "ResultSetLocalUngroupedParenSODA{soda=true}", "ResultSetLocalUngroupedColNameRendering"
    };
    private static final String[] OBSERVATIONS = {"iterator", "listener", "listener", "statement-metadata"};
    private static final int[] ITERATOR_SNAPSHOTS = {3, 0, 0, 0};
    private static final int[] CASE_RECORDS = {3, 4, 4, 2};
    private static final String[] STATIC_IDS = {
            "java-15d21d1c89c691d1be52", "java-dac296ac610fa9db8d71", "java-4d4e4747a49e26787a4d"
    };
    private static final String EPL_ITERATOR = "@name('s0') select intPrimitive as c0, sum(intPrimitive,"
            + " group_by:()) as sum0, sum(intPrimitive, group_by:(theString)) as sum1 from SupportBean#keepall";
    private static final String EPL_PAREN = "@name('s0') select longPrimitive, sum(longPrimitive) as c0,"
            + " sum(group_by:(),longPrimitive) as c1, sum(longPrimitive,group_by:()) as c2,"
            + " sum(longPrimitive,group_by:theString) as c3,"
            + " sum(longPrimitive,group_by:(theString,intPrimitive)) as c4 from SupportBean";
    private static final String EPL_COLNAME = "@name('s0') select count(*, group_by:(theString, intPrimitive)),"
            + " count(group_by:theString, *) from SupportBean";
    private static final String[] STEP_OPS = {
            "case", "send", "snapshot", "send", "snapshot", "send", "snapshot",
            "case", "send", "send", "send", "send",
            "case", "send", "send", "send", "send",
            "case", "deployed", "types"
    };
    private static final String[] SEND_STRINGS = {
            "E1", "E2", "E1", "E1", "E1", "E2", "E2", "E1", "E1", "E2", "E2"
    };
    private static final int[] SEND_INTS = {10, 20, 30, 1, 2, 1, 2, 1, 2, 1, 2};
    private static final long[] SEND_LONGS = {0, 0, 0, 10, 11, 12, 13, 10, 11, 12, 13};
    private static final String[] SNAPSHOT_FIELDS = {"c0", "sum0", "sum1"};
    private static final long[][][] SNAPSHOT_ROWS = {
            {{10, 10, 10}},
            {{10, 30, 10}, {20, 30, 20}},
            {{10, 60, 40}, {20, 60, 20}, {30, 60, 40}},
    };
    private static final String[] LISTENER_FIELDS = {"c0", "c1", "c2", "c3", "c4", "longPrimitive"};
    private static final long[][] LISTENER_ROWS = {
            {10, 10, 10, 10, 10, 10},
            {21, 21, 21, 21, 11, 11},
            {33, 33, 33, 12, 12, 12},
            {46, 46, 46, 25, 13, 13},
    };
    private static final String[] COLNAME_FIELDS = {
            "count(*,group_by:(theString,intPrimitive))", "count(group_by:theString,*)"
    };

    private ResultSetQueryTypeLocalGroupByUngroupedScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetQueryTypeLocalGroupByUngroupedScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        JsonArray steps = scenario.get("steps").asArray();
        JsonArray records = new JsonArray();
        for (int index = 0; index < CASES.length; index++) {
            runCase(steps, records, index);
        }
        if (records.size() != 13) {
            throw new IllegalStateException("expected thirteen trace records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, JsonArray records, int index) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + RUNTIMES[index], configuration);
        runtime.getEventService().advanceTime(0L);
        int before = records.size();
        try {
            EPStatement statement = deploy(runtime, configuration, index);
            ListenerWriter writer = null;
            if ("listener".equals(OBSERVATIONS[index])) {
                writer = new ListenerWriter(records, statement, runtime, CASES[index]);
                statement.addListener(writer);
            }
            boolean active = false;
            int markers = 0;
            int snapshots = 0;
            int deployed = 0;
            int types = 0;
            for (int stepIndex = 0; stepIndex < steps.size(); stepIndex++) {
                JsonObject step = object(steps.get(stepIndex), "step " + stepIndex);
                String operation = string(step, "op");
                if ("case".equals(operation)) {
                    if (CASES[index].equals(string(step, "case"))) {
                        if (++markers != 1) {
                            throw new IllegalStateException("duplicate active case marker");
                        }
                        active = true;
                    } else if (active) {
                        active = false;
                    }
                    continue;
                }
                if (!active) {
                    continue;
                }
                if ("send".equals(operation)) {
                    send(runtime, step, stepIndex);
                } else if ("snapshot".equals(operation)) {
                    requireFields(step, "op", "statement", "mode");
                    if (!"s0".equals(string(step, "statement")) || !"any".equals(string(step, "mode"))) {
                        throw new IllegalArgumentException("snapshot metadata is not pinned at step " + stepIndex);
                    }
                    if (snapshots >= SNAPSHOT_ROWS.length) {
                        throw new IllegalArgumentException("unexpected snapshot at step " + stepIndex);
                    }
                    JsonArray rows = snapshotRows(statement, snapshots);
                    snapshots++;
                    records.add(new JsonObject().add("case", CASES[index]).add("operation", "snapshot")
                            .add("statement", statement.getName()).add("sequence", snapshots)
                            .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
                            .add("new", rows));
                } else if ("deployed".equals(operation)) {
                    requireFields(step, "op", "statement");
                    if (!"s0".equals(string(step, "statement"))) {
                        throw new IllegalArgumentException("deployed metadata is not pinned at step " + stepIndex);
                    }
                    deployed++;
                    records.add(new JsonObject().add("case", CASES[index]).add("operation", "deployed")
                            .add("statement", statement.getName()).add("sequence", 0));
                } else if ("types".equals(operation)) {
                    requireFields(step, "op", "statement");
                    if (!"s0".equals(string(step, "statement"))) {
                        throw new IllegalArgumentException("types metadata is not pinned at step " + stepIndex);
                    }
                    types++;
                    records.add(new JsonObject().add("case", CASES[index]).add("operation", "types")
                            .add("statement", statement.getName()).add("sequence", 0)
                            .add("value", outputTypes(statement)));
                } else {
                    throw new IllegalArgumentException("unsupported operation at step " + stepIndex + ": " + operation);
                }
            }
            if (markers != 1) {
                throw new IllegalStateException("case marker count mismatch for " + CASES[index]);
            }
            if (writer != null && writer.sequence != LISTENER_ROWS.length) {
                throw new IllegalStateException("expected four listener records, got " + writer.sequence);
            }
            if (snapshots != ITERATOR_SNAPSHOTS[index]) {
                throw new IllegalStateException("expected " + ITERATOR_SNAPSHOTS[index] + " snapshot records, got "
                        + snapshots);
            }
            if ((deployed == 1) != ("statement-metadata".equals(OBSERVATIONS[index]))
                    || (types == 1) != ("statement-metadata".equals(OBSERVATIONS[index]))) {
                throw new IllegalStateException("statement-metadata step count mismatch for " + CASES[index]);
            }
            if (records.size() - before != CASE_RECORDS[index]) {
                throw new IllegalStateException("expected " + CASE_RECORDS[index] + " records for " + CASES[index]
                        + ", got " + (records.size() - before));
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static EPStatement deploy(EPRuntime runtime, Configuration configuration, int index) throws Exception {
        DeploymentOptions options = new DeploymentOptions().setDeploymentId(ID + "-" + CASES[index]);
        EPDeployment deployment;
        if (index == 2) {
            EPStatementObjectModel model = EPCompilerProvider.getCompiler().eplToModel(EPL_PAREN, configuration);
            if (!EPL_PAREN.trim().equals(model.toEPL())) {
                throw new IllegalStateException("SODA model does not round-trip the EPL text");
            }
            Module module = new Module();
            module.getItems().add(new ModuleItem(model));
            module.setModuleText(model.toEPL());
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(module,
                    new CompilerArguments(runtime.getRuntimePath()));
            deployment = runtime.getDeploymentService().deploy(compiled, options);
            EPStatement statement = findStatement(deployment);
            if (!EPL_PAREN.trim().equals(statement.getProperty(StatementProperty.EPL))) {
                throw new IllegalStateException("SODA statement EPL property is not pinned");
            }
            return statement;
        }
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl(index),
                new CompilerArguments(runtime.getRuntimePath()));
        deployment = runtime.getDeploymentService().deploy(compiled, options);
        return findStatement(deployment);
    }

    private static String epl(int index) {
        if (index == 0) {
            return EPL_ITERATOR;
        }
        if (index == 3) {
            return EPL_COLNAME;
        }
        return EPL_PAREN;
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        EPStatement[] statements = deployment.getStatements();
        if (statements == null || statements.length != 1 || !"s0".equals(statements[0].getName())) {
            throw new IllegalStateException("expected exactly one statement named s0");
        }
        return statements[0];
    }

    private static JsonArray snapshotRows(EPStatement statement, int snapshot) {
        List<EventBean> events = new ArrayList<>();
        for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
            events.add(iterator.next());
        }
        long[][] expected = SNAPSHOT_ROWS[snapshot];
        if (events.size() != expected.length) {
            throw new IllegalStateException("snapshot " + (snapshot + 1) + " has " + events.size()
                    + " iterator rows, want " + expected.length);
        }
        // The Java execution asserts the rows in any order; each expected row must
        // match its own iterator row while the trace keeps the iterator's order.
        boolean[] matched = new boolean[events.size()];
        for (long[] wanted : expected) {
            boolean found = false;
            for (int index = 0; index < events.size(); index++) {
                if (!matched[index] && matchesSnapshotRow(events.get(index), wanted, snapshot)) {
                    matched[index] = true;
                    found = true;
                    break;
                }
            }
            if (!found) {
                throw new IllegalStateException("snapshot " + (snapshot + 1) + " has no iterator row ["
                        + wanted[0] + ", " + wanted[1] + ", " + wanted[2] + "]");
            }
        }
        JsonArray rows = new JsonArray();
        for (EventBean event : events) {
            rows.add(row(event));
        }
        return rows;
    }

    private static boolean matchesSnapshotRow(EventBean event, long[] expected, int snapshot) {
        String[] names = event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        if (!Arrays.equals(names, SNAPSHOT_FIELDS)) {
            throw new IllegalStateException("snapshot result field metadata is not pinned: " + Arrays.toString(names));
        }
        Object c0 = event.get("c0");
        Object sum0 = event.get("sum0");
        Object sum1 = event.get("sum1");
        if (!(c0 instanceof Integer) || !(sum0 instanceof Integer) || !(sum1 instanceof Integer)) {
            throw new IllegalStateException("snapshot " + (snapshot + 1) + " iterator row is not boxed Integer: ["
                    + c0 + ", " + sum0 + ", " + sum1 + "]");
        }
        return ((Integer) c0).longValue() == expected[0] && ((Integer) sum0).longValue() == expected[1]
                && ((Integer) sum1).longValue() == expected[2];
    }

    private static JsonArray outputTypes(EPStatement statement) {
        String[] names = statement.getEventType().getPropertyNames();
        if (names.length != COLNAME_FIELDS.length
                || !COLNAME_FIELDS[0].equals(names[0]) || !COLNAME_FIELDS[1].equals(names[1])) {
            throw new IllegalStateException("rendered output column names are not pinned: " + Arrays.toString(names));
        }
        EventPropertyDescriptor[] descriptors = statement.getEventType().getPropertyDescriptors();
        if (descriptors.length != COLNAME_FIELDS.length) {
            throw new IllegalStateException("expected two property descriptors, got " + descriptors.length);
        }
        JsonArray value = new JsonArray();
        for (int index = 0; index < descriptors.length; index++) {
            if (!COLNAME_FIELDS[index].equals(descriptors[index].getPropertyName())) {
                throw new IllegalStateException("descriptor " + index + " = " + descriptors[index].getPropertyName()
                        + ", want " + COLNAME_FIELDS[index]);
            }
            String type = simpleTypeName(descriptors[index].getPropertyType());
            if (!"Long".equals(type)) {
                throw new IllegalStateException("descriptor " + COLNAME_FIELDS[index] + " type " + type + ", want Long");
            }
            value.add(new JsonObject().add("name", COLNAME_FIELDS[index]).add("type", type));
        }
        return value;
    }

    private static String simpleTypeName(Class<?> type) {
        if (type == String.class) {
            return "String";
        }
        if (type == Integer.class || type == int.class) {
            return "Integer";
        }
        if (type == Long.class || type == long.class) {
            return "Long";
        }
        if (type == java.util.Map.class) {
            return "Map";
        }
        if (type.isArray()) {
            return simpleTypeName(type.getComponentType()) + "[]";
        }
        String simple = type.getSimpleName();
        if (simple.isEmpty()) {
            throw new IllegalStateException("anonymous property type " + type);
        }
        return simple;
    }

    private static void send(EPRuntime runtime, JsonObject step, int stepIndex) {
        requireFields(step, "op", "eventType", "payload");
        if (!"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("unexpected event type at step " + stepIndex);
        }
        JsonObject payload = object(step.get("payload"), "payload step " + stepIndex);
        requireFields(payload, "theString", "intPrimitive", "longPrimitive");
        SupportBean bean = new SupportBean(string(payload, "theString"), integer(payload, "intPrimitive"));
        bean.setLongPrimitive(longNumber(payload, "longPrimitive"));
        runtime.getEventService().sendEventBean(bean, "SupportBean");
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
        validateStringArray(scenario.get("javaRuntimes"), RUNTIMES, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("expected four cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation",
                    "iteratorSnapshots", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIMES[index].equals(string(definition, "runtimeId"))
                    || !NAMES[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != ITERATOR_SNAPSHOTS[index]
                    || !epl(index).equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != STEP_OPS.length) {
            throw new IllegalArgumentException("scenario must contain exactly twenty steps");
        }
        int sendIndex = 0;
        int snapshotIndex = 0;
        int markerIndex = 0;
        int deployedIndex = 0;
        int typesIndex = 0;
        for (int index = 0; index < STEP_OPS.length; index++) {
            JsonObject step = object(steps.get(index), "step " + index);
            String operation = string(step, "op");
            if (!STEP_OPS[index].equals(operation)) {
                throw new IllegalArgumentException("step " + index + " operation is " + operation + ", want "
                        + STEP_OPS[index]);
            }
            if ("case".equals(operation)) {
                requireFields(step, "op", "case");
                if (markerIndex >= CASES.length || !CASES[markerIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("case marker is not pinned at step " + index);
                }
                markerIndex++;
            } else if ("send".equals(operation)) {
                validateSend(step, index, SEND_STRINGS[sendIndex], SEND_INTS[sendIndex], SEND_LONGS[sendIndex]);
                sendIndex++;
            } else if ("snapshot".equals(operation)) {
                requireFields(step, "op", "statement", "mode");
                if (!"s0".equals(string(step, "statement")) || !"any".equals(string(step, "mode"))) {
                    throw new IllegalArgumentException("snapshot is not pinned at step " + index);
                }
                snapshotIndex++;
            } else if ("deployed".equals(operation)) {
                requireFields(step, "op", "statement");
                if (!"s0".equals(string(step, "statement")) || ++deployedIndex != 1) {
                    throw new IllegalArgumentException("deployed step is not pinned at " + index);
                }
            } else {
                requireFields(step, "op", "statement");
                if (!"s0".equals(string(step, "statement")) || ++typesIndex != 1) {
                    throw new IllegalArgumentException("types step is not pinned at " + index);
                }
            }
        }
        if (markerIndex != CASES.length || sendIndex != SEND_STRINGS.length
                || snapshotIndex != ITERATOR_SNAPSHOTS[0] || deployedIndex != 1 || typesIndex != 1) {
            throw new IllegalArgumentException("scenario step census is not pinned");
        }
    }

    private static void validateSend(JsonObject step, int index, String expectedString,
                                     int expectedInt, long expectedLong) {
        requireFields(step, "op", "eventType", "payload");
        if (!"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send event type is not pinned at step " + index);
        }
        JsonObject payload = object(step.get("payload"), "payload step " + index);
        requireFields(payload, "theString", "intPrimitive", "longPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || integer(payload, "intPrimitive") != expectedInt
                || longNumber(payload, "longPrimitive") != expectedLong) {
            throw new IllegalArgumentException("send payload is not pinned at step " + index);
        }
    }

    private static JsonObject row(EventBean event) {
        String[] properties = event.getEventType().getPropertyNames().clone();
        Arrays.sort(properties);
        JsonObject fields = new JsonObject();
        for (String property : properties) {
            fields.add(property, normalize(event.get(property)));
        }
        return new JsonObject().add("kind", "row").add("fields", fields);
    }

    private static JsonArray rows(EventBean[] events) {
        JsonArray output = new JsonArray();
        for (EventBean event : events) {
            output.add(row(event));
        }
        return output;
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
        return Json.value(String.valueOf(value));
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

    private static final class ListenerWriter implements UpdateListener {
        private final JsonArray records;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private final String caseName;
        private int sequence;

        private ListenerWriter(JsonArray records, EPStatement statement, EPRuntime runtime, String caseName) {
            this.records = records;
            this.statement = statement;
            this.runtime = runtime;
            this.caseName = caseName;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            int next = sequence + 1;
            if (next > LISTENER_ROWS.length) {
                throw new IllegalStateException("unexpected listener callback after four records");
            }
            if (newEvents == null || newEvents.length != 1
                    || (oldEvents != null && oldEvents.length != 0)) {
                throw new IllegalStateException("listener callback must contain one new row only");
            }
            long now = runtime.getEventService().getCurrentTime();
            if (now != 0L) {
                throw new IllegalStateException("unexpected callback time");
            }
            assertRow(newEvents[0], next);
            sequence = next;
            records.add(new JsonObject().add("case", caseName).add("operation", "listener")
                    .add("statement", statement.getName()).add("sequence", sequence)
                    .add("time", Instant.ofEpochMilli(now).toString()).add("new", rows(newEvents)));
        }

        private void assertRow(EventBean row, int sequence) {
            String[] names = row.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            if (!Arrays.equals(names, LISTENER_FIELDS)) {
                throw new IllegalStateException("result field metadata is not pinned: " + Arrays.toString(names));
            }
            long[] expected = LISTENER_ROWS[sequence - 1];
            for (int index = 0; index < LISTENER_FIELDS.length; index++) {
                Object value = row.get(LISTENER_FIELDS[index]);
                if (!(value instanceof Long) || ((Long) value).longValue() != expected[index]) {
                    throw new IllegalStateException("listener record " + sequence + " field " + LISTENER_FIELDS[index]
                            + " = " + value + ", want " + expected[index]);
                }
            }
        }
    }
}
