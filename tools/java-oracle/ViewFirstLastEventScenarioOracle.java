import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
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
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the Draft 4.442 'view-first-last-event'
 * unit: ViewFirstEvent ordinals 0/1 (ViewFirstEventSceneOne,
 * ViewFirstEventMarketData), ViewFirstLength ordinals 0/1
 * (ViewFirstLengthSceneOne, ViewFirstLengthMarketData) and ViewLastEvent
 * ordinals 0/1 (ViewLastEventSceneOne, ViewLastEventMarketData).  Each Java
 * execution is one scenario case inside its own runtime; every statement
 * selects irstream so the listener observes both the insert and the remove
 * stream.
 *
 * The trace pins the two behaviors under test.  First, firstevent and
 * firstlength admit only the first (or first n) inserts and silently drop
 * every later insert: the listener is never invoked for a dropped event, so
 * the trace carries no record for those sends.  Second, lastevent keeps only
 * the newest event, so every insert delivers an insert/remove pair whose old
 * row is the previously retained event (null for the first insert).
 * Snapshot steps iterate the statement and emit the window contents the
 * suite's assertPropsPerRowIterator[/AnyOrder] assertions read; "any"-mode
 * steps sort rows canonically by serialized fields because the AnyOrder
 * assertions do not pin iteration order.
 *
 * SupportMarketDataBean lives in regression-lib, outside the oracle
 * classpath, so market events are map events carrying the four members the
 * pinned makeMarketDataEvent helper supplies (symbol, price, volume, feed;
 * id stays unset).  SupportBean sends use the real bean class with the two
 * members the scenario payloads carry (theString, intPrimitive).
 */
