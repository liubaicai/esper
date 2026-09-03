import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.AnnotationPart;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.soda.FilterStream;
import com.espertech.esper.common.client.soda.FromClause;
import com.espertech.esper.common.client.soda.Expressions;
import com.espertech.esper.common.client.soda.OutputLimitClause;
import com.espertech.esper.common.client.soda.RowLimitClause;
import com.espertech.esper.common.client.soda.SelectClause;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.util.SerializableObjectCopier;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBeanNumeric;
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
import java.util.Collections;
import java.util.HashSet;
import java.util.Iterator;
import java.util.Set;

/** Direct Esper 9.0.0 oracle for ResultSetOutputLimitRowLimit ordinal 9. */
public final class ResultSetOutputLimitRowLimitVariableScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-output-limit-row-limit-variable";
    private static final String DESCRIPTION =
            "ResultSetOutputLimitRowLimit ordinal 9: variable-backed dynamic limit and offset across comma, keyword, and SODA deployment forms.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowLimit.java";
    private static final String[] CASES = {"variable-comma", "variable-keyword", "variable-soda"};
    private static final String RUNTIME = "java-runtime-c591e5e05eb22cddfd00";
    private static final String EXECUTION = "ResultSetLengthOffsetVariable";
    private static final String STATIC_ID = "java-299a17bb6319c7f2e6de";
    private static final String EPL_COMMA =
            "@name('s0') select * from SupportBean#length(5) output every 5 events limit myoffset, myrows";
    private static final String EPL_KEYWORD =
            "@name('s0') select * from SupportBean#length(5) output every 5 events limit myrows offset myoffset";
    private static final String EPL_KEYWORD_BODY =
            "select * from SupportBean#length(5) output every 5 events limit myrows offset myoffset";
    private static final String[] SETUP_EPLS = {
            "@public create variable int myrows = 2",
            "@public create variable int myoffset = 1",
            "on SupportBeanNumeric set myrows = intOne, myoffset = intTwo"};
    private static final String[] FINAL_ONE =
            {"1", "2", "1", "6", "1", null, null, "2", "-1", "-1", "0"};
    private static final String[] FINAL_TWO =
            {"1", "1", "2", "6", "4", null, "2", null, "4", "0", "0"};

    private ResultSetOutputLimitRowLimitVariableScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOutputLimitRowLimitVariableScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        rejectDuplicateKeys(parsed);
        JsonObject scenario = object(parsed, "scenario");
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        JsonArray steps = scenario.get("steps").asArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(steps, caseIndex, records);
        }

        if (records.size() != 69) {
            throw new IllegalStateException("expected exactly 69 trace records, got " + records.size());
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
        if (snapshots != 63 || listeners != 6) {
            throw new IllegalStateException("unexpected trace counts: snapshots=" + snapshots
                    + ", listeners=" + listeners);
        }
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBeanNumeric.class);

        String runtimeURI = "parity-" + ID + "-" + RUNTIME + "-" + caseIndex;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            for (int setupIndex = 0; setupIndex < SETUP_EPLS.length; setupIndex++) {
                EPCompiled setup = EPCompilerProvider.getCompiler().compile(
                        SETUP_EPLS[setupIndex], new CompilerArguments(runtime.getRuntimePath()));
                runtime.getDeploymentService().deploy(setup,
                        new DeploymentOptions().setDeploymentId(ID + "-setup-" + caseIndex + "-" + setupIndex));
            }

            EPCompiled query;
            if (caseIndex == 0) {
                query = EPCompilerProvider.getCompiler().compile(
                        EPL_COMMA, new CompilerArguments(runtime.getRuntimePath()));
            } else if (caseIndex == 1) {
                query = EPCompilerProvider.getCompiler().compile(
                        EPL_KEYWORD, new CompilerArguments(runtime.getRuntimePath()));
            } else {
                query = compileSoda(runtime);
            }
            EPDeployment deployment = runtime.getDeploymentService().deploy(query,
                    new DeploymentOptions().setDeploymentId(ID + "-query-" + caseIndex));
            EPStatement statement = findStatement(deployment);
            TraceWriter writer = new TraceWriter(records, statement, runtime, CASES[caseIndex]);
            statement.addListener(writer);
            replayCase(steps, CASES[caseIndex], runtime, writer);
            if (writer.listenerCount != 2 || writer.snapshotCount != 21) {
                throw new IllegalStateException("unexpected record counts for " + CASES[caseIndex]
                        + ": listeners=" + writer.listenerCount + ", snapshots=" + writer.snapshotCount);
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static EPCompiled compileSoda(EPRuntime runtime) throws Exception {
        EPStatementObjectModel model = new EPStatementObjectModel();
        model.setSelectClause(SelectClause.createWildcard());
        model.setFromClause(FromClause.create(
                FilterStream.create("SupportBean").addView("length", Expressions.constant(5))));
        model.setOutputLimitClause(OutputLimitClause.create(5));
        model.setRowLimitClause(RowLimitClause.create("myrows", "myoffset"));
        if (!EPL_KEYWORD_BODY.equals(model.toEPL())) {
            throw new IllegalStateException("SODA toEPL mismatch: " + model.toEPL());
        }
        model.setAnnotations(Collections.singletonList(AnnotationPart.nameAnnotation("s0")));
        EPStatementObjectModel copied = SerializableObjectCopier.copyMayFail(model);
        Module module = new Module();
        module.getItems().add(new ModuleItem(copied));
        module.setModuleText(copied.toEPL());
        return EPCompilerProvider.getCompiler().compile(module,
                new CompilerArguments(runtime.getRuntimePath()));
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
            requireFields(step, "op", "case", "eventType", "payload");
            if (!caseName.equals(string(step, "case"))) {
                throw new IllegalArgumentException("send case is not pinned");
            }
            JsonObject payload = object(step.get("payload"), "send payload");
            String eventType = string(step, "eventType");
            if ("SupportBean".equals(eventType)) {
                requireFields(payload, "theString", "intPrimitive");
                runtime.getEventService().sendEventBean(
                        new SupportBean(string(payload, "theString"), integer(payload, "intPrimitive")), eventType);
            } else if ("SupportBeanNumeric".equals(eventType)) {
                requireFields(payload, "intOne", "intTwo");
                runtime.getEventService().sendEventBean(
                        new SupportBeanNumeric(nullableInteger(payload, "intOne"), nullableInteger(payload, "intTwo")),
                        eventType);
            } else {
                throw new IllegalArgumentException("unsupported event type " + eventType);
            }
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
        validateStringArray(scenario.get("javaRuntimes"), new String[]{RUNTIME, RUNTIME, RUNTIME}, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), new String[]{EXECUTION, EXECUTION, EXECUTION}, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), new String[]{STATIC_ID, STATIC_ID, STATIC_ID}, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly three cases");
        }
        String[] observations = {"listener+iterator", "listener+iterator", "listener+iterator+soda"};
        String[] epls = {EPL_COMMA, EPL_KEYWORD, EPL_KEYWORD};
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation",
                    "iteratorSnapshots", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != 9
                    || !RUNTIME.equals(string(definition, "runtimeId"))
                    || !EXECUTION.equals(string(definition, "executionName"))
                    || !observations[index].equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 21
                    || !epls[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != CASES.length * 47) {
            throw new IllegalArgumentException("scenario must contain exactly 141 steps");
        }
        int index = 0;
        for (String caseName : CASES) {
            JsonObject marker = object(steps.get(index++), "case marker");
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op")) || !caseName.equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker is not pinned");
            }
            index = validateSnapshot(steps, index, caseName);
            index = validateSupportBean(steps, index, caseName, "E1", 1);
            for (int event = 2; event <= 6; event++) {
                index = validateSupportBean(steps, index, caseName, "E" + event, event);
                index = validateSnapshot(steps, index, caseName);
            }
            int[] first = {2, -1, 10, 6};
            int[] second = {3, 0, 0, 3};
            for (int event = 7; event <= 10; event++) {
                index = validateNumeric(steps, index, caseName, first[event - 7], second[event - 7]);
                index = validateSupportBean(steps, index, caseName, "E" + event, event);
                index = validateSnapshot(steps, index, caseName);
            }
            for (int probe = 0; probe < FINAL_ONE.length; probe++) {
                index = validateNumeric(steps, index, caseName,
                        FINAL_ONE[probe], FINAL_TWO[probe]);
                index = validateSnapshot(steps, index, caseName);
            }
        }
        if (index != steps.size()) {
            throw new IllegalArgumentException("scenario has trailing steps");
        }
    }

    private static int validateSnapshot(JsonArray steps, int index, String caseName) {
        JsonObject snapshot = object(steps.get(index++), "snapshot step");
        requireFields(snapshot, "op", "case", "statement");
        if (!"snapshot".equals(string(snapshot, "op"))
                || !caseName.equals(string(snapshot, "case"))
                || !"s0".equals(string(snapshot, "statement"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName);
        }
        return index;
    }

    private static int validateSupportBean(JsonArray steps, int index, String caseName,
                                           String expectedString, int expectedInt) {
        JsonObject send = object(steps.get(index++), "SupportBean send");
        requireFields(send, "op", "case", "eventType", "payload");
        JsonObject payload = object(send.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        if (!"send".equals(string(send, "op")) || !caseName.equals(string(send, "case"))
                || !"SupportBean".equals(string(send, "eventType"))
                || !expectedString.equals(string(payload, "theString"))
                || integer(payload, "intPrimitive") != expectedInt) {
            throw new IllegalArgumentException("SupportBean send is not pinned for " + caseName);
        }
        return index;
    }

    private static int validateNumeric(JsonArray steps, int index, String caseName,
                                       int expectedOne, int expectedTwo) {
        JsonObject send = object(steps.get(index++), "SupportBeanNumeric setter");
        requireFields(send, "op", "case", "eventType", "payload");
        JsonObject payload = object(send.get("payload"), "SupportBeanNumeric payload");
        requireFields(payload, "intOne", "intTwo");
        if (!"send".equals(string(send, "op")) || !caseName.equals(string(send, "case"))
                || !"SupportBeanNumeric".equals(string(send, "eventType"))) {
            throw new IllegalArgumentException("SupportBeanNumeric send is not pinned for " + caseName);
        }
        validateInteger(payload, "intOne", expectedOne);
        validateInteger(payload, "intTwo", expectedTwo);
        return index;
    }

    private static int validateNumeric(JsonArray steps, int index, String caseName,
                                       String expectedOne, String expectedTwo) {
        JsonObject send = object(steps.get(index++), "SupportBeanNumeric setter");
        requireFields(send, "op", "case", "eventType", "payload");
        JsonObject payload = object(send.get("payload"), "SupportBeanNumeric payload");
        requireFields(payload, "intOne", "intTwo");
        if (!"send".equals(string(send, "op")) || !caseName.equals(string(send, "case"))
                || !"SupportBeanNumeric".equals(string(send, "eventType"))) {
            throw new IllegalArgumentException("SupportBeanNumeric send is not pinned for " + caseName);
        }
        validateNullableInteger(payload, "intOne", expectedOne);
        validateNullableInteger(payload, "intTwo", expectedTwo);
        return index;
    }

    private static void validateInteger(JsonObject object, String name, int expected) {
        if (integer(object, name) != expected) {
            throw new IllegalArgumentException(name + " is not pinned");
        }
    }

    private static void validateNullableInteger(JsonObject object, String name, String expected) {
        JsonValue value = object.get(name);
        if (value == null) {
            throw new IllegalArgumentException(name + " is missing");
        }
        if (expected == null) {
            if (!value.isNull()) {
                throw new IllegalArgumentException(name + " must be JSON null");
            }
            return;
        }
        if (!(value instanceof JsonNumber) || !expected.equals(value.toString())) {
            throw new IllegalArgumentException(name + " is not pinned");
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

    private static void requireFields(JsonObject object, String... expected) {
        if (object == null || object.size() != expected.length
                || !new HashSet<>(object.names()).equals(new HashSet<>(Arrays.asList(expected)))) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
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

    private static Integer nullableInteger(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null) {
            throw new IllegalArgumentException(name + " is missing");
        }
        if (value.isNull()) {
            return null;
        }
        return integer(object, name);
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
        private int listenerCount;
        private int snapshotCount;

        private TraceWriter(JsonArray records, EPStatement statement, EPRuntime runtime, String caseName) {
            this.records = records;
            this.statement = statement;
            this.runtime = runtime;
            this.caseName = caseName;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            if (newEvents == null || newEvents.length != 2
                    || (oldEvents != null && oldEvents.length != 0)) {
                throw new IllegalStateException("unexpected variable row-limit listener shape for " + caseName);
            }
            listenerCount++;
            records.add(new JsonObject().add("case", caseName).add("operation", "listener")
                    .add("statement", statement.getName()).add("sequence", listenerCount)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
                    .add("new", rows(newEvents)));
        }

        private void snapshot(JsonObject step) {
            requireFields(step, "op", "case", "statement");
            if (!"snapshot".equals(string(step, "op"))
                    || !caseName.equals(string(step, "case"))
                    || !"s0".equals(string(step, "statement"))) {
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
            for (EventBean event : events) {
                result.add(row(event));
            }
            return result;
        }

        private static JsonObject row(EventBean event) {
            String[] names = event.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            JsonObject fields = new JsonObject();
            for (String name : names) {
                fields.add(name, normalize(event.get(name)));
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
