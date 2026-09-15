import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.aggfunc.AggregationFunction;
import com.espertech.esper.common.client.hook.aggfunc.AggregationFunctionFactory;
import com.espertech.esper.common.client.hook.aggfunc.AggregationFunctionFactoryContext;
import com.espertech.esper.common.client.hook.aggfunc.AggregationFunctionForge;
import com.espertech.esper.common.client.hook.aggfunc.AggregationFunctionMode;
import com.espertech.esper.common.client.hook.aggfunc.AggregationFunctionModeManaged;
import com.espertech.esper.common.client.hook.aggfunc.AggregationFunctionValidationContext;
import com.espertech.esper.common.client.hook.aggmultifunc.AggregationMultiFunctionAccessor;
import com.espertech.esper.common.client.hook.aggmultifunc.AggregationMultiFunctionAccessorFactory;
import com.espertech.esper.common.client.hook.aggmultifunc.AggregationMultiFunctionAccessorFactoryContext;
import com.espertech.esper.common.client.hook.aggmultifunc.AggregationMultiFunctionAccessorMode;
import com.espertech.esper.common.client.hook.aggmultifunc.AggregationMultiFunctionAccessorModeManaged;
import com.espertech.esper.common.client.hook.aggmultifunc.AggregationMultiFunctionAgentMode;
import com.espertech.esper.common.client.hook.aggmultifunc.AggregationMultiFunctionAggregationMethodContext;
import com.espertech.esper.common.client.hook.aggmultifunc.AggregationMultiFunctionAggregationMethodMode;
import com.espertech.esper.common.client.hook.aggmultifunc.AggregationMultiFunctionDeclarationContext;
import com.espertech.esper.common.client.hook.aggmultifunc.AggregationMultiFunctionForge;
import com.espertech.esper.common.client.hook.aggmultifunc.AggregationMultiFunctionHandler;
import com.espertech.esper.common.client.hook.aggmultifunc.AggregationMultiFunctionState;
import com.espertech.esper.common.client.hook.aggmultifunc.AggregationMultiFunctionStateFactory;
import com.espertech.esper.common.client.hook.aggmultifunc.AggregationMultiFunctionStateFactoryContext;
import com.espertech.esper.common.client.hook.aggmultifunc.AggregationMultiFunctionStateKey;
import com.espertech.esper.common.client.hook.aggmultifunc.AggregationMultiFunctionStateMode;
import com.espertech.esper.common.client.hook.aggmultifunc.AggregationMultiFunctionStateModeManaged;
import com.espertech.esper.common.client.hook.aggmultifunc.AggregationMultiFunctionValidationContext;
import com.espertech.esper.common.client.hook.forgeinject.InjectionStrategyClassNewInstance;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.type.EPType;
import com.espertech.esper.common.client.type.EPTypeClass;
import com.espertech.esper.common.client.type.EPTypePremade;
import com.espertech.esper.common.internal.epl.expression.core.ExprEvaluator;
import com.espertech.esper.common.internal.epl.expression.core.ExprEvaluatorContext;
import com.espertech.esper.common.internal.epl.expression.core.ExprValidationException;
import com.espertech.esper.common.internal.rettype.EPChainableType;
import com.espertech.esper.common.internal.rettype.EPChainableTypeHelper;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.io.DataInput;
import java.io.DataOutput;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Java oracle for the ResultSetQueryTypeLocalGroupBy local-group-closure pair:
 * ResultSetLocalGroupedOnSelect (the named-window on-select sum with a
 * statement-wide group_by:() sibling) and
 * ResultSetLocalUngroupedAggAdditionalAndPlugin (the ungrouped selection that
 * mixes built-in local-group accessors with the concatstring plugin aggregation
 * and the sc plugin aggregation multi-function).
 *
 * local-group-on-select deploys three statements in one deployment: a keepall
 * named window over the SupportBean map type, the insert feeding it, and
 *
 *   on SupportBean_S0 select theString, sum(intPrimitive) as c0,
 *      sum(intPrimitive, group_by:()) as c1 from MyWindow group by theString
 *
 * Both on-select trigger sends answer one callback of three new rows (one per
 * theString group).  The Java assertion is any-order, so the oracle sorts each
 * delivery ascending by theString - the documented canonical representation
 * choice matching the local-group row-remove precedent - and verifies every row
 * against the pinned vectors before recording it.
 *
 * local-group-agg-additional-plugin replays the fifteen-expression ungrouped
 * selection over SupportBean.  The concatstring plugin aggregation is a local
 * mirror of SupportConcatWManagedAggregationFunctionForge: it joins received
 * non-null string values with single spaces (enter appends delimiter + value,
 * leave removes the last contribution, null values are skipped).  The sc plugin
 * aggregation multi-function mirrors the SupportAggMFMultiRTForge plain "sc"
 * path: an append-only state of the evaluated Integer values (ever semantics,
 * applyLeave no-op) whose accessor returns the collection.  Both are registered
 * through the same configuration APIs the regression run uses.  Four sends each
 * answer one single-row callback that is verified against the pinned vectors
 * (c0..c3 Long, c4/c5 String, c6/c7 collections rendered as JSON int arrays in
 * insertion order, c8/c9 Boolean, c10/c11 rate(null at time zero), c12/c13
 * nth(second-most-recent value or null)) before recording.
 *
 * Any observed deviation from the pinned vectors fails the run: the oracle
 * never adapts its expectations to what the engine emits.
 *
 * Records follow the having-scenario protocol: one record per listener
 * delivery, sequence numbering per case from 1, time rendered from the current
 * engine time (zero here, so 1970-01-01T00:00:00Z), and row fields in pinned
 * declaration order with the shared normalization rules (integral numbers as
 * long, other numbers as double, strings passthrough, null as
 * {"state":"null"}).
 */