public final class ViewFirstLastEventScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "view-first-last-event";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewFirstEvent.java";
    private static final String JAVA_SOURCE2 =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewFirstLength.java";
    private static final String JAVA_SOURCE3 =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewLastEvent.java";
    private static final String DESCRIPTION =
            "ViewFirstEvent ordinals 0/1 (firstevent silent drops), ViewFirstLength ordinals 0/1 (firstlength capacity drops) and ViewLastEvent ordinals 0/1 (lastevent insert/remove pairs), all under irstream with iterator snapshots";

    private static final String[] CASES = {
            "firstevent-scene-one", "firstevent-marketdata",
            "firstlength-scene-one", "firstlength-marketdata",
            "lastevent-scene-one", "lastevent-marketdata"};
    private static final int[] ORDINALS = {0, 1, 0, 1, 0, 1};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-607d915be914a5dce34f",
            "java-runtime-5c32ea97d29ecbe957cb",
            "java-runtime-d99cd0eba0b2e2ca64ba",
            "java-runtime-2389705da83584b445a1",
            "java-runtime-260c9f5af6a4a10d5c80",
            "java-runtime-af419392d33ae4b948d8"
    };
    private static final String[] EXECUTIONS = {
            "ViewFirstEventSceneOne",
            "ViewFirstEventMarketData",
            "ViewFirstLengthSceneOne",
            "ViewFirstLengthMarketData",
            "ViewLastEventSceneOne",
            "ViewLastEventMarketData"
    };
    private static final String[] STATIC_IDS = {
            "java-9c592c8d69c0251839f3",
            "java-9c592c8d69c0251839f3",
            "java-b74c5027c5e9385c91fc",
            "java-b74c5027c5e9385c91fc",
            "java-1c8907d6332b94606e35",
            "java-1c8907d6332b94606e35"
    };
    private static final String[] OBSERVATIONS = {
            "listener+snapshot",
            "listener+snapshot",
            "listener+snapshot",
            "listener+snapshot",
            "listener+snapshot",
            "listener+snapshot"
    };

    // ViewFirstEventSceneOne EPL, byte-exact from ViewFirstEvent (capital-N
    // @Name, single space after "from").
    private static final String FIRSTEVENT_SCENE_ONE_EPL =
            "@Name('s0') select irstream theString as c0, intPrimitive as c1 from SupportBean#firstevent()";

    // ViewFirstEventMarketData EPL, byte-exact from ViewFirstEvent (lowercase
    // @name, single space after "from").
    private static final String FIRSTEVENT_MARKETDATA_EPL =
            "@name('s0') select irstream * from SupportMarketDataBean#firstevent()";

    // ViewFirstLengthSceneOne EPL, byte-exact from ViewFirstLength.
    private static final String FIRSTLENGTH_SCENE_ONE_EPL =
            "@Name('s0') select irstream theString as c0 from SupportBean#firstlength(2)";

    // ViewFirstLengthMarketData EPL, byte-exact from ViewFirstLength
    // including the double space after "from".
    private static final String FIRSTLENGTH_MARKETDATA_EPL =
            "@name('s0') select irstream * from  SupportMarketDataBean#firstlength(3)";

    // ViewLastEventSceneOne EPL, byte-exact from ViewLastEvent.
    private static final String LASTEVENT_SCENE_ONE_EPL =
            "@Name('s0') select irstream theString as c0, intPrimitive as c1 from SupportBean#lastevent()";

    // ViewLastEventMarketData EPL, byte-exact from ViewLastEvent including
    // the double space after "from".
    private static final String LASTEVENT_MARKETDATA_EPL =
            "@name('s0') select irstream * from  SupportMarketDataBean#lastevent()";

    private ViewFirstLastEventScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ViewFirstLastEventScenarioOracle <scenario.json>");
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
        Map<String, Object> marketType = new HashMap<>();
        marketType.put("symbol", String.class);
        marketType.put("price", double.class);
        marketType.put("volume", long.class);
        marketType.put("feed", String.class);
        configuration.getCommon().addEventType("SupportMarketDataBean", marketType);

        String runtimeURI = "parity-" + ID + "-" + runtimeId;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            EPStatement s0 = null;
            boolean active = false;
            int deployIndex = 0;
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
                    String statement = step.getString("statement", "");
                    if (!"s0".equals(statement)) {
                        throw new IllegalArgumentException("unexpected deploy statement " + statement);
                    }
                    if (deployIndex != 0) {
                        throw new IllegalArgumentException("unexpected deploy index " + deployIndex);
                    }
                    deployIndex++;
                    String epl = s0Epl(caseName);
                    EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                            new CompilerArguments(configuration));
                    EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                            new DeploymentOptions().setDeploymentId(ID + "-" + caseIndex + "-" + deployIndex));
                    s0 = findStatement(deployment);
                    if (s0 == null) {
                        throw new IllegalStateException("deployment has no statement named s0");
                    }
                    s0.addListener(writer);
                } else if ("undeploy-all".equals(operation)) {
                    runtime.getDeploymentService().undeployAll();
                    s0 = null;
                } else if ("send".equals(operation)) {
                    sendEvent(runtime, step);
                } else if ("snapshot".equals(operation)) {
                    String statement = step.getString("statement", "");
                    if (s0 == null || !statement.equals(s0.getName())) {
                        throw new IllegalArgumentException("snapshot statement " + statement
                                + " is not deployed in case " + caseName);
                    }
                    String mode = step.getString("mode", "");
                    if (!"ordered".equals(mode) && !"any".equals(mode)) {
                        throw new IllegalArgumentException("unsupported snapshot mode " + mode);
                    }
                    records.add(snapshot(runtime, s0, caseName, mode));
                } else {
                    throw new IllegalArgumentException("unsupported step op " + operation);
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static String s0Epl(String caseName) {
        switch (caseName) {
            case "firstevent-scene-one":
                return FIRSTEVENT_SCENE_ONE_EPL;
            case "firstevent-marketdata":
                return FIRSTEVENT_MARKETDATA_EPL;
            case "firstlength-scene-one":
                return FIRSTLENGTH_SCENE_ONE_EPL;
            case "firstlength-marketdata":
                return FIRSTLENGTH_MARKETDATA_EPL;
            case "lastevent-scene-one":
                return LASTEVENT_SCENE_ONE_EPL;
            case "lastevent-marketdata":
                return LASTEVENT_MARKETDATA_EPL;
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
        } else if ("SupportMarketDataBean".equals(eventType)) {
            Map<String, Object> event = new HashMap<>();
            event.put("symbol", payload.getString("symbol", null));
            JsonValue price = payload.get("price");
            event.put("price", price instanceof JsonNumber ? ((JsonNumber) price).asDouble() : 0.0d);
            JsonValue volume = payload.get("volume");
            event.put("volume", volume instanceof JsonNumber ? ((JsonNumber) volume).asLong() : 0L);
            event.put("feed", payload.getString("feed", null));
            runtime.getEventService().sendEventMap(event, "SupportMarketDataBean");
        } else {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
    }

    /**
     * Snapshot of the statement iterator: one record holding either the
     * engine iterator order (ordered steps, the order the suite's exact-order
     * iterator assertions pin) or, for any-mode steps, the rows sorted
     * canonically by serialized fields (the suite's AnyOrder assertions).
     * The record always carries the new array, empty when the window is
     * empty, matching the Go runner's explicit empty-slice rendering.
     */
    private static JsonObject snapshot(EPRuntime runtime, EPStatement statement,
                                       String caseName, String mode) {
        List<JsonObject> fieldRows = new ArrayList<>();
        for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
            fieldRows.add(fields(iterator.next()));
        }
        if ("any".equals(mode)) {
            fieldRows.sort(Comparator.comparing(JsonObject::toString));
        }
        JsonArray rows = new JsonArray();
        for (JsonObject fieldRow : fieldRows) {
            rows.add(new JsonObject().add("kind", "row").add("fields", fieldRow));
        }
        return new JsonObject()
                .add("case", caseName)
                .add("operation", "snapshot")
                .add("statement", statement.getName())
                .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
                .add("new", rows);
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaSource2", "javaSource3", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags",
                "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))
                || !JAVA_SOURCE2.equals(string(scenario, "javaSource2"))
                || !JAVA_SOURCE3.equals(string(scenario, "javaSource3"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTIONS, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly six cases");
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
                    || !s0Epl(CASES[index]).equals(string(definition, "epl"))) {
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

    private static JsonObject fields(EventBean event) {
        JsonObject fields = new JsonObject();
        String[] names = event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        for (String name : names) {
            fields.add(name, normalize(event.get(name)));
        }
        return fields;
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value instanceof EventBean[] events) {
            JsonArray array = new JsonArray();
            for (EventBean event : events) {
                array.add(new JsonObject().add("kind", "row").add("fields", fields(event)));
            }
            return array;
        }
        if (value instanceof EventBean event) {
            return new JsonObject().add("kind", "row").add("fields", fields(event));
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

        private long nextSequence() {
            return ++sequence;
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
                    .add("sequence", nextSequence())
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
                output.add(new JsonObject().add("kind", "row").add("fields", fields(event)));
            }
            return output;
        }
    }
}
