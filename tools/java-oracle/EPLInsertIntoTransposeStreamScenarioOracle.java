import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBeanComplexProps;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;
import org.apache.avro.Schema;
import org.apache.avro.generic.GenericData;

import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

import static org.apache.avro.SchemaBuilder.record;

/**
 * Oracle for the EPLInsertIntoTransposeStream observable executions:
 * CreateSchemaPOJO, MapAndObjectArrayAndOthers (map/objectarray/avro/json/
 * jsonprovided), FunctionToStreamWithProps, FunctionToStream,
 * SingleColumnInsert, EventJoinMap, EventJoinPOJO and POJOPropertyStream.
 *
 * The transpose payloads are produced by the same single-row functions the
 * Java regression suite wires (generateMap/generateOA/generateAvro/
 * generateJson/custom/customOne/customTwo), and the target beans use the real
 * SupportBean/SupportBeanComplexProps classes from the fixed esper-common
 * classpath, so routed event values reflect the original bean assertions
 * exactly. Statement EPL mirrors the Java suite text (including @public
 * auto-created insert-into targets resolved through the runtime path, which
 * the suite's RegressionPath reproduces). Each per-case driver mirrors the
 * Go parity test's observed listener surface (which statements are subscribed
 * and in which phase relative to the sends), so the persisted trace's per-case
 * record counts and field values match the Go assertions record for record.
 *
 * Harness note: for MapAndObjectArrayAndOthers the Java suite re-uses the
 * literal schema name "MySchema" on each representation iteration, while this
 * oracle names them MySchemaMap/MySchemaOA/MySchemaAvro/MySchemaJSON/
 * MySchemaJsonProvided. Each case runs in a fresh runtime and the observable
 * trace contract (routed p0/p1 values, record counts, phase order) is
 * identical, so the split is a per-representation naming simplification with
 * equal observable surface.
 *
 * Internal timer disabled so trace timestamps are epoch zero.
 */
public final class EPLInsertIntoTransposeStreamScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private EPLInsertIntoTransposeStreamScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: EPLInsertIntoTransposeStreamScenarioOracle <scenario.json>");
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
            "create-schema-pojo",
            "map",
            "objectarray",
            "avro",
            "json",
            "jsonprovided",
            "function-to-stream-with-props",
            "function-to-stream",
            "single-column-insert",
            "event-join-map",
            "event-join-pojo",
            "pojo-property-stream"
        };
        for (String caseName : cases) {
            if (hasCase(steps, caseName)) {
                runCase(steps, caseName, records);
            }
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

    /** Ordered send steps belonging to one case. */
    private static List<JsonObject> caseSends(JsonArray allSteps, String caseName) {
        List<JsonObject> sends = new ArrayList<>();
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
                sends.add(step);
            }
        }
        return sends;
    }

    private static void runCase(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        configuration.getCommon().addEventType("SupportBeanComplexProps", SupportBeanComplexProps.class);
        configuration.getCompiler().addPlugInSingleRowFunction("generateMap", EPLInsertIntoTransposeStreamScenarioOracle.class.getName(), "localGenerateMap");
        configuration.getCompiler().addPlugInSingleRowFunction("generateOA", EPLInsertIntoTransposeStreamScenarioOracle.class.getName(), "localGenerateOA");
        configuration.getCompiler().addPlugInSingleRowFunction("generateAvro", EPLInsertIntoTransposeStreamScenarioOracle.class.getName(), "localGenerateAvro");
        configuration.getCompiler().addPlugInSingleRowFunction("generateJson", EPLInsertIntoTransposeStreamScenarioOracle.class.getName(), "localGenerateJson");
        configuration.getCompiler().addPlugInSingleRowFunction("custom", EPLInsertIntoTransposeStreamScenarioOracle.class.getName(), "makeSupportBean");
        configuration.getCompiler().addPlugInSingleRowFunction("customOne", EPLInsertIntoTransposeStreamScenarioOracle.class.getName(), "makeSupportBean");
        configuration.getCompiler().addPlugInSingleRowFunction("customTwo", EPLInsertIntoTransposeStreamScenarioOracle.class.getName(), "makeSupportBeanNumeric");
        configuration.getCommon().getEventMeta().getAvroSettings().setEnableAvro(true);

        switch (caseName) {
            case "single-column-insert" -> {
                // The suite config registers SupportBeanNumeric before the exec
                // statements compile; mirror with our class-backed bean.
                configuration.getCommon().addEventType("SupportBeanNumeric", MySupportBeanNumeric.class);
            }
            case "event-join-map" -> {
                Map<String, Object> metadata = new LinkedHashMap<>();
                metadata.put("id", String.class);
                configuration.getCommon().addEventType("AEventTE", metadata);
                configuration.getCommon().addEventType("BEventTE", metadata);
            }
            case "event-join-pojo" -> {
                configuration.getCommon().addEventType("SupportBean_A", MySupportBeanA.class);
                configuration.getCommon().addEventType("SupportBean_B", MySupportBeanB.class);
            }
            default -> { }
        }

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-tx-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            Driver driver = new Driver(configuration, runtime, allSteps, caseName, records);
            switch (caseName) {
                case "create-schema-pojo" -> driver.createSchemaPojo();
                case "map" -> driver.representation("Map", "@EventRepresentation('map')",
                    "insert into MySchemaMap select transpose(generateMap(theString, intPrimitive)) from SupportBean");
                case "objectarray" -> driver.representation("OA", "@EventRepresentation('objectarray')",
                    "insert into MySchemaOA select transpose(generateOA(theString, intPrimitive)) from SupportBean");
                case "avro" -> driver.representation("Avro", "@EventRepresentation('avro')",
                    "insert into MySchemaAvro select transpose(generateAvro(theString, intPrimitive)) from SupportBean");
                case "json" -> driver.representation("JSON", "@EventRepresentation('json')",
                    "insert into MySchemaJSON select transpose(generateJson(theString, intPrimitive)) from SupportBean");
                case "jsonprovided" -> driver.representation("JsonProvided", "@JsonSchema(className='" + MyLocalJsonProvidedMySchema.class.getName() + "') @EventRepresentation('json')",
                    "insert into MySchemaJsonProvided select transpose(generateJson(theString, intPrimitive)) from SupportBean");
                case "function-to-stream-with-props" -> driver.functionToStreamWithProps();
                case "function-to-stream" -> driver.functionToStream();
                case "single-column-insert" -> driver.singleColumnInsert();
                case "event-join-map" -> driver.eventJoinMap();
                case "event-join-pojo" -> driver.eventJoinPojo();
                case "pojo-property-stream" -> driver.pojoPropertyStream();
                default -> throw new IllegalArgumentException("unsupported case " + caseName);
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * Per-case driver. EPL mirrors the Java suite text, including @public
     * auto-created insert-into targets resolved through the runtime path (the
     * regression harness's RegressionPath). Listener subscription phases match
     * the Go parity test, so trace record counts align with its assertions.
     */
    private static final class Driver {
        private final Configuration configuration;
        private final EPRuntime runtime;
        private final List<JsonObject> sends;
        private final TraceWriter writer;
        private int deployCount;

        private Driver(Configuration configuration, EPRuntime runtime, JsonArray allSteps, String caseName, JsonArray records) {
            this.configuration = configuration;
            this.runtime = runtime;
            this.sends = caseSends(allSteps, caseName);
            this.writer = new TraceWriter(records, caseName, runtime);
        }

        private EPCompiled compile(String epl) throws Exception {
            CompilerArguments compilerArguments = new CompilerArguments(configuration);
            compilerArguments.getPath().add(runtime.getRuntimePath());
            return EPCompilerProvider.getCompiler().compile(epl, compilerArguments);
        }

        private EPDeployment deploy(String epl) throws Exception {
            EPCompiled compiled = compile(epl);
            return runtime.getDeploymentService().deploy(compiled,
                new DeploymentOptions().setDeploymentId("parity-tx-" + writer.caseName + "-" + (deployCount++)));
        }

        /** Deploys a single-statement module and returns its first statement. */
        private EPStatement deployStatement(String epl) throws Exception {
            EPDeployment deployment = deploy(epl);
            if (deployment.getStatements().length == 0) {
                throw new IllegalStateException("deployment has no statement");
            }
            return deployment.getStatements()[0];
        }

        /** Deploys one statement and observes it under the given name. */
        private EPStatement deployObserved(String epl, String name) throws Exception {
            EPStatement statement = findStatement(deploy(epl), name);
            writer.observe(statement);
            return statement;
        }

        private EPStatement findStatement(EPDeployment deployment, String name) {
            for (EPStatement candidate : deployment.getStatements()) {
                if (name.equals(candidate.getName())) {
                    return candidate;
                }
            }
            throw new IllegalStateException("deployment has no statement named " + name);
        }

        /** Delivers one scenario send step by index. */
        private void sendStep(int index) {
            sendEvent(runtime, sends.get(index));
        }


        private void createSchemaPojo() throws Exception {
            String epl =
                "create schema SupportBeanTwo as " + MySupportBeanTwo.class.getName() + ";\n" +
                "on SupportBean event insert into astream select transpose(" + EPLInsertIntoTransposeStreamScenarioOracle.class.getName() + ".makeSB2Event(event));\n" +
                "on SupportBean event insert into bstream select transpose(" + EPLInsertIntoTransposeStreamScenarioOracle.class.getName() + ".makeSB2Event(event));\n" +
                "@name('a') select * from astream;\n" +
                "@name('b') select * from bstream;\n";
            EPDeployment deployment = deploy(epl);
            writer.observe(findStatement(deployment, "a"));
            writer.observe(findStatement(deployment, "b"));
            sendStep(0);
        }

        private void representation(String rep, String annotation, String producerEpl) throws Exception {
            deploy(annotation + "@public @buseventtype create schema MySchema" + rep + "(p0 string, p1 int);");
            // producer s0 routes through the pre-registered/created target;
            // the consumer stays subscribed across both producers.
            EPStatement producer = deployStatement("@name('p') " + producerEpl);
            EPStatement consumer = deployStatement("@name('s0-consumer') select * from MySchema" + rep);
            writer.observe(consumer);
            sendStep(0);
            sendStep(1);
            undeploy(producer);
            deploy("@name('p') " + producerEpl);
            sendStep(2);
        }

        private void functionToStreamWithProps() throws Exception {
            deploy("@public insert into MyStream select " +
                "1 as dummy, transpose(custom('O' || theString, 10)) from SupportBean(theString like 'I%')");
            EPStatement consumer = deployStatement("@name('s0') select * from MyStream");
            writer.observe(consumer);
            sendStep(0);
        }

        private void functionToStream() throws Exception {
            deploy("@name('first') @public insert into OtherStream " +
                "select transpose(custom('O' || theString, 10)) from SupportBean(theString like 'I%')");
            EPStatement s0 = deployStatement("@name('s0') select * from OtherStream(theString like 'O%')");
            writer.observe(s0);
            sendStep(0); // I1 -> s0 observes OI1.
            undeploy(s0);
            // "second" producer reuses the already-existing OtherStream; its
            // own listener observes the next routed event, matching Java.
            deployObserved("@name('second') insert into OtherStream " +
                "select transpose(custom('O' || theString, 10)) from SupportBean(theString like 'I%')", "second");
            sendStep(1); // I2 -> second observes OI2.
        }

        private void singleColumnInsert() throws Exception {
            EPStatement same = deployObserved("@name('same-s0') insert into SupportBean " +
                "select transpose(customOne('O' || theString, 10)) from SupportBean(theString like 'I%')", "same-s0");
            sendStep(0); // I1 -> same-s0 observes OI1.
            undeploy(same);
            deployObserved("@name('numeric-s0') insert into SupportBeanNumeric " +
                "select transpose(customTwo(intPrimitive, intPrimitive+1)) as col1 from SupportBean(theString like 'I%')", "numeric-s0");
            sendStep(1); // I2 -> numeric-s0 observes {10, 11}.
        }

        private void eventJoinMap() throws Exception {
            deploy("@public insert into MyStreamTE select a, b from AEventTE#keepall as a, BEventTE#keepall as b;");
            EPStatement s0 = deployStatement("@name('s0') select a.id, b.id from MyStreamTE");
            writer.observe(s0);
            sendStep(0); // A1 alone: join not yet complete, no output.
            sendStep(1); // B1 completes the join -> a.id=A1, b.id=B1.
        }

        private void eventJoinPojo() throws Exception {
            deploy("@public insert into MyStream2Bean select a.* as a, b.* as b from SupportBean_A#keepall as a, SupportBean_B#keepall as b;");
            EPStatement s0 = deployStatement("@name('s0') select a.id, b.id from MyStream2Bean");
            writer.observe(s0);
            sendStep(0); // A1 alone: no output.
            sendStep(1); // B1 completes the join -> a.id=A1, b.id=B1.
        }

        private void pojoPropertyStream() throws Exception {
            deploy("@public insert into MyStreamComplex select nested as inneritem from SupportBeanComplexProps;");
            EPStatement s0 = deployStatement("@name('s0') select * from MyStreamComplex");
            writer.observe(s0);
            sendStep(0); // one ComplexProps event -> inneritem.nestedValue = nestedValue.
        }

        private void undeploy(EPStatement statement) throws Exception {
            writer.unobserve(statement);
            runtime.getDeploymentService().undeploy(statement.getDeploymentId());
        }
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (eventType) {
            case "SupportBean" -> {
                SupportBean bean = new SupportBean(
                    payload.getString("theString", null),
                    payload.getInt("intPrimitive", 0));
                runtime.getEventService().sendEventBean(bean, eventType);
            }
            case "SupportBeanComplexProps" -> {
                runtime.getEventService().sendEventBean(SupportBeanComplexProps.makeDefaultBean(), eventType);
            }
            case "AEventTE", "BEventTE" -> {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("id", payload.getString("id", null));
                runtime.getEventService().sendEventMap(event, eventType);
            }
            case "SupportBean_A" -> {
                MySupportBeanA bean = new MySupportBeanA(payload.getString("id", null));
                runtime.getEventService().sendEventBean(bean, eventType);
            }
            case "SupportBean_B" -> {
                MySupportBeanB bean = new MySupportBeanB(payload.getString("id", null));
                runtime.getEventService().sendEventBean(bean, eventType);
            }
            default -> throw new IllegalArgumentException("unsupported event type " + eventType);
        }
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

        private void observe(EPStatement statement) {
            statement.addListener(this);
        }

        private void unobserve(EPStatement statement) {
            statement.removeListener(this);
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement, EPRuntime ignoredRuntime) {
            append(++sequence, statement.getName(), newEvents, oldEvents);
        }

        private void append(long sequence, String statementName, EventBean[] newEvents, EventBean[] oldEvents) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statementName)
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
            if (value instanceof EventBean eventBean) {
                return normalizeEventBean(eventBean);
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
            return Json.value(String.valueOf(value));
        }

        private JsonValue normalizeEventBean(EventBean eventBean) {
            JsonObject object = new JsonObject();
            object.add("__type", eventBean.getEventType().getName());
            String[] names = eventBean.getEventType().getPropertyNames().clone();
            java.util.Arrays.sort(names);
            for (String name : names) {
                Object value;
                try {
                    value = eventBean.get(name);
                } catch (com.espertech.esper.common.client.PropertyAccessException unreadable) {
                    continue;
                }
                object.add(name, normalize(value));
            }
            return object;
        }
    }

    // UDF helpers mirroring the Java suite's wiring.

    public static Map<String, Object> localGenerateMap(String string, int intPrimitive) {
        Map<String, Object> out = new java.util.HashMap<>();
        out.put("p0", string);
        out.put("p1", intPrimitive);
        return out;
    }

    public static Object[] localGenerateOA(String string, int intPrimitive) {
        return new Object[]{string, intPrimitive};
    }

    public static GenericData.Record localGenerateAvro(String string, int intPrimitive) {
        Schema schema = record("name").fields().requiredString("p0").requiredInt("p1").endRecord();
        GenericData.Record record = new GenericData.Record(schema);
        record.put("p0", string);
        record.put("p1", intPrimitive);
        return record;
    }

    public static String localGenerateJson(String string, int intPrimitive) {
        JsonObject object = new JsonObject();
        object.add("p0", string);
        object.add("p1", intPrimitive);
        return object.toString();
    }

    public static SupportBean makeSupportBean(String theString, Integer intPrimitive) {
        return new SupportBean(theString, intPrimitive == null ? 0 : intPrimitive);
    }

    public static MySupportBeanNumeric makeSupportBeanNumeric(Integer intOne, Integer intTwo) {
        return new MySupportBeanNumeric(intOne == null ? 0 : intOne, intTwo == null ? 0 : intTwo);
    }

    public static MySupportBeanTwo makeSB2Event(SupportBean sb) {
        return new MySupportBeanTwo(sb.getTheString(), sb.getIntPrimitive());
    }

    // Bean classes mirroring the Java source underlyings.

    public static class MySupportBeanTwo {
        private final String stringTwo;
        private final int intPrimitiveTwo;

        public MySupportBeanTwo(String stringTwo, int intPrimitiveTwo) {
            this.stringTwo = stringTwo;
            this.intPrimitiveTwo = intPrimitiveTwo;
        }

        public String getStringTwo() {
            return stringTwo;
        }

        public int getIntPrimitiveTwo() {
            return intPrimitiveTwo;
        }
    }

    public static class MySupportBeanNumeric {
        private final Integer intOne;
        private final Integer intTwo;

        public MySupportBeanNumeric(Integer intOne, Integer intTwo) {
            this.intOne = intOne;
            this.intTwo = intTwo;
        }

        public Integer getIntOne() {
            return intOne;
        }

        public Integer getIntTwo() {
            return intTwo;
        }
    }

    /**
     * SupportBeanSpecialGetterNested-compatible props via delegate; the
     * TraceWriter handles it through normalize.
     */
    public static class MySupportBeanA {
        private final String id;

        public MySupportBeanA(String id) {
            this.id = id;
        }

        public String getId() {
            return id;
        }
    }

    public static class MySupportBeanB {
        private final String id;

        public MySupportBeanB(String id) {
            this.id = id;
        }

        public String getId() {
            return id;
        }
    }

    public static class MyLocalJsonProvidedMySchema {
        public String p0;
        public int p1;
    }
}
