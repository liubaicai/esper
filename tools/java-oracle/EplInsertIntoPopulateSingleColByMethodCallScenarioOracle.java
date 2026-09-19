import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventType;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeAvro;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.util.JsonEventObject;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerContext;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.internal.avro.core.AvroGenericDataBackedEventBean;
import com.espertech.esper.common.internal.avro.support.SupportAvroUtil;
import com.espertech.esper.common.internal.event.bean.core.BeanEventType;
import com.espertech.esper.common.internal.event.core.MappedEventBean;
import com.espertech.esper.common.internal.event.core.ObjectArrayBackedEventBean;
import com.espertech.esper.common.internal.event.core.WrapperEventType;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.util.JavaClassHelper;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportMarketDataBean;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import org.apache.avro.Schema;
import org.apache.avro.SchemaBuilder;
import org.apache.avro.generic.GenericData;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Comparator;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.TreeSet;

/**
 * Java oracle for the EPL insert-into populate-single-column-by-method-call
 * parity scenario, replaying the single
 * EPLInsertIntoPopulateSingleColByMethodCall execution
 * (java-runtime-abe5e5cbda9667e7e112) across nine cases with one fresh
 * runtime per case.
 *
 * The implicit rounds (implicit-bean, implicit-map, implicit-oa,
 * implicit-avro, implicit-json) deploy the suite's verbatim pair
 * "@name('s1') @public insert into {Prefix}_Stream select * from {origin}"
 * then "@name('s2') @public insert into {Prefix}_Stream select
 * com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib.{fn}(s0)
 * from {eventType} as s0". s1's listener is attached but stays silent
 * because the sent event is the sibling type; s2's listener observes the
 * single delivered row. The configured rounds (configured-map,
 * configured-oa, configured-avro, configured-json) deploy
 * "@name('insert') insert into {target} select ...{fn}(s0) from {origin}
 * as s0" then "@name('s0') select * from {target}", and s0's listener
 * observes the delivered row.
 *
 * Each SupportStaticMethodLib.convertEvent* UDF wraps the source event's
 * "two" property in pipe characters; convertEvent maps a
 * SupportMarketDataBean to a SupportBean carrying the symbol and the
 * intValue'd volume. Each round sends exactly one event.
 *
 * Record protocol (frozen work-unit contract): listener records carry the
 * asserted listener's rendered rows (new = rows, old = []); bean rows
 * render the asserted-field projection {theString,intPrimitive} while
 * map/object-array/Avro/JSON rows render all schema fields sorted by name.
 * Value records pin the suite's underlying-type assertions as schema-kind
 * names (Struct/Map/ObjectArray/Avro/JSON): implicit cases emit value(s1)
 * then value(s2) at deploy time before the send's listener record;
 * configured cases emit value(s0) after the send's listener record,
 * mirroring assertEventNew timing. Internal timer disabled so trace
 * timestamps are epoch zero.
 */
public class EplInsertIntoPopulateSingleColByMethodCallScenarioOracle {

    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "epl-insert-into-populate-single-col-method-call";
    private static final String LIB = "com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib";

    private enum Variant {
        IMPLICIT, CONFIGURED
    }

    /**
     * One round of the pinned run(): implicit rounds carry prefix, origin
     * (s1's from-type), sentType (s2's from-type and the sent event type)
     * and eventTypeType (the delivered event's asserted eventType class);
     * configured rounds carry origin (the insert's from-type and the sent
     * event type), target and eventBeanType (the delivered event's
     * asserted EventBean class).
     */
    private static final class CaseSpec {
        private final String caseName;
        private final Variant variant;
        private final String prefix;
        private final String origin;
        private final String fn;
        private final String sentType;
        private final String target;
        private final Class<?> eventTypeType;
        private final Class<?> eventBeanType;
        private final Class<?> underlyingType;
        private final String[] propertyNames;
        private final Object[] propertyValues;

        private CaseSpec(String caseName, Variant variant, String prefix, String origin, String fn,
                         String sentType, String target, Class<?> eventTypeType, Class<?> eventBeanType,
                         Class<?> underlyingType, String[] propertyNames, Object[] propertyValues) {
            this.caseName = caseName;
            this.variant = variant;
            this.prefix = prefix;
            this.origin = origin;
            this.fn = fn;
            this.sentType = sentType;
            this.target = target;
            this.eventTypeType = eventTypeType;
            this.eventBeanType = eventBeanType;
            this.underlyingType = underlyingType;
            this.propertyNames = propertyNames;
            this.propertyValues = propertyValues;
        }
    }

