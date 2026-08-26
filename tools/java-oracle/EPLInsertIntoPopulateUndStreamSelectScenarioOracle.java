import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventType;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.avro.support.SupportAvroUtil;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import org.apache.avro.Schema;
import org.apache.avro.generic.GenericData;

import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;
import java.io.Serializable;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.Iterator;
import java.util.List;
import java.util.Map;

/**
 * Oracle for the EPLInsertIntoPopulateUndStreamSelect observable executions:
 * EPLInsertIntoNamedWindowInheritsMap (named-window-inherits-map),
 * EPLInsertIntoNamedWindowRep (named-window-rep) and
 * EPLInsertIntoStreamInsertWWidenOA (stream-insert-w-widen). The fourth
 * execution, EPLInsertIntoInvalid, stays outside this work unit per contract.
 *
 * Statement text mirrors the pinned suite verbatim (including @public
 * auto-created insert-into targets resolved through the runtime path, which
 * the suite's RegressionPath reproduces here by compiling every module
 * against the accumulated runtime path plus the runtime itself). Each case
 * runs in a fresh runtime named parity-iups-&lt;case&gt; whose deployments are
 * numbered parity-iups-&lt;case&gt;-&lt;n&gt;. Within one case the
 * representation iterations reuse the suite's schema/statement names (A, C,
 * MyWindow, s0, Src, D1..D4) exactly like the pinned environment reuses them
 * across undeployAll cycles, so trace statement labels match the suite.
 *
 * Record protocol (frozen work-unit contract):
 * - listener rows via s0 subscribers carry operation "listener";
 * - window iteration renders as an operation "snapshot" row record;
 * - longs render as integers, doubles render through the Go runner's
 *   javaDoubleString convention (integral doubles gain a ".0" suffix, so the
 *   pinned 1d renders as "1.0"); absent properties render {"state":"null"};
 * - nested event fragments (Incident.event) render as
 *   {"kind":"row","fields":{...}} with sorted property names, matching the
 *   Go runner's compat normalizer byte for byte.
 *
 * Internal timer disabled so trace timestamps are epoch zero.
 */