public class ResultSetQueryTypeLocalGroupClosureScenarioOracle {

    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";

    // Pinned local-group-on-select rows, canonical theString order:
    // {theString, c0 (per-group sum), c1 (statement-wide sum)}.  The engine
    // renders these sums as Integer, exactly like the Java regression's int
    // literals in assertPropsPerRowLastNewAnyOrder.
    private static final Object[][][] ON_SELECT_ROWS = {
            {{"E1", 40, 150}, {"E2", 70, 150}, {"E3", 40, 150}},
            {{"E1", 100, 210}, {"E2", 70, 210}, {"E3", 40, 210}},
    };

    // Pinned plugin-case rows in delivery order.  Per delivery:
    // intPrimitive, c0..c3 (Long), c4/c5 (String), c6/c7 (int collections),
    // c8/c9 (Boolean), c10/c11 (Double or null), c12/c13 (Integer or null).
    private static final PluginRow[] PLUGIN_ROWS = {
            new PluginRow(10, 1, 1, 1, 1, "10", "10",
                    new int[] {10}, new int[] {10}, false, false,
                    null, null, null, null),
            new PluginRow(20, 1, 2, 1, 2, "20", "10 20",
                    new int[] {20}, new int[] {10, 20}, false, false,
                    null, null, null, 10),
            new PluginRow(-1, 1, 2, 2, 3, "10 -1", "10 20 -1",
                    new int[] {10, -1}, new int[] {10, 20, -1}, false, false,
                    null, null, 10, 20),
            new PluginRow(30, 2, 3, 2, 4, "20 30", "10 20 -1 30",
                    new int[] {20, 30}, new int[] {10, 20, -1, 30}, false, false,
                    null, null, 20, -1),
    };

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ResultSetQueryTypeLocalGroupClosureScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            JsonObject caseDef = caseVal.asObject();
            runCase(allSteps, caseDef.getString("case", ""), caseDef.getString("runtimeId", ""), records);
        }

        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
        root.add("id", scenario.getString("id", ""));
        root.add("scenario", scenario.getString("description", ""));
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        JsonArray recordsArr = new JsonArray();
        for (JsonObject record : records) {
            recordsArr.add(record);
        }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }

    private static void runCase(JsonArray allSteps, String caseName, String runtimeId, List<JsonObject> records)
            throws Exception {
        Configuration config = new Configuration();
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", int.class);
        config.getCommon().addEventType("SupportBean", beanType);
        config.getCommon().addEventType("SupportBean_S0", SupportBean_S0.class);
        // The regression run registers both plugin aggregations at suite level;
        // the oracle mirrors that for every case.
        config.getCompiler().addPlugInAggregationFunctionForge(
                "concatstring", LocalConcatAggregationFunctionForge.class.getName());
        config.getCompiler().addPlugInAggregationMultiFunction(
                new com.espertech.esper.common.client.configuration.compiler.ConfigurationCompilerPlugInAggregationMultiFunction(
                        new String[] {"sc"}, LocalAggMFMultiRTForge.class.getName()));
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "ResultSetQueryTypeLocalGroupClosureScenarioOracle-" + runtimeId + "-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(buildEPL(caseName), new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());

            RecordWriter writer = new RecordWriter(caseName, records);
            boolean foundS0 = false;
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    foundS0 = true;
                    candidate.addListener(writer);
                }
            }
            if (!foundS0) {
                throw new IllegalStateException("statement s0 was not deployed for case " + caseName);
            }

            boolean inCase = false;
            for (JsonValue stepVal : allSteps) {
                JsonObject step = stepVal.asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    inCase = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                if ("send".equals(op)) {
                    sendEvent(runtime, step);
                    continue;
                }
                throw new IllegalStateException("unknown op: " + op);
            }
            writer.assertComplete();

        } finally {
            runtime.destroy();
        }
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("SupportBean".equals(eventType)) {
            Map<String, Object> event = new HashMap<>();
            event.put("theString", payload.getString("theString", null));
            event.put("intPrimitive", payload.getInt("intPrimitive", 0));
            runtime.getEventService().sendEventMap(event, eventType);
            return;
        }
        if ("SupportBean_S0".equals(eventType)) {
            runtime.getEventService().sendEventBean(new SupportBean_S0(payload.getInt("id", 0)), eventType);
            return;
        }
        throw new IllegalStateException("unknown eventType: " + eventType);
    }

    /** Verbatim transcriptions of the pinned ResultSetQueryTypeLocalGroupBy modules. */
    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "local-group-on-select" ->
                "create window MyWindow#keepall as SupportBean;" +
                    "insert into MyWindow select * from SupportBean;" +
                    "@name('s0') on SupportBean_S0 select theString, sum(intPrimitive) as c0, " +
                    "sum(intPrimitive, group_by:()) as c1 from MyWindow group by theString;";
            case "local-group-agg-additional-plugin" ->
                "@name('s0') select intPrimitive, " +
                    " countever(*, intPrimitive>0, group_by:(theString)) as c0," +
                    " countever(*, intPrimitive>0, group_by:()) as c1," +
                    " countever(*, group_by:(theString)) as c2," +
                    " countever(*, group_by:()) as c3," +
                    " concatstring(Integer.toString(intPrimitive), group_by:(theString)) as c4," +
                    " concatstring(Integer.toString(intPrimitive), group_by:()) as c5," +
                    " sc(intPrimitive, group_by:(theString)) as c6," +
                    " sc(intPrimitive, group_by:()) as c7," +
                    " leaving(group_by:(theString)) as c8," +
                    " leaving(group_by:()) as c9," +
                    " rate(3, group_by:(theString)) as c10," +
                    " rate(3, group_by:()) as c11," +
                    " nth(intPrimitive, 1, group_by:(theString)) as c12," +
                    " nth(intPrimitive, 1, group_by:()) as c13" +
                    " from SupportBean as sb";
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof Collection<?> collection) {
            JsonArray array = new JsonArray();
            for (Object item : collection) {
                array.add(normalize(item));
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
        return Json.value(String.valueOf(value));
    }

    /** The pinned single-row delivery of the plugin case. */
    private record PluginRow(
            int intPrimitive,
            long c0, long c1, long c2, long c3,
            String c4, String c5,
            int[] c6, int[] c7,
            boolean c8, boolean c9,
            Double c10, Double c11,
            Integer c12, Integer c13) {
    }

    private static final class RecordWriter implements UpdateListener {
        private final String caseName;
        private final List<JsonObject> records;
        private int sequence;
        private int deliveries;
        // Esper swallows listener dispatch exceptions into its (unconfigured)
        // log4j, so every assertion failure is retained and re-raised by
        // assertComplete at the end of the case, failing the run.
        private RuntimeException failure;

        private RecordWriter(String caseName, List<JsonObject> records) {
            this.caseName = caseName;
            this.records = records;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement, EPRuntime rt) {
            try {
                boolean hasNew = newEvents != null && newEvents.length > 0;
                boolean hasOld = oldEvents != null && oldEvents.length > 0;
                if (!hasNew && !hasOld) {
                    return;
                }
                if (hasOld) {
                    throw new IllegalStateException(caseName + " delivery " + (deliveries + 1)
                            + " carried " + oldEvents.length + " old rows; every pinned delivery is new-only");
                }
                long now = rt.getEventService().getCurrentTime();
                EventBean[] rendered = switch (caseName) {
                    case "local-group-on-select" -> verifyOnSelect(newEvents);
                    case "local-group-agg-additional-plugin" -> verifyPlugin(newEvents);
                    default -> throw new IllegalStateException("unknown case: " + caseName);
                };
                sequence++;
                JsonObject record = new JsonObject();
                record.add("case", caseName);
                record.add("operation", "listener");
                record.add("statement", statement.getName());
                record.add("time", Instant.ofEpochMilli(now).toString());
                record.add("sequence", sequence);
                JsonArray rows = new JsonArray();
                for (EventBean event : rendered) {
                    JsonObject item = new JsonObject();
                    item.add("kind", "row");
                    item.add("fields", fields(event));
                    rows.add(item);
                }
                record.add("new", rows);
                records.add(record);
            } catch (RuntimeException | AssertionError ex) {
                // The runtime swallows listener exceptions, so surface the
                // failing assertion before it is lost and keep it for
                // assertComplete.
                ex.printStackTrace(System.err);
                if (failure == null) {
                    failure = ex instanceof RuntimeException runtime ? runtime : new RuntimeException(ex);
                }
                throw ex instanceof RuntimeException runtime ? runtime : new RuntimeException(ex);
            }
        }

        private void assertComplete() {
            if (failure != null) {
                throw failure;
            }
            int expected = "local-group-on-select".equals(caseName)
                    ? ON_SELECT_ROWS.length
                    : PLUGIN_ROWS.length;
            if (deliveries != expected) {
                throw new IllegalStateException(caseName + " expected " + expected
                        + " listener deliveries, got " + deliveries);
            }
        }

        /** Sorts the delivery ascending by theString and pins every row. */
        private EventBean[] verifyOnSelect(EventBean[] newEvents) {
            int delivery = ++deliveries;
            if (delivery > ON_SELECT_ROWS.length) {
                throw new IllegalStateException(caseName + " unexpected delivery " + delivery);
            }
            if (newEvents.length != ON_SELECT_ROWS[delivery - 1].length) {
                throw new IllegalStateException(caseName + " delivery " + delivery + " carries " + newEvents.length
                        + " rows, want " + ON_SELECT_ROWS[delivery - 1].length);
            }
            EventBean[] sorted = newEvents.clone();
            Arrays.sort(sorted, (left, right) ->
                    ((String) left.get("theString")).compareTo((String) right.get("theString")));
            Object[][] wanted = ON_SELECT_ROWS[delivery - 1];
            for (int i = 0; i < sorted.length; i++) {
                assertFields(sorted[i], delivery, i,
                        new String[] {"theString", "c0", "c1"},
                        new Object[] {wanted[i][0], wanted[i][1], wanted[i][2]});
            }
            return sorted;
        }

        private EventBean[] verifyPlugin(EventBean[] newEvents) {
            int delivery = ++deliveries;
            if (delivery > PLUGIN_ROWS.length) {
                throw new IllegalStateException(caseName + " unexpected delivery " + delivery);
            }
            if (newEvents.length != 1) {
                throw new IllegalStateException(caseName + " delivery " + delivery + " carries " + newEvents.length
                        + " rows, want 1");
            }
            EventBean row = newEvents[0];
            PluginRow wanted = PLUGIN_ROWS[delivery - 1];
            assertFields(row, delivery, 0,
                    new String[] {"intPrimitive", "c0", "c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8", "c9",
                            "c10", "c11", "c12", "c13"},
                    new Object[] {wanted.intPrimitive(), wanted.c0(), wanted.c1(), wanted.c2(), wanted.c3(),
                            wanted.c4(), wanted.c5(), wanted.c6(), wanted.c7(), wanted.c8(), wanted.c9(),
                            wanted.c10(), wanted.c11(), wanted.c12(), wanted.c13()});
            return newEvents;
        }

        /**
         * Pins the field set and every field value of one row; collection
         * expectations compare element-by-element in insertion order.
         */
        private void assertFields(EventBean row, int delivery, int rowIndex, String[] names, Object[] wanted) {
            String label = caseName + " delivery " + delivery + " row " + rowIndex;
            for (int i = 0; i < names.length; i++) {
                String name = names[i];
                Object actual = row.get(name);
                Object expected = wanted[i];
                if (expected instanceof int[] ints) {
                    if (!(actual instanceof Collection<?> collection) || collection.size() != ints.length) {
                        throw new AssertionError(label + " field " + name + " = " + describe(actual)
                                + ", want an int collection of size " + ints.length);
                    }
                    int index = 0;
                    for (Object item : collection) {
                        if (!(item instanceof Integer integer) || integer != ints[index]) {
                            throw new AssertionError(label + " field " + name + "[" + index + "] = " + item
                                    + ", want " + ints[index]);
                        }
                        index++;
                    }
                    continue;
                }
                if (expected == null) {
                    if (actual != null) {
                        throw new AssertionError(label + " field " + name + " = " + describe(actual) + ", want null");
                    }
                    continue;
                }
                if (!expected.getClass().isInstance(actual) || !expected.equals(actual)) {
                    throw new AssertionError(label + " field " + name + " = " + describe(actual) + ", want "
                            + expected + " (" + expected.getClass().getSimpleName() + ")");
                }
            }
        }

        private String describe(Object value) {
            return value == null ? "null" : String.valueOf(value) + " (" + value.getClass().getSimpleName() + ")";
        }

        /** Row fields in pinned declaration order. */
        private JsonObject fields(EventBean event) {
            JsonObject fields = new JsonObject();
            switch (caseName) {
                case "local-group-on-select" ->
                    addAll(fields, event, "theString", "c0", "c1");
                case "local-group-agg-additional-plugin" ->
                    addAll(fields, event, "intPrimitive", "c0", "c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8",
                            "c9", "c10", "c11", "c12", "c13");
                default -> throw new IllegalStateException("unknown case: " + caseName);
            }
            return fields;
        }

        private void addAll(JsonObject fields, EventBean event, String... names) {
            for (String name : names) {
                fields.add(name, normalize(event.get(name)));
            }
        }
    }

    /**
     * Local mirror of SupportConcatWManagedAggregationFunctionForge: validates a
     * single string parameter, yields String values, and injects
     * LocalConcatAggregationFunction through a managed mode with HA serde.
     */
    public static class LocalConcatAggregationFunctionForge implements AggregationFunctionForge {
        public void setFunctionName(String functionName) {
        }

        public void validate(AggregationFunctionValidationContext validationContext) throws ExprValidationException {
            EPType paramType = validationContext.getParameterTypes()[0];
            if (!EPTypePremade.STRING.getEPType().equals(paramType)) {
                throw new ExprValidationException("Invalid parameter type '" + paramType + "'");
            }
        }

        public EPTypeClass getValueType() {
            return EPTypePremade.STRING.getEPType();
        }

        public AggregationFunctionMode getAggregationFunctionMode() {
            AggregationFunctionModeManaged mode = new AggregationFunctionModeManaged();
            mode.setHasHA(true);
            mode.setSerde(LocalConcatAggregationFunctionSerde.class);
            mode.setInjectionStrategyAggregationFunctionFactory(
                    new InjectionStrategyClassNewInstance(LocalConcatAggregationFunctionFactory.class.getName()));
            return mode;
        }
    }

    /** Mirror of SupportConcatWManagedAggregationFunctionSerde. */
    public static class LocalConcatAggregationFunctionSerde {
        public static void write(DataOutput output, AggregationFunction value) throws java.io.IOException {
            output.writeUTF(((LocalConcatAggregationFunction) value).getValue());
        }

        public static AggregationFunction read(DataInput input) throws java.io.IOException {
            String current = input.readUTF();
            if (current.isEmpty()) {
                return new LocalConcatAggregationFunction();
            }
            return new LocalConcatAggregationFunction(new StringBuilder(current));
        }
    }

    /** Mirror of SupportConcatWManagedAggregationFunctionFactory. */
    public static class LocalConcatAggregationFunctionFactory implements AggregationFunctionFactory {
        public AggregationFunction newAggregator(AggregationFunctionFactoryContext ctx) {
            return new LocalConcatAggregationFunction();
        }
    }

    /**
     * Mirror of SupportConcatWManagedAggregationFunction: joins received
     * non-null string values with single spaces; enter appends, leave removes
     * the last contribution.
     */
    public static class LocalConcatAggregationFunction implements AggregationFunction {
        protected static final char DELIMITER = ' ';
        private StringBuilder builder;
        private String delimiter;

        public LocalConcatAggregationFunction() {
            builder = new StringBuilder();
            delimiter = "";
        }

        public LocalConcatAggregationFunction(StringBuilder builder) {
            this.builder = builder;
            this.delimiter = String.valueOf(DELIMITER);
        }

        public void enter(Object value) {
            if (value != null) {
                builder.append(delimiter);
                builder.append(value.toString());
                delimiter = String.valueOf(DELIMITER);
            }
        }

        public void leave(Object value) {
            if (value != null) {
                builder.delete(0, value.toString().length() + 1);
            }
        }

        public String getValue() {
            return builder.toString();
        }

        public void clear() {
            builder = new StringBuilder();
            delimiter = "";
        }
    }

    /** Mirror of SupportAggMFMultiRTForge restricted to the "sc" function. */
    public static class LocalAggMFMultiRTForge implements AggregationMultiFunctionForge {
        public void addAggregationFunction(AggregationMultiFunctionDeclarationContext declarationContext) {
        }

        public AggregationMultiFunctionHandler validateGetHandler(AggregationMultiFunctionValidationContext ctx) {
            return new LocalAggMFMultiRTHandler(ctx);
        }
    }

    /**
     * Mirror of SupportAggMFMultiRTHandler for "sc": the array-coll scalar
     * state injected with the parameter evaluator and evaluation type, the
     * coll-scalar accessor, and a collection-of-single-value return type.
     */
    public static class LocalAggMFMultiRTHandler implements AggregationMultiFunctionHandler {
        private final AggregationMultiFunctionValidationContext validationContext;

        public LocalAggMFMultiRTHandler(AggregationMultiFunctionValidationContext validationContext) {
            this.validationContext = validationContext;
        }

        public AggregationMultiFunctionStateKey getAggregationStateUniqueKey() {
            // Never share anything, exactly like the non-se1/se2 original path.
            return new AggregationMultiFunctionStateKey() {
            };
        }

        public AggregationMultiFunctionStateMode getStateMode() {
            InjectionStrategyClassNewInstance injectionStrategy =
                    new InjectionStrategyClassNewInstance(LocalAggMFMultiRTArrayCollScalarStateFactory.EPTYPE)
                            .addExpression("evaluator", validationContext.getAllParameterExpressions()[0])
                            .addConstant("evaluationType",
                                    validationContext.getAllParameterExpressions()[0].getForge().getEvaluationType());
            return new AggregationMultiFunctionStateModeManaged()
                    .setInjectionStrategyAggregationStateFactory(injectionStrategy);
        }

        public AggregationMultiFunctionAccessorMode getAccessorMode() {
            InjectionStrategyClassNewInstance injectionStrategy =
                    new InjectionStrategyClassNewInstance(LocalAggMFMultiRTCollScalarAccessorFactory.EPTYPE);
            return new AggregationMultiFunctionAccessorModeManaged()
                    .setInjectionStrategyAggregationAccessorFactory(injectionStrategy);
        }

        public EPChainableType getReturnType() {
            return EPChainableTypeHelper.collectionOfSingleValue(
                    (EPTypeClass) validationContext.getAllParameterExpressions()[0].getForge().getEvaluationType());
        }

        public AggregationMultiFunctionAgentMode getAgentMode() {
            throw new UnsupportedOperationException("This implementation does not support tables");
        }

        public AggregationMultiFunctionAggregationMethodMode getAggregationMethodMode(
                AggregationMultiFunctionAggregationMethodContext ctx) {
            return null; // not implemented
        }
    }

    /** Mirror of SupportAggMFMultiRTArrayCollScalarStateFactory. */
    public static class LocalAggMFMultiRTArrayCollScalarStateFactory implements AggregationMultiFunctionStateFactory {
        public final static EPTypeClass EPTYPE = new EPTypeClass(LocalAggMFMultiRTArrayCollScalarStateFactory.class);

        private ExprEvaluator evaluator;
        private EPTypeClass evaluationType;

        public AggregationMultiFunctionState newState(AggregationMultiFunctionStateFactoryContext ctx) {
            return new LocalAggMFMultiRTArrayCollScalarState(this);
        }

        public ExprEvaluator getEvaluator() {
            return evaluator;
        }

        public void setEvaluator(ExprEvaluator evaluator) {
            this.evaluator = evaluator;
        }

        public void setEvaluationType(EPTypeClass evaluationType) {
            this.evaluationType = evaluationType;
        }

        public EPTypeClass getEvaluationType() {
            return evaluationType;
        }
    }

    /**
     * Mirror of SupportAggMFMultiRTArrayCollScalarState: an append-only list of
     * the evaluated values (ever semantics, applyLeave no-op).
     */
    public static class LocalAggMFMultiRTArrayCollScalarState implements AggregationMultiFunctionState {
        private final LocalAggMFMultiRTArrayCollScalarStateFactory factory;
        private final List<Object> values = new ArrayList<>();

        public LocalAggMFMultiRTArrayCollScalarState(LocalAggMFMultiRTArrayCollScalarStateFactory factory) {
            this.factory = factory;
        }

        public void applyEnter(EventBean[] eventsPerStream, ExprEvaluatorContext exprEvaluatorContext) {
            values.add(factory.getEvaluator().evaluate(eventsPerStream, true, exprEvaluatorContext));
        }

        public void applyLeave(EventBean[] eventsPerStream, ExprEvaluatorContext exprEvaluatorContext) {
            // ever semantics
        }

        public void clear() {
            values.clear();
        }

        public int size() {
            return values.size();
        }

        public Collection<Object> getValueAsCollection() {
            return values;
        }
    }

    /** Mirror of SupportAggMFMultiRTCollScalarAccessorFactory. */
    public static class LocalAggMFMultiRTCollScalarAccessorFactory implements AggregationMultiFunctionAccessorFactory {
        public final static EPTypeClass EPTYPE = new EPTypeClass(LocalAggMFMultiRTCollScalarAccessorFactory.class);

        public AggregationMultiFunctionAccessor newAccessor(AggregationMultiFunctionAccessorFactoryContext ctx) {
            return new LocalAggMFMultiRTCollScalarAccessor();
        }
    }

    /** Mirror of SupportAggMFMultiRTCollScalarAccessor. */
    public static class LocalAggMFMultiRTCollScalarAccessor implements AggregationMultiFunctionAccessor {
        public Object getValue(AggregationMultiFunctionState state, EventBean[] eventsPerStream, boolean isNewData,
                               ExprEvaluatorContext exprEvaluatorContext) {
            return ((LocalAggMFMultiRTArrayCollScalarState) state).getValueAsCollection();
        }

        public Collection<EventBean> getEnumerableEvents(AggregationMultiFunctionState state,
                                                        EventBean[] eventsPerStream, boolean isNewData,
                                                        ExprEvaluatorContext exprEvaluatorContext) {
            return null;
        }

        public EventBean getEnumerableEvent(AggregationMultiFunctionState state, EventBean[] eventsPerStream,
                                           boolean isNewData, ExprEvaluatorContext exprEvaluatorContext) {
            return null;
        }

        public Collection<Object> getEnumerableScalar(AggregationMultiFunctionState state,
                                                     EventBean[] eventsPerStream, boolean isNewData,
                                                     ExprEvaluatorContext exprEvaluatorContext) {
            return ((LocalAggMFMultiRTArrayCollScalarState) state).getValueAsCollection();
        }
    }
}
