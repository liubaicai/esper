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
import java.util.Iterator;
import java.util.Set;

/** Direct Esper 9.0.0 oracle for ResultSetOutputLimitRowLimit ordinals 0 and 4. */
public final class ResultSetOutputLimitRowLimitContextGroupedScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-output-limit-row-limit-context-grouped";
    private static final String DESCRIPTION =
            "ResultSetOutputLimitRowLimit ordinals 0 and 4: order-optimized limit-one batch/context termination and fully grouped ordered aggregate iterator behavior.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowLimit.java";

    private static final String CASE_LIMIT_ONE = "limit-one-order-optimization";
    private static final String CASE_FULLY_GROUPED = "fully-grouped-ordered";
    private static final String[] CASES = {CASE_LIMIT_ONE, CASE_FULLY_GROUPED};
    private static final int[] ORDINALS = {0, 4};
    private static final String[] RUNTIMES = {
            "java-runtime-0a4f187046b734b5dc9a",
            "java-runtime-19d52dc587ea246a3e07"};
    private static final String[] EXECUTIONS = {
            "ResultSetLimitOneWithOrderOptimization",
            "ResultSetFullyGroupedOrdered"};
    private static final String[] STATIC_IDS = {
            "java-3f5845fdac39997e9b3d",
            "java-bf49a03f52cc732a2cf6"};

    private static final String EPL_BATCH_SINGLE =
            "@name('s0') select theString from SupportBean#length_batch(10) order by theString limit 1";
    private static final String EPL_BATCH_MULTI =
            "@name('s0') select theString, intPrimitive from SupportBean#length_batch(5) order by theString asc, intPrimitive desc limit 1";
    private static final String EPL_CONTEXT_SINGLE =
            "@name('s0') context StartS0EndS1 select theString from SupportBean#keepall output snapshot when terminated order by theString limit 1";
    private static final String EPL_CONTEXT_MULTI =
            "@name('s0') context StartS0EndS1 select theString, intPrimitive from SupportBean#keepall output snapshot when terminated order by theString asc, intPrimitive desc limit 1";
    private static final String EPL_FULLY_GROUPED =
            "@name('s0') select theString, sum(intPrimitive) as mysum from SupportBean#length(5) group by theString order by sum(intPrimitive) limit 2";
    private static final String CONTEXT_DECLARATION =
            "@public create context StartS0EndS1 as start SupportBean_S0 end SupportBean_S1";

    private static final String[] PHASE_LABELS = {
            "batch-single-1", "batch-single-2", "batch-single-3",
            "batch-multi-1", "batch-multi-2", "batch-multi-3",
            "context-single-1", "context-single-2", "context-single-3",
            "context-multi-1", "context-multi-2", "context-multi-3"};
    private static final String[][] SINGLE_VALUES = {
            {"F", "Q", "R", "T", "M", "T", "A", "I", "P", "B"},
            {"P", "Q", "P", "T", "P", "T", "P", "P", "P", "B"},
            {"C", "P", "Q", "P", "T", "P", "T", "P", "P", "P", "X"}};
    private static final String[][] MULTI_VALUES = {
            {"F", "X", "F", "G", "X"},
            {"X", "G", "H", "G", "X"},
            {"G", "G", "G", "G", "G"}};
    private static final int[][] MULTI_INTS = {
            {10, 8, 8, 10, 1},
            {10, 12, 100, 10, 1},
            {10, 8, 8, 10, 11}};
    private static final String[] GROUPED_VALUES = {"E1", "E2", "E3", "E3", "E2"};
    private static final int[] GROUPED_INTS = {90, 5, 60, 40, 1000};

    private ResultSetOutputLimitRowLimitContextGroupedScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOutputLimitRowLimitContextGroupedScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        rejectDuplicateKeys(parsed);
        JsonObject scenario = object(parsed, "scenario");
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        JsonArray steps = scenario.get("steps").asArray();
        runLimitOne(steps, records);
        runFullyGrouped(steps, records);
        if (records.size() != 30) {
            throw new IllegalStateException("expected exactly 30 trace records, got " + records.size());
        }
        int snapshots = 0;
        int listeners = 0;
        for (JsonValue value : records) {
            JsonObject record = object(value, "trace record");
            String operation = string(record, "operation");
            if ("snapshot".equals(operation)) {
                snapshots++;
            } else if ("listener".equals(operation)) {
                listeners++;
            } else {
                throw new IllegalStateException("unsupported trace operation " + operation);
            }
        }
        if (snapshots != 18 || listeners != 12) {
            throw new IllegalStateException("unexpected trace counts: snapshots=" + snapshots
                    + ", listeners=" + listeners);
        }
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runLimitOne(JsonArray steps, JsonArray records) throws Exception {
        Configuration configuration = configuration();
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + RUNTIMES[0], configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        int[] listenerSequence = {0};
        int index = 1;
        try {
            for (int phase = 0; phase < PHASE_LABELS.length; phase++) {
                boolean context = phase >= 6;
                boolean single = phase < 3 || phase >= 6 && phase < 9;
                String epl;
                if (context) {
                    epl = CONTEXT_DECLARATION + ";\n" + (single ? EPL_CONTEXT_SINGLE : EPL_CONTEXT_MULTI);
                } else {
                    epl = single ? EPL_BATCH_SINGLE : EPL_BATCH_MULTI;
                }
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                        epl, new CompilerArguments(runtime.getRuntimePath()));
                EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                        new DeploymentOptions().setDeploymentId(ID + "-limit-one-" + phase));
                EPStatement statement = findStatement(deployment);
                TraceWriter writer = new TraceWriter(records, statement, runtime, CASE_LIMIT_ONE, listenerSequence);
                statement.addListener(writer);
                int sourceCount = single ? SINGLE_VALUES[phase % 3].length : MULTI_VALUES[phase % 3].length;
                index = replayLimitPhase(steps, index, PHASE_LABELS[phase], sourceCount, context,
                        runtime, writer, CASE_LIMIT_ONE);
                if (writer.snapshotCount != 1 || writer.listenerCount != 1) {
                    throw new IllegalStateException("phase " + PHASE_LABELS[phase]
                            + " emitted snapshots=" + writer.snapshotCount
                            + ", listeners=" + writer.listenerCount + "; expected one of each");
                }
                runtime.getDeploymentService().undeployAll();
            }

        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
        if (index != 129 || listenerSequence[0] != 12) {
            throw new IllegalStateException("ordinal 0 replay count mismatch: index=" + index
                    + ", listeners=" + listenerSequence[0]);
        }
    }

    private static int replayLimitPhase(JsonArray steps, int index, String label, int sourceCount, boolean context,
                                        EPRuntime runtime, TraceWriter writer, String caseName) {
        send(runtime, object(steps.get(index++), "limit-one start"), caseName);
        int beforeSnapshot = context ? sourceCount : sourceCount - 1;
        for (int i = 0; i < beforeSnapshot; i++) {
            send(runtime, object(steps.get(index++), "limit-one source"), caseName);
        }
        JsonObject snapshot = object(steps.get(index++), "limit-one snapshot");
        if (!label.equals(string(snapshot, "label"))) {
            throw new IllegalArgumentException("unexpected phase label " + string(snapshot, "label"));
        }
        writer.snapshot(snapshot);
        for (int i = beforeSnapshot; i < sourceCount; i++) {
            send(runtime, object(steps.get(index++), "limit-one boundary source"), caseName);
        }
        send(runtime, object(steps.get(index++), "limit-one end"), caseName);
        return index;
    }

    private static void runFullyGrouped(JsonArray steps, JsonArray records) throws Exception {
        Configuration configuration = configuration();
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + RUNTIMES[1], configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                    EPL_FULLY_GROUPED, new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID + "-fully-grouped"));
            EPStatement statement = findStatement(deployment);
            // The pinned execution attaches a listener, but listener output is not an
            // observation for this iterator-only case. Keep it deliberately unrecorded.
            statement.addListener((newEvents, oldEvents, ignoredStatement, ignoredRuntime) -> { });

            int index = 130;
            JsonObject initial = object(steps.get(index++), "fully-grouped initial snapshot");
            takeSnapshot(statement, runtime, initial, CASE_FULLY_GROUPED, records);
            for (int i = 0; i < GROUPED_VALUES.length; i++) {
                send(runtime, object(steps.get(index++), "fully-grouped source"), CASE_FULLY_GROUPED);
                JsonObject snapshot = object(steps.get(index++), "fully-grouped snapshot");
                takeSnapshot(statement, runtime, snapshot, CASE_FULLY_GROUPED, records);
            }
            if (index != steps.size()) {
                throw new IllegalStateException("fully grouped replay has trailing steps");
            }
            int snapshotCount = 0;
            for (JsonValue value : records) {
                JsonObject record = object(value, "trace record");
                if (CASE_FULLY_GROUPED.equals(string(record, "case"))) {
                    snapshotCount++;
                    if (!"snapshot".equals(string(record, "operation"))) {
                        throw new IllegalStateException("fully grouped case emitted listener output");
                    }
                }
            }
            if (snapshotCount != 6) {
                throw new IllegalStateException("fully grouped snapshot count is " + snapshotCount);
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static Configuration configuration() {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCommon().addEventType(SupportBean_S1.class);
        return configuration;
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        EPStatement result = null;
        for (EPStatement candidate : deployment.getStatements()) {
            if (!"s0".equals(candidate.getName())) {
                continue;
            }
            if (result != null) {
                throw new IllegalStateException("multiple statements named s0");
            }
            result = candidate;
        }
        if (result == null) {
            throw new IllegalStateException("statement s0 was not deployed");
        }
        return result;
    }

    private static void send(EPRuntime runtime, JsonObject step, String caseName) {
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("send step is not pinned for " + caseName);
        }
        String eventType = string(step, "eventType");
        JsonObject payload = object(step.get("payload"), "send payload");
        if ("SupportBean".equals(eventType)) {
            requireFields(payload, "theString", "intPrimitive");
            runtime.getEventService().sendEventBean(
                    new SupportBean(string(payload, "theString"), integer(payload, "intPrimitive")), eventType);
        } else if ("SupportBean_S0".equals(eventType)) {
            requireFields(payload, "id");
            runtime.getEventService().sendEventBean(new SupportBean_S0(integer(payload, "id")), eventType);
        } else if ("SupportBean_S1".equals(eventType)) {
            requireFields(payload, "id");
            runtime.getEventService().sendEventBean(new SupportBean_S1(integer(payload, "id")), eventType);
        } else {
            throw new IllegalArgumentException("unsupported event type " + eventType);
        }
    }

    private static void takeSnapshot(EPStatement statement, EPRuntime runtime, JsonObject step,
                                     String caseName, JsonArray records) {
        requireFields(step, "op", "case", "statement", "mode");
        if (!"snapshot".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !"s0".equals(string(step, "statement")) || !"ordered".equals(string(step, "mode"))) {
            throw new IllegalArgumentException("snapshot metadata is not pinned for " + caseName);
        }
        JsonArray rows = new JsonArray();
        Iterator<EventBean> iterator = statement.iterator();
        while (iterator.hasNext()) {
            rows.add(row(iterator.next()));
        }
        records.add(new JsonObject().add("case", caseName).add("operation", "snapshot")
                .add("statement", statement.getName()).add("sequence", 0)
                .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
                .add("new", rows));
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

    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short || value instanceof Byte) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Float || value instanceof Double) {
            double number = ((Number) value).doubleValue();
            if (number == Math.rint(number) && !Double.isInfinite(number)) {
                return Json.value((long) number);
            }
            return Json.value(number);
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
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version")) || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIMES, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTIONS, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly two cases");
        }
        String[] caseEpls = {EPL_BATCH_SINGLE, EPL_FULLY_GROUPED};
        int[] iteratorSnapshots = {12, 6};
        String[] observations = {"listener+iterator", "iterator"};
        for (int i = 0; i < CASES.length; i++) {
            JsonObject definition = object(cases.get(i), "case definition " + i);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation",
                    "iteratorSnapshots", "epl");
            if (!CASES[i].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[i]
                    || !RUNTIMES[i].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[i].equals(string(definition, "executionName"))
                    || !observations[i].equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != iteratorSnapshots[i]
                    || !caseEpls[i].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + i);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 141) {
            throw new IllegalArgumentException("scenario must contain exactly 141 steps");
        }
        int index = 0;
        JsonObject marker = object(steps.get(index++), "ordinal 0 marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !CASES[0].equals(string(marker, "case"))) {
            throw new IllegalArgumentException("ordinal 0 marker is not pinned");
        }
        for (int phase = 0; phase < PHASE_LABELS.length; phase++) {
            boolean single = phase < 3 || phase >= 6 && phase < 9;
            String[] strings = single ? SINGLE_VALUES[phase % 3] : MULTI_VALUES[phase % 3];
            int[] numbers = single ? null : MULTI_INTS[phase % 3];
            validateBoundarySend(object(steps.get(index++), "ordinal 0 start " + phase), CASES[0],
                    "SupportBean_S0", 0);
            int beforeSnapshot = phase < 6 ? strings.length - 1 : strings.length;
            for (int event = 0; event < beforeSnapshot; event++) {
                validateBeanSend(object(steps.get(index++), "ordinal 0 source " + phase + "/" + event),
                        CASES[0], strings[event], numbers == null ? 0 : numbers[event]);
            }
            JsonObject snapshot = object(steps.get(index++), "ordinal 0 snapshot " + phase);
            requireFields(snapshot, "op", "case", "statement", "label", "mode");
            if (!"snapshot".equals(string(snapshot, "op"))
                    || !CASES[0].equals(string(snapshot, "case"))
                    || !"s0".equals(string(snapshot, "statement"))
                    || !PHASE_LABELS[phase].equals(string(snapshot, "label"))
                    || !"ordered".equals(string(snapshot, "mode"))) {
                throw new IllegalArgumentException("ordinal 0 snapshot is not pinned at phase " + phase);
            }
            for (int event = beforeSnapshot; event < strings.length; event++) {
                validateBeanSend(object(steps.get(index++), "ordinal 0 boundary source " + phase), CASES[0],
                        strings[event], numbers == null ? 0 : numbers[event]);
            }
            validateBoundarySend(object(steps.get(index++), "ordinal 0 end " + phase), CASES[0],
                    "SupportBean_S1", 0);
        }

        marker = object(steps.get(index++), "ordinal 4 marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !CASES[1].equals(string(marker, "case"))) {
            throw new IllegalArgumentException("ordinal 4 marker is not pinned");
        }
        JsonObject initial = object(steps.get(index++), "ordinal 4 initial snapshot");
        requireFields(initial, "op", "case", "statement", "mode");
        if (!"snapshot".equals(string(initial, "op")) || !CASES[1].equals(string(initial, "case"))
                || !"s0".equals(string(initial, "statement")) || !"ordered".equals(string(initial, "mode"))) {
            throw new IllegalArgumentException("ordinal 4 initial snapshot is not pinned");
        }
        for (int event = 0; event < GROUPED_VALUES.length; event++) {
            validateBeanSend(object(steps.get(index++), "ordinal 4 source " + event), CASES[1],
                    GROUPED_VALUES[event], GROUPED_INTS[event]);
            JsonObject snapshot = object(steps.get(index++), "ordinal 4 snapshot " + event);
            requireFields(snapshot, "op", "case", "statement", "mode");
            if (!"snapshot".equals(string(snapshot, "op")) || !CASES[1].equals(string(snapshot, "case"))
                    || !"s0".equals(string(snapshot, "statement")) || !"ordered".equals(string(snapshot, "mode"))) {
                throw new IllegalArgumentException("ordinal 4 snapshot is not pinned at event " + event);
            }
        }
        if (index != steps.size()) {
            throw new IllegalArgumentException("scenario has trailing steps");
        }
    }

    private static void validateBoundarySend(JsonObject step, String caseName, String eventType, int id) {
        requireFields(step, "op", "case", "eventType", "payload");
        JsonObject payload = object(step.get("payload"), "boundary payload");
        requireFields(payload, "id");
        if (!"send".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !eventType.equals(string(step, "eventType")) || integer(payload, "id") != id) {
            throw new IllegalArgumentException("boundary send is not pinned");
        }
    }

    private static void validateBeanSend(JsonObject step, String caseName, String expectedString, int expectedInt) {
        requireFields(step, "op", "case", "eventType", "payload");
        JsonObject payload = object(step.get("payload"), "bean payload");
        requireFields(payload, "theString", "intPrimitive");
        if (!"send".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))
                || !expectedString.equals(string(payload, "theString"))
                || integer(payload, "intPrimitive") != expectedInt) {
            throw new IllegalArgumentException("bean send is not pinned");
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
        JsonValue value = object.get(name);
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        String text = value.toString();
        if (!text.matches("-?(0|[1-9][0-9]*)")) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        try {
            long parsed = Long.parseLong(text, 10);
            if (parsed < Integer.MIN_VALUE || parsed > Integer.MAX_VALUE) {
                throw new NumberFormatException("outside int range");
            }
            return (int) parsed;
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(name + " must be an integer JSON number", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String label) {
        JsonArray actual = array(value, label);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(label + " is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            if (!actual.get(index).isString() || !expected[index].equals(actual.get(index).asString())) {
                throw new IllegalArgumentException(label + " is not pinned");
            }
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

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private final String caseName;
        private final int[] listenerSequence;
        private int snapshotCount;
        private int listenerCount;

        private TraceWriter(JsonArray records, EPStatement statement, EPRuntime runtime,
                            String caseName, int[] listenerSequence) {
            this.records = records;
            this.statement = statement;
            this.runtime = runtime;
            this.caseName = caseName;
            this.listenerSequence = listenerSequence;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            if (newEvents == null && (oldEvents == null || oldEvents.length == 0)) {
                return;
            }
            if (newEvents == null || newEvents.length != 1 || oldEvents != null && oldEvents.length != 0) {
                throw new IllegalStateException("unexpected listener shape for " + caseName);
            }
            listenerCount++;
            int sequence = ++listenerSequence[0];
            records.add(new JsonObject().add("case", caseName).add("operation", "listener")
                    .add("statement", statement.getName()).add("sequence", sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
                    .add("new", rows(newEvents)));
        }

        private void snapshot(JsonObject step) {
            requireFields(step, "op", "case", "statement", "label", "mode");
            if (!"snapshot".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                    || !"s0".equals(string(step, "statement")) || !"ordered".equals(string(step, "mode"))) {
                throw new IllegalArgumentException("snapshot metadata is not pinned for " + caseName);
            }
            JsonArray snapshotRows = new JsonArray();
            Iterator<EventBean> iterator = statement.iterator();
            while (iterator.hasNext()) {
                snapshotRows.add(row(iterator.next()));
            }
            snapshotCount++;
            records.add(new JsonObject().add("case", caseName).add("operation", "snapshot")
                    .add("statement", statement.getName()).add("sequence", 0)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
                    .add("new", snapshotRows));
        }

        private static JsonArray rows(EventBean[] events) {
            JsonArray result = new JsonArray();
            if (events == null) {
                return result;
            }
            for (EventBean event : events) {
                result.add(row(event));
            }
            return result;
        }
    }
}