    private static final CaseSpec[] CASES = {
        new CaseSpec("implicit-bean", Variant.IMPLICIT, "Bean", "SupportBean", "convertEvent",
            "SupportMarketDataBean", null, BeanEventType.class, null, SupportBean.class,
            new String[]{"theString"}, new Object[]{"ACME"}),
        new CaseSpec("implicit-map", Variant.IMPLICIT, "Map", "MapOne", "convertEventMap",
            "MapTwo", null, WrapperEventType.class, null, Map.class,
            new String[]{"one", "two"}, new Object[]{"1", "|2|"}),
        new CaseSpec("configured-map", Variant.CONFIGURED, null, "MapTwo", "convertEventMap",
            null, "MapOne", null, MappedEventBean.class, HashMap.class,
            new String[]{"one", "two"}, new Object[]{"3", "|4|"}),
        new CaseSpec("implicit-oa", Variant.IMPLICIT, "OA", "OAOne", "convertEventObjectArray",
            "OATwo", null, WrapperEventType.class, null, Object[].class,
            new String[]{"one", "two"}, new Object[]{"1", "|2|"}),
        new CaseSpec("configured-oa", Variant.CONFIGURED, null, "OATwo", "convertEventObjectArray",
            null, "OAOne", null, ObjectArrayBackedEventBean.class, Object[].class,
            new String[]{"one", "two"}, new Object[]{"3", "|4|"}),
        new CaseSpec("implicit-avro", Variant.IMPLICIT, "Avro", "AvroOne", "convertEventAvro",
            "AvroTwo", null, WrapperEventType.class, null, GenericData.Record.class,
            new String[]{"one", "two"}, new Object[]{"1", "|2|"}),
        new CaseSpec("configured-avro", Variant.CONFIGURED, null, "AvroTwo", "convertEventAvro",
            null, "AvroOne", null, AvroGenericDataBackedEventBean.class, GenericData.Record.class,
            new String[]{"one", "two"}, new Object[]{"3", "|4|"}),
        new CaseSpec("implicit-json", Variant.IMPLICIT, "Json", "JsonOne", "convertEventJson",
            "JsonTwo", null, WrapperEventType.class, null, JsonEventObject.class,
            new String[]{"one", "two"}, new Object[]{"1", "|2|"}),
        new CaseSpec("configured-json", Variant.CONFIGURED, null, "JsonTwo", "convertEventJson",
            null, "JsonOne", null, Object.class, Object.class,
            new String[]{"one", "two"}, new Object[]{"3", "|4|"}),
    };

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EplInsertIntoPopulateSingleColByMethodCallScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        if (!SCENARIO_ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("unexpected scenario id " + scenario.getString("id", ""));
        }
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            String caseName = caseVal.asObject().getString("case", "");
            runCase(allSteps, specFor(caseName), records);
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
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

    private static CaseSpec specFor(String caseName) {
        for (CaseSpec spec : CASES) {
            if (spec.caseName.equals(caseName)) {
                return spec;
            }
        }
        throw new IllegalStateException("unknown case: " + caseName);
    }

