import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.time.Instant;
import java.util.ArrayList;
import java.util.Collections;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Oracle for the EPLInsertIntoTransposePattern executions:
 * ThisAsColumn, TransposePOJOEventPattern and TransposeMapEventPattern.
 *
 * Statement EPL mirrors the Java suite text verbatim (create window with a
 * `this` column, pattern -> insert into with tagged full-event columns, and
 * the Map-event pattern form). The two window executions are observed through
 * iterator snapshots (operation "snapshot") after each send, matching the
 * Java suite's assertPropsPerRowIteratorAnyOrder; the pattern executions are
 * observed through listeners (operation "listener") exactly as the Go parity
 * test subscribes them. Internal timer disabled so trace timestamps are epoch
 * zero; the #time(1 day) windows behave as keepall for the short trace.
 *
 * Target beans use the real SupportBeanWithThis / SupportBean-A/B classes from
 * the fixed esper-common classpath where available; the MySupportBeanA/B
 * mirrors keep the classpath self-contained like the transpose-stream oracle.
 */
public final class EPLInsertIntoTransposePatternScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private EPLInsertIntoTransposePatternScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: EPLInsertIntoTransposePatternScenarioOracle <scenario.json>");
        }
        JsonArray steps = Json.parse(new String(java.nio.file.Files.readAllBytes(java.nio.file.Path.of(args[0]))))
            .asObject().get("steps").asArray();
        String[] cases = {"this-as-column", "pattern-pojo", "pattern-map"};
        JsonArray records = new JsonArray();
        for (String caseName : cases) {
            if (hasCase(steps, caseName)) {
                runCase(steps, caseName, records);
            }
        }
        JsonObject trace = new JsonObject()
            .add("version", VERSION)
            .add("id", "epl-insert-into-transpose-pattern")
            .add("records", records);
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
        configuration.getCommon().addEventType("SupportBeanWithThis", SupportBeanWithThisMirror.class);

        switch (caseName) {
            case "pattern-pojo" -> {
                configuration.getCommon().addEventType("SupportBean_A", MySupportBeanA.class);
                configuration.getCommon().addEventType("SupportBean_B", MySupportBeanB.class);
            }
            case "pattern-map" -> {
                Map<String, Object> metadata = new LinkedHashMap<>();
                metadata.put("id", String.class);
                configuration.getCommon().addEventType("AEventMap", metadata);
                configuration.getCommon().addEventType("BEventMap", metadata);
            }
            default -> { }
        }

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-txp-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            Driver driver = new Driver(configuration, runtime, allSteps, caseName, records);
            switch (caseName) {
                case "this-as-column" -> driver.thisAsColumn();
                case "pattern-pojo" -> driver.patternPojo();
                case "pattern-map" -> driver.patternMap();
                default -> throw new IllegalArgumentException("unsupported case " + caseName);
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * Per-case driver. EPL mirrors the Java suite text; snapshot operations
     * capture the windows' iterator state while listener operations capture
     * routed/consumer events.
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
                new DeploymentOptions().setDeploymentId("parity-txp-" + writer.caseName + "-" + (deployCount++)));
        }

        private EPStatement deployStatement(String epl) throws Exception {
            EPDeployment deployment = deploy(epl);
            if (deployment.getStatements().length == 0) {
                throw new IllegalStateException("deployment has no statement");
            }
            return deployment.getStatements()[0];
        }

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

        private void thisAsColumn() throws Exception {
            // Exec 0: OneWindow with alertId + this columns.
            deploy("@name('window') @public create window OneWindow#time(1 day) as select theString as alertId, this from SupportBeanWithThis");
            EPStatement producerA = deployStatement("@name('producer-A') insert into OneWindow " +
                "select '1' as alertId, stream0.quote.this as this " +
                "from pattern [every quote=SupportBeanWithThis(theString='A')] as stream0");
            EPStatement producerB = deployStatement("@name('producer-B') insert into OneWindow " +
                "select '2' as alertId, stream0.quote as this " +
                "from pattern [every quote=SupportBeanWithThis(theString='B')] as stream0");
            EPStatement oneWindowView = deployStatement("@name('window-view') select * from OneWindow");

            // TwoWindow with alertId + wildcard expansion.
            deploy("@name('window-2') @public create window TwoWindow#time(1 day) as select theString as alertId, * from SupportBeanWithThis");
            EPStatement producerC = deployStatement("@name('producer-C') insert into TwoWindow " +
                "select '3' as alertId, quote.* " +
                "from pattern [every quote=SupportBeanWithThis(theString='C')] as stream0");
            EPStatement twoWindowView = deployStatement("@name('window-2-view') select * from TwoWindow");

            sendStep(0); // A -> OneWindow {1, this=a(10)}
            writer.snapshot(oneWindowView, "snapshot");
            sendStep(1); // B -> OneWindow {1,a(10)},{2,b(20)}
            writer.snapshot(oneWindowView, "snapshot");
            sendStep(2); // C -> TwoWindow {3,30}
            writer.snapshot(twoWindowView, "snapshot");
        }

        private void patternPojo() throws Exception {
            deploy("@public insert into MyStreamABBean select a, b from pattern [a=SupportBean_A -> b=SupportBean_B]");
            EPStatement s0 = deployStatement("@name('s0') select a.id, b.id from MyStreamABBean");
            writer.observe(s0);
            sendStep(0); // A1 alone: pattern not complete, no output.
            sendStep(1); // B1 completes pattern -> a.id=A1, b.id=B1.
        }

        private void patternMap() throws Exception {
            EPStatement i1 = deployObserved("@name('i1') @public insert into MyStreamABMap " +
                "select a, b from pattern [a=AEventMap -> b=BEventMap]", "i1");
            EPStatement s0 = deployStatement("@name('s0') select a.id, b.id from MyStreamABMap");
            writer.observe(s0);
            sendStep(0); // AEventMap alone: pattern not complete.
            sendStep(1); // BEventMap completes -> i1 observes a,b maps; s0 observes a.id/b.id.
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
            case "SupportBeanWithThis" -> {
                SupportBeanWithThisMirror bean = new SupportBeanWithThisMirror(
                    payload.getString("theString", null),
                    payload.getInt("intPrimitive", 0));
                runtime.getEventService().sendEventBean(bean, eventType);
            }
            case "SupportBean_A" -> {
                MySupportBeanA bean = new MySupportBeanA(payload.getString("id", null));
                runtime.getEventService().sendEventBean(bean, eventType);
            }
            case "SupportBean_B" -> {
                MySupportBeanB bean = new MySupportBeanB(payload.getString("id", null));
                runtime.getEventService().sendEventBean(bean, eventType);
            }
            case "AEventMap", "BEventMap" -> {
                Map<String, Object> event = new LinkedHashMap<>();
                event.put("id", payload.getString("id", null));
                runtime.getEventService().sendEventMap(event, eventType);
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

        private void snapshot(EPStatement statement, String operation) {
            List<EventBean> events = new ArrayList<>();
            Iterator<EventBean> iterator = statement.iterator();
            while (iterator.hasNext()) {
                events.add(iterator.next());
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", operation)
                    .add("statement", statement.getName())
                    .add("sequence", 0)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = results(events.toArray(new EventBean[0]));
            if (newArray.size() > 0) {
                record.add("new", newArray);
            }
            records.add(record);
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
            if (value instanceof SupportBeanWithThisMirror bean) {
                JsonObject object = new JsonObject();
                object.add("__type", "SupportBeanWithThis");
                object.add("intPrimitive", normalize(bean.getIntPrimitive()));
                object.add("theString", normalize(bean.getTheString()));
                return object;
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
                Collections.sort(keys);
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

    /** SupportBeanWithThis mirror with a self-referencing getThis(). */
    public static class SupportBeanWithThisMirror {
        private final String theString;
        private final int intPrimitive;

        public SupportBeanWithThisMirror(String theString, int intPrimitive) {
            this.theString = theString;
            this.intPrimitive = intPrimitive;
        }

        public SupportBeanWithThisMirror getThis() {
            return this;
        }

        public String getTheString() {
            return theString;
        }

        public int getIntPrimitive() {
            return intPrimitive;
        }
    }

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
}
