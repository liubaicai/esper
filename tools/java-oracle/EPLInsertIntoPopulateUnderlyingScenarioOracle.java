import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeAvro;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;
import org.apache.avro.Schema;

import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBeanComplexProps;
import com.espertech.esper.common.internal.support.SupportBean_N;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.regressionlib.support.bean.SupportBeanArrayEvent;
import com.espertech.esper.regressionlib.support.bean.SupportBean_ST0;
import com.espertech.esper.regressionlib.support.bean.SupportBean_ST1;
import com.espertech.esper.regressionlib.support.bean.SupportBeanCtorOne;
import com.espertech.esper.regressionlib.support.bean.SupportBeanCtorThree;
import com.espertech.esper.regressionlib.support.bean.SupportBeanCtorTwo;
import com.espertech.esper.regressionlib.support.bean.SupportBeanObject;
import com.espertech.esper.regressionlib.support.bean.SupportEventWithCtorSameType;
import java.beans.Introspector;
import java.beans.PropertyDescriptor;
import java.lang.reflect.Modifier;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;

import static org.apache.avro.SchemaBuilder.record;

/**
 * Oracle for EPLInsertIntoPopulateUnderlying observable executions:
 * PopulateBeanSimple (select-names, insert-names, boxed-conversion),
 * BeanWildcard, PopulateBeanObjects (arrays/maps, nested, null value),
 * PopulateUnderlyingSimple (map/object-array/avro), EPLInsertIntoCtor
 * (select-names, boxable-null-into-primitive, join-wildcard, ignored column
 * list, same-type ctor), EPLInsertIntoCtorWithPattern,
 * EPLInsertIntoBeanJoin (wildcard, select-names, local-class target) and
 * EPLInsertIntoWindowAggregationAtEventBean. Multi-stage Java executions
 * replay as one scenario case per stage because the case protocol deploys
 * each case into a fresh runtime (the Java suite undeploys between stages).
 * Registered event types mirror the Go parity test's registered schemas;
 * MyLocalTarget registers the suite's public static nested class. BeanJoin
 * stage 3 (fully-qualified class-name target) is a Java naming artifact and
 * is not replayed. CharSequence compat and the INVALIDITY execution are
 * compile-only in Java and covered by Go Build-time rejection instead.
 */
