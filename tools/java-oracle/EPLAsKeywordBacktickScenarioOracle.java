import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.fireandforget.EPFireAndForgetQueryResult;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
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
 * Covers the behavioral trio of the suite (EPLOtherFromClause ordinal 1,
 * two-stream lastevent join with the reserved-word aliases `order` and
 * `select`; EPLOtherUpdateIStream ordinal 3, update-istream alias rewrite;
 * EPLOtherSubselect ordinal 5, lastevent subselect alias) plus the
 * FAF/on-trigger/merge trio (EPLOtherFAFUpdateDelete ordinal 0, FAF
 * insert/update/delete through the `order` alias; EPLOtherOnTrigger
 * ordinal 2, on-trigger table select through the `order` alias;
 * EPLOthernMergeAndUpdateAndSelect ordinal 4, merge and update on-trigger
 * statements with a final on-trigger select), by replaying each
 * execution's deterministic event sequence in a fresh runtime and
 * recording the observable listener rows as {case, sequence, new, old?}.
 *
 * The EPL strings are transcribed byte-exact from the suite; the six
 * targeted executions are single Java literals, so there are no string
 * concatenation artifacts. The update-istream case replays the suite's two
 * sequential compileDeploy calls as two modules in order, with the un-named
 * update-istream statement deployed before the named s0 consumer. An
 * update-istream rewrite is applied to the event before any consumer
 * observes it, so the s0 row shows p00 rewritten to the p01 value. The
 * FAF/on-trigger/merge cases replay the suite's RegressionPath usage by
 * compiling every deployment and every fire-and-forget statement against
 * the runtime path, mirroring RegressionEnvironment.compileFAF which
 * routes compileQuery through the same CompilerArguments path. FAF
 * mutations emit no records; FAF selects emit operation "faf" records
 * named faf-select-N whose rows use the same rendering as listener rows,
 * an empty window rendering an empty new array. Each case's sequence
 * counter is shared across its FAF and listener records and increments
 * only when a record is emitted. The suite's env.milestone(0) calls in
 * EPLOtherFromClause, EPLOtherFAFUpdateDelete and
 * EPLOthernMergeAndUpdateAndSelect are harness lifecycle bookkeeping with
 * no observable effect and are not replayed.
 *
 * SupportBean_S0, SupportBean_S1 and SupportBean are the real
 * common-module classes (com.espertech.esper.common.internal.support),
 * which are on the run script's fixed classpath, registered under the
 * suite's event type names. The send helper mirrors the suite's
 * constructor usage exactly: SupportBean_S0(id), SupportBean_S0(id, p00),
 * SupportBean_S0(id, p00, p01), SupportBean_S1(id), SupportBean_S1(id,
 * p10) and SupportBean(theString, intPrimitive); an absent or null JSON
 * field selects the suite's shape (the merge case's explicit theString
 * null with intPrimitive 0 matches the suite's new SupportBean()).
 *
 * Listener rows record the output event's property surface: sorted
 * top-level property names for the update-istream and subselect rows, the
 * v1 and c0 surfaces of the on-trigger rows, the p0/p1 named-window
 * surface of the FAF rows, and the suite-asserted property paths order,
 * order.p00, select, select.p10 for the join row
 * (assertPropsNew("s0", "order,select,order.p00,select.p10", ...)). Join
 * fragment columns arrive as EventBean wrappers over the exact sent
 * instances; after unwrapping they render as nested objects of the sorted
 * bean event properties (id, p00..p03 and id, p10..p13, nulls as JSON
 * null), pinning the asserted event contents without depending on Java
 * toString formatting. Scalars render as strings per the
 * insertinto-event-precedence oracle convention. All scenarios are
 * istream-only, so old rows never fire; the writer still emits an "old"
 * array should one ever appear.
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
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-as-keyword-" + caseName, configuration);
        runtime.initialize();

        CaseSequencer sequencer = new CaseSequencer(records, caseName);

        try {
            switch (caseName) {
                case "faf-update-delete":
                    runFafUpdateDelete(runtime, configuration, sequencer);
                    break;
                case "on-trigger-table-select":
                    runOnTriggerTableSelect(allSteps, runtime, configuration, sequencer);
                    break;
                case "merge-update-select":
                    runMergeUpdateSelect(allSteps, runtime, configuration, sequencer);
                    break;
                default:
                    List<EPDeployment> deployments = new ArrayList<>();
                    for (String module : eplModulesFor(caseName)) {
                        deployments.add(compileDeploy(runtime, configuration, caseName, module, deployments.size()));
                    }

                    for (EPDeployment deployment : deployments) {
                        attachS0Listener(sequencer, deployment);
                    }

                    replayCase(allSteps, caseName, runtime);
                    break;
            }

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

    private static EPDeployment compileDeploy(EPRuntime runtime, Configuration configuration, String caseName,
                                              String module, int deploymentIndex) throws Exception {
        CompilerArguments compilerArguments = new CompilerArguments(configuration);
        compilerArguments.getPath().add(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(module, compilerArguments);
        return runtime.getDeploymentService().deploy(compiled,
                new DeploymentOptions().setDeploymentId("parity-as-keyword-" + caseName + "-" + deploymentIndex));
    }

    private static void attachS0Listener(CaseSequencer sequencer, EPDeployment deployment) {
        for (EPStatement candidate : deployment.getStatements()) {
            if ("s0".equals(candidate.getName())) {
                new TraceWriter(sequencer, candidate);
            }
        }
    }

    /**
     * Compiles a fire-and-forget statement against the runtime path (the
     * oracle mirror of the suite's RegressionPath plus
     * RegressionEnvironment.compileFAF) and executes it.
     */
    private static EPFireAndForgetQueryResult executeFaf(EPRuntime runtime, Configuration configuration, String epl) throws Exception {
        CompilerArguments compilerArguments = new CompilerArguments(configuration);
        compilerArguments.getPath().add(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compileQuery(epl, compilerArguments);
        return runtime.getFireAndForgetService().executeQuery(compiled);
    }

    private static void emitFafRecord(CaseSequencer sequencer, String statement, EventBean[] rows) {
        JsonArray newArr = new JsonArray();
        for (EventBean event : rows) {
            newArr.add(TraceWriter.row(sequencer.caseName, event));
        }
        sequencer.emit("faf", statement, newArr);
    }

    /**
     * EPLOtherFAFUpdateDelete (ordinal 0): FAF insert, update and delete
     * through the backtick-quoted `order` alias against a keepall named
     * window; only the three FAF selects emit records, the last with zero
     * rows.
     */
    private static void runFafUpdateDelete(EPRuntime runtime, Configuration configuration, CaseSequencer sequencer) throws Exception {
        compileDeploy(runtime, configuration, "faf-update-delete",
                "@public create window MyWindowFAF#keepall as (p0 string, p1 string)", 0);

        executeFaf(runtime, configuration, "insert into MyWindowFAF select 'a' as p0, 'b' as p1");
        emitFafRecord(sequencer, "faf-select-1", executeFaf(runtime, configuration, "select * from MyWindowFAF").getArray());

        executeFaf(runtime, configuration, "update MyWindowFAF as `order` set `order`.p0 = `order`.p1");
        emitFafRecord(sequencer, "faf-select-2", executeFaf(runtime, configuration, "select * from MyWindowFAF").getArray());

        executeFaf(runtime, configuration, "delete from MyWindowFAF as `order` where `order`.p0 = 'b'");
        emitFafRecord(sequencer, "faf-select-3", executeFaf(runtime, configuration, "select * from MyWindowFAF").getArray());
    }

    /**
     * EPLOtherOnTrigger (ordinal 2): two FAF table inserts, then the
     * on-trigger select from the table through the `order` alias; the
     * single S0 event from the scenario steps fires the s0 listener row.
     */
    private static void runOnTriggerTableSelect(JsonArray allSteps, EPRuntime runtime, Configuration configuration,
                                                CaseSequencer sequencer) throws Exception {
        compileDeploy(runtime, configuration, "on-trigger-table-select",
                "@public create table MyTable(k1 string primary key, v1 string)", 0);
        executeFaf(runtime, configuration, "insert into MyTable select 'x' as k1, 'y' as v1");
        executeFaf(runtime, configuration, "insert into MyTable select 'a' as k1, 'b' as v1");
        EPDeployment deployment = compileDeploy(runtime, configuration, "on-trigger-table-select",
                "@name('s0') on SupportBean_S0 as `order` select v1 from MyTable where `order`.p00 = k1", 1);
        attachS0Listener(sequencer, deployment);

        replayCase(allSteps, "on-trigger-table-select", runtime);
    }

    /**
     * EPLOthernMergeAndUpdateAndSelect (ordinal 4): FAF insert, merge and
     * update on-trigger statements without listeners, three FAF selects
     * interleaved with the scenario's S0/S1 sends, then the final
     * on-trigger select whose s0 listener row is the fourth record.
     */
    private static void runMergeUpdateSelect(JsonArray allSteps, EPRuntime runtime, Configuration configuration,
                                             CaseSequencer sequencer) throws Exception {
        compileDeploy(runtime, configuration, "merge-update-select",
                "@public create window MyWindowMerge#keepall as (p0 string, p1 string)", 0);
        executeFaf(runtime, configuration, "insert into MyWindowMerge select 'a' as p0, 'b' as p1");
        compileDeploy(runtime, configuration, "merge-update-select",
                "on SupportBean_S0 merge MyWindowMerge as `order` when matched then update set `order`.p1 = `order`.p0", 1);
        compileDeploy(runtime, configuration, "merge-update-select",
                "on SupportBean_S1 update MyWindowMerge as `order` set p0 = 'x'", 2);

        emitFafRecord(sequencer, "faf-select-1", executeFaf(runtime, configuration, "select * from MyWindowMerge").getArray());
        replaySendAt(allSteps, "merge-update-select", runtime, 0);
        emitFafRecord(sequencer, "faf-select-2", executeFaf(runtime, configuration, "select * from MyWindowMerge").getArray());
        replaySendAt(allSteps, "merge-update-select", runtime, 1);
        emitFafRecord(sequencer, "faf-select-3", executeFaf(runtime, configuration, "select * from MyWindowMerge").getArray());

        EPDeployment deployment = compileDeploy(runtime, configuration, "merge-update-select",
                "@name('s0') on SupportBean select `order`.p0 as c0 from MyWindowMerge as `order`", 3);
        attachS0Listener(sequencer, deployment);
        replaySendAt(allSteps, "merge-update-select", runtime, 2);
    }

    /**
     * Executes the occurrence-th send step of the case, keeping the
     * scenario steps the single source of the event sequence.
     */
    private static void replaySendAt(JsonArray allSteps, String caseName, EPRuntime runtime, int occurrence) {
        int seen = 0;
        for (JsonValue stepVal : allSteps) {
            JsonObject step = stepVal.asObject();
            if (!"send".equals(step.getString("op", ""))) {
                continue;
            }
            if (!caseName.equals(step.getString("case", ""))) {
                continue;
            }
            if (seen++ != occurrence) {
                continue;
            }
            send(runtime, step);
            return;
        }
        throw new IllegalArgumentException("no send step " + occurrence + " for case: " + caseName);
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
        } else if ("SupportBean".equals(eventType)) {
            String theString = optionalString(step, "theString");
            int intPrimitive = step.getInt("intPrimitive", 0);
            runtime.getEventService().sendEventBean(new SupportBean(theString, intPrimitive), "SupportBean");
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

    /**
     * Per-case record sink with a sequence counter shared across the
     * case's FAF and listener records, incremented only when a record is
     * emitted.
     */
    private static final class CaseSequencer {
        private final List<JsonObject> records;
        private final String caseName;
        private int sequence = 0;

        CaseSequencer(List<JsonObject> records, String caseName) {
            this.records = records;
            this.caseName = caseName;
        }

        void emit(String operation, String statement, JsonArray newRows) {
            sequence++;
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", operation);
            record.add("statement", statement);
            record.add("sequence", sequence);
            record.add("time", "1970-01-01T00:00:00Z");
            record.add("new", newRows);
            records.add(record);
        }

        void emitListener(JsonArray newRows, JsonArray oldRows) {
            sequence++;
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", "s0");
            record.add("sequence", sequence);
            record.add("time", "1970-01-01T00:00:00Z");
            if (newRows != null) {
                record.add("new", newRows);
            }
            if (oldRows != null) {
                record.add("old", oldRows);
            }
            records.add(record);
        }
    }

    private static final class TraceWriter implements UpdateListener {
        private final CaseSequencer sequencer;

        TraceWriter(CaseSequencer sequencer, EPStatement statement) {
            this.sequencer = sequencer;
            statement.addListener(this);
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement, EPRuntime runtime) {
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if (!hasNew && !hasOld) {
                return;
            }
            sequencer.emitListener(
                    hasNew ? rows(sequencer.caseName, newEvents) : null,
                    hasOld ? rows(sequencer.caseName, oldEvents) : null);
        }

        private static JsonArray rows(String caseName, EventBean[] events) {
            JsonArray arr = new JsonArray();
            for (EventBean event : events) {
                arr.add(row(caseName, event));
            }
            return arr;
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