    private static void runCase(JsonArray allSteps, CaseSpec spec, List<JsonObject> records) throws Exception {
        // Preconfigured types mirror TestSuiteEPLInsertInto.configure: the
        // bean classes, MapOne/MapTwo {one:string,two:string}, OAOne/OATwo
        // props [one,two] types [string,string] and AvroOne/AvroTwo over the
        // shared record("name"){one,two} schema with Avro enabled. JSON
        // types are created mid-run by the suite, not preconfigured.
        Configuration config = new Configuration();
        config.getCommon().addEventType(SupportBean.class);
        config.getCommon().addEventType(SupportMarketDataBean.class);
        Map<String, Object> mapType = new HashMap<>();
        mapType.put("one", String.class);
        mapType.put("two", String.class);
        config.getCommon().addEventType("MapOne", mapType);
        config.getCommon().addEventType("MapTwo", mapType);
        String[] oaProps = {"one", "two"};
        Object[] oaTypes = {String.class, String.class};
        config.getCommon().addEventType("OAOne", oaProps, oaTypes);
        config.getCommon().addEventType("OATwo", oaProps, oaTypes);
        config.getCommon().getEventMeta().getAvroSettings().setEnableAvro(true);
        Schema avroSchema = SchemaBuilder.record("name").fields()
            .requiredString("one").requiredString("two").endRecord();
        config.getCommon().addEventTypeAvro("AvroOne", new ConfigurationCommonEventTypeAvro(avroSchema));
        config.getCommon().addEventTypeAvro("AvroTwo", new ConfigurationCommonEventTypeAvro(avroSchema));
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        // The pinned runner configures SupportExceptionHandlerFactoryRethrow,
        // so statement exceptions rethrow on the sending thread; mirror that
        // wrapper so failures surface as exceptions rather than log lines.
        config.getRuntime().getExceptionHandling().addClass(RethrowExceptionHandlerFactory.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
            "EplInsertIntoPopulateSingleColByMethodCallScenarioOracle-" + spec.caseName, config);
        runtime.getEventService().advanceTime(0);
        // Callback records buffer here and are flushed after each step in
        // the frozen contract order (listener, value).
        List<JsonObject> pending = new ArrayList<>();
        try {
            // The suite creates the JSON types mid-run through the same
            // compileDeploy(path) channel before the JSON rounds.
            if (spec.caseName.endsWith("-json")) {
                compileDeploy(config, runtime,
                    "@buseventtype @public create json schema JsonOne(one string, two string);\n" +
                        "@buseventtype @public create json schema JsonTwo(one string, two string);\n");
            }

            Map<String, Integer> listenerSeq = new HashMap<>();
            boolean[] valueEmitted = {false};
            UpdateListener listener = newListener(spec, runtime, pending, listenerSeq, valueEmitted);

            if (spec.variant == Variant.IMPLICIT) {
                String streamName = spec.prefix + "_Stream";
                EPStatement s1 = deployOne(config, runtime,
                    "@name('s1') @public insert into " + streamName + " select * from " + spec.origin, "s1");
                s1.addListener(listener);
                assertSubclass(s1.getEventType().getUnderlyingType(), spec.underlyingType, spec.caseName, "s1");
                records.add(valueRecord(spec.caseName, "s1", kindOf(s1.getEventType().getUnderlyingType())));

                EPStatement s2 = deployOne(config, runtime,
                    "@name('s2') @public insert into " + streamName + " select " + LIB + "." + spec.fn +
                        "(s0) from " + spec.sentType + " as s0", "s2");
                s2.addListener(listener);
                assertSubclass(s2.getEventType().getUnderlyingType(), spec.underlyingType, spec.caseName, "s2");
                records.add(valueRecord(spec.caseName, "s2", kindOf(s2.getEventType().getUnderlyingType())));
            } else {
                deployOne(config, runtime,
                    "@name('insert') insert into " + spec.target + " select " + LIB + "." + spec.fn +
                        "(s0) from " + spec.origin + " as s0", "insert");
                EPStatement s0 = deployOne(config, runtime,
                    "@name('s0') select * from " + spec.target, "s0");
                s0.addListener(listener);
            }

            boolean inCase = false;
            for (JsonValue stepVal : allSteps) {
                JsonObject step = stepVal.asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    inCase = spec.caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                if ("send".equals(op)) {
                    sendEvent(runtime, step);
                } else {
                    throw new IllegalStateException("unsupported op " + op);
                }
                flushPending(pending, records);
            }
            flushPending(pending, records);
        } finally {
            runtime.destroy();
            flushPending(pending, records);
        }
    }

