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
import java.util.Comparator;
import java.util.HashMap;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for ViewParameterizedByContext context-parameterized view
 * scenarios.
 *
 * Covers all three ViewParameterizedByContext executions replayed as three
 * scenario cases, each case running against its own fresh
 * runtime and replaying its pinned module verbatim: length-window replays
 * ViewParameterizedByContextLengthWindow and doc-sample replays
 * ViewParameterizedByContextDocSample, the documentation form of the same
 * statement (transcribed with its trailing semicolon and newline). Both
 * create the context CtxInitToTerm initiated by
 * SupportContextInitEventWLength as miewl terminated after 1 year and deploy
 * s0 selecting context.miewl.id and count(*) over
 * SupportBean(theString=context.miewl.id)#length(context.miewl.intSize), so
 * the length-window size is parameterized by the intSize of the event that
 * initiated each context partition and the count(*) row per partition only
 * admits value events whose theString matches the partition id. The frozen
 * schedule initiates partitions P1=2, P2=4, P3=3 and drives value events
 * through them: after the first value event the per-partition counts are
 * P1=0, P2=1, P3=0, and after ten full rounds the counts cap at the
 * partition sizes P1=2, P2=4, P3=3, staying there after one more round.
 * Multi-row snapshots carry "mode":"any" and are sorted by the canonical
 * JSON text of each row's sorted-fields object so they compare
 * order-insensitively across engines.
 *
 * more-windows replays ViewParameterizedByContextMoreWindows as twelve
 * sequential per-kind cycles over the same pinned two-statement module shape
 * (create context ... terminated after 1 year;\ncontext CtxInitToTerm select
 * * from SupportBean#<window>), one per window-parameterized kind:
 * length_batch, time, ext_timed, time_batch, ext_timed_batch,
 * time_length_batch, time_accum, firstlength, firsttime, sort, rank and
 * time_order, each sized by context.miewl.intSize. Each cycle compiles its
 * kind's module fresh (no cross-cycle caching, mirroring the pinned
 * re-compilation), deploys it, records one "deployed" marker
 * {case, operation:"deployed", statement} - the pinned observable that the
 * kind compiled, deployed and its context partitions initialized from the
 * following P1(2) and P2(20) init sends; the pinned execution attaches no
 * listener - and then undeploys. Undeployment destroys the same-named
 * context with the module, so the next cycle's context re-creation is
 * legal.
 *
 * Configuration follows the pinned regression schema: SupportBean is
 * mirrored locally as {theString string, intPrimitive int, doubleBoxed
 * Double} and the context initiating event is mirrored locally as
 * {id string, intSize int} with its (String, int) constructor, because the
 * pinned beans live outside the oracle classpath. The internal timer is
 * disabled and the clock is pinned with advanceTime(0) at runtime creation
 * because the scenario owns no advance-time steps, so listener record times
 * render from epoch like the Go runner's pinned virtual clock.
 *
 * Deployment happens once per case, at the pinned point: the frozen steps
 * contain no explicit deployed op, so the oracle deploys lazily right before
 * the first step that needs the statement - before the context-init sends,
 * exactly the pinned compileDeploy order. An explicit {op:"deployed"} step
 * would deploy at its own position instead. buildEPL puts the
 * create-context statement first, so deploying the module creates the
 * context before the s0 statement starts consuming.
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
public class ViewParameterizedByContextScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ViewParameterizedByContextScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        // Every cases[] entry runs independently against its own runtime,
        // matching one runtime ID per execution. more-windows is a special
        // case: twelve per-kind deploy/init/undeploy cycles with no listener.
        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            String caseName = caseVal.asObject().getString("case", "");
            if ("more-windows".equals(caseName)) {
                runMoreWindowsCase(allSteps, caseName, records);
            } else {
                runCase(allSteps, caseName, records);
            }
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
        config.getCommon().addEventType("SupportBean", LocalSupportBean.class);
        config.getCommon().addEventType("SupportContextInitEventWLength", LocalSupportContextInitEventWLength.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ViewParameterizedByContextScenarioOracle-" + caseName, config);
        // The scenario owns no advance-time steps, so the clock is pinned to
        // epoch at creation to match the Go runner's pinned virtual clock in
        // listener record times.
        runtime.getEventService().advanceTime(0);
        try {
            CaseDeployment deployment = new CaseDeployment();
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
                    ensureDeployed(runtime, config, caseName, "s0", deployment, seq, records);
                    sendEvent(runtime, step);
                    continue;
                }
                if ("advance-time".equals(op)) {
                    runtime.getEventService().advanceTime(Instant.parse(step.getString("at", "")).toEpochMilli());
                    continue;
                }
                if ("deployed".equals(op)) {
                    if (deployment.deployed) {
                        throw new IllegalStateException("unexpected deployed op for case " + caseName);
                    }
                    ensureDeployed(runtime, config, caseName, step.getString("statement", "s0"), deployment, seq, records);
                    continue;
                }
                if ("snapshot".equals(op)) {
                    ensureDeployed(runtime, config, caseName, step.getString("statement", "s0"), deployment, seq, records);
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "snapshot");
                    record.add("statement", step.getString("statement", "s0"));
                    JsonArray newRows = rows(deployment.statement.iterator());
                    if ("any".equals(step.getString("mode", ""))) {
                        newRows = sortRowsCanonical(newRows);
                    }
                    record.add("new", newRows);
                    records.add(record);
                    continue;
                }
                throw new IllegalStateException("unknown op: " + op);
            }

        } finally {
            runtime.destroy();
        }
    }

    /**
     * Special path for the more-windows case: twelve sequential cycles of
     * {deployed → init P1(2), P2(20) → undeploy}, one per window kind. Each
     * deployed step compiles the next kind's module fresh (the previous
     * module was undeployed, so the same-named context re-creation is
     * legal), deploys it, and emits one deployed marker record; the pinned
     * execution attaches no listener. A deployed step after all twelve
     * kinds are consumed throws.
     */
    private static void runMoreWindowsCase(JsonArray allSteps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = new Configuration();
        config.getCommon().addEventType("SupportBean", LocalSupportBean.class);
        config.getCommon().addEventType("SupportContextInitEventWLength", LocalSupportContextInitEventWLength.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ViewParameterizedByContextScenarioOracle-" + caseName, config);
        // The scenario owns no advance-time steps, so the clock is pinned to
        // epoch at creation to match the Go runner's pinned virtual clock in
        // record times.
        runtime.getEventService().advanceTime(0);
        try {
            int[] cycleIndex = new int[] {0};
            String[] activeDeploymentId = new String[] {null};

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
                if ("deployed".equals(op)) {
                    if (cycleIndex[0] >= MORE_WINDOWS_WINDOWS.length) {
                        throw new IllegalStateException("unexpected deployed op for case " + caseName +
                            ": all " + MORE_WINDOWS_WINDOWS.length + " window kinds are consumed");
                    }
                    EPCompiled compiled = EPCompilerProvider.getCompiler().compile(moreWindowsEPL(cycleIndex[0]), new CompilerArguments(config));
                    EPDeployment deployed = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
                    activeDeploymentId[0] = deployed.getDeploymentId();
                    cycleIndex[0]++;
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "deployed");
                    record.add("statement", step.getString("statement", "s0"));
                    records.add(record);
                    continue;
                }
                if ("send".equals(op)) {
                    sendEvent(runtime, step);
                    continue;
                }
                if ("undeploy".equals(op)) {
                    if (activeDeploymentId[0] == null) {
                        throw new IllegalStateException("undeploy without a deployment for case " + caseName);
                    }
                    runtime.getDeploymentService().undeploy(activeDeploymentId[0]);
                    activeDeploymentId[0] = null;
                    continue;
                }
                if ("advance-time".equals(op)) {
                    runtime.getEventService().advanceTime(Instant.parse(step.getString("at", "")).toEpochMilli());
                    continue;
                }
                throw new IllegalStateException("unknown op: " + op);
            }

        } finally {
            runtime.destroy();
        }
    }

    /** Per-case deployment state: the s0 statement plus a deployed flag. */
    private static final class CaseDeployment {
        private EPStatement statement;
        private boolean deployed;
    }

    /**
     * Deploys the case module once. buildEPL puts the create-context
     * statement first, so the deployment creates CtxInitToTerm before the
     * s0 statement starts consuming.
     */
    private static void ensureDeployed(EPRuntime runtime, Configuration config, String caseName,
                                       String statementName, CaseDeployment deployment, int[] seq,
                                       List<JsonObject> records) throws Exception {
        if (deployment.deployed) {
            return;
        }
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(buildEPL(caseName), new CompilerArguments(config));
        EPDeployment deployed = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
        deployment.statement = runtime.getDeploymentService().getStatement(deployed.getDeploymentId(), statementName);
        if (deployment.statement == null) {
            throw new IllegalStateException("statement " + statementName + " was not deployed for case " + caseName);
        }
        addListener(runtime, caseName, seq, records, deployment.statement);
        deployment.deployed = true;
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
     * Sorts snapshot rows by the canonical key - the JSON text of each row's
     * sorted-fields object - so multi-row snapshots compare
     * order-insensitively across engines ("mode":"any").
     */
    private static JsonArray sortRowsCanonical(JsonArray rowArray) {
        List<JsonObject> items = new ArrayList<>();
        for (JsonValue item : rowArray) {
            items.add(item.asObject());
        }
        items.sort(Comparator.comparing(item -> item.get("fields").toString()));
        JsonArray sorted = new JsonArray();
        for (JsonObject item : items) {
            sorted.add(item);
        }
        return sorted;
    }

    /**
     * The twelve window-parameterized kinds of the pinned
     * ViewParameterizedByContextMoreWindows execution, in pinned call order.
     */
    private static final String[] MORE_WINDOWS_WINDOWS = {
        "length_batch(context.miewl.intSize)",
        "time(context.miewl.intSize)",
        "ext_timed(longPrimitive, context.miewl.intSize)",
        "time_batch(context.miewl.intSize)",
        "ext_timed_batch(longPrimitive, context.miewl.intSize)",
        "time_length_batch(context.miewl.intSize, context.miewl.intSize)",
        "time_accum(context.miewl.intSize)",
        "firstlength(context.miewl.intSize)",
        "firsttime(context.miewl.intSize)",
        "sort(context.miewl.intSize, intPrimitive)",
        "rank(theString, context.miewl.intSize, theString)",
        "time_order(longPrimitive, context.miewl.intSize)",
    };

    /**
     * Verbatim transcription of the pinned runAssertionWindow module shape:
     * the same-named context plus an (unnamed, like the pinned module)
     * context-parameterized select over the given window kind.
     */
    private static String moreWindowsEPL(int cycleIndex) {
        return "create context CtxInitToTerm initiated by SupportContextInitEventWLength as miewl terminated after 1 year;\n" +
            "context CtxInitToTerm select * from SupportBean#" + MORE_WINDOWS_WINDOWS[cycleIndex];
    }

    /** Verbatim transcriptions of the pinned ViewParameterizedByContext modules. */
    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "length-window" ->
                "create context CtxInitToTerm initiated by SupportContextInitEventWLength as miewl terminated after 1 year;\n" +
                    "@name('s0') context CtxInitToTerm select context.miewl.id as id, count(*) as cnt " +
                    "from SupportBean(theString=context.miewl.id)#length(context.miewl.intSize)";
            case "doc-sample" ->
                "create context CtxInitToTerm initiated by SupportContextInitEventWLength as miewl terminated after 1 year;\n" +
                    "@name('s0') context CtxInitToTerm select context.miewl.id as id, count(*) as cnt " +
                    "from SupportBean(theString=context.miewl.id)#length(context.miewl.intSize);\n";
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("SupportContextInitEventWLength".equals(eventType)) {
            JsonValue intSizeVal = payload.get("intSize");
            LocalSupportContextInitEventWLength event = new LocalSupportContextInitEventWLength(
                payload.getString("id", null),
                intSizeVal instanceof JsonNumber ? ((JsonNumber) intSizeVal).asInt() : 0);
            runtime.getEventService().sendEventBean(event, eventType);
            return;
        }
        if ("SupportBean".equals(eventType)) {
            LocalSupportBean event = new LocalSupportBean();
            JsonValue theStringVal = payload.get("theString");
            if (theStringVal instanceof JsonString) {
                event.setTheString(((JsonString) theStringVal).asString());
            }
            JsonValue intPrimitiveVal = payload.get("intPrimitive");
            if (intPrimitiveVal instanceof JsonNumber) {
                event.setIntPrimitive(((JsonNumber) intPrimitiveVal).asInt());
            }
            JsonValue doubleBoxedVal = payload.get("doubleBoxed");
            if (doubleBoxedVal instanceof JsonNumber) {
                event.setDoubleBoxed(((JsonNumber) doubleBoxedVal).asDouble());
            }
            runtime.getEventService().sendEventBean(event, eventType);
            return;
        }
        throw new IllegalStateException("unknown eventType: " + eventType);
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
            // covers map projections, which surface as bare JSON objects
            // rather than scalars
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

    /**
     * Local mirror of the pinned SupportBean regression bean members in use.
     * longPrimitive is required by the more-windows ext_timed /
     * ext_timed_batch / time_order kinds; bare sends never set it, and the
     * long primitive defaults to 0 like the pinned bean.
     */
    public static class LocalSupportBean {
        private String theString;
        private int intPrimitive;
        private long longPrimitive;
        private Double doubleBoxed;

        public String getTheString() {
            return theString;
        }

        public void setTheString(String theString) {
            this.theString = theString;
        }

        public int getIntPrimitive() {
            return intPrimitive;
        }

        public void setIntPrimitive(int intPrimitive) {
            this.intPrimitive = intPrimitive;
        }

        public long getLongPrimitive() {
            return longPrimitive;
        }

        public Double getDoubleBoxed() {
            return doubleBoxed;
        }

        public void setDoubleBoxed(Double doubleBoxed) {
            this.doubleBoxed = doubleBoxed;
        }
    }

    /** Local mirror of the pinned SupportContextInitEventWLength bean. */
    public static class LocalSupportContextInitEventWLength {
        private final String id;
        private final int intSize;

        public LocalSupportContextInitEventWLength(String id, int intSize) {
            this.id = id;
            this.intSize = intSize;
        }

        public String getId() {
            return id;
        }

        public int getIntSize() {
            return intSize;
        }
    }
}
