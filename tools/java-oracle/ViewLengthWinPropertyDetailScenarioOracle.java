import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
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
import java.time.Instant;
import java.util.ArrayList;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Properties;
import java.util.TreeSet;

/**
 * Java oracle for the ViewLengthWin property-detail length-window scenario.
 *
 * Covers the ViewLengthWinWPropertyDetail execution replayed as the
 * w-property-detail scenario case, running against its own fresh runtime and
 * replaying the pinned module verbatim (only the @name('s0') annotation is a
 * sanctioned addition): a length(3) window over SupportBeanComplexProps
 * projecting mapped('keyOne') as a, indexed[1] as b,
 * nested.nestedNested.nestedNestedValue as c, mapProperty and
 * arrayProperty[0], gated by the three-property where clause on
 * mapped('keyOne'), indexed[1] and the nested nestedNestedValue. The
 * admit/filter/admit schedule follows the pinned test: the default bean
 * (mapped keyOne=valueOne, indexed [1,2], mapProperty {xOne:yOne, xTwo:yTwo},
 * arrayProperty [10,20,30]) is admitted; the resend with indexed[1] set to
 * Integer.MIN_VALUE satisfies every where term except indexed[1] = 2 and is
 * filtered out; the resend with indexed[1] back at 2 is admitted again. Two
 * listener records, no snapshots.
 *
 * Configuration follows the pinned regression schema: SupportBeanComplexProps
 * is mirrored locally as a bean {mapped java.util.Properties, indexed int[],
 * nested LocalNested, mapProperty java.util.Map, arrayProperty int[]} because
 * the pinned bean lives outside the oracle classpath; the keyed
 * getMapped(String)/getIndexed(int) overloads make Esper resolve mapped and
 * indexed as mapped/indexed properties while nested resolves as
 * nested-fragment navigation, which the pinned module's
 * nested.nestedNested.nestedNestedValue access requires (a java.util.Map
 * typed member would resolve as a mapped property and reject the dot
 * navigation). Sends build the bean directly from the payload (mapped as
 * Properties, mapProperty as a string Map, nested as
 * new LocalNested(new LocalNestedNested(value)), indexed and arrayProperty
 * as int[] arrays) via sendEventBean. The internal timer is disabled and the
 * clock is pinned with advanceTime(0) at runtime creation because the
 * scenario owns no advance-time steps, so listener record times render from
 * epoch like the Go runner's pinned virtual clock; this case deploys at the
 * default time zero.
 *
 * The single deployed step compiles buildEPL, deploys it plainly, fetches
 * the named statement and attaches the standard listener.
 *
 * Listener records follow the standard protocol: one record per delivered
 * batch containing rows, one sequence counter per case starting at 1, time
 * rendered from the current engine time, and new/old row arrays rendered
 * with the scalar normalization rules, where map values render as bare
 * JsonObject objects with TreeSet-sorted String.valueOf keys and recursively
 * normalized values. Snapshot records carry no time or sequence: step
 * {op:"snapshot", statement:"s0"} iterates the named statement and emits its
 * current window contents.
 */
public class ViewLengthWinPropertyDetailScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ViewLengthWinPropertyDetailScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        // Every cases[] entry runs independently against its own runtime,
        // matching one runtime ID per execution.
        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            String caseName = caseVal.asObject().getString("case", "");
            runCase(allSteps, caseName, records);
        }

        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
        root.add("id", scenario.getString("id", ""));
        root.add("scenario", scenario.getString("description", ""));
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
        config.getCommon().addEventType("SupportBeanComplexProps", LocalSupportBeanComplexProps.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ViewLengthWinPropertyDetailScenarioOracle-" + caseName, config);
        // The scenario owns no advance-time steps, so the clock is pinned to
        // epoch at creation (as in ViewLengthBatchScenarioOracle) to match
        // the Go runner's pinned virtual clock in listener record times.
        runtime.getEventService().advanceTime(0);
        try {
            EPStatement s0 = null;
            int[] seq = new int[] {0};

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
                if ("advance-time".equals(op)) {
                    runtime.getEventService().advanceTime(Instant.parse(step.getString("at", "")).toEpochMilli());
                    continue;
                }
                if ("deployed".equals(op)) {
                    String statementName = step.getString("statement", "s0");
                    EPCompiled compiled = EPCompilerProvider.getCompiler().compile(buildEPL(caseName), new CompilerArguments(config));
                    EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
                    s0 = runtime.getDeploymentService().getStatement(deployment.getDeploymentId(), statementName);
                    if (s0 == null) {
                        throw new IllegalStateException("statement " + statementName + " was not deployed for case " + caseName);
                    }
                    addListener(runtime, caseName, seq, records, s0);
                    continue;
                }
                if ("snapshot".equals(op)) {
                    if (s0 == null) {
                        throw new IllegalStateException("snapshot requires statement s0 for case " + caseName);
                    }
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "snapshot");
                    record.add("statement", step.getString("statement", "s0"));
                    record.add("new", rows(s0.iterator()));
                    records.add(record);
                    continue;
                }
                throw new IllegalStateException("unknown op: " + op);
            }

        } finally {
            runtime.destroy();
        }
    }

    private static void addListener(EPRuntime runtime, String caseName, int[] seq, List<JsonObject> records,
                                    EPStatement statement) {
        statement.addListener((newData, oldData, stmt, rt) -> {
            boolean hasNew = newData != null && newData.length > 0;
            boolean hasOld = oldData != null && oldData.length > 0;
            if (!hasNew && !hasOld) {
                return;
            }
            seq[0]++;
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", stmt.getName());
            record.add("sequence", seq[0]);
            record.add("time", Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());
            if (hasNew) {
                record.add("new", rows(newData));
            }
            if (hasOld) {
                record.add("old", rows(oldData));
            }
            records.add(record);
        });
    }

    /**
     * Verbatim transcription of the pinned ViewLengthWinWPropertyDetail
     * module, preserving the fragment concatenation character-for-character:
     * no space between "as a," and "indexed[1]", the trailing fragment
     * spaces accumulating into "arrayProperty[0]   from" and
     * "#length(3)  where", and the leading where-clause fragment spaces into
     * "'valueOne' and  indexed[1]" and "= 2 and  nested.".
     */
    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "w-property-detail" ->
                "@name('s0') select mapped('keyOne') as a," +
                    "indexed[1] as b, nested.nestedNested.nestedNestedValue as c, mapProperty, " +
                    "arrayProperty[0] " +
                    "  from SupportBeanComplexProps#length(3) " +
                    " where mapped('keyOne') = 'valueOne' and " +
                    " indexed[1] = 2 and " +
                    " nested.nestedNested.nestedNestedValue = 'nestedNestedValue'";
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("SupportBeanComplexProps".equals(eventType)) {
            LocalSupportBeanComplexProps event = new LocalSupportBeanComplexProps(
                toProperties(payload.get("mapped")),
                toIntArray(payload.get("indexed")),
                new LocalNested(new LocalNestedNested(
                    payload.get("nested").asObject().get("nestedNested").asObject().getString("nestedNestedValue", null))),
                toStringMap(payload.get("mapProperty")),
                toIntArray(payload.get("arrayProperty")));
            runtime.getEventService().sendEventBean(event, eventType);
            return;
        }
        throw new IllegalStateException("unknown eventType: " + eventType);
    }

    /** Builds the mapped Properties event member from a JSON object payload member. */
    private static Properties toProperties(JsonValue value) {
        Properties properties = new Properties();
        properties.putAll(toStringMap(value));
        return properties;
    }

    /** Builds a string-valued map event member from a JSON object payload member. */
    private static Map<String, String> toStringMap(JsonValue value) {
        if (!(value instanceof JsonObject)) {
            throw new IllegalStateException("expected a JSON object for string-map event member");
        }
        Map<String, String> map = new LinkedHashMap<>();
        JsonObject object = value.asObject();
        for (String name : object.names()) {
            JsonValue item = object.get(name);
            if (item instanceof JsonString) {
                map.put(name, ((JsonString) item).asString());
            } else {
                throw new IllegalStateException("unsupported string-map entry value for " + name);
            }
        }
        return map;
    }

    private static int[] toIntArray(JsonValue value) {
        if (!(value instanceof JsonArray)) {
            throw new IllegalStateException("expected a JSON array for int[] event member");
        }
        JsonArray array = value.asArray();
        int[] ints = new int[array.size()];
        for (int i = 0; i < array.size(); i++) {
            ints[i] = ((JsonNumber) array.get(i)).asInt();
        }
        return ints;
    }

    private static JsonArray rows(EventBean[] events) {
        JsonArray array = new JsonArray();
        for (EventBean event : events) {
            array.add(row(event));
        }
        return array;
    }

    private static JsonArray rows(Iterator<EventBean> iterator) {
        JsonArray array = new JsonArray();
        while (iterator.hasNext()) {
            array.add(row(iterator.next()));
        }
        return array;
    }

    private static JsonObject row(EventBean event) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        JsonObject fields = new JsonObject();
        for (String prop : new TreeSet<>(java.util.Arrays.asList(event.getEventType().getPropertyNames()))) {
            fields.add(prop, normalize(event.get(prop)));
        }
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
            // covers the mapProperty projection, which surfaces as a bare
            // JSON object rather than a scalar or a row wrapper
            Map<?, ?> mapValue = (Map<?, ?>) value;
            TreeSet<String> keys = new TreeSet<>();
            for (Object key : mapValue.keySet()) {
                keys.add(String.valueOf(key));
            }
            JsonObject object = new JsonObject();
            for (String key : keys) {
                object.add(key, normalize(mapValue.get(key)));
            }
            return object;
        }
        if (value instanceof Object[]) {
            // window-array projection values surface as arrays rather than
            // scalars
            JsonArray array = new JsonArray();
            for (Object item : (Object[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        return Json.value(String.valueOf(value));
    }

    /** Local mirror of the pinned SupportBeanComplexProps regression bean readable surface. */
    public static class LocalSupportBeanComplexProps {
        private final Properties mapped;
        private final int[] indexed;
        private final LocalNested nested;
        private final Map<String, String> mapProperty;
        private final int[] arrayProperty;

        public LocalSupportBeanComplexProps(Properties mapped, int[] indexed, LocalNested nested,
                                            Map<String, String> mapProperty, int[] arrayProperty) {
            this.mapped = mapped;
            this.indexed = indexed;
            this.nested = nested;
            this.mapProperty = mapProperty;
            this.arrayProperty = arrayProperty;
        }

        public String getMapped(String key) {
            return (String) mapped.get(key);
        }

        public int getIndexed(int index) {
            return indexed[index];
        }

        public LocalNested getNested() {
            return nested;
        }

        public Map<String, String> getMapProperty() {
            return mapProperty;
        }

        public int[] getArrayProperty() {
            return arrayProperty;
        }
    }

    /** Local mirror of the pinned SupportBeanSpecialGetterNested fragment. */
    public static class LocalNested {
        private final LocalNestedNested nestedNested;

        public LocalNested(LocalNestedNested nestedNested) {
            this.nestedNested = nestedNested;
        }

        public LocalNestedNested getNestedNested() {
            return nestedNested;
        }
    }

    /** Local mirror of the pinned SupportBeanSpecialGetterNestedNested fragment. */
    public static class LocalNestedNested {
        private final String nestedNestedValue;

        public LocalNestedNested(String nestedNestedValue) {
            this.nestedNestedValue = nestedNestedValue;
        }

        public String getNestedNestedValue() {
            return nestedNestedValue;
        }
    }
}