    /**
     * Asserted listener (s2 implicit / s0 configured; also attached to the
     * silent s1 exactly like the suite). Mirrors assertEventNew: exactly
     * one newData row and no oldData, then the round's event-type /
     * EventBean-class / underlying-class / property assertions. For
     * configured rounds the value record pinning the delivered event's
     * underlying kind is emitted right after the listener record,
     * mirroring assertEventNew timing.
     */
    private static UpdateListener newListener(CaseSpec spec, EPRuntime runtime, List<JsonObject> pending,
                                              Map<String, Integer> listenerSeq, boolean[] valueEmitted) {
        return (newData, oldData, statement, rt) -> {
            if ((newData == null || newData.length == 0) && (oldData == null || oldData.length == 0)) {
                return;
            }
            JsonObject record = new JsonObject();
            record.add("case", spec.caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", listenerSeq.merge(statement.getName(), 1, Integer::sum));
            record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            record.add("new", renderRows(newData));
            record.add("old", renderRows(oldData));
            pending.add(record);

            if (newData == null || newData.length != 1 || (oldData != null && oldData.length != 0)) {
                throw new IllegalStateException("expected exactly one newData row and no oldData for statement " +
                    statement.getName() + " in case " + spec.caseName);
            }
            EventBean theEvent = newData[0];
            if (spec.variant == Variant.IMPLICIT) {
                assertSubclass(theEvent.getEventType().getClass(), spec.eventTypeType, spec.caseName,
                    statement.getName());
                assertSubclass(theEvent.getUnderlying().getClass(), spec.underlyingType, spec.caseName,
                    statement.getName());
            } else {
                assertSubclass(theEvent.getUnderlying().getClass(), spec.underlyingType, spec.caseName,
                    statement.getName());
                assertSubclass(theEvent.getClass(), spec.eventBeanType, spec.caseName, statement.getName());
            }
            assertProps(theEvent, spec);

            if (spec.variant == Variant.CONFIGURED && !valueEmitted[0]) {
                valueEmitted[0] = true;
                pending.add(valueRecord(spec.caseName, statement.getName(),
                    kindOf(theEvent.getUnderlying().getClass())));
            }
        };
    }

    private static void assertSubclass(Class<?> candidate, Class<?> expectedSuper, String caseName,
                                       String statementName) {
        if (!JavaClassHelper.isSubclassOrImplementsInterface(candidate, expectedSuper)) {
            throw new IllegalStateException("case " + caseName + " statement " + statementName + ": " +
                candidate.getName() + " is not a subclass or implementation of " + expectedSuper.getName());
        }
    }

    /** Mirrors EPAssertionUtil.assertProps over the round's asserted fields. */
    private static void assertProps(EventBean event, CaseSpec spec) {
        for (int i = 0; i < spec.propertyNames.length; i++) {
            Object actual = event.get(spec.propertyNames[i]);
            if (!Objects.equals(actual, spec.propertyValues[i])) {
                throw new IllegalStateException("case " + spec.caseName + " property " + spec.propertyNames[i] +
                    " expected " + spec.propertyValues[i] + " but was " + actual);
            }
        }
    }

    /**
     * Maps an underlying class to the frozen schema-kind name:
     * SupportBean-&gt;Struct, Map-&gt;Map, Object[]-&gt;ObjectArray,
     * GenericData.Record-&gt;Avro, JsonEventObject-&gt;JSON.
     */
    private static String kindOf(Class<?> underlyingClass) {
        // JsonEventObject extends Map, so the JSON check must run before
        // the Map check.
        if (JsonEventObject.class.isAssignableFrom(underlyingClass)) {
            return "JSON";
        }
        if (SupportBean.class.isAssignableFrom(underlyingClass)) {
            return "Struct";
        }
        if (Map.class.isAssignableFrom(underlyingClass)) {
            return "Map";
        }
        if (Object[].class.isAssignableFrom(underlyingClass)) {
            return "ObjectArray";
        }
        if (GenericData.Record.class.isAssignableFrom(underlyingClass)) {
            return "Avro";
        }
        throw new IllegalStateException("no kind mapping for underlying class " + underlyingClass.getName());
    }

    private static JsonObject valueRecord(String caseName, String statementName, String kind) {
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "value");
        record.add("statement", statementName);
        record.add("sequence", 0);
        record.add("value", kind);
        return record;
    }

    /**
     * Emits buffered callback records in the frozen contract order
     * listener, value.
     */
    private static void flushPending(List<JsonObject> pending, List<JsonObject> records) {
        if (pending.isEmpty()) {
            return;
        }
        pending.sort(Comparator.comparingInt(EplInsertIntoPopulateSingleColByMethodCallScenarioOracle::operationOrder));
        records.addAll(pending);
        pending.clear();
    }

    private static int operationOrder(JsonObject record) {
        return "listener".equals(record.getString("operation", "")) ? 0 : 1;
    }

