import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.soda.AnnotationPart;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.soda.Expressions;
import com.espertech.esper.common.client.soda.FilterStream;
import com.espertech.esper.common.client.soda.FromClause;
import com.espertech.esper.common.client.soda.SelectClause;
import com.espertech.esper.common.client.soda.View;
import com.espertech.esper.common.internal.util.SerializableObjectCopier;
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for EPLOtherSelectWildcardWAdditional wildcard-with-additional
 * select-clause scenarios.
 *
 * Covers the nine pinned executions: single-om (SODA object-model twin of
 * Single, built with the fluent model API and round-tripped through
 * SerializableObjectCopier before compile), single, single-insert-into
 * (wildcard-plus-concat inserted into SomeEvent and replayed by a downstream
 * select), join-insert-into (two-stream join inserted into SomeJoinEvent),
 * join-no-common (cross join over SupportBeanSimple and a market-data map
 * type without a where clause), join-common (SupportBean_A x SupportBean_B
 * id concat deployed both with and without an equijoin where clause in one
 * runtime), combined-props (nested indexed/mapped access over
 * SupportBeanCombinedProps), wildcard-map (concat over the map event type
 * MyMapEventIntString) and invalid-repeated (compile-only; the duplicate
 * output column name must be rejected by the compiler).
 *
 * Standard listener protocol only: every deployed statement carries a
 * listener whose updates are recorded as sorted-field rows. Nested values
 * (join columns eventOne/eventTwo, indexed/array members) render as
 * recursive sorted-field rows via normalize. The invalid case deploys
 * nothing and emits a "deployed" marker record after the pinned module is
 * rejected with EPCompileException.
 */
