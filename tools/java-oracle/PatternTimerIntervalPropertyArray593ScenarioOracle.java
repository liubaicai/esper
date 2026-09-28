import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.math.BigDecimal;
import java.math.BigInteger;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.Map;
import java.util.TreeSet;

/**
 * Direct Esper 9.0.0 oracle for the pattern-timer-interval-property-
 * array-593 unit: PatternObserverTimerInterval ord 7
 * PatternIntervalSpecExpressionWithPropertyArray
 * (java-runtime-6155a2b0181e2169f2a4, static
 * java-1422565b568236b2bfea, flags []). The interval spec reads the
 * repeated-tag array `a` produced by `[2] a=SupportBean`:
 * `a[0].intPrimitive+a[1].intPrimitive seconds` is evaluated once when
 * the repeat completes (TimerIntervalObserverFactory
 * computeDelta(beginState)), not at deploy and not per-tick.
 *
 * The pinned sequence mirrors the Java source verbatim: advanceTime(0),
 * compileDeploy the pinned EPL and attach the s0 listener,
 * advanceTime(10000), sendEventBean E1 (intPrimitive=3 — the first
 * `[2]` element is captured but the observer is not armed yet),
 * sendEventBean E2 (intPrimitive=2 — the repeat completes and arms a
 * 5000ms observer at the completion instant), advanceTime(14999) must
 * deliver nothing (strict < boundary), milestone(0) is a savepoint
 * carrying no step, advanceTime(15000) delivers exactly one row
 * {a0id:E1, a1id:E2}, undeployAll.
 *
 * The trace carries only the listener records; the advance-time and
 * send steps produce none by construction.
 */
public final class PatternTimerIntervalPropertyArray593ScenarioOracle {

    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "pattern-timer-interval-property-array-593";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternObserverTimerInterval.java";
    private static final String CASE = "property-array";

    // Byte-exact EPL (PatternObserverTimerInterval.java:319 verbatim —
    // the space after `pattern [` is pinned).
    private static final String EPL = "@name('s0') select a[0].theString as a0id, a[1].theString as a1id "
            + "from pattern [ [2] a=SupportBean -> timer:interval(a[0].intPrimitive+a[1].intPrimitive seconds)]";

    private static final int EXPECTED_STEPS = 8; // case + 7 pinned ops
    private static final int EXPECTED_RECORDS = 1;