    /**
     * Sequential deploys mirror env.compileDeploy(epl, path): each module
     * sees the @public types of previous deployments through the
     * accumulated runtime path.
     */
    private static EPDeployment compileDeploy(Configuration config, EPRuntime runtime, String epl) throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(config);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        return runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
    }

    private static EPStatement deployOne(Configuration config, EPRuntime runtime, String epl,
                                         String statementName) throws Exception {
        EPDeployment deployment = compileDeploy(config, runtime, epl);
        for (EPStatement stmt : deployment.getStatements()) {
            if (statementName.equals(stmt.getName())) {
                return stmt;
            }
        }
        throw new IllegalStateException("deployment has no statement named " + statementName);
    }

    /**
     * Mirrors the suite's FunctionSendEventWType sends: FBEANWTYPE
     * (sendEventBean), FMAPWTYPE (sendEventMap), FOAWTYPE
     * (sendEventObjectArray), FAVROWTYPE (GenericData.validate then
     * sendEventAvro over the preconfigured AvroTwo schema, like
     * env.runtimeAvroSchemaPreconfigured) and FJSONWTYPE (sendEventJson of
     * the payload rendered as a JSON object string).
     */
    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String type = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (type) {
            case "SupportMarketDataBean" -> {
                JsonValue feedVal = payload.get("feed");
                String feed = feedVal == null || feedVal.isNull() ? null : feedVal.asString();
                SupportMarketDataBean event = new SupportMarketDataBean(
                    payload.getString("symbol", null),
                    payload.get("price").asDouble(),
                    payload.get("volume").asLong(),
                    feed);
                runtime.getEventService().sendEventBean(event, "SupportMarketDataBean");
            }
            case "MapTwo" -> {
                Map<String, Object> event = new HashMap<>();
                event.put("one", payload.getString("one", null));
                event.put("two", payload.getString("two", null));
                runtime.getEventService().sendEventMap(event, "MapTwo");
            }
            case "OATwo" -> runtime.getEventService().sendEventObjectArray(
                new Object[]{payload.getString("one", null), payload.getString("two", null)}, "OATwo");
            case "AvroTwo" -> {
                EventType eventType = runtime.getEventTypeService().getEventTypePreconfigured("AvroTwo");
                Schema schema = SupportAvroUtil.getAvroSchema(eventType);
                GenericData.Record record = new GenericData.Record(schema);
                record.put("one", payload.getString("one", null));
                record.put("two", payload.getString("two", null));
                GenericData.get().validate(record.getSchema(), record);
                runtime.getEventService().sendEventAvro(record, "AvroTwo");
            }
            case "JsonTwo" -> {
                JsonObject object = new JsonObject();
                object.add("one", payload.getString("one", null));
                object.add("two", payload.getString("two", null));
                runtime.getEventService().sendEventJson(object.toString(), "JsonTwo");
            }
            default -> throw new IllegalStateException("unknown eventType: " + type);
        }
    }

    private static JsonArray renderRows(EventBean[] events) {
        JsonArray rows = new JsonArray();
        if (events == null) {
            return rows;
        }
        for (EventBean event : events) {
            rows.add(renderRow(event));
        }
        return rows;
    }

    private static JsonObject renderRow(EventBean event) {
        JsonObject fields = new JsonObject();
        if (event.getUnderlying() instanceof SupportBean) {
            // The bean round asserts only theString; render the
            // asserted-field projection {theString,intPrimitive}
            // (intPrimitive=0 is deterministic from the source event) so
            // the trace carries no unasserted engine defaults.
            fields.add("intPrimitive", normalize(event.get("intPrimitive")));
            fields.add("theString", normalize(event.get("theString")));
        } else {
            for (String prop : new TreeSet<>(Arrays.asList(event.getEventType().getPropertyNames()))) {
                fields.add(prop, normalize(event.get(prop)));
            }
        }
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        item.add("fields", fields);
        return item;
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
        if (value instanceof EventBean) {
            EventBean inner = (EventBean) value;
            JsonObject fields = new JsonObject();
            for (String prop : new TreeSet<>(Arrays.asList(inner.getEventType().getPropertyNames()))) {
                fields.add(prop, normalize(inner.get(prop)));
            }
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        if (value instanceof SupportBean) {
            // Same asserted-field projection as renderRow so a nested bean
            // fragment never renders unasserted engine defaults.
            SupportBean bean = (SupportBean) value;
            JsonObject fields = new JsonObject();
            fields.add("intPrimitive", normalize(bean.getIntPrimitive()));
            fields.add("theString", normalize(bean.getTheString()));
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        return Json.value(String.valueOf(value));
    }

    /**
     * Mirrors the pinned runner's SupportExceptionHandlerFactoryRethrow:
     * statement exceptions rethrow on the sending thread wrapped as
     * "Unexpected exception in statement '&lt;name&gt;': &lt;cause&gt;".
     */
    public static class RethrowExceptionHandlerFactory implements ExceptionHandlerFactory {
        @Override
        public ExceptionHandler getHandler(ExceptionHandlerFactoryContext context) {
            return new ExceptionHandler() {
                @Override
                public void handle(ExceptionHandlerContext context) {
                    throw new RuntimeException("Unexpected exception in statement '" + context.getStatementName() +
                        "': " + context.getThrowable().getMessage(), context.getThrowable());
                }
            };
        }
    }
}
