import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_A;
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
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for ResultSetOutputLimitRowPerGroup ordinals
 * 0/36/37/38 (the output-first cluster over a named-window grouped source).
 * Each Java execution is one scenario case; the first-having case runs the
 * four EPL variants (plain, join, order-by, order-by-join) as sequential
 * deploy/undeploy cycles inside one runtime, matching the single Java
 * execution.  SupportBean/SupportBean_A are the shared regression beans;
 * SupportMarketDataBean is a map type because the regression-lib jar is not
 * on the oracle classpath.  The oracle replays the scenario steps verbatim:
 * deploy "all" builds the infra EPL plus the case s0 EPL in one module;
 * deploy "infra"/"s0"/"s0-var" build separate modules so the s0 undeploy
 * leaves the named window populated (mirroring undeployModuleContaining).
 */
public final class ResultSetOutputLimitRowPerGroupFirstScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-output-limit-row-per-group-first";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowPerGroup.java";
    private static final String DESCRIPTION =
            "ResultSetOutputLimitRowPerGroup ordinals 0/36/37/38: output first over a named-window grouped source";

    private static final String[] CASES = {
            "first-when-then", "first-having", "first-crontab", "first-every-n"};
    private static final int[] ORDINALS = {0, 36, 37, 38};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-ebcdaaea9afc00a686d0",
            "java-runtime-e499360495e769be65ec",
            "java-runtime-198f226a1080d3e02cf0",
            "java-runtime-97e7a2a759efa357a2af"
    };
    private static final String[] EXECUTIONS = {
            "ResultSetOutputLimitRowPerGroup$ResultSetOutputFirstWhenThen",
            "ResultSetOutputLimitRowPerGroup$ResultSetOutputFirstHavingJoinNoJoin",
            "ResultSetOutputLimitRowPerGroup$ResultSetOutputFirstCrontab",
            "ResultSetOutputLimitRowPerGroup$ResultSetOutputFirstEveryNEvents"
    };
    private static final String[] STATIC_IDS = {
            "java-7f81a475c41fed0a0174",
            "java-54ea6581114dde53dd15",
            "java-f46b70b32c0f05f9d1b1",
            "java-494e8f25a4614f7880af"
    };
    private static final String[] BASE_EPLS = {
            "@name('s0') select theString, sum(intPrimitive) as value from MyWindow group by theString output first when varoutone then set varoutone = false",
            "@name('s0') select theString, sum(intPrimitive) as value from MyWindow group by theString having sum(intPrimitive) > 20 output first every 2 events",
            "@name('s0') select theString, sum(intPrimitive) as value from MyWindow group by theString output first at (*/2, *, *, *, *)",
            "@name('s0') select theString, sum(intPrimitive) as value from MyWindow group by theString output first every 3 events"
    };

    private static final String INFRA_EPL =
            "create window MyWindow#keepall as SupportBean;\n" +
            "insert into MyWindow select * from SupportBean;\n" +
            "on SupportMarketDataBean md delete from MyWindow mw where mw.intPrimitive = md.price;\n";
    private static final String INFRA_PUBLIC_EPL =
            "@public create window MyWindow#keepall as SupportBean;\n" +
            "insert into MyWindow select * from SupportBean;\n" +
            "on SupportMarketDataBean md delete from MyWindow mw where mw.intPrimitive = md.price;\n";
    private static final String[] HAVING_EPLS = {
            "@name('s0') select theString, sum(intPrimitive) as value from MyWindow group by theString having sum(intPrimitive) > 20 output first every 2 events",
            "@name('s0') select theString, sum(intPrimitive) as value from MyWindow mv, SupportBean_A#keepall a where a.id = mv.theString group by theString having sum(intPrimitive) > 20 output first every 2 events",
            "@name('s0') select theString, sum(intPrimitive) as value from MyWindow group by theString having sum(intPrimitive) > 20 output first every 2 events order by theString asc",
            "@name('s0') select theString, sum(intPrimitive) as value from MyWindow mv, SupportBean_A#keepall a where a.id = mv.theString group by theString having sum(intPrimitive) > 20 output first every 2 events order by theString asc"
    };
    private static final String EVERY_N_VAR_EPL =
            "@name('s0') select theString, sum(intPrimitive) as value from MyWindow group by theString output first every myvar_local events";

    private ResultSetOutputLimitRowPerGroupFirstScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOutputLimitRowPerGroupFirstScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBean_A.class);
        Map<String, Object> marketType = new HashMap<>();
        marketType.put("symbol", String.class);
        marketType.put("price", Double.class);
        marketType.put("volume", Long.class);
        marketType.put("feed", String.class);
        configuration.getCommon().addEventType("SupportMarketDataBean", marketType);
        configuration.getCommon().addVariable("varoutone", boolean.class, false);
        configuration.getCommon().addVariable("myvar_local", int.class, 1);

        String runtimeURI = "parity-" + ID + "-" + runtimeId;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        try {
            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            boolean active = false;
            int deployIndex = 0;
            Map<String, EPDeployment> deployments = new HashMap<>();
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
                if ("advance-time".equals(operation)) {
                    long at = Instant.parse(step.getString("at", "")).toEpochMilli();
                    runtime.getEventService().advanceTime(at);
                } else if ("deploy".equals(operation)) {
                    String statement = step.getString("statement", "");
                    String epl;
                    if ("all".equals(statement)) {
                        epl = INFRA_EPL + s0Epl(caseName, deployIndex);
                        deployIndex++;
                    } else if ("infra".equals(statement)) {
                        epl = INFRA_PUBLIC_EPL;
                    } else if ("s0".equals(statement)) {
                        epl = s0Epl(caseName, deployIndex);
                        deployIndex++;
                    } else if ("s0-var".equals(statement)) {
                        epl = EVERY_N_VAR_EPL;
                        deployIndex++;
                    } else {
                        throw new IllegalArgumentException("unexpected deploy statement " + statement);
                    }
                    EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                            new CompilerArguments(runtime.getRuntimePath()));
                    EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                            new DeploymentOptions().setDeploymentId(ID + "-" + caseIndex + "-" + statement + "-" + deployIndex));
                    deployments.put(statement, deployment);
                    EPStatement s0 = findStatement(deployment);
                    if (s0 != null) {
                        s0.addListener(writer);
                    }
                } else if ("undeploy".equals(operation)) {
                    String statement = step.getString("statement", "");
                    EPDeployment deployment = deployments.remove(statement);
                    if (deployment == null) {
                        throw new IllegalArgumentException("undeploy of unknown statement " + statement);
                    }
                    runtime.getDeploymentService().undeploy(deployment.getDeploymentId());
                } else if ("undeploy-all".equals(operation)) {
                    runtime.getDeploymentService().undeployAll();
                    deployments.clear();
                } else if ("set-variable".equals(operation)) {
                    String name = step.getString("name", "");
                    JsonValue payload = step.get("payload");
                    Object value;
                    if (payload.isBoolean()) {
                        value = payload.asBoolean();
                    } else if (payload.isNumber()) {
                        value = payload.asInt();
                    } else {
                        throw new IllegalArgumentException("unsupported variable payload " + payload);
                    }
                    runtime.getVariableService().setVariableValue(null, name, value);
                } else if ("send".equals(operation)) {
                    sendEvent(runtime, step);
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static String s0Epl(String caseName, int deployIndex) {
        switch (caseName) {
            case "first-when-then":
                return BASE_EPLS[0];
            case "first-having":
                return HAVING_EPLS[deployIndex];
            case "first-crontab":
                return BASE_EPLS[2];
            case "first-every-n":
                return BASE_EPLS[3];
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
            runtime.getEventService().sendEventBean(bean, "SupportBean");
        } else if ("SupportBean_A".equals(eventType)) {
            runtime.getEventService().sendEventBean(new SupportBean_A(payload.getString("id", null)), "SupportBean_A");
        } else if ("SupportMarketDataBean".equals(eventType)) {
            Map<String, Object> event = new HashMap<>();
            event.put("symbol", payload.getString("symbol", null));
            event.put("price", payload.getDouble("price", 0));
            event.put("volume", payload.get("volume") == null || payload.get("volume").isNull()
                    ? null : payload.get("volume").asLong());
            event.put("feed", payload.get("feed") == null || payload.get("feed").isNull()
                    ? null : payload.get("feed").asString());
            runtime.getEventService().sendEventMap(event, "SupportMarketDataBean");
        } else {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
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
                    || !"listener".equals(string(definition, "observation"))
                    || !BASE_EPLS[index].equals(string(definition, "epl"))) {
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
    }
}