    private PatternTimerIntervalPropertyArray593ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: PatternTimerIntervalPropertyArray593ScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                SCENARIO_ID + "-" + CASE, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);

        JsonArray records = new JsonArray();
        try {
            EPStatement statement = null;
            for (JsonValue stepValue : scenario.get("steps").asArray()) {
                JsonObject step = stepValue.asObject();
                String operation = step.getString("op", "");
                switch (operation) {
                    case "case":
                        break;
                    case "advance-time":
                        runtime.getEventService().advanceTime(
                                Instant.parse(step.getString("at", "")).toEpochMilli());
                        break;
                    case "deploy": {
                        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                                EPL, new CompilerArguments(runtime.getRuntimePath()));
                        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
                        if (deployment.getStatements().length != 1
                                || !"s0".equals(deployment.getStatements()[0].getName())) {
                            throw new IllegalStateException("expected one s0 statement");
                        }
                        statement = deployment.getStatements()[0];
                        statement.addListener(new TraceWriter(records, statement, runtime));
                        break;
                    }
                    case "send":
                        runtime.getEventService().advanceTime(
                                Instant.parse(step.getString("at", "")).toEpochMilli());
                        JsonObject payload = step.get("payload").asObject();
                        runtime.getEventService().sendEventBean(
                                new SupportBean(payload.getString("theString", ""),
                                        payload.get("intPrimitive").asInt()), "SupportBean");
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }

        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS
                    + " records, got " + records.size());
        }
        JsonObject record = records.get(0).asObject();
        if (!"1970-01-01T00:00:15Z".equals(record.getString("time", ""))) {
            throw new IllegalStateException("firing time is not the pinned 15000ms boundary: "
                    + record);
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", SCENARIO_ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaSourceFiles", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags",
                "cases", "steps");
        if (!VERSION.equals(scenario.getString("version", ""))
                || !SCENARIO_ID.equals(scenario.getString("id", ""))
                || !JAVA_COMMIT.equals(scenario.getString("javaCommit", ""))
                || !JAVA_SOURCE.equals(scenario.getString("javaSource", ""))) {
            throw new IllegalStateException("scenario metadata is not pinned");
        }
        if (scenario.get("cases").asArray().size() != 1) {
            throw new IllegalStateException("scenario must carry exactly one case");
        }
        JsonObject definition = scenario.get("cases").asArray().get(0).asObject();
        requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                "observation", "epls");
        if (!CASE.equals(definition.getString("case", ""))
                || definition.get("ordinal").asInt() != 7
                || !"java-runtime-6155a2b0181e2169f2a4".equals(definition.getString("runtimeId", ""))
                || !"PatternIntervalSpecExpressionWithPropertyArray".equals(
                        definition.getString("executionName", ""))
                || definition.get("epls").asArray().size() != 1
                || !EPL.equals(definition.get("epls").asArray().get(0).asString())) {
            throw new IllegalStateException("scenario case definition is not pinned");
        }
        JsonArray steps = scenario.get("steps").asArray();
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalStateException("expected " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        String[] expectedOps = {"case", "advance-time", "deploy", "send", "send",
                "advance-time", "advance-time", "undeploy-all"};
        for (int index = 0; index < expectedOps.length; index++) {
            JsonObject step = steps.get(index).asObject();
            if (!expectedOps[index].equals(step.getString("op", ""))) {
                throw new IllegalStateException("step " + index + " op is not "
                        + expectedOps[index]);
            }
            if ("deploy".equals(expectedOps[index])
                    && !EPL.equals(step.getString("epl", ""))) {
                throw new IllegalStateException("deploy step EPL is not pinned");
            }
        }
    }

    private static void requireFields(JsonObject object, String... expectedNames) {
        if (!new TreeSet<>(object.names()).equals(new TreeSet<>(Arrays.asList(expectedNames)))) {
            throw new IllegalStateException("object fields are not the pinned set");
        }
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored,
                           EPRuntime ignoredRuntime) {
            if ((newEvents == null || newEvents.length == 0)
                    && (oldEvents == null || oldEvents.length == 0)) {
                return;
            }
            JsonObject record = new JsonObject()
                    .add("case", CASE)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            JsonArray oldRows = rows(oldEvents);
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                output.add(row(event));
            }
            return output;
        }
    }

    private static JsonObject row(EventBean event) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        JsonObject values = new JsonObject();
        for (String name : new TreeSet<>(Arrays.asList(event.getEventType().getPropertyNames()))) {
            values.add(name, normalize(event.get(name)));
        }
        item.add("fields", values);
        return item;
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value instanceof Double && ((Double) value).isNaN()
                || value instanceof Float && ((Float) value).isNaN()) {
            return new JsonObject().add("state", "nan");
        }
        if (value instanceof BigDecimal) {
            return Json.value(((BigDecimal) value).toPlainString());
        }
        if (value instanceof BigInteger) {
            return Json.value(value.toString());
        }
        if (value instanceof EventBean) {
            return normalize(((EventBean) value).getUnderlying());
        }
        if (value instanceof Object[]) {
            JsonArray array = new JsonArray();
            for (Object item : (Object[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof Iterable<?>) {
            JsonArray array = new JsonArray();
            for (Object item : (Iterable<?>) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof Map<?, ?>) {
            TreeSet<String> keys = new TreeSet<>();
            Map<?, ?> mapValue = (Map<?, ?>) value;
            for (Object key : mapValue.keySet()) {
                keys.add(String.valueOf(key));
            }
            JsonObject object = new JsonObject();
            for (String key : keys) {
                object.add(key, normalize(mapValue.get(key)));
            }
            return object;
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
        if (value instanceof Character) {
            return Json.value(String.valueOf(value));
        }
        return Json.value(String.valueOf(value));
    }

    /** Mirrors SupportExceptionHandlerFactoryRethrow from the regression harness. */
    public static class HarnessRethrowExceptionHandlerFactory implements ExceptionHandlerFactory {
        @Override
        public ExceptionHandler getHandler(ExceptionHandlerFactoryContext context) {
            return handlerContext -> {
                throw new RuntimeException("Unexpected exception in statement '"
                        + handlerContext.getStatementName() + "': "
                        + handlerContext.getThrowable().getMessage(),
                        handlerContext.getThrowable());
            };
        }
    }
}
