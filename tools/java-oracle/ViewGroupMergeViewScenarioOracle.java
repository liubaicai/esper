import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
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
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Iterator;
import java.util.List;
import java.util.TreeSet;

/**
 * Java oracle for ViewGroup grouped-window scenarios (merge-view family).
 *
 * Covers three pinned executions of ViewGroup, each replayed as its own
 * case against a fresh runtime with the pinned module transcribed verbatim
 * (only the byte-exact EPL text; no other annotations are added):
 *
 * merge-view-union-aggregate replays ViewGroupObjectArrayEvent (execution
 * ordinal 0, runtime java-runtime-a3b6bef89e22a122cc7a):
 * select p1,sum(p2) as sp2 from OAEventStringInt#groupwin(p1)#length(2),
 * sending A/10, B/11, A/12, A/13 and pinning the per-group Long sum
 * evolution 10, 21, 33, 36 through istream-only listener records.
 *
 * length-win-groups replays ViewGroupLengthWin (ordinal 14, runtime
 * java-runtime-639bc9b69621f3b6a417): select irstream theString as c0,
 * intPrimitive as c1 from SupportBean#groupwin(theString)#length(3),
 * sending E1/1, E2/20, E1/2, E2/21, E2/22, E1/3 (istream-only while the
 * per-group windows are not full), then E2/23 evicting E2/20 and E1/-1
 * evicting E1/1 as irstream pairs. The suite's iterator assertions here
 * use assertPropsPerRowIteratorAnyOrder, i.e. Java itself leaves
 * cross-group iteration order unpinned, so no snapshot steps are recorded
 * for this case.
 *
 * ViewGroupMultiProperty (ordinal 6, groupwin(symbol, feed, volume)#uni(
 * price)) is deferred from this unit: #uni is a per-group derived-value
 * view that delivers irstream pairs on every insert whose old row is the
 * previous datapoints, and the Go fluent adaptation models #uni as a
 * select aggregate over a keepall group window, which produces no old
 * events on insert — the pinned old rows {size: N-1} have no Go
 * counterpart yet (the same adaptation gap class as ViewGroup ords
 * 1/4/5). The snapshot operation handler is retained so the deferred
 * case's ordered iterator pinning can be reinstated later with scenario
 * steps only.
 *
 * Suite milestones are harness ordering markers with no observable engine
 * output and record nothing. The OAEventStringInt event type mirrors the
 * pinned regression-run registration as an object-array type with property
 * names {p1, p2} and types {String, int}, sent through
 * sendEventObjectArray exactly like env.sendEventObjectArray. SupportBean
 * is the common-module class already on the classpath and is constructed
 * with the suite's two-arg (theString, intPrimitive) constructor.
 *
 * Listener records follow the standard protocol: one record per delivered
 * batch (not per row), sequence numbering per case from 1, time rendered
 * from the current engine time, and new/old row arrays rendered with the
 * scalar normalization rules; a batch is skipped only when the engine
 * delivers neither new nor old rows. Snapshot records carry no time or
 * sequence: step {op:"snapshot", statement:"s0"} iterates the deployed s0
 * statement and emits its current window contents.
 */
public class ViewGroupMergeViewScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ViewGroupMergeViewScenarioOracle <scenario.json>");
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
        // Mirrors the pinned regression-run registration of OAEventStringInt
        // as an object-array event type (TestSuiteView): names {p1, p2},
        // types {String, int}.
        config.getCommon().addEventType("OAEventStringInt",
            new String[] {"p1", "p2"}, new Object[] {String.class, int.class});
        config.getCommon().addEventType("SupportBean", SupportBean.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("ViewGroupMergeViewScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(buildEPL(caseName),
                new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            EPStatement s0 = runtime.getDeploymentService().getStatement(deployment.getDeploymentId(), "s0");
            if (s0 == null) {
                throw new IllegalStateException("statement s0 was not deployed for case " + caseName);
            }
            int[] seq = new int[] {0};
            s0.addListener((newData, oldData, statement, rt) -> {
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
                if ("snapshot".equals(op)) {
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

    /** Verbatim transcriptions of the pinned ViewGroup modules. */
    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "merge-view-union-aggregate" ->
                "@name('s0') select p1,sum(p2) as sp2 from OAEventStringInt#groupwin(p1)#length(2)";
            case "length-win-groups" ->
                "@Name('s0') select irstream theString as c0,intPrimitive as c1 from SupportBean#groupwin(theString)#length(3)";
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("OAEventStringInt".equals(eventType)) {
            // Mirrors env.sendEventObjectArray(new Object[]{p1, p2}, "OAEventStringInt").
            runtime.getEventService().sendEventObjectArray(
                new Object[] {payload.getString("p1", null), payload.getInt("p2", 0)}, eventType);
            return;
        }
        if ("SupportBean".equals(eventType)) {
            // Mirrors the suite helper sendSupportBean(env, theString, intPrimitive).
            runtime.getEventService().sendEventBean(
                new SupportBean(payload.getString("theString", null), payload.getInt("intPrimitive", 0)),
                eventType);
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
        return Json.value(String.valueOf(value));
    }
}
