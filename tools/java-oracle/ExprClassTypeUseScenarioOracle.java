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
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the four ExprClassTypeUse executions
 * (ordinals 0-3) replayed as one differential chain:
 *
 * enum (ord 0, ExprClassTypeUseEnum): an inlined_class MyLevel enum whose
 * MEDIUM constant's getLevelCode() accessor is selected; c0=2.
 *
 * const (ord 1, ExprClassTypeConst): an inlined_class MyConstants whose
 * public static final VALUE field is selected; c0="test".
 *
 * inner-class (ord 2, ExprClassTypeInnerClass): an inlined_class
 * MyConstants whose nested MyInnerClass static VALUE field is selected
 * through the $ binary name; c0="abc".
 *
 * new-keyword (ord 3, ExprClassTypeNewKeyword): an inlined_class MyResult
 * instantiated per event by new MyResult(theString); the Java execution
 * asserts getId() reflectively, so the trace renders the MyResult instance
 * through getId() and c0="E1".
 *
 * Mirroring the regression harness, each case compiles and deploys the
 * pinned EPL verbatim (inlined_class triple-quote class text included),
 * attaches one s0 listener, sends one SupportBean("E1", 0), then
 * undeploys. The TraceWriter skips null/null listener callbacks; Integer,
 * String and Boolean values render as-is and a MyResult renders through
 * its getId() method like the reflective assertion.
 */