public class EPLOtherSelectWildcardWAdditionalScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLOtherSelectWildcardWAdditionalScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            String caseName = caseVal.asObject().getString("case", "");
            runCase(allSteps, caseName, records);
        }

        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
        root.add("id", scenario.getString("id", ""));
        root.add("javaCommit", "9e1b9f1cc9117fea4bf33ab043762c045d73839c");
        root.add("java", System.getProperty("java.version"));
        JsonArray recordsArr = new JsonArray();
        for (JsonObject record : records) {
            recordsArr.add(record);
        }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }

    private static void runCase(JsonArray allSteps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = new Configuration();
        config.getCommon().addEventType("SupportBeanSimple", LocalSupportBeanSimple.class);
        Map<String, Object> marketType = new HashMap<>();
        marketType.put("symbol", String.class);
        marketType.put("price", double.class);
        marketType.put("volume", long.class);
        marketType.put("feed", String.class);
        config.getCommon().addEventType("SupportMarketDataBean", marketType);
        config.getCommon().addEventType("SupportBean_A", LocalSupportBeanA.class);
        config.getCommon().addEventType("SupportBean_B", LocalSupportBeanB.class);
        config.getCommon().addEventType("SupportBeanCombinedProps", LocalSupportBeanCombinedProps.class);
        Map<String, Object> mapEventType = new HashMap<>();
        mapEventType.put("int", Integer.class);
        mapEventType.put("theString", String.class);
        config.getCommon().addEventType("MyMapEventIntString", mapEventType);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLOtherSelectWildcardWAdditionalScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            boolean invalidCase = "invalid-repeated".equals(caseName);

            // The invalid case is compile-only: the pinned module must be
            // rejected, so it deploys nothing and owns no statements.
            if (!invalidCase) {
                EPCompiled compiled;
                if ("single-om".equals(caseName)) {
                    EPStatementObjectModel model = buildSingleOM();
                    // pinned EPLOtherSingleOM round-trips the unannotated
                    // model through toEPL before naming the statement
                    String expected = "select *, myString||myString as concat from SupportBeanSimple#length(5)";
                    if (!expected.equals(model.toEPL())) {
                        throw new IllegalStateException("single-om model toEPL mismatch: " + model.toEPL());
                    }
                    model.setAnnotations(java.util.Collections.singletonList(AnnotationPart.nameAnnotation("s0")));
                    // mirrors RegressionEnvironment: a SODA model compiles as
                    // a single-item module after the serializability copy
                    EPStatementObjectModel omCopy = SerializableObjectCopier.copyMayFail(model);
                    Module omModule = new Module();
                    omModule.getItems().add(new ModuleItem(omCopy));
                    omModule.setModuleText(omCopy.toEPL());
                    compiled = EPCompilerProvider.getCompiler().compile(omModule, new CompilerArguments(config));
                } else {
                    compiled = EPCompilerProvider.getCompiler().compile(buildEPL(caseName), new CompilerArguments(config));
                }
                EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());

                int[] seq = new int[] {0};
                for (EPStatement stmt : deployment.getStatements()) {
                    stmt.addListener((newData, oldData, statement, rt) -> {
                        boolean hasNew = newData != null && newData.length > 0;
                        boolean hasOld = oldData != null && oldData.length > 0;
                        if (!hasNew && !hasOld) {
                            return;
                        }
                        seq[0]++;
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "listener");
                        record.add("statement", statement.getName());
                        record.add("sequence", seq[0]);
                        record.add("time", java.time.Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());
                        if (hasNew) {
                            record.add("new", rows(newData));
                        }
                        if (hasOld) {
                            record.add("old", rows(oldData));
                        }
                        records.add(record);
                    });
                }
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
                if ("deployed".equals(op)) {
                    if (!invalidCase || !"invalid".equals(step.getString("statement", ""))) {
                        throw new IllegalStateException("unexpected deployed op for case " + caseName);
                    }
                    try {
                        EPCompilerProvider.getCompiler().compile(buildEPL(caseName), new CompilerArguments(config));
                    } catch (EPCompileException rejected) {
                        // pinned behavior: duplicate output column name myString fails validation
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "deployed");
                        record.add("statement", "invalid");
                        records.add(record);
                        continue;
                    }
                    throw new IllegalStateException("compile unexpectedly succeeded for case " + caseName);
                }
                throw new IllegalStateException("unknown op: " + op);
            }

        } finally {
            runtime.destroy();
        }
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (eventType) {
            case "SupportBeanSimple" -> {
                LocalSupportBeanSimple event = new LocalSupportBeanSimple(
                    payload.getString("myString", null),
                    payload.getInt("myInt", 0));
                runtime.getEventService().sendEventBean(event, "SupportBeanSimple");
            }
            case "SupportMarketDataBean" -> {
                Map<String, Object> event = new HashMap<>();
                event.put("symbol", payload.getString("symbol", null));
                event.put("price", payload.getDouble("price", 0d));
                event.put("volume", payload.getLong("volume", 0L));
                event.put("feed", stringOrNull(payload.get("feed")));
                runtime.getEventService().sendEventMap(event, "SupportMarketDataBean");
            }
            case "SupportBean_A" ->
                runtime.getEventService().sendEventBean(new LocalSupportBeanA(payload.getString("id", null)), "SupportBean_A");
            case "SupportBean_B" ->
                runtime.getEventService().sendEventBean(new LocalSupportBeanB(payload.getString("id", null)), "SupportBean_B");
            case "SupportBeanCombinedProps" -> {
                JsonArray nested = payload.get("indexed").asArray();
                LocalSupportBeanCombinedProps.NestedLevOne[] indexed =
                    new LocalSupportBeanCombinedProps.NestedLevOne[nested.size()];
                for (int i = 0; i < indexed.length; i++) {
                    JsonValue entry = nested.get(i);
                    if (!(entry instanceof JsonObject)) {
                        // null slot stays empty, mirroring the pinned [3]
                        continue;
                    }
                    Map<String, String> keysAndValues = new HashMap<>();
                    for (Member member : (JsonObject) entry) {
                        keysAndValues.put(member.getName(), member.getValue().asString());
                    }
                    indexed[i] = new LocalSupportBeanCombinedProps.NestedLevOne(keysAndValues);
                }
                runtime.getEventService().sendEventBean(new LocalSupportBeanCombinedProps(indexed), "SupportBeanCombinedProps");
            }
            case "MyMapEventIntString" -> {
                Map<String, Object> event = new HashMap<>();
                event.put("int", payload.getInt("int", 0));
                event.put("theString", payload.getString("theString", null));
                runtime.getEventService().sendEventMap(event, "MyMapEventIntString");
            }
            default -> throw new IllegalStateException("unknown eventType: " + eventType);
        }
    }

    /** Verbatim transcriptions of the pinned EPLOtherSelectWildcardWAdditional modules. */
    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "single" ->
                "@name('s0') select *, myString||myString as concat from SupportBeanSimple#length(5)";
            case "single-insert-into" ->
                "@name('insert') @public insert into SomeEvent select *, myString||myString as concat from SupportBeanSimple#length(5);\n" +
                    "@name('s0') select * from SomeEvent#length(5)";
            case "join-insert-into" ->
                "@name('insert') @public insert into SomeJoinEvent select *, myString||myString as concat " +
                    "from SupportBeanSimple#length(5) as eventOne, SupportMarketDataBean#length(5) as eventTwo;\n" +
                    "@name('s0') select * from SomeJoinEvent#length(5)";
            case "join-no-common" ->
                "@name('s0') select *, myString||myString as concat " +
                    "from SupportBeanSimple#length(5) as eventOne, SupportMarketDataBean#length(5) as eventTwo";
            case "join-common" ->
                "@name('s0') select *, eventOne.id||eventTwo.id as concat " +
                    "from SupportBean_A#length(5) as eventOne, SupportBean_B#length(5) as eventTwo;\n" +
                    "@name('s1') select *, eventOne.id||eventTwo.id as concat " +
                    "from SupportBean_A#length(5) as eventOne, SupportBean_B#length(5) as eventTwo " +
                    "where eventOne.id = eventTwo.id";
            case "combined-props" ->
                "@name('s0') select *, indexed[0].mapped('0ma').value||indexed[0].mapped('0mb').value as concat from SupportBeanCombinedProps#length(5)";
            case "wildcard-map" ->
                "@name('s0') select *, theString||theString as concat from MyMapEventIntString#length(5)";
            case "invalid-repeated" ->
                "select *, myString||myString as myString from SupportBeanSimple#length(5)";
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    /**
     * SODA object-model twin of the pinned EPLOtherSingleOM construction:
     * wildcard plus a concat expression aliased concat over
     * SupportBeanSimple#length(5).
     */
    private static EPStatementObjectModel buildSingleOM() {
        EPStatementObjectModel model = new EPStatementObjectModel();
        model.setSelectClause(SelectClause.createWildcard().add(Expressions.concat("myString", "myString"), "concat"));
        model.setFromClause(FromClause.create(FilterStream.create("SupportBeanSimple").addView(View.create("length", Expressions.constant(5)))));
        return model;
    }

    private static String stringOrNull(JsonValue value) {
        return value instanceof JsonString ? value.asString() : null;
    }

    private static JsonArray rows(EventBean[] events) {
        JsonArray arr = new JsonArray();
        for (EventBean event : events) {
            JsonObject item = new JsonObject();
            item.add("kind", "row");
            JsonObject fields = new JsonObject();
            for (String prop : new TreeSet<>(java.util.Arrays.asList(event.getEventType().getPropertyNames()))) {
                fields.add(prop, normalize(event.get(prop)));
            }
            item.add("fields", fields);
            arr.add(item);
        }
        return arr;
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
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
        if (value instanceof Map<?, ?>) {
            Map<?, ?> mapValue = (Map<?, ?>) value;
            TreeSet<String> keys = new TreeSet<>();
            for (Object key : mapValue.keySet()) {
                keys.add(String.valueOf(key));
            }
            JsonObject fields = new JsonObject();
            for (String key : keys) {
                fields.add(key, normalize(mapValue.get(key)));
            }
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        if (value instanceof LocalSupportBeanSimple) {
            LocalSupportBeanSimple bean = (LocalSupportBeanSimple) value;
            JsonObject fields = new JsonObject();
            fields.add("myInt", normalize(bean.getMyInt()));
            fields.add("myString", normalize(bean.getMyString()));
            return row(fields);
        }
        if (value instanceof LocalSupportBeanA) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(((LocalSupportBeanA) value).getId()));
            return row(fields);
        }
        if (value instanceof LocalSupportBeanB) {
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(((LocalSupportBeanB) value).getId()));
            return row(fields);
        }
        if (value instanceof LocalSupportBeanCombinedProps.NestedLevOne) {
            LocalSupportBeanCombinedProps.NestedLevOne nested = (LocalSupportBeanCombinedProps.NestedLevOne) value;
            JsonObject fields = new JsonObject();
            fields.add("mapprop", normalize(nested.getMapprop()));
            fields.add("nestLevOneVal", normalize(nested.getNestLevOneVal()));
            return row(fields);
        }
        if (value instanceof LocalSupportBeanCombinedProps.NestedLevTwo) {
            JsonObject fields = new JsonObject();
            fields.add("value", normalize(((LocalSupportBeanCombinedProps.NestedLevTwo) value).getValue()));
            return row(fields);
        }
        if (value instanceof Object[]) {
            JsonArray array = new JsonArray();
            for (Object item : (Object[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        return Json.value(String.valueOf(value));
    }

    private static JsonValue row(JsonObject fields) {
        JsonObject rowObj = new JsonObject();
        rowObj.add("kind", "row");
        rowObj.add("fields", fields);
        return rowObj;
    }

    /** Local mirror of the pinned SupportBeanSimple regression bean. */
    public static class LocalSupportBeanSimple {
        private final String myString;
        private final int myInt;

        public LocalSupportBeanSimple(String myString, int myInt) {
            this.myString = myString;
            this.myInt = myInt;
        }

        public String getMyString() {
            return myString;
        }

        public int getMyInt() {
            return myInt;
        }
    }

    /** Local mirror of the pinned SupportBean_A regression bean. */
    public static class LocalSupportBeanA {
        private final String id;

        public LocalSupportBeanA(String id) {
            this.id = id;
        }

        public String getId() {
            return id;
        }
    }

    /** Local mirror of the pinned SupportBean_B regression bean. */
    public static class LocalSupportBeanB {
        private final String id;

        public LocalSupportBeanB(String id) {
            this.id = id;
        }

        public String getId() {
            return id;
        }
    }

    /** Local mirror of the pinned SupportBeanCombinedProps regression bean. */
    public static class LocalSupportBeanCombinedProps {
        private final NestedLevOne[] indexed;

        public LocalSupportBeanCombinedProps(NestedLevOne[] indexed) {
            this.indexed = indexed;
        }

        public NestedLevOne getIndexed(int index) {
            return indexed[index];
        }

        public NestedLevOne[] getArray() {
            return indexed;
        }

        public static class NestedLevOne {
            private final Map<String, NestedLevTwo> map = new HashMap<>();

            public NestedLevOne(Map<String, String> keysAndValues) {
                for (Map.Entry<String, String> entry : keysAndValues.entrySet()) {
                    map.put(entry.getKey(), new NestedLevTwo(entry.getValue()));
                }
            }

            public NestedLevTwo getMapped(String key) {
                return map.get(key);
            }

            public Map<String, NestedLevTwo> getMapprop() {
                return map;
            }

            public String getNestLevOneVal() {
                return "abc";
            }
        }

        public static class NestedLevTwo {
            private final String value;

            public NestedLevTwo(String value) {
                this.value = value;
            }

            public String getValue() {
                return value;
            }
        }
    }
}
