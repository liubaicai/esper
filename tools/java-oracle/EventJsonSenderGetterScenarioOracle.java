import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventPropertyGetter;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.util.EventSenderJson;
import com.espertech.esper.common.client.json.util.JsonEventObject;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerContext;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Collection;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for the EventJson sender/getter parity scenario.
 *
 * Covers the two registered executions of the event/json sender-getter
 * subdomain. Sender-parse-and-send replays EventJsonEventSenderParseAndSend:
 * one compileDeploy carries the byte-exact pair
 * "@public @buseventtype @JsonSchema create json schema MyEvent(p1 string);\n"
 * plus "@name('s0') select * from MyEvent;\n"; the runtime EventSenderJson for
 * MyEvent is looked up, parses the exact byte payload {"p1": "abc"} (one ASCII
 * space after the colon - the parse input is contractual and never
 * re-serialized), and sends the parsed JsonEventObject underlying, yielding a
 * single s0 listener row {p1:"abc"}. Getter-map-type replays
 * EventJsonGetterMapType: the byte-exact pair
 * "@public @buseventtype create json schema JsonEvent(prop java.util.Map);\n"
 * plus "@name('s0') select * from JsonEvent" (no trailing newline) deploys,
 * the send rebuilds the minimaljson tree new
 * JsonObject().add("prop", new JsonObject().add("x", "y")) structurally from
 * the scenario payload so minimaljson itself emits the compact bytes
 * {"prop":{"x":"y"}} (no spaces) through runtime sendEventJson, and the
 * listener row projects the declared java.util.Map property with the nested
 * kind/row convention: {"kind":"row","fields":{"x":"y"}} with sorted keys.
 * After the send the in-process getter surface is pinned like
 * env.assertEventNew: the mapped getter prop('x') must be advertised and
 * return "y", while the dynamic path prop.somefield? must not be advertised
 * (getter null); that divergence is documented in the manifest capability
 * remaining.
 *
 * Records follow the standard protocol: automatic s0 listener records with
 * per-case sequences starting at one, time frozen at epoch zero because the
 * internal timer is disabled and time is advanced to zero only, per-case
 * fresh runtimes, and undeployAll at the end of each case.
 */
