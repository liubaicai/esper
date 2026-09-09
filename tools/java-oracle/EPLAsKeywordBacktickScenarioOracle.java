import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

/**
 * Java oracle for the EPLOtherAsKeywordBacktick backtick-quoted as-alias
 * scenarios.
 *
 * Covers the behavioral trio of the suite: EPLOtherFromClause (ordinal 1,
 * two-stream lastevent join with the reserved-word aliases `order` and
 * `select`), EPLOtherUpdateIStream (ordinal 3, update-istream alias
 * rewrite), and EPLOtherSubselect (ordinal 5, lastevent subselect alias),
 * by replaying each execution's deterministic event sequence in a fresh
 * runtime and recording the observable listener rows as
 * {case, sequence, new, old?}.
 *
 * The EPL strings are transcribed byte-exact from the suite; the three
 * targeted executions are single Java literals, so there are no string
 * concatenation artifacts. The update-istream case replays the suite's two
 * sequential compileDeploy calls as two modules in order, with the un-named
 * update-istream statement deployed before the named s0 consumer. An
 * update-istream rewrite is applied to the event before any consumer
 * observes it, so the s0 row shows p00 rewritten to the p01 value. The
 * suite's env.milestone(0) in EPLOtherFromClause is harness lifecycle
 * bookkeeping with no listener-observable effect and is not replayed.
 *
 * SupportBean_S0 and SupportBean_S1 are the real common-module classes
 * (com.espertech.esper.common.internal.support), which are on the run
 * script's fixed classpath, registered under the suite's event type names.
 * The send helper mirrors the suite's constructor usage exactly:
 * SupportBean_S0(id), SupportBean_S0(id, p00), SupportBean_S0(id, p00,
 * p01), SupportBean_S1(id) and SupportBean_S1(id, p10); an absent JSON
 * field selects the shorter constructor (the scenarios never pass explicit
 * null property strings).
 *
 * Listener rows record the output event's property surface: sorted
 * top-level property names for the update-istream and subselect rows, and
 * the suite-asserted property paths order, order.p00, select, select.p10
 * for the join row (assertPropsNew("s0",
 * "order,select,order.p00,select.p10", ...)). Join fragment columns arrive
 * as EventBean wrappers over the exact sent instances; after unwrapping
 * they render as nested objects of the sorted bean event properties (id,
 * p00..p03 and id, p10..p13, nulls as JSON null), pinning the asserted
 * event contents without depending on Java toString formatting. Scalars
 * render as strings per the insertinto-event-precedence oracle convention.
 * All three scenarios are istream-only, so old rows never fire; the writer
 * still emits an "old" array should one ever appear.
 */
public class EPLAsKeywordBacktickScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLAsKeywordBacktickScenarioOracle <scenario.json>");
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
        root.add("javaCommit", "9e1b9f1cc9117fea4bf33ab043762c045d73839c");
        root.add("java", "17");
        JsonArray out = new JsonArray();
        for (JsonObject r : records) {
            out.add(r);
        }
        root.add("records", out);
        System.out.println(root.toString());
    }

    private static void runCase(JsonArray allSteps, String caseName, List<JsonObject> records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType("SupportBean_S0", SupportBean_S0.class);
        configuration.getCommon().addEventType("SupportBean_S1", SupportBean_S1.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-as-keyword-" + caseName, configuration);
        runtime.initialize();

        try {
            List<EPDeployment> deployments = new ArrayList<>();
            for (String module : eplModulesFor(caseName)) {
                CompilerArguments compilerArguments = new CompilerArguments(configuration);
                compilerArguments.getPath().add(runtime.getRuntimePath());
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(module, compilerArguments);
                deployments.add(runtime.getDeploymentService().deploy(compiled,
                        new DeploymentOptions().setDeploymentId(
                                "parity-as-keyword-" + caseName + "-" + deployments.size())));
            }

            for (EPDeployment deployment : deployments) {
                for (EPStatement candidate : deployment.getStatements()) {
                    if ("s0".equals(candidate.getName())) {
                        new TraceWriter(records, caseName, candidate);
                    }
                }
            }

            replayCase(allSteps, caseName, runtime);

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * Per-case deployment modules in suite compileDeploy order, each EPL
     * string byte-exact from EPLOtherAsKeywordBacktick.
     */
    private static List<String> eplModulesFor(String caseName) {
        List<String> modules = new ArrayList<>();
        switch (caseName) {
            case "update-istream":
                // EPLOtherUpdateIStream (ordinal 3): the update-istream
                // statement rewrites p00 through the backtick-quoted `order`
                // alias before any consumer observes the event.
                modules.add("update istream SupportBean_S0 as `order` set p00=`order`.p01");
                modules.add("@name('s0') select * from SupportBean_S0");
                break;
            case "subselect-groups":
                // EPLOtherSubselect (ordinal 5): a lastevent subselect reads
                // the previous S0 event through the `order` alias.
                modules.add("@name('s0') select (select `order`.p00 from SupportBean_S0#lastevent as `order`) as c0 from SupportBean_S1");
                break;
            case "from-clause-join":
                // EPLOtherFromClause (ordinal 1): two-stream lastevent join
                // with the reserved-word aliases `order` and `select`.
                modules.add("@name('s0') select * from SupportBean_S0#lastevent as `order`, SupportBean_S1#lastevent as `select`");
                break;
            default:
                throw new IllegalArgumentException("unknown case: " + caseName);
        }
        return modules;
    }

    private static void replayCase(JsonArray allSteps, String caseName, EPRuntime runtime) {
        boolean active = false;
        for (JsonValue stepVal : allSteps) {
            JsonObject step = stepVal.asObject();
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
        int id = step.getInt("id", 0);
        if ("SupportBean_S0".equals(eventType)) {
            String p00 = optionalString(step, "p00");
            String p01 = optionalString(step, "p01");
            SupportBean_S0 bean;
            if (p01 != null) {
                bean = new SupportBean_S0(id, p00, p01);
            } else if (p00 != null) {
                bean = new SupportBean_S0(id, p00);
            } else {
                bean = new SupportBean_S0(id);
            }
            runtime.getEventService().sendEventBean(bean, "SupportBean_S0");
        } else if ("SupportBean_S1".equals(eventType)) {
            String p10 = optionalString(step, "p10");
            SupportBean_S1 bean = p10 != null ? new SupportBean_S1(id, p10) : new SupportBean_S1(id);
            runtime.getEventService().sendEventBean(bean, "SupportBean_S1");
        } else {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
    }

    private static String optionalString(JsonObject step, String name) {
        JsonValue value = step.get(name);
        if (value == null || value.isNull()) {
            return null;
        }
        if (value.isString()) {
            return value.asString();
        }
        throw new IllegalArgumentException(name + " must be a JSON string or null");
    }

    private static final class TraceWriter implements UpdateListener {
        private final List<JsonObject> records;
        private final String caseName;
        private int sequence = 0;

        TraceWriter(List<JsonObject> records, String caseName, EPStatement statement) {
            this.records = records;
            this.caseName = caseName;
            statement.addListener(this);
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement, EPRuntime runtime) {
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if (!hasNew && !hasOld) {
                return;
            }
            sequence++;
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", "s0");
            record.add("sequence", sequence);
            record.add("time", "1970-01-01T00:00:00Z");
            if (hasNew) {
                JsonArray newArr = new JsonArray();
                for (EventBean event : newEvents) {
                    newArr.add(row(caseName, event));
                }
                record.add("new", newArr);
            }
            if (hasOld) {
                JsonArray oldArr = new JsonArray();
                for (EventBean event : oldEvents) {
                    oldArr.add(row(caseName, event));
                }
                record.add("old", oldArr);
            }
            records.add(record);
        }

        private static JsonObject row(String caseName, EventBean event) {
            JsonObject fields = new JsonObject();
            for (String name : columnsFor(caseName, event)) {
                fields.add(name, normalize(event.get(name)));
            }
            JsonObject evt = new JsonObject();
            evt.add("kind", "row");
            evt.add("fields", fields);
            return evt;
        }
    }

    /**
     * Recorded property paths per row. The join row uses the suite's
     * asserted paths (order, select plus the nested order.p00 and
     * select.p10); every other row dumps the output event's full sorted
     * top-level property surface.
     */
    private static List<String> columnsFor(String caseName, EventBean event) {
        if ("from-clause-join".equals(caseName)) {
            return Arrays.asList("order", "order.p00", "select", "select.p10");
        }
        String[] names = event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        return Arrays.asList(names);
    }

    private static JsonValue normalize(Object value) {
        if (value instanceof EventBean) {
            // Join fragment columns arrive as EventBean wrappers over the
            // exact sent instances; render the underlying.
            value = ((EventBean) value).getUnderlying();
        }
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value instanceof SupportBean_S0) {
            SupportBean_S0 bean = (SupportBean_S0) value;
            return new JsonObject()
                    .add("id", bean.getId())
                    .add("p00", stringOrNull(bean.getP00()))
                    .add("p01", stringOrNull(bean.getP01()))
                    .add("p02", stringOrNull(bean.getP02()))
                    .add("p03", stringOrNull(bean.getP03()));
        }
        if (value instanceof SupportBean_S1) {
            SupportBean_S1 bean = (SupportBean_S1) value;
            return new JsonObject()
                    .add("id", bean.getId())
                    .add("p10", stringOrNull(bean.getP10()))
                    .add("p11", stringOrNull(bean.getP11()))
                    .add("p12", stringOrNull(bean.getP12()))
                    .add("p13", stringOrNull(bean.getP13()));
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).doubleValue());
        }
        if (value instanceof String) {
            return Json.value((String) value);
        }
        return Json.value(String.valueOf(value));
    }

    private static JsonValue stringOrNull(String value) {
        return value == null ? new JsonObject().add("state", "null") : Json.value(value);
    }
}