public final class ExprClassTypeUseScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "expr-class-type-use";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/clazz/ExprClassTypeUse.java";

    private static final String DESCRIPTION =
            "ExprClassTypeUse ordinals 0-3 (all executions): enum binds the inlined MyLevel.MEDIUM enum constant and calls getLevelCode(); const binds the MyConstants.VALUE public static final field; inner-class binds MyConstants$MyInnerClass.VALUE through the $ binary name; new-keyword instantiates new MyResult(theString) per event and asserts getId() reflectively. Each case deploys s0 once, sends one SupportBean(\"E1\", 0), then undeploys. Go has no inlined_class directive, so the EPL text is never replayed: class members bind as typed Go expressions (Literal/Func0/Construct) and the new-keyword result reads through GetId so both traces carry c0=\"E1\".";

    private static final String[] CASES = {
            "enum", "const", "inner-class", "new-keyword"};
    private static final int[] ORDINALS = {0, 1, 2, 3};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-c92db51ca175df9b4a5c",
            "java-runtime-8015ac40d3460b710377",
            "java-runtime-6daa4310cb2a1762a0dd",
            "java-runtime-d6258e053355ac85022b"
    };
    private static final String[] EXECUTIONS = {
            "ExprClassTypeUseEnum",
            "ExprClassTypeConst",
            "ExprClassTypeInnerClass",
            "ExprClassTypeNewKeyword"
    };
    private static final String[] STATIC_IDS = {
            "java-5d5ce4aa6c78b6bf048d",
            "java-f9f7cebbd7aac634448e",
            "java-a6911da1ff68fc3f399c",
            "java-13f14291ee7d14670c89"
    };
    private static final String[] OBSERVATIONS = {
            "listener; MyLevel.MEDIUM.getLevelCode() reads the inlined enum constant's accessor: SupportBean(\"E1\",0) yields c0=2",
            "listener; MyConstants.VALUE reads the inlined class's public static final field: SupportBean(\"E1\",0) yields c0=\"test\"",
            "listener; MyConstants$MyInnerClass.VALUE reads the nested class's static field through the $ binary name: SupportBean(\"E1\",0) yields c0=\"abc\"",
            "listener; new MyResult(theString) instantiates the inlined class per event and the Java execution asserts getId() reflectively: SupportBean(\"E1\",0) yields c0 rendered as \"E1\""
    };

    // Pinned EPL transcriptions (verbatim from ExprClassTypeUse.java,
    // including the escapeClass trailing space and newline).
    private static final String[] CASE_EPLS = {
            "inlined_class \"\"\"\n" +
                    "public enum MyLevel {\n" +
                    "  HIGH(3), MEDIUM(2), LOW(1);\n" +
                    "  final int levelCode;\n" +
                    "  MyLevel(int levelCode) {this.levelCode = levelCode;}\n" +
                    "  public int getLevelCode() {return levelCode;}\n" +
                    "}\"\"\" \n" +
                    "@name('s0') select MyLevel.MEDIUM.getLevelCode() as c0 from SupportBean",
            "inlined_class \"\"\"\n" +
                    "public class MyConstants {\n" +
                    "  public final static String VALUE = \"test\";\n" +
                    "}\"\"\" \n" +
                    "@name('s0') select MyConstants.VALUE as c0 from SupportBean",
            "inlined_class \"\"\"\n" +
                    "public class MyConstants {\n" +
                    "  public static class MyInnerClass {" +
                    "    public final static String VALUE = \"abc\";\n" +
                    "  }" +
                    "}\"\"\" \n" +
                    "@name('s0') select MyConstants$MyInnerClass.VALUE as c0 from SupportBean",
            "inlined_class \"\"\"\n" +
                    "public class MyResult {\n" +
                    "  private final String id;\n" +
                    "  public MyResult(String id) {this.id = id;}\n" +
                    "  public String getId() {return id;}\n" +
                    "}\"\"\" \n" +
                    "@name('s0') select new MyResult(theString) as c0 from SupportBean"
    };

    private static final int EXPECTED_STEPS = 16;
    private static final int EXPECTED_RECORDS = 4;

    private ExprClassTypeUseScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ExprClassTypeUseScenarioOracle <scenario.json>");
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
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS
                    + " records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", SCENARIO_ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, String caseName, String runtimeId,
                                int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);

        String runtimeURI = "parity-" + SCENARIO_ID + "-" + runtimeId;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            TraceWriter writer = new TraceWriter(records, caseName, runtime);
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
                if ("deploy".equals(operation)) {
                    // The pinned EPL is compiled verbatim: the inlined_class
                    // triple-quote class text plus the @name('s0') select.
                    EPCompiled compiled = EPCompilerProvider.getCompiler()
                            .compile(CASE_EPLS[caseIndex], new CompilerArguments(configuration));
                    EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                            new DeploymentOptions().setDeploymentId(SCENARIO_ID + "-" + caseIndex));
                    findStatement(deployment).addListener(writer);
                } else if ("undeploy-all".equals(operation)) {
                    runtime.getDeploymentService().undeployAll();
                } else if ("send".equals(operation)) {
                    sendEvent(runtime, step);
                } else {
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
        if (!"SupportBean".equals(eventType)) {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
        JsonObject payload = step.get("payload").asObject();
        requireFields(payload, "theString", "intPrimitive");
        runtime.getEventService().sendEventBean(
                new SupportBean(payload.getString("theString", null),
                        payload.getInt("intPrimitive", 0)),
                eventType);
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !SCENARIO_ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTIONS, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly four cases");
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
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: each case marker is followed by the
     * case's pinned steps — deploy s0, the single SupportBean send and
     * undeploy-all. Unknown step fields are rejected.
     */
    private static void validateSteps(JsonArray steps) {
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + EXPECTED_STEPS + " steps, got " + steps.size());
        }
        String[] details = {"deploy", "send", "undeploy-all"};
        int cursor = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(cursor), "case marker " + cursor);
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op")) || !CASES[caseIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker " + cursor + " is not pinned");
            }
            cursor++;
            for (String operation : details) {
                JsonObject step = object(steps.get(cursor), "step " + cursor);
                if (!operation.equals(string(step, "op"))
                        || !CASES[caseIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("step " + cursor + " is not pinned");
                }
                switch (operation) {
                    case "deploy":
                        requireFields(step, "op", "case", "statement");
                        if (!"s0".equals(string(step, "statement"))) {
                            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
                        }
                        break;
                    case "send":
                        requireFields(step, "op", "case", "eventType", "payload");
                        if (!"SupportBean".equals(string(step, "eventType"))) {
                            throw new IllegalArgumentException("send step " + cursor + " is not pinned");
                        }
                        JsonObject payload = object(step.get("payload"), "send payload " + cursor);
                        requireFields(payload, "theString", "intPrimitive");
                        if (!"E1".equals(string(payload, "theString"))
                                || integer(payload, "intPrimitive") != 0) {
                            throw new IllegalArgumentException("send payload " + cursor + " is not pinned");
                        }
                        break;
                    case "undeploy-all":
                        requireFields(step, "op", "case");
                        break;
                    default:
                        throw new IllegalArgumentException("unsupported operation at step " + cursor);
                }
                cursor++;
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
        long value = longNumber(object, name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        return (int) value;
    }

    private static long longNumber(JsonObject object, String name) {
        if (!(object.get(name) instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        String text = object.get(name).toString();
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
                    .add("sequence", ++sequence)
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

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
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
            if ("MyResult".equals(value.getClass().getSimpleName())) {
                // The Java execution asserts the new-keyword result
                // reflectively (getId() == "E1"); the trace renders the
                // instance through the same getter so both hosts carry
                // c0="E1".
                try {
                    Method getId = value.getClass().getMethod("getId");
                    return Json.value(String.valueOf(getId.invoke(value)));
                } catch (ReflectiveOperationException ex) {
                    throw new IllegalStateException("MyResult getId() is not readable", ex);
                }
            }
            return Json.value(String.valueOf(value));
        }
    }
}