public class EventJsonSenderGetterScenarioOracle {
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EventJsonSenderGetterScenarioOracle <scenario.json>");
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
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        JsonArray recordsArr = new JsonArray();
        for (JsonObject record : records) {
            recordsArr.add(record);
        }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }

    private static void runCase(JsonArray allSteps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = configure();
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EventJsonSenderGetterScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            ListenerRecorder listener = new ListenerRecorder(caseName, runtime, records);
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
                switch (op) {
                    case "deploy" -> deploy(config, runtime, caseName, step.getString("statement", ""), listener);
                    case "send-json" -> sendJson(runtime, caseName, step, listener);
                    default -> throw new IllegalStateException("unsupported op " + op);
                }
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * Mirrors the regression runner defaults relevant here: no preconfigured
     * types (both schemas are created by the deployed EPL), internal timer
     * off, and statement exceptions rethrow on the sending thread instead of
     * being absorbed by the default handler.
     */
    private static Configuration configure() {
        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        config.getRuntime().getExceptionHandling().addClass(RethrowExceptionHandlerFactory.class);
        return config;
    }

    /** Case-name to byte-exact pinned EPL; every case deploys at most s0. */
    private static String eplFor(String caseName, String statementName) {
        if (!"s0".equals(statementName)) {
            throw new IllegalStateException("unknown statement " + statementName + " in case " + caseName);
        }
        return switch (caseName) {
            case "json-sender-parse-and-send" ->
                "@public @buseventtype @JsonSchema create json schema MyEvent(p1 string);\n" +
                    "@name('s0') select * from MyEvent;\n";
            case "json-getter-map-type" ->
                "@public @buseventtype create json schema JsonEvent(prop java.util.Map);\n" +
                    "@name('s0') select * from JsonEvent";
            default -> throw new IllegalStateException("case " + caseName + " deploys no statements");
        };
    }

    private static void deploy(Configuration config, EPRuntime runtime, String caseName,
                               String statementName, UpdateListener listener) throws Exception {
        EPCompiled compiled = EPCompilerProvider.getCompiler()
            .compile(eplFor(caseName, statementName), new CompilerArguments(config));
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
        for (EPStatement added : deployment.getStatements()) {
            if ("s0".equals(added.getName())) {
                added.addListener(listener);
            }
        }
    }

    /**
     * Dispatches the two JSON send flows. "parse-send" mirrors
     * EventJsonEventSenderParseAndSend: the runtime sender lookup, parse of
     * the exact payload bytes, then sendEvent of the parsed underlying.
     * "send-event-json" mirrors EventJsonGetterMapType: the scenario payload
     * is rebuilt as a minimaljson tree whose toString emits the compact bytes,
     * then handed to runtime sendEventJson; the getter surface is asserted
     * afterwards like env.assertEventNew.
     */
    private static void sendJson(EPRuntime runtime, String caseName, JsonObject step,
                                 ListenerRecorder listener) {
        String eventType = step.getString("eventType", "");
        String mode = step.getString("mode", "");
        switch (mode) {
            case "parse-send" -> {
                EventSenderJson sender = (EventSenderJson) runtime.getEventService().getEventSender(eventType);
                JsonEventObject underlying = (JsonEventObject) sender.parse(step.getString("payloadText", ""));
                sender.sendEvent(underlying);
            }
            case "send-event-json" -> {
                JsonObject payload = step.get("payload").asObject();
                runtime.getEventService().sendEventJson(rebuildMinimalJson(payload).toString(), eventType);
                if ("json-getter-map-type".equals(caseName)) {
                    assertGetterSurface(listener);
                }
            }
            default -> throw new IllegalStateException("unknown send-json mode " + mode);
        }
    }

    /**
     * Rebuilds the scenario payload as a minimaljson JsonObject tree so the
     * compact serialization bytes are produced by minimaljson itself, exactly
     * like new JsonObject().add("prop", new JsonObject().add("x", "y")).
     */
    private static JsonObject rebuildMinimalJson(JsonObject payload) {
        JsonObject out = new JsonObject();
        for (String name : payload.names()) {
            JsonValue value = payload.get(name);
            if (value instanceof JsonObject nested) {
                out.add(name, rebuildMinimalJson(nested));
            } else {
                out.add(name, value);
            }
        }
        return out;
    }

    /**
     * Mirrors env.assertEventNew for EventJsonGetterMapType: exactly one new
     * s0 event, the mapped getter prop('x') is advertised and returns "y",
     * and the dynamic path prop.somefield? is not advertised (null getter).
     */
    private static void assertGetterSurface(ListenerRecorder listener) {
        EventBean[] newEvents = listener.lastNewEvents();
        check(newEvents != null && newEvents.length == 1,
            "expected exactly one new s0 event, got " + (newEvents == null ? "none" : newEvents.length));
        EventBean event = newEvents[0];
        EventPropertyGetter getterMapped = event.getEventType().getGetter("prop('x')");
        check(getterMapped != null, "getter prop('x') must be advertised for the declared map");
        check("y".equals(getterMapped.get(event)), "prop('x') must return y");
        check(event.getEventType().getGetter("prop.somefield?") == null,
            "getter prop.somefield? must not be advertised");
    }

    private static void check(boolean condition, String message) {
        if (!condition) {
            throw new IllegalStateException(message);
        }
    }

    private static JsonObject renderRow(EventBean event) {
        JsonObject fields = new JsonObject();
        for (String prop : new TreeSet<>(java.util.Arrays.asList(event.getEventType().getPropertyNames()))) {
            fields.add(prop, normalize(event.get(prop)));
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
        if (value instanceof Collection<?>) {
            JsonArray items = new JsonArray();
            for (Object element : (Collection<?>) value) {
                items.add(normalize(element));
            }
            return items;
        }
        return Json.value(String.valueOf(value));
    }

    /**
     * Automatic s0 listener mirroring env.addListener("s0"): every update
     * emits one listener record whose sequence increments per case. This
     * scenario has no remove-streams, so only the new stream is recorded.
     */
    private static final class ListenerRecorder implements UpdateListener {
        private final String caseName;
        private final EPRuntime runtime;
        private final List<JsonObject> records;
        private long sequence;
        private EventBean[] lastNewEvents;

        private ListenerRecorder(String caseName, EPRuntime runtime, List<JsonObject> records) {
            this.caseName = caseName;
            this.runtime = runtime;
            this.records = records;
        }

        private EventBean[] lastNewEvents() {
            return lastNewEvents;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored, EPRuntime ignoredRuntime) {
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", "s0");
            record.add("sequence", ++sequence);
            record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = new JsonArray();
            if (newEvents != null) {
                for (EventBean event : newEvents) {
                    newArray.add(renderRow(event));
                }
            }
            record.add("new", newArray);
            records.add(record);
            lastNewEvents = newEvents;
        }
    }

    /**
     * Mirrors the pinned runner's SupportExceptionHandlerFactoryRethrow:
     * statement exceptions rethrow on the sending thread instead of being
     * absorbed by the default handler.
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
