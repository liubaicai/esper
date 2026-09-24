import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBean_ST0;
import com.espertech.esper.regressionlib.support.bean.SupportBean_ST0_Container;
import com.espertech.esper.regressionlib.support.bean.SupportCollection;
import com.espertech.esper.regressionlib.suite.expr.enummethod.ExprEnumMinMax;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.math.BigDecimal;
import java.math.BigInteger;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.Collections;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the three remaining ExprEnumSelectFrom
 * executions (ordinals 1, 3 and 4) replayed as one differential chain:
 *
 * events-windex-wsize (ord 1, ExprEnumSelectFromEventsWIndexWSize):
 * contained.selectFrom over SupportBean_ST0_Container.contained projecting
 * new{v0=v.id,v1=i} and new{v0=v.id,v1=i + 100*s} anonymous map rows with
 * the element+index and element+index+size lambda footprints; a null
 * contained yields null columns and an empty contained yields present
 * empty collections.
 *
 * scalar-plain (ord 3, ExprEnumSelectFromScalarPlain): strvals.selectFrom
 * over SupportCollection.strvals through the extractNum plug-in single-row
 * function (ExprEnumMinMax.MyService semantics: Integer.parseInt of the
 * value after its leading marker).
 *
 * scalar-windex-wsize (ord 4, ExprEnumSelectFromScalarWIndexWSize):
 * strvals.selectFrom concatenating the element with its index (c0) and
 * with index and input size (c1) through Integer.toString.
 *
 * Mirroring SupportEvalRunner, each case deploys "@name('s0') <case EPL>"
 * once, sends every assertion event, then undeploys. The pinned case EPL
 * is the contract text verbatim (explicit "as cN" aliases, double space in
 * "selectFrom( (v, i)", no-space assignments in "new {v0=v.id,v1=i}"). The
 * regression-lib support beans are on the oracle classpath, so the event
 * types register as bean types and sends deliver sendEventBean like the
 * suite; extractNum registers against ExprEnumMinMax.MyService exactly as
 * TestSuiteExprEnum does. selectFrom results render as JSON arrays of
 * plain JSON objects (Map rows) or scalars (Integer/String collections);
 * null renders {"state":"null"} and an empty collection renders [].
 * The TraceWriter skips null/null listener callbacks.
 */