public final class EPLInsertIntoPopulateUnderlyingScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private EPLInsertIntoPopulateUnderlyingScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: EPLInsertIntoPopulateUnderlyingScenarioOracle <scenario.json>");
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]))).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        String scenarioID = scenario.getString("id", "");
        if (scenarioID.isBlank()) {
            throw new IllegalArgumentException("scenario id is required");
        }
        JsonArray steps = scenario.get("steps").asArray();
        if (steps == null || steps.size() == 0) {
            throw new IllegalArgumentException("scenario steps are required");
        }
        JsonObject trace = new JsonObject()
                .add("version", VERSION)
                .add("id", scenarioID);
        JsonArray records = new JsonArray();
        trace.add("records", records);

        String[] cases = {
            "populate-bean-simple-select-names",
            "populate-bean-simple-insert-names",
            "populate-bean-simple-boxed-conversion",
            "bean-wildcard",
            "populate-bean-objects-arrays-maps",
            "populate-bean-objects-nested",
            "populate-bean-objects-null-value",
            "populate-underlying-map",
            "populate-underlying-objectarray",
            "populate-underlying-avro",
            "ctor-variants-select-names",
            "ctor-variants-boxable-null",
            "ctor-variants-join-wildcard",
            "ctor-variants-column-list-ignored",
            "ctor-variants-same-type-ctor",
            "ctor-with-pattern",
            "bean-join-populate-wildcard",
            "bean-join-populate-select-names",
            "bean-join-populate-local-class",
            "window-aggregation-eventbean"
        };
        for (String caseName : cases) {
            if (!hasCase(steps, caseName)) {
                continue;
            }
            runCase(steps, caseName, records);
        }
        System.out.println(trace);
    }

    private static boolean hasCase(JsonArray steps, String wanted) {
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            if ("case".equals(step.getString("op", "")) && wanted.equals(step.getString("case", ""))) {
                return true;
            }
        }
        return false;
    }

    private static void runCase(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addImport(com.espertech.esper.common.internal.support.SupportEnum.class);
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        configuration.getCommon().addEventType("MyMap", myMapType());
        configuration.getCommon().addEventType("MySupportMap", supportBeanType());
        configuration.getCommon().addEventType("SupportBeanComplexProps", SupportBeanComplexProps.class);
        configuration.getCommon().addEventType("SupportBean_N", SupportBean_N.class);
        configuration.getCommon().addEventType("SupportBean_S0", SupportBean_S0.class);
        configuration.getCommon().addEventType("SupportBean_ST0", SupportBean_ST0.class);
        configuration.getCommon().addEventType("SupportBean_ST1", SupportBean_ST1.class);
        configuration.getCommon().addEventType("SupportBeanCtorOne", SupportBeanCtorOne.class);
        configuration.getCommon().addEventType("SupportBeanCtorTwo", SupportBeanCtorTwo.class);
        configuration.getCommon().addEventType("SupportBeanCtorThree", SupportBeanCtorThree.class);
        configuration.getCommon().addEventType("SupportEventWithCtorSameType", SupportEventWithCtorSameType.class);
        configuration.getCommon().addEventType("SupportBeanObject", SupportBeanObject.class);
        configuration.getCommon().addEventType("SupportBeanArrayEvent", SupportBeanArrayEvent.class);
        // The suite's local target is a public static nested class; register
        // it by name so the scenario EPL can use the simple name instead of
        // the suite's fully-qualified "$MyLocalTarget" spelling.
        configuration.getCommon().addEventType("MyLocalTarget",
            "com.espertech.esper.regressionlib.suite.epl.insertinto.EPLInsertIntoPopulateUnderlying$MyLocalTarget");

        if (caseName.equals("populate-underlying-map")) {
            Map<String, Object> def = new LinkedHashMap<>();
            def.put("intVal", int.class);
            def.put("stringVal", String.class);
            def.put("doubleVal", double.class);
            configuration.getCommon().addEventType("MyMapType", def);
        } else if (caseName.equals("populate-underlying-objectarray")) {
            configuration.getCommon().addEventType("MyOAType", new String[]{"intVal", "stringVal", "doubleVal"},
                new Object[]{int.class, String.class, double.class});
        } else if (caseName.equals("populate-underlying-avro")) {
            Schema schema = record("MyAvroType").fields()
                .requiredInt("intVal").requiredString("stringVal").requiredDouble("doubleVal").endRecord();
            configuration.getCommon().getEventMeta().getAvroSettings().setEnableAvro(true);
            configuration.getCommon().addEventTypeAvro("MyAvroType", new ConfigurationCommonEventTypeAvro(schema));
        }


        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-iipu-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            List<String> epls = eplsFor(caseName);
            EPStatement statement = null;
            int deploymentIndex = 0;
            for (String epl : epls) {
                CompilerArguments compilerArguments = new CompilerArguments(configuration);
                compilerArguments.getPath().add(runtime.getRuntimePath());
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArguments);
                EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                        new DeploymentOptions().setDeploymentId("parity-iipu-" + caseName + "-" + (deploymentIndex++)));
                if (statement == null) {
                    for (EPStatement candidate : deployment.getStatements()) {
                        if ("s0".equals(candidate.getName())) {
                            statement = candidate;
                            break;
                        }
                    }
                }
            }
            if (statement == null) {
                throw new IllegalStateException("statement s0 was not deployed");
            }
            TraceWriter writer = new TraceWriter(records, caseName, statement, runtime);
            statement.addListener(writer);
            replayCase(allSteps, caseName, runtime);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static Map<String, Object> supportBeanType() {
        Map<String, Object> def = new LinkedHashMap<>();
        // Matches the fixed Java config for MySupportMap: intPrimitive,
        // longBoxed, theString and boolPrimitive (in declaration order).
        def.put("intPrimitive", int.class);
        def.put("longBoxed", Long.class);
        def.put("theString", String.class);
        def.put("boolPrimitive", Boolean.class);
        return def;
    }
    private static Map<String, Object> myMapType() {
        Map<String, Object> def = new LinkedHashMap<>();
        def.put("intBoxed", Integer.class);
        def.put("floatBoxed", Float.class);
        def.put("intArr", int[].class);
        def.put("mapProp", Map.class);
        def.put("nested", SupportBeanComplexProps.SupportBeanSpecialGetterNested.class);
        def.put("nestedValue", String.class);
        def.put("nestedNestedValue", String.class);
        return def;
    }


    private static List<String> eplsFor(String caseName) {
        List<String> epls = new ArrayList<>();
        switch (caseName) {
            case "populate-bean-simple-select-names" -> {
                epls.add("@name('i1') insert into SupportBean select " +
                    "'E1' as theString, 1 as intPrimitive, 2 as intBoxed, 3L as longPrimitive," +
                    "null as longBoxed, true as boolPrimitive, " +
                    "'x' as charPrimitive, 0xA as bytePrimitive, " +
                    "8.0f as floatPrimitive, 9.0d as doublePrimitive, " +
                    "0x05 as shortPrimitive, SupportEnum.ENUM_VALUE_2 as enumValue " +
                    " from MyMap");
                epls.add("@name('s0') select * from SupportBean");
            }
            case "populate-bean-simple-insert-names" ->
                epls.add("@name('s0') insert into SupportBean(theString, intPrimitive, intBoxed, longPrimitive," +
                    "longBoxed, boolPrimitive, charPrimitive, bytePrimitive, floatPrimitive, doublePrimitive, " +
                    "shortPrimitive, enumValue) select " +
                    "'E1', 1, 2, 3L, null, true, 'x', 0xA, 8.0f, 9.0d, 0x05, SupportEnum.ENUM_VALUE_2 from MyMap");
            case "populate-bean-simple-boxed-conversion" ->
                epls.add("@name('s0') insert into SupportBean(longBoxed, doubleBoxed) select intBoxed, floatBoxed from MyMap");
            case "bean-wildcard" ->
                epls.add("@name('s0') insert into SupportBean select * from MySupportMap");
            case "populate-bean-objects-arrays-maps" ->
                epls.add("@name('s0') insert into SupportBeanComplexProps(arrayProperty,objectArray,mapProperty) select intArr,{10,20,30},mapProp from MyMap");
            case "populate-bean-objects-nested" ->
                epls.add("@name('s0') insert into SupportBeanComplexProps(nested) select nested from MyMap");
            case "populate-bean-objects-null-value" ->
                epls.add("@name('s0') insert into SupportBean select 'B' as theString, intBoxed as intPrimitive from SupportBean(theString='A')");
            case "populate-underlying-map", "populate-underlying-objectarray", "populate-underlying-avro" -> {
                String typeName = caseName.endsWith("-map") ? "MyMapType" : caseName.endsWith("-objectarray") ? "MyOAType" : "MyAvroType";
                epls.add("@name('s0') insert into " + typeName + " select intPrimitive as intVal, theString as stringVal, doubleBoxed as doubleVal from SupportBean");
            }
            case "ctor-variants-select-names" ->
                epls.add("@name('s0') insert into SupportBeanCtorOne select theString, intBoxed, intPrimitive, boolPrimitive from SupportBean");
            case "ctor-variants-boxable-null" ->
                epls.add("@name('s0') insert into SupportBeanCtorOne select theString, null, intBoxed from SupportBean");
            case "ctor-variants-join-wildcard" ->
                epls.add("@name('s0') insert into SupportBeanCtorTwo select * from SupportBean_ST0#lastevent, SupportBean_ST1#lastevent");
            case "ctor-variants-column-list-ignored" ->
                epls.add("@name('s0') insert into SupportBeanCtorOne(theString, intPrimitive) select 'E1', 5 from SupportBean");
            case "ctor-variants-same-type-ctor" ->
                epls.add("@name('s0') insert into SupportEventWithCtorSameType select c1,c2 from SupportBean(theString='b1')#lastevent as c1, SupportBean(theString='b2')#lastevent as c2");
            case "ctor-with-pattern" ->
                epls.add("@name('s0') insert into SupportBeanCtorThree select s, e FROM PATTERN [" +
                    "every s=SupportBean_ST0 -> [2] e=SupportBean_ST1]");
            case "bean-join-populate-wildcard" ->
                epls.add("@name('s0') insert into SupportBeanObject select * from SupportBean_N#lastevent as one, SupportBean_S0#lastevent as two");
            case "bean-join-populate-select-names" ->
                epls.add("@name('s0') insert into SupportBeanObject select one, two from SupportBean_N#lastevent as one, SupportBean_S0#lastevent as two");
            case "bean-join-populate-local-class" ->
                epls.add("@name('s0') insert into MyLocalTarget select 1 as value from SupportBean_N");
            case "window-aggregation-eventbean" ->
                epls.add("@name('s0') insert into SupportBeanArrayEvent select window(*) @eventbean from SupportBean#keepall");
            default -> throw new IllegalArgumentException("unsupported case " + caseName);
        }
        return epls;
    }

    private static void replayCase(JsonArray allSteps, String caseName, EPRuntime runtime) {
        boolean active = false;
        for (int i = 0; i < allSteps.size(); i++) {
            JsonObject step = allSteps.get(i).asObject();
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                active = caseName.equals(step.getString("case", ""));
                continue;
            }
            if (!active) {
                continue;
            }
            if ("send".equals(op)) {
                send(runtime, step);
            }
        }
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new LinkedHashMap<>();
        switch (eventType) {
            case "SupportBean" -> {
                SupportBean bean = new SupportBean(
                    payload.getString("theString", null),
                    payload.getInt("intPrimitive", 0));
                if (payload.get("intBoxed") != null && !payload.get("intBoxed").isNull()) {
                    bean.setIntBoxed(payload.get("intBoxed").asInt());
                }
                if (payload.get("longBoxed") != null && !payload.get("longBoxed").isNull()) {
                    bean.setLongBoxed(payload.get("longBoxed").asLong());
                }
                if (payload.get("floatBoxed") != null && !payload.get("floatBoxed").isNull()) {
                    bean.setFloatBoxed(payload.get("floatBoxed").asFloat());
                }
                if (payload.get("doubleBoxed") != null && !payload.get("doubleBoxed").isNull()) {
                    bean.setDoubleBoxed(payload.get("doubleBoxed").asDouble());
                }
                if (payload.get("boolPrimitive") != null && !payload.get("boolPrimitive").isNull()) {
                    bean.setBoolPrimitive(payload.get("boolPrimitive").asBoolean());
                }
                runtime.getEventService().sendEventBean(bean, eventType);
            }
            case "MySupportMap" -> {
                event.put("theString", payload.getString("theString", null));
                event.put("intPrimitive", payload.getInt("intPrimitive", 0));
                if (payload.get("intBoxed") != null && !payload.get("intBoxed").isNull()) {
                    event.put("intBoxed", payload.get("intBoxed").asInt());
                }
                if (payload.get("longBoxed") != null && !payload.get("longBoxed").isNull()) {
                    event.put("longBoxed", payload.get("longBoxed").asLong());
                }
                if (payload.get("floatBoxed") != null && !payload.get("floatBoxed").isNull()) {
                    event.put("floatBoxed", payload.get("floatBoxed").asFloat());
                }
                if (payload.get("doubleBoxed") != null && !payload.get("doubleBoxed").isNull()) {
                    event.put("doubleBoxed", payload.get("doubleBoxed").asDouble());
                }
                if (payload.get("boolPrimitive") != null && !payload.get("boolPrimitive").isNull()) {
                    event.put("boolPrimitive", payload.get("boolPrimitive").asBoolean());
                }
                runtime.getEventService().sendEventMap(event, eventType);
            }
            case "MyMap" -> {
                if (payload.get("intBoxed") != null && !payload.get("intBoxed").isNull()) {
                    event.put("intBoxed", payload.get("intBoxed").asInt());
                }
                if (payload.get("floatBoxed") != null && !payload.get("floatBoxed").isNull()) {
                    event.put("floatBoxed", payload.get("floatBoxed").asFloat());
                }
                if (payload.get("intArr") != null && payload.get("intArr").isArray()) {
                    JsonArray arr = payload.get("intArr").asArray();
                    int[] ints = new int[arr.size()];
                    for (int i = 0; i < arr.size(); i++) {
                        ints[i] = arr.get(i).asInt();
                    }
                    event.put("intArr", ints);
                }
                if (payload.get("mapProp") != null && payload.get("mapProp").isObject()) {
                    Map<String, Object> inner = new LinkedHashMap<>();
                    for (var member : payload.get("mapProp").asObject()) {
                        inner.put(member.getName(), member.getValue().asString());
                    }
                    event.put("mapProp", inner);
                }
                if (payload.get("nested") != null && payload.get("nested").isObject()) {
                    JsonObject nested = payload.get("nested").asObject();
                    event.put("nested", new SupportBeanComplexProps.SupportBeanSpecialGetterNested(
                        nested.getString("nestedValue", null), nested.getString("nestedNestedValue", null)));
                }
                runtime.getEventService().sendEventMap(event, eventType);
            }
            case "SupportBean_ST0" ->
                runtime.getEventService().sendEventBean(
                    new SupportBean_ST0(payload.getString("id", null), payload.getInt("p00", 0)), eventType);
            case "SupportBean_ST1" ->
                runtime.getEventService().sendEventBean(
                    new SupportBean_ST1(payload.getString("id", null), payload.getInt("p10", 0)), eventType);
            case "SupportBean_S0" ->
                runtime.getEventService().sendEventBean(
                    new SupportBean_S0(payload.getInt("id", 0)), eventType);
            case "SupportBean_N" ->
                runtime.getEventService().sendEventBean(
                    new SupportBean_N(
                        payload.getInt("intPrimitive", 0),
                        nullableInt(payload, "intBoxed"),
                        payload.getDouble("doublePrimitive", 0d),
                        nullableDouble(payload, "doubleBoxed"),
                        payload.getBoolean("boolPrimitive", false),
                        nullableBoolean(payload, "boolBoxed")), eventType);
            default -> throw new IllegalArgumentException("unsupported event type " + eventType);
        }
    }

    private static Integer nullableInt(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        return value == null || value.isNull() ? null : value.asInt();
    }

    private static Double nullableDouble(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        return value == null || value.isNull() ? null : value.asDouble();
    }

    private static Boolean nullableBoolean(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        return value == null || value.isNull() ? null : value.asBoolean();
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
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored, EPRuntime ignoredRuntime) {
            append(++sequence, newEvents, oldEvents);
        }

        private void append(long sequence, EventBean[] newEvents, EventBean[] oldEvents) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = results(newEvents);
            JsonArray oldArray = results(oldEvents);
            if (newArray.size() > 0) {
                record.add("new", newArray);
            }
            if (oldArray.size() > 0) {
                record.add("old", oldArray);
            }
            records.add(record);
        }
        private JsonArray results(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                java.util.Arrays.sort(names);
                for (String name : names) {
                    Object value;
                    try {
                        value = event.get(name);
                    } catch (com.espertech.esper.common.client.PropertyAccessException unreadable) {
                        // Bean property names without a readable getter (for
                        // example SupportBeanComplexProps.indexed) cannot be
                        // observed; skip them like the Java assertions do.
                        continue;
                    }
                    fields.add(name, normalize(value));
                }
                output.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return output;
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
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
            if (value instanceof Map<?, ?> map) {
                JsonObject object = new JsonObject();
                List<String> keys = new ArrayList<>();
                for (Object key : map.keySet()) {
                    keys.add(String.valueOf(key));
                }
                java.util.Collections.sort(keys);
                for (String key : keys) {
                    object.add(key, normalize(map.get(key)));
                }
                return object;
            }
            if (value.getClass().isArray()) {
                JsonArray array = new JsonArray();
                int length = java.lang.reflect.Array.getLength(value);
                for (int i = 0; i < length; i++) {
                    array.add(normalize(java.lang.reflect.Array.get(value, i)));
                }
                return array;
            }
            if (value instanceof SupportBeanComplexProps.SupportBeanSpecialGetterNested nestedBean) {
                JsonObject object = new JsonObject();
                object.add("nestedValue", normalize(nestedBean.getNestedValue()));
                object.add("nestedNestedValue", normalize(
                    nestedBean.getNestedNested() == null ? null : nestedBean.getNestedNested().getNestedNestedValue()));
                return object;
            }
            if (EXPANDABLE_EVENT_BEAN_MEMBERS.contains(value.getClass())) {
                return normalizeBeanProperties(value);
            }
            return Json.value(String.valueOf(value));
        }

        // Event-typed members of routed rows (constructor targets, join
        // populated object members, window(*) @eventbean arrays) expand into
        // sorted property maps, mirroring the map-value convention above.
        private static final Set<Class<?>> EXPANDABLE_EVENT_BEAN_MEMBERS = new HashSet<>(Arrays.asList(
            SupportBean.class, SupportBean_N.class, SupportBean_S0.class,
            SupportBean_ST0.class, SupportBean_ST1.class));

        private JsonValue normalizeBeanProperties(Object bean) {
            JsonObject object = new JsonObject();
            Map<String, Object> values = new TreeMap<>();
            try {
                for (PropertyDescriptor descriptor : Introspector.getBeanInfo(bean.getClass(), Object.class)
                        .getPropertyDescriptors()) {
                    if (descriptor.getReadMethod() == null ||
                        Modifier.isStatic(descriptor.getReadMethod().getModifiers())) {
                        continue;
                    }
                    values.put(descriptor.getName(), descriptor.getReadMethod().invoke(bean));
                }
            } catch (java.beans.IntrospectionException | ReflectiveOperationException e) {
                throw new IllegalStateException("failed to expand bean properties of " + bean.getClass(), e);
            }
            for (Map.Entry<String, Object> entry : values.entrySet()) {
                object.add(entry.getKey(), normalize(entry.getValue()));
            }
            return object;
        }
    }
}
