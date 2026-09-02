import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.AnnotationPart;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.soda.Expressions;
import com.espertech.esper.common.client.soda.FilterStream;
import com.espertech.esper.common.client.soda.FromClause;
import com.espertech.esper.common.client.soda.RowLimitClause;
import com.espertech.esper.common.client.soda.SelectClause;
import com.espertech.esper.common.client.soda.StreamSelector;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.util.SerializableObjectCopier;
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

/** Direct Esper 9.0.0 oracle for ResultSetOutputLimitRowLimit ordinals 1 and 3. */
public final class ResultSetOutputLimitRowLimitScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-output-limit-row-limit";
    private static final String DESCRIPTION =
            "ResultSetOutputLimitRowLimit ordinals 1 and 3: length-batch wildcard row limit with old/new streams and SODA round-trip.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowLimit.java";
    private static final String[] CASES = {"batch-no-offset-no-order", "batch-offset-no-order-om"};
    private static final int[] ORDINALS = {1, 3};
    private static final String[] RUNTIMES = {
            "java-runtime-840e283d8ac639591083",
            "java-runtime-20d0d1451ce549bf53b5"};
    private static final String[] EXECUTIONS = {
            "ResultSetBatchNoOffsetNoOrder",
            "ResultSetBatchOffsetNoOrderOM"};
    private static final String[] STATIC_IDS = {
            "java-2d2f2a8e404c07c322c6",
            "java-789d7de75e91392eb5e1"};
    private static final String EPL_NAMED =
            "@name('s0') select irstream * from SupportBean#length_batch(3) limit 1";
    private static final String EPL_UNNAMED =
            "select irstream * from SupportBean#length_batch(3) limit 1";

    private ResultSetOutputLimitRowLimitScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ResultSetOutputLimitRowLimitScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(scenario.get("steps").asArray(), caseIndex, records);
        }
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        String runtimeURI = "parity-" + ID + "-" + RUNTIMES[caseIndex];
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled compiled;
            if (caseIndex == 0) {
                compiled = EPCompilerProvider.getCompiler().compile(EPL_NAMED,
                        new CompilerArguments(runtime.getRuntimePath()));
            } else {
                EPStatementObjectModel model = new EPStatementObjectModel();
                model.setSelectClause(SelectClause.createWildcard()
                        .streamSelector(StreamSelector.RSTREAM_ISTREAM_BOTH));
                model.setFromClause(FromClause.create(
                        FilterStream.create("SupportBean").addView("length_batch", Expressions.constant(3))));
                model.setRowLimitClause(RowLimitClause.create(1));
                if (!EPL_UNNAMED.equals(model.toEPL())) {
                    throw new IllegalStateException("SODA toEPL mismatch: " + model.toEPL());
                }
                model.setAnnotations(java.util.Collections.singletonList(AnnotationPart.nameAnnotation("s0")));
                EPStatementObjectModel copied = SerializableObjectCopier.copyMayFail(model);
                Module module = new Module();
                module.getItems().add(new ModuleItem(copied));
                module.setModuleText(copied.toEPL());
                compiled = EPCompilerProvider.getCompiler().compile(module,
                        new CompilerArguments(runtime.getRuntimePath()));
            }
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID + "-" + caseIndex));
            EPStatement statement = findStatement(deployment);
            TraceWriter writer = new TraceWriter(records, statement, runtime, CASES[caseIndex]);
            statement.addListener(writer);
            replayCase(steps, CASES[caseIndex], runtime, writer);
            if (writer.listenerSequence != 2 || writer.snapshotSequence != 6) {
                throw new IllegalStateException("unexpected record counts for " + CASES[caseIndex]
                        + ": listeners=" + writer.listenerSequence + ", snapshots=" + writer.snapshotSequence);
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
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

    private static void replayCase(JsonArray steps, String caseName, EPRuntime runtime, TraceWriter writer) {
        boolean active = false;
        for (int index = 0; index < steps.size(); index++) {
            JsonObject step = object(steps.get(index), "step " + index);
            String operation = string(step, "op");
            if ("case".equals(operation)) {
                active = caseName.equals(string(step, "case"));
                continue;
            }
            if (!active) {
                continue;
            }
            if ("snapshot".equals(operation)) {
                writer.snapshot(step);
                continue;
            }
            if (!"send".equals(operation)) {
                throw new IllegalArgumentException("unsupported operation " + operation);
            }
            JsonObject payload = object(step.get("payload"), "payload step " + index);
            runtime.getEventService().sendEventBean(
                    new SupportBean(string(payload, "theString"), integer(payload, "intPrimitive")),
                    "SupportBean");
        }
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
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation",
                    "iteratorSnapshots", "epl");
            String expectedObservation = index == 0 ? "listener+iterator" : "listener+iterator+soda";
            String expectedEPL = index == 0 ? EPL_NAMED : EPL_UNNAMED;
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIMES[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !expectedObservation.equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 6
                    || !expectedEPL.equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }
        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 26) {
            throw new IllegalArgumentException("scenario must contain exactly 26 steps");
        }
        int index = 0;
        for (String caseName : CASES) {
            JsonObject marker = object(steps.get(index++), "case marker");
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op")) || !caseName.equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker is not pinned");
            }
            String[] names = {"E1", "E2", "E3", "E4", "E5", "E6"};
            for (int eventIndex = 0; eventIndex < names.length; eventIndex++) {
                JsonObject snapshot = object(steps.get(index++), "snapshot step");
                requireFields(snapshot, "op", "case", "statement");
                if (!"snapshot".equals(string(snapshot, "op"))
                        || !caseName.equals(string(snapshot, "case"))
                        || !"s0".equals(string(snapshot, "statement"))) {
                    throw new IllegalArgumentException("snapshot step is not pinned");
                }
                JsonObject send = object(steps.get(index++), "send step");
                requireFields(send, "op", "case", "eventType", "payload");
                if (!"send".equals(string(send, "op")) || !caseName.equals(string(send, "case"))
                        || !"SupportBean".equals(string(send, "eventType"))) {
                    throw new IllegalArgumentException("send step is not pinned");
                }
                JsonObject payload = object(send.get("payload"), "send payload");
                requireFields(payload, "theString", "intPrimitive");
                if (!names[eventIndex].equals(string(payload, "theString"))
                        || integer(payload, "intPrimitive") != eventIndex + 1) {
                    throw new IllegalArgumentException("send payload is not pinned");
                }
            }
        }
        if (index != steps.size()) {
            throw new IllegalArgumentException("scenario has trailing steps");
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

    private static void requireFields(JsonObject object, String... expected) {
        if (object == null || object.size() != expected.length
                || !new HashSet<>(object.names()).equals(new HashSet<>(Arrays.asList(expected)))) {
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

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        JsonArray actual = array(value, name);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(name + " is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            if (!actual.get(index).isString() || !expected[index].equals(actual.get(index).asString())) {
                throw new IllegalArgumentException(name + " is not pinned");
            }
        }
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be an array");
        }
        return value.asArray();
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be an object");
        }
        return value.asObject();
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private final String caseName;
        private int listenerSequence;
        private int snapshotSequence;

        private TraceWriter(JsonArray records, EPStatement statement, EPRuntime runtime, String caseName) {
            this.records = records;
            this.statement = statement;
            this.runtime = runtime;
            this.caseName = caseName;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            if (newEvents == null && oldEvents == null) {
                return;
            }
            if (newEvents != null && newEvents.length == 1 && oldEvents != null && oldEvents.length == 1) {
                listenerSequence++;
                JsonObject record = base("listener", listenerSequence);
                record.add("new", rows(newEvents));
                record.add("old", rows(oldEvents));
                records.add(record);
                return;
            }
            if (newEvents == null || newEvents.length != 1 || (oldEvents != null && oldEvents.length != 0)) {
                throw new IllegalStateException("unexpected row-limit listener shape");
            }
            listenerSequence++;
            JsonObject record = base("listener", listenerSequence);
            record.add("new", rows(newEvents));
            records.add(record);
        }

        private void snapshot(JsonObject step) {
            if (!"s0".equals(string(step, "statement"))) {
                throw new IllegalArgumentException("snapshot statement is not pinned");
            }
            JsonArray snapshotRows = new JsonArray();
            java.util.Iterator<EventBean> iterator = statement.iterator();
            while (iterator.hasNext()) {
                snapshotRows.add(rows(new EventBean[]{iterator.next()}).get(0));
            }
            snapshotSequence++;
            JsonObject record = base("snapshot", 0);
            record.add("new", snapshotRows);
            records.add(record);
        }
        private JsonObject base(String operation, int sequence) {
            return new JsonObject().add("case", caseName).add("operation", operation)
                    .add("statement", statement.getName()).add("sequence", sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray result = new JsonArray();
            if (events == null) {
                return result;
            }
            for (EventBean event : events) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                result.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return result;
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof Integer || value instanceof Long || value instanceof Short || value instanceof Byte) {
                return Json.value(((Number) value).longValue());
            }
            if (value instanceof Character) {
                return Json.value(String.valueOf(value));
            }
            if (value instanceof Number) {
                return Json.value(((Number) value).doubleValue());
            }
            if (value instanceof Boolean) {
                return Json.value((Boolean) value);
            }
            return Json.value(String.valueOf(value));
        }
    }
}