public final class EPLInsertIntoPopulateUndStreamSelectScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "epl-insert-into-populate-und-stream-select";

    private enum Rep {
        OBJECTARRAY, MAP, AVRO, JSON, JSONCLASSPROVIDED, DEFAULT
    }

    /** Inventory order of the three differential-verified executions. */
    private static final String[] CASES = {
        "named-window-inherits-map",
        "named-window-rep",
        "stream-insert-w-widen",
    };

    /** EventRepresentationChoice.values() minus the json-provided-class rep. */
    private static final Rep[] NAMED_WINDOW_REPS = {
        Rep.OBJECTARRAY, Rep.MAP, Rep.AVRO, Rep.JSON, Rep.DEFAULT,
    };

    /** All six representations including json-provided. */
    private static final Rep[] STREAM_W_WIDEN_REPS = Rep.values();

    private EPLInsertIntoPopulateUndStreamSelectScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length < 1 || args.length > 4) {
            throw new IllegalArgumentException("usage: ... <scenario.json> [caseFilter] [seqOffset] [phaseFilter]");
        }
        String filterCase = args.length >= 2 ? args[1] : null;
        long seqOffset = args.length >= 3 ? Long.parseLong(args[2]) : 0L;
        String phaseFilter = args.length == 4 ? args[3] : null;
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        if (!SCENARIO_ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("unexpected scenario id " + scenario.getString("id", ""));
        }
        JsonArray allSteps = scenario.get("steps").asArray();

        JsonObject trace = new JsonObject()
                .add("version", VERSION)
                .add("id", SCENARIO_ID);
        JsonArray records = new JsonArray();
        trace.add("records", records);

        if (filterCase == null || "named-window-inherits-map".equals(filterCase)) {
            if (hasCase(allSteps, "named-window-inherits-map")) {
                runNamedWindowInheritsMap(
                    caseSends(allSteps, "named-window-inherits-map"),
                    caseSnapshots(allSteps, "named-window-inherits-map"),
                    records, new Seq(seqOffset));
            }
        }
        if (filterCase == null || "named-window-rep".equals(filterCase)) {
            if (hasCase(allSteps, "named-window-rep")) {
                runNamedWindowRep(
                    caseSends(allSteps, "named-window-rep"),
                    caseSnapshots(allSteps, "named-window-rep"),
                    records, new Seq(seqOffset), phaseFilter);
            }
        }
        if (filterCase == null || "stream-insert-w-widen".equals(filterCase)) {
            if (hasCase(allSteps, "stream-insert-w-widen")) {
                runStreamInsertWWiden(
                    caseSends(allSteps, "stream-insert-w-widen"),
                    caseSnapshots(allSteps, "stream-insert-w-widen"),
                    records, new Seq(seqOffset));
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

    /** Ordered steps of one op kind inside one case marker segment. */
    private static List<JsonObject> caseStepsOfOp(JsonArray allSteps, String caseName, String op) {
        List<JsonObject> found = new ArrayList<>();
        boolean active = false;
        for (int i = 0; i < allSteps.size(); i++) {
            JsonObject step = allSteps.get(i).asObject();
            String stepOp = step.getString("op", "");
            if ("case".equals(stepOp)) {
                active = caseName.equals(step.getString("case", ""));
                continue;
            }
            if (active && op.equals(stepOp)) {
                found.add(step);
            }
        }
        return found;
    }

    private static List<JsonObject> caseSends(JsonArray allSteps, String caseName) {
        return caseStepsOfOp(allSteps, caseName, "send");
    }

    private static List<JsonObject> caseSnapshots(JsonArray allSteps, String caseName) {
        return caseStepsOfOp(allSteps, caseName, "snapshot");
    }

    /**
     * exec0 NamedWindowInheritsMap: OA schemas where Incident.event is typed
     * as the supertype Event, a merge rule with OptionalProperty+Cast chain
     * and subtype instance insertion, observed through one window iteration
     * rendered as a snapshot record.
     */
    private static void runNamedWindowInheritsMap(List<JsonObject> sends, List<JsonObject> snapshots,
                                                  JsonArray records, Seq seq) throws Exception {
        String caseName = "named-window-inherits-map";
        if (sends.size() != 1 || snapshots.size() != 1) {
            throw new IllegalArgumentException(caseName + " needs exactly one send and one snapshot");
        }
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-iups-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName, runtime, seq);

            String epl = "@public @buseventtype @public create objectarray schema Event();\n" +
                "@public @buseventtype create objectarray schema ChildEvent(id string, action string) inherits Event;\n" +
                "@public @buseventtype create objectarray schema Incident(name string, event Event);\n" +
                "@Name('window') create window IncidentWindow#keepall as Incident;\n" +
                "\n" +
                "on ChildEvent e\n" +
                "    merge IncidentWindow w\n" +
                "    where e.id = cast(w.event.id? as string)\n" +
                "    when not matched\n" +
                "        then insert (name, event) select 'ChildIncident', e \n" +
                "            where e.action = 'INSERT'\n" +
                "    when matched\n" +
                "        then update set w.event = e \n" +
                "            where e.action = 'INSERT'\n" +
                "        then delete\n" +
                "            where e.action = 'CLEAR';";
            CompilerArguments compilerArguments = new CompilerArguments(configuration);
            compilerArguments.getPath().add(runtime.getRuntimePath());
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArguments);
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                new DeploymentOptions().setDeploymentId(deploymentId(caseName, 0)));

            sendStep(runtime, sends.get(0));
            writer.appendSnapshot(snapshots.get(0).getString("statement", ""), findStatement(deployment, "window"));
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * exec1 NamedWindowRep: five representation iterations (json-provided
     * skipped exactly like the pinned assertion comment requires), each with
     * phase a (select underlying) followed by module undeploy while the
     * window persists, then phase b (select underlying plus property).
     */
    private static void runNamedWindowRep(List<JsonObject> sends, List<JsonObject> snapshots,
                                          JsonArray records, Seq seq, String phaseFilter) throws Exception {
        String caseName = "named-window-rep";
        if (!snapshots.isEmpty()) {
            throw new IllegalArgumentException(caseName + " carries listener records only");
        }
        if (sends.size() != NAMED_WINDOW_REPS.length * 2) {
            throw new IllegalArgumentException(caseName + " needs two sends per representation");
        }
        int repOrdinal = 0;
        // Fresh runtime per representation: undeployAll does not release
        // @public path types, so same-name schemas cannot re-deploy inside
        // one runtime. Mirrors the Go runner's fresh environment per rep.
        for (Rep rep : NAMED_WINDOW_REPS) {
            Configuration configuration = new Configuration();
            configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
            configuration.getCommon().getEventMeta().getAvroSettings().setEnableAvro(true);
            EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-iups-" + caseName + "-" + repOrdinal, configuration);
            ((EPRuntimeSPI) runtime).initialize(0L);
            try {
                TraceWriter writer = new TraceWriter(records, caseName, runtime, seq);
                List<EPCompiled> path = new ArrayList<>();

                // One compilation per JVM invocation: json+inheritance
                // underlyings may be generated exactly once per process.
                String producers;
                if ("b".equals(phaseFilter)) {
                    producers = "insert into MyWindow select mya.*, 1 as addprop from A as mya;";
                } else {
                    producers = "insert into MyWindow select mya.* from A as mya;";
                }
                String schemaEpl =
                    annotationText(rep) + "@name('schema') @public @buseventtype create schema A as (myint int, mystr string);\n" +
                    annotationText(rep) + "@public @buseventtype create schema C as (addprop int) inherits A;\n" +
                    "@public create window MyWindow#time(5 days) as C;\n" +
                    "@name('s0') select * from MyWindow;\n" +
                    producers;
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(schemaEpl,
                    compilerArguments(configuration, runtime, path));
                EPDeployment schemaDeployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(deploymentId(caseName, deployCountFor(repOrdinal))));
                EPStatement s0 = findStatement(schemaDeployment, "s0");
                writer.observe(s0);

                if ("b".equals(phaseFilter)) {
                    sendSourceEvent(runtime, rep, "A", schemaDeployment.getDeploymentId(), sends.get(repOrdinal * 2 + 1));
                } else {
                    sendSourceEvent(runtime, rep, "A", schemaDeployment.getDeploymentId(), sends.get(repOrdinal * 2));
                }
                writer.unobserve(s0);
            } finally {
                runtime.destroy();
            }
            repOrdinal++;
        }
    }

    /** Per-rep deployment numbering restarts with each fresh runtime. */
    private static int deployCountFor(int repOrdinal) {
        return repOrdinal * 100;
    }

    /** Compiler arguments carrying the live runtime path (schemas stay deployed per rep). */
    private static CompilerArguments compilerArguments(Configuration configuration, EPRuntime runtime,
                                                       List<EPCompiled> path) {
        CompilerArguments arguments = new CompilerArguments(configuration);
        arguments.getPath().add(runtime.getRuntimePath());
        return arguments;
    }

    /**
     * exec2 StreamInsertWWidenOA: all six reps x six inserting-statement
     * observations. The listener sits on every inserting statement ("s0")
     * exactly like runStreamInsertAssertion, which also pins the numeric
     * literal typings: addprop renders long 1 as integer 1 and double 1d as
     * the string "1.0".
     */
    private static void runStreamInsertWWiden(List<JsonObject> sends, List<JsonObject> snapshots,
                                              JsonArray records, Seq seq) throws Exception {
        String caseName = "stream-insert-w-widen";
        if (!snapshots.isEmpty()) {
            throw new IllegalArgumentException(caseName + " carries listener records only");
        }
        if (sends.size() != STREAM_W_WIDEN_REPS.length * 6) {
            throw new IllegalArgumentException(caseName + " needs six sends per representation");
        }
        int sendIndex = 0;
        int repOrdinal = 0;
        // Fresh runtime per representation (see runNamedWindowRep).
        for (Rep rep : STREAM_W_WIDEN_REPS) {
            Configuration configuration = new Configuration();
            configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
            configuration.getCommon().getEventMeta().getAvroSettings().setEnableAvro(true);
            EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-iups-" + caseName + "-" + repOrdinal, configuration);
            ((EPRuntimeSPI) runtime).initialize(0L);
            try {
                TraceWriter writer = new TraceWriter(records, caseName, runtime, seq);
                List<EPCompiled> path = new ArrayList<>();

                String srcEpl = annotationWJsonProvided(rep, "MyLocalJsonProvidedSrc") +
                    "@name('schema') @public @buseventtype create schema Src as (myint int, mystr string)";
                EPCompiled srcCompiled = EPCompilerProvider.getCompiler().compile(srcEpl,
                    compilerArguments(configuration, runtime, path));
                path.add(srcCompiled);
                EPDeployment srcDeployment = runtime.getDeploymentService().deploy(srcCompiled,
                    new DeploymentOptions().setDeploymentId(deploymentId(caseName, deployCountFor(repOrdinal))));

                String[][] stages = {
                    {"MyLocalJsonProvidedD1", "@public create schema D1 as (myint int, mystr string, addprop long)",
                        "insert into D1 select 1 as addprop, mysrc.* from Src as mysrc"},
                    {"MyLocalJsonProvidedD2", "@public create schema D2 as (mystr string, myint int, addprop double)",
                        "insert into D2 select 1 as addprop, mysrc.* from Src as mysrc"},
                    {"MyLocalJsonProvidedD3", "@public create schema D3 as (mystr string, addprop int)",
                        "insert into D3 select 1 as addprop, mysrc.* from Src as mysrc"},
                    {"MyLocalJsonProvidedD4", "@public create schema D4 as (myint int, mystr string)",
                        "insert into D4 select mysrc.* from Src as mysrc"},
                    {null, null, "insert into D4 select mysrc.*, 999 as myint, 'xxx' as mystr from Src as mysrc"},
                    {null, null, "insert into D4 select 999 as myint, 'xxx' as mystr, mysrc.* from Src as mysrc"},
                };
                int stageIndex = 0;
                for (String[] stage : stages) {
                    if (stage[1] != null) {
                        EPCompiled stageCompiled = EPCompilerProvider.getCompiler().compile(
                            annotationWJsonProvided(rep, stage[0]) + stage[1],
                            compilerArguments(configuration, runtime, path));
                        path.add(stageCompiled);
                        runtime.getDeploymentService().deploy(stageCompiled,
                            new DeploymentOptions().setDeploymentId(
                                deploymentId(caseName, deployCountFor(repOrdinal) + 1 + stageIndex)));
                    }
                    EPCompiled s0Compiled = EPCompilerProvider.getCompiler().compile(
                        "@name('s0') " + stage[2], compilerArguments(configuration, runtime, path));
                    EPDeployment s0Deployment = runtime.getDeploymentService().deploy(s0Compiled,
                        new DeploymentOptions().setDeploymentId(
                            deploymentId(caseName, deployCountFor(repOrdinal) + 50 + stageIndex)));
                    EPStatement s0 = findStatement(s0Deployment, "s0");
                    writer.observe(s0);
                    sendSourceEvent(runtime, rep, "Src", srcDeployment.getDeploymentId(), sends.get(sendIndex++));
                    writer.unobserve(s0);
                    runtime.getDeploymentService().undeploy(s0Deployment.getDeploymentId());
                    stageIndex++;
                }
            } finally {
                runtime.destroy();
            }
            repOrdinal++;
        }
    }

    private static String deploymentId(String caseName, int index) {
        return "parity-iups-" + caseName + "-" + index;
    }

    private static void sendStep(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if (!"ChildEvent".equals(eventType)) {
            throw new IllegalArgumentException("unsupported event type " + eventType);
        }
        runtime.getEventService().sendEventObjectArray(
            new Object[]{payload.getString("id", null), payload.getString("action", null)}, eventType);
    }

    /**
     * Mirrors the pinned sendEvent helper semantics per representation for
     * the two-property source events A/Src ({myint:int, mystr:string}); the
     * Avro branch resolves the deployed schema through the deployment that
     * carries @name('schema'), like env.runtimeAvroSchemaByDeployment.
     */
    private static void sendSourceEvent(EPRuntime runtime, Rep rep, String eventType,
                                        String schemaDeploymentId, JsonObject step) {
        JsonObject payload = step.get("payload").asObject();
        int myint = payload.getInt("myint", 0);
        String mystr = payload.getString("mystr", null);
        switch (rep) {
            case OBJECTARRAY -> runtime.getEventService().sendEventObjectArray(new Object[]{myint, mystr}, eventType);
            case MAP, DEFAULT -> {
                Map<String, Object> event = new HashMap<>();
                event.put("myint", myint);
                event.put("mystr", mystr);
                runtime.getEventService().sendEventMap(event, eventType);
            }
            case AVRO -> {
                EventType deployedType = runtime.getEventTypeService().getEventType(schemaDeploymentId, eventType);
                Schema schema = SupportAvroUtil.getAvroSchema(deployedType);
                GenericData.Record record = new GenericData.Record(schema);
                record.put("myint", myint);
                record.put("mystr", mystr);
                runtime.getEventService().sendEventAvro(record, eventType);
            }
            case JSON, JSONCLASSPROVIDED -> {
                JsonObject object = new JsonObject();
                object.add("myint", myint);
                object.add("mystr", mystr);
                runtime.getEventService().sendEventJson(object.toString(), eventType);
            }
            default -> throw new IllegalStateException("unhandled representation " + rep);
        }
    }

    private static EPStatement findStatement(EPDeployment deployment, String name) {
        for (EPStatement candidate : deployment.getStatements()) {
            if (name.equals(candidate.getName())) {
                return candidate;
            }
        }
        throw new IllegalStateException("deployment has no statement named " + name);
    }

    /**
     * Mirrors EventRepresentationChoice.getAnnotationText(): only the plain
     * representation annotations; JSONCLASSPROVIDED must use the
     * WJsonProvided form instead.
     */
    private static String annotationText(Rep rep) {
        switch (rep) {
            case OBJECTARRAY:
                return "@EventRepresentation('objectarray')";
            case MAP:
                return "@EventRepresentation('map')";
            case AVRO:
                return "@EventRepresentation('avro')";
            case JSON:
                return "@EventRepresentation('json')";
            case DEFAULT:
                return "";
            default:
                throw new IllegalStateException("representation requires the WJsonProvided form " + rep);
        }
    }

    /**
     * Mirrors EventRepresentationChoice.getAnnotationTextWJsonProvided(Class):
     * only JSONCLASSPROVIDED differs from the plain annotation text, prefixing
     * "@JsonSchema(className='...')" referencing the local MyLocalJsonProvided*
     * mirror classes declared at the bottom of this file.
     */
    private static String annotationWJsonProvided(Rep rep, String providedClassSimpleName) {
        switch (rep) {
            case JSONCLASSPROVIDED:
                return "@JsonSchema(className='" + EPLInsertIntoPopulateUndStreamSelectScenarioOracle.class.getName()
                    + "$" + providedClassSimpleName + "') @EventRepresentation('json')";
            default:
                return annotationText(rep);
        }
    }

    /**
     * Listener/snapshot trace writer emitting esper-parity/v1 records. Nested
     * EventBean fragments render as {"kind":"row","fields":{...}}, doubles go
     * through the javaDoubleString convention and absent properties render
     * {"state":"null"}, matching the Go runner's compat normalizer.
     */
    /** Mutable sequence holder shared by every writer of one invocation. */
    private static final class Seq {
        private long value;

        private Seq(long start) {
            this.value = start;
        }

        private long bump() {
            return ++value;
        }
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPRuntime runtime;
        private final Seq sequence;

        private TraceWriter(JsonArray records, String caseName, EPRuntime runtime, Seq sequence) {
            this.records = records;
            this.caseName = caseName;
            this.runtime = runtime;
            this.sequence = sequence;
        }

        private void observe(EPStatement statement) {
            statement.addListener(this);
        }

        private void unobserve(EPStatement statement) {
            statement.removeListener(this);
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement, EPRuntime ignoredRuntime) {
            append(sequence.bump(), statement.getName(), newEvents, oldEvents);
        }

        /** Window iterator assertion rendered as a snapshot-style row record. */
        private void appendSnapshot(String statementName, EPStatement statement) {
            List<EventBean> events = new ArrayList<>();
            for (Iterator<EventBean> it = statement.iterator(); it.hasNext(); ) {
                events.add(it.next());
            }
            append(statementName, events.toArray(new EventBean[0]), null, true);
        }

        // Final sequence numbering is assigned by the shell merge step's
        // renumber-to-one-based-N pass; the per-invocation bump only keeps
        // intra-invocation ordering stable.
        private void append(String statementName, EventBean[] newEvents, EventBean[] oldEvents, boolean snapshot) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", snapshot ? "snapshot" : "listener")
                    .add("statement", statementName)
                    .add("sequence", sequence.bump())
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
                output.add(new JsonObject().add("kind", "row").add("fields", fieldsOf(event)));
            }
            return output;
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof EventBean eventBean) {
                JsonObject object = new JsonObject();
                object.add("kind", "row");
                object.add("fields", fieldsOf(eventBean));
                return object;
            }
            if (value instanceof Double || value instanceof Float) {
                return Json.value(javaDoubleString(((Number) value).doubleValue()));
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
            return Json.value(String.valueOf(value));
        }

        /** Frozen record protocol: fragments render as rows-of-fields. */
        private JsonObject fieldsOf(EventBean eventBean) {
            JsonObject fields = new JsonObject();
            String[] names = eventBean.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            for (String name : names) {
                Object value;
                try {
                    value = eventBean.get(name);
                } catch (com.espertech.esper.common.client.PropertyAccessException unreadable) {
                    continue;
                }
                fields.add(name, normalize(value));
            }
            return fields;
        }

        /**
         * Mirrors the Go runner's javaDoubleString convention:
         * strconv.FormatFloat(v, 'g', -1, 64) gains a ".0" suffix when the
         * shortest rendering carries no decimal separator or exponent. The
         * pinned scenario only produces integral doubles such as 1d -> "1.0".
         */
        private static String javaDoubleString(double value) {
            if (value == Math.rint(value) && !Double.isInfinite(value)) {
                return (long) value + ".0";
            }
            return Double.toString(value);
        }
    }

    // Local mirrors of the suite's json-provided underlyings; field names and
    // types match the pinned MyLocalJsonProvided* classes so the
    // @JsonSchema(className=...) annotations resolve identically.

    public static class MyLocalJsonProvidedSrc implements Serializable {
        public int myint;
        public String mystr;
    }

    public static class MyLocalJsonProvidedD1 implements Serializable {
        public int myint;
        public String mystr;
        public long addprop;
    }

    public static class MyLocalJsonProvidedD2 implements Serializable {
        public String mystr;
        public int myint;
        public double addprop;
    }

    public static class MyLocalJsonProvidedD3 implements Serializable {
        public String mystr;
        public int addprop;
    }

    public static class MyLocalJsonProvidedD4 implements Serializable {
        public int myint;
        public String mystr;
    }
}
