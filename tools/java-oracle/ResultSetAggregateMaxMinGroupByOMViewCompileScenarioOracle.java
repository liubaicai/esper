import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
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
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Direct Esper oracle for the OM and ViewCompile executions.
 */
public final class ResultSetAggregateMaxMinGroupByOMViewCompileScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "resultset-aggregate-minmax-groupby-om-viewcompile";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateMaxMinGroupBy.java";
    private static final String MINMAX_OM = "minmax-om";
    private static final String MINMAX_VIEW_COMPILE = "minmax-view-compile";
    private static final String[] CASES = {MINMAX_OM, MINMAX_VIEW_COMPILE};
    private static final int[] ORDINALS = {1, 2};
    private static final String[] RUNTIME_IDS = {"java-runtime-2408009c4113ea2214c4", "java-runtime-30dbed587648347e610a"};
    private static final String[] EXECUTION_NAMES = {"ResultSetAggregateMinMaxOM", "ResultSetAggregateMinMaxViewCompile"};
    private static final int[] SEND_COUNTS = {12, 12};
    private static final int[] MINMAX_CALLBACK_ROW_COUNTS = {1, 1, 1, 1, 1, 2, 2, 2, 1, 1, 1, 1};
    private static final int[] RECORD_COUNTS = {12, 12};
    private static final String[] SYMBOLS = {"DELL", "DELL", "DELL", "DELL", "DELL", "IBM", "IBM", "IBM", "IBM", "IBM", "IBM", "IBM"};
    private static final Long[] VOLUMES = {50L, 30L, 30L, 90L, 100L, 20L, 5L, 15L, 18L, null, null, null};

    private ResultSetAggregateMaxMinGroupByOMViewCompileScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ResultSetAggregateMaxMinGroupByOMViewCompileScenarioOracle <scenario.json>");
            System.exit(2);
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        validateScenario(scenario);
        JsonArray steps = scenario.get("steps").asArray();
        JsonArray records = new JsonArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) runCase(steps, caseIndex, records);
        if (records.size() != 24) throw new IllegalStateException("expected 24 listener records, got " + records.size());
        JsonObject trace = new JsonObject().add("version", VERSION).add("id", SCENARIO_ID)
                .add("javaCommit", PINNED_COMMIT).add("java", System.getProperty("java.version")).add("records", records);
        System.out.println(trace);
    }

    private static void validateScenario(JsonObject scenario) {
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        if (!SCENARIO_ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("unsupported scenario id: " + scenario.getString("id", ""));
        }
        if (!PINNED_COMMIT.equals(scenario.getString("javaCommit", ""))) {
            throw new IllegalArgumentException("scenario javaCommit is not pinned");
        }
        if (!JAVA_SOURCE.equals(scenario.getString("javaSource", ""))) {
            throw new IllegalArgumentException("scenario javaSource is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");

        JsonValue caseValue = scenario.get("cases");
        if (caseValue == null || !caseValue.isArray() || caseValue.asArray().size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain the two selected cases");
        }
        JsonArray caseDefinitions = caseValue.asArray();
        for (int i = 0; i < CASES.length; i++) {
            JsonObject definition = object(caseDefinitions.get(i), "case definition " + i);
            if (!CASES[i].equals(definition.getString("case", ""))
                    || definition.getInt("ordinal", -1) != ORDINALS[i]
                    || !RUNTIME_IDS[i].equals(definition.getString("runtimeId", ""))
                    || !EXECUTION_NAMES[i].equals(definition.getString("executionName", ""))
                    || !"listener".equals(definition.getString("observation", ""))
                    || !eplFor(CASES[i]).equals(definition.getString("epl", ""))) {
                throw new IllegalArgumentException("case metadata mismatch at index " + i);
            }
        }

        JsonValue stepValue = scenario.get("steps");
        if (stepValue == null || !stepValue.isArray() || stepValue.asArray().size() != 26) {
            throw new IllegalArgumentException("scenario must contain exactly 26 steps");
        }
        JsonArray steps = stepValue.asArray();
        int stepIndex = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(stepIndex++), "case marker " + caseIndex);
            if (!"case".equals(marker.getString("op", "")) || !CASES[caseIndex].equals(marker.getString("case", ""))) {
                throw new IllegalArgumentException("cases must appear once in source order");
            }
            for (int eventIndex = 0; eventIndex < SEND_COUNTS[caseIndex]; eventIndex++) {
                JsonObject step = object(steps.get(stepIndex), "send step " + stepIndex);
                validateMarketSend(step, SYMBOLS[eventIndex], VOLUMES[eventIndex], stepIndex);
                stepIndex++;
            }
        }
        if (stepIndex != steps.size()) throw new IllegalArgumentException("scenario contains trailing steps");
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        if (value == null || !value.isArray() || value.asArray().size() != expected.length) {
            throw new IllegalArgumentException(name + " must contain exactly " + expected.length + " values");
        }
        JsonArray values = value.asArray();
        for (int i = 0; i < expected.length; i++) {
            if (!expected[i].equals(values.get(i).asString())) {
                throw new IllegalArgumentException(name + " mismatch at index " + i);
            }
        }
    }

    private static void validateMarketSend(JsonObject step, String expectedSymbol,
                                           Long expectedVolume, int stepIndex) {
        if (!"send".equals(step.getString("op", ""))
                || !"SupportMarketDataBean".equals(step.getString("eventType", ""))) {
            throw new IllegalArgumentException("step " + stepIndex + " must send SupportMarketDataBean");
        }
        JsonValue payloadValue = step.get("payload");
        if (payloadValue == null || !payloadValue.isObject()) {
            throw new IllegalArgumentException("step " + stepIndex + " payload is required");
        }
        JsonObject payload = payloadValue.asObject();
        if (payload.size() != 3
                || !payload.names().contains("symbol")
                || !payload.names().contains("volume")
                || !payload.names().contains("price")) {
            throw new IllegalArgumentException("step " + stepIndex
                    + " payload must contain exactly symbol, volume, and price");
        }
        JsonValue symbol = payload.get("symbol");
        if (symbol == null || !symbol.isString() || !expectedSymbol.equals(symbol.asString())) {
            throw new IllegalArgumentException("step " + stepIndex + " symbol mismatch");
        }
        JsonValue price = payload.get("price");
        if (price == null || !isIntegralNumber(price) || price.asLong() != 0L) {
            throw new IllegalArgumentException("step " + stepIndex + " price must be integer zero");
        }
        JsonValue volume = payload.get("volume");
        if (expectedVolume == null) {
            if (volume == null || !volume.isNull()) {
                throw new IllegalArgumentException("step " + stepIndex + " volume must be null");
            }
        } else if (volume == null || !isIntegralNumber(volume) || volume.asLong() != expectedVolume) {
            throw new IllegalArgumentException("step " + stepIndex + " volume mismatch");
        }
    }

    private static boolean isIntegralNumber(JsonValue value) {
        if (!value.isNumber()) {
            return false;
        }
        try {
            return value.asDouble() == value.asLong();
        } catch (RuntimeException ex) {
            return false;
        }
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be an object");
        }
        return value.asObject();
    }

    private static void runCase(JsonArray steps, int caseIndex, JsonArray records) throws Exception {
        String caseName = CASES[caseIndex];
        String runtimeURI = "parity-resultset-aggregate-minmax-groupby-om-viewcompile-" + RUNTIME_IDS[caseIndex];

        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> marketType = new HashMap<>();
        marketType.put("symbol", String.class);
        marketType.put("volume", Long.class);
        marketType.put("price", Double.class);
        marketType.put("feed", String.class);
        configuration.getCommon().addEventType("SupportMarketDataBean", marketType);

        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled compiled;
            String epl = eplFor(caseName);
            if (MINMAX_OM.equals(caseName)) {
                EPStatementObjectModel model = new EPStatementObjectModel();
                model.setSelectClause(com.espertech.esper.common.client.soda.SelectClause.create()
                        .streamSelector(com.espertech.esper.common.client.soda.StreamSelector.RSTREAM_ISTREAM_BOTH)
                        .add("symbol")
                        .add(com.espertech.esper.common.client.soda.Expressions.min("volume"), "minVol")
                        .add(com.espertech.esper.common.client.soda.Expressions.max("volume"), "maxVol")
                        .add(com.espertech.esper.common.client.soda.Expressions.minDistinct("volume"), "minDistVol")
                        .add(com.espertech.esper.common.client.soda.Expressions.maxDistinct("volume"), "maxDistVol"));
                model.setFromClause(com.espertech.esper.common.client.soda.FromClause.create(
                        com.espertech.esper.common.client.soda.FilterStream.create("SupportMarketDataBean")
                                .addView("length", com.espertech.esper.common.client.soda.Expressions.constant(3))));
                model.setWhereClause(com.espertech.esper.common.client.soda.Expressions.or()
                        .add(com.espertech.esper.common.client.soda.Expressions.eq("symbol", "DELL"))
                        .add(com.espertech.esper.common.client.soda.Expressions.eq("symbol", "IBM"))
                        .add(com.espertech.esper.common.client.soda.Expressions.eq("symbol", "GE")));
                model.setGroupByClause(com.espertech.esper.common.client.soda.GroupByClause.create("symbol"));
                model = SerializableObjectCopier.copyMayFail(model);
                String expected = "select irstream symbol, min(volume) as minVol, max(volume) as maxVol, min(distinct volume) as minDistVol, max(distinct volume) as maxDistVol from SupportMarketDataBean#length(3) where symbol=\"DELL\" or symbol=\"IBM\" or symbol=\"GE\" group by symbol";
                if (!expected.equals(model.toEPL())) {
                    throw new IllegalStateException("minmax-om model toEPL mismatch: " + model.toEPL());
                }
                model.setAnnotations(java.util.Collections.singletonList(
                        com.espertech.esper.common.client.soda.AnnotationPart.nameAnnotation("s0")));
                Module module = new Module();
                module.getItems().add(new ModuleItem(model));
                module.setModuleText(model.toEPL());
                compiled = EPCompilerProvider.getCompiler().compile(module,
                        new CompilerArguments(runtime.getRuntimePath()));
            } else {
                EPStatementObjectModel model = EPCompilerProvider.getCompiler().eplToModel(epl, configuration);
                model = SerializableObjectCopier.copyMayFail(model);
                if (!epl.equals(model.toEPL())) {
                    throw new IllegalStateException("minmax-view-compile model toEPL mismatch: " + model.toEPL());
                }
                Module module = new Module();
                module.getItems().add(new ModuleItem(model));
                module.setModuleText(model.toEPL());
                compiled = EPCompilerProvider.getCompiler().compile(module,
                        new CompilerArguments(runtime.getRuntimePath()));
            }
            EPDeployment deployment = runtime.getDeploymentService().deploy(
                    compiled, new DeploymentOptions().setDeploymentId(runtimeURI));
            EPStatement statement = findStatement(deployment, caseName);
            TraceWriter writer = new TraceWriter(records, caseName, statement, runtime);
            statement.addListener(writer);
            replay(steps, caseIndex, runtime);
            if (writer.sequence != RECORD_COUNTS[caseIndex]) {
                throw new IllegalStateException("case " + caseName + " produced "
                        + writer.sequence + " records, expected " + RECORD_COUNTS[caseIndex]);
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static EPStatement findStatement(EPDeployment deployment, String caseName) {
        for (EPStatement statement : deployment.getStatements()) {
            if ("s0".equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("statement s0 was not deployed for case " + caseName);
    }

    private static String eplFor(String caseName) {
        String body = "select irstream symbol, min(volume) as minVol, max(volume) as maxVol, min(distinct volume) as minDistVol, max(distinct volume) as maxDistVol from SupportMarketDataBean#length(3) where symbol=\"DELL\" or symbol=\"IBM\" or symbol=\"GE\" group by symbol";
        if (MINMAX_OM.equals(caseName)) return body;
        if (MINMAX_VIEW_COMPILE.equals(caseName)) return "@name('s0') " + body;
        throw new IllegalArgumentException("unsupported case " + caseName);
    }
    private static void replay(JsonArray steps, int caseIndex, EPRuntime runtime) {
        String caseName = CASES[caseIndex];
        boolean active = false;
        int sends = 0;
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = object(steps.get(i), "step " + i);
            String operation = step.getString("op", "");
            if ("case".equals(operation)) {
                active = caseName.equals(step.getString("case", ""));
                continue;
            }
            if (!active) {
                continue;
            }
            if (!"send".equals(operation)) {
                throw new IllegalArgumentException("unsupported operation in case " + caseName + ": " + operation);
            }
            send(runtime, step);
            sends++;
        }
        if (sends != SEND_COUNTS[caseIndex]) {
            throw new IllegalStateException("case " + caseName + " replayed " + sends
                    + " sends, expected " + SEND_COUNTS[caseIndex]);
        }
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        event.put("symbol", payload.getString("symbol", null));
        JsonValue volume = payload.get("volume");
        event.put("volume", volume == null || volume.isNull() ? null : volume.asLong());
        event.put("price", payload.getDouble("price", 0.0d));
        event.put("feed", null);
        runtime.getEventService().sendEventMap(event, "SupportMarketDataBean");
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if (!hasNew && !hasOld) {
                return;
            }
            if ((MINMAX_OM.equals(caseName) || MINMAX_VIEW_COMPILE.equals(caseName))
                    && (!hasNew || !hasOld || newEvents.length != oldEvents.length
                    || sequence >= MINMAX_CALLBACK_ROW_COUNTS.length
                    || newEvents.length != MINMAX_CALLBACK_ROW_COUNTS[(int) sequence])) {
                throw new IllegalStateException("MinMax listener callback row count does not match expected grouped update");
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            if (hasNew) {
                record.add("new", rows(newEvents));
            }
            if (hasOld) {
                record.add("old", rows(oldEvents));
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
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
            if (value instanceof Float || value instanceof Double) {
                double number = ((Number) value).doubleValue();
                if (number == Math.rint(number) && !Double.isInfinite(number)) {
                    return Json.value((long) number);
                }
                return Json.value(number);
            }
            if (value instanceof Integer || value instanceof Short || value instanceof Byte) {
                return Json.value(((Number) value).intValue());
            }
            if (value instanceof Number) {
                return Json.value(((Number) value).longValue());
            }
            if (value instanceof Boolean) {
                return Json.value((Boolean) value);
            }
            return Json.value(String.valueOf(value));
        }
    }
}