public final class ExprEnumSelectFromScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "expr-enum-select-from";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumSelectFrom.java";

    private static final String DESCRIPTION =
            "ExprEnumSelectFrom ordinals 1, 3 and 4 (the remaining executions): events-windex-wsize projects SupportBean_ST0_Container.contained into new{v0,v1} map rows with element/index/size lambda footprints; scalar-plain projects SupportCollection.strvals through the extractNum plug-in single-row function; scalar-windex-wsize projects strvals into index- and size-suffixed strings through Integer.toString concatenation. Null collections yield null columns and empty collections yield present empty results. Each case deploys s0 once, sends every assertion event, then undeploys.";

    private static final String[] CASES = {
            "events-windex-wsize", "scalar-plain", "scalar-windex-wsize"};
    private static final int[] ORDINALS = {1, 3, 4};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-54fc91f276056a1d66be",
            "java-runtime-f8cd483364f4584c1bba",
            "java-runtime-754f50455eb82b9e64a7"
    };
    private static final String[] EXECUTIONS = {
            "ExprEnumSelectFromEventsWIndexWSize",
            "ExprEnumSelectFromScalarPlain",
            "ExprEnumSelectFromScalarWIndexWSize"
    };
    private static final String[] STATIC_IDS = {
            "java-952dc3d4e9027831151a",
            "java-4b6f2e5fc3f78b140fdb",
            "java-0a95beaf68eb949bdc86"
    };
    private static final String[] OBSERVATIONS = {
            "listener; contained.selectFrom projects SupportBean_ST0 elements into new{v0,v1} map rows: c0 carries the element index, c1 carries i + 100*s; a null contained yields null columns and an empty contained yields present empty collections",
            "listener; strvals.selectFrom applies the extractNum plug-in single-row function (Integer.parseInt of the value after its leading marker); a null strvals yields null and an empty strvals yields a present empty collection",
            "listener; strvals.selectFrom concatenates the element with its index (c0) and with index and input size (c1) through Integer.toString; a null strvals yields null columns and an empty strvals yields present empty collections"
    };
    private static final String[] CASE_EPLS = {
            "select contained.selectFrom( (v, i) => new {v0=v.id,v1=i}) as c0, contained.selectFrom( (v, i, s) => new {v0=v.id,v1=i + 100*s}) as c1 from SupportBean_ST0_Container",
            "select strvals.selectFrom(v => extractNum(v)) as c0 from SupportCollection",
            "select strvals.selectFrom( (v, i) => v || '_' || Integer.toString(i)) as c0, strvals.selectFrom( (v, i, s) => v || '_' || Integer.toString(i) || '_' || Integer.toString(s)) as c1 from SupportCollection"
    };

    private static final int EXPECTED_STEPS = 21;
    private static final int EXPECTED_RECORDS = 12;

    private ExprEnumSelectFromScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ExprEnumSelectFromScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBean_ST0_Container.class);
        configuration.getCommon().addEventType(SupportBean_ST0.class);
        configuration.getCommon().addEventType(SupportCollection.class);
        configuration.getCompiler().addPlugInSingleRowFunction("extractNum",
                ExprEnumMinMax.MyService.class.getName(), "extractNum");

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
                    String epl = "@name('s0') " + CASE_EPLS[caseIndex];
                    // CompilerArguments(configuration) carries the compiler-level
                    // plug-in single-row function (extractNum); the runtime path
                    // does not (ContextHashScenarioOracle precedent).
                    EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                            new CompilerArguments(configuration));
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
        JsonObject payload = step.get("payload").asObject();
        if ("SupportBean_ST0_Container".equals(eventType)) {
            JsonValue contained = payload.get("contained");
            if (contained == null || contained.isNull()) {
                // SupportBean_ST0_Container.make3ValueNull().
                runtime.getEventService().sendEventBean(
                        new SupportBean_ST0_Container(null), eventType);
                return;
            }
            JsonArray items = contained.asArray();
            List<SupportBean_ST0> list = new ArrayList<>();
            for (int index = 0; index < items.size(); index++) {
                JsonObject bean = items.get(index).asObject();
                // SupportBean_ST0_Container.make3Value: each triplet is
                // id,key0,p00 through the three-arg constructor.
                list.add(new SupportBean_ST0(bean.getString("id", null),
                        bean.getString("key0", null), bean.getInt("p00", 0)));
            }
            runtime.getEventService().sendEventBean(new SupportBean_ST0_Container(list), eventType);
        } else if ("SupportCollection".equals(eventType)) {
            JsonValue strvals = payload.get("strvals");
            SupportCollection bean = new SupportCollection();
            if (strvals == null || strvals.isNull()) {
                // SupportCollection.makeString(null): toListString returns
                // null for both strvals and strvalstwo.
                bean.setStrvals(null);
                bean.setStrvalstwo(null);
            } else {
                JsonArray items = strvals.asArray();
                List<String> list = new ArrayList<>();
                for (int index = 0; index < items.size(); index++) {
                    JsonValue item = items.get(index);
                    // SupportCollection.toListString maps the literal "null"
                    // element to a null entry.
                    list.add(item.isNull() ? null : item.asString());
                }
                // makeString assigns the same parsed list to both fields.
                bean.setStrvals(list);
                bean.setStrvalstwo(list);
            }
            runtime.getEventService().sendEventBean(bean, eventType);
        } else {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
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
            throw new IllegalArgumentException("scenario must contain exactly three cases");
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
     * case's pinned steps — deploy s0, the four assertion sends and
     * undeploy-all. Unknown step fields are rejected.
     */
    private static void validateSteps(JsonArray steps) {
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + EXPECTED_STEPS + " steps, got " + steps.size());
        }
        String[][] details = {
                {"deploy", "send", "send", "send", "send", "undeploy-all"},
                {"deploy", "send", "send", "send", "send", "undeploy-all"},
                {"deploy", "send", "send", "send", "send", "undeploy-all"},
        };
        String[][] eventTypes = {
                {"SupportBean_ST0_Container"},
                {"SupportCollection"},
                {"SupportCollection"},
        };
        int cursor = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(cursor), "case marker " + cursor);
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op")) || !CASES[caseIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker " + cursor + " is not pinned");
            }
            cursor++;
            for (String operation : details[caseIndex]) {
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
                        if (!eventTypes[caseIndex][0].equals(string(step, "eventType"))) {
                            throw new IllegalArgumentException("send step " + cursor + " is not pinned");
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
            if (value instanceof BigDecimal) {
                return Json.value(((BigDecimal) value).toPlainString());
            }
            if (value instanceof BigInteger) {
                return Json.value(value.toString());
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
            if (value instanceof Map<?, ?> map) {
                // selectFrom new{...} rows render as plain JSON objects,
                // matching the Go []map[string]any normalization.
                JsonObject fields = new JsonObject();
                List<String> names = new ArrayList<>();
                for (Object key : map.keySet()) {
                    names.add(String.valueOf(key));
                }
                Collections.sort(names);
                for (String name : names) {
                    fields.add(name, normalize(map.get(name)));
                }
                return fields;
            }
            if (value instanceof Collection<?> collection) {
                // selectFrom collections render as JSON arrays: Map rows as
                // objects, scalar elements (Integer/String) as values.
                JsonArray array = new JsonArray();
                for (Object element : collection) {
                    array.add(normalize(element));
                }
                return array;
            }
            if (value instanceof Object[] objects) {
                JsonArray array = new JsonArray();
                for (Object object : objects) {
                    array.add(normalize(object));
                }
                return array;
            }
            if (value instanceof Integer || value instanceof Long || value instanceof Short
                    || value instanceof Byte) {
                return Json.value(((Number) value).longValue());
            }
            if (value instanceof Number) {
                double number = ((Number) value).doubleValue();
                if (number == Math.rint(number) && !Double.isInfinite(number)) {
                    return Json.value((long) number);
                }
                return Json.value(number);
            }
            if (value instanceof Boolean) {
                return Json.value((Boolean) value);
            }
            if (value instanceof Character character) {
                return Json.value(String.valueOf(character));
            }
            return Json.value(String.valueOf(value));
        }
    }
}
