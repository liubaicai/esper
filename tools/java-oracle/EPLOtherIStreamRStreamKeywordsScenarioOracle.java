import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerContext;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.AnnotationPart;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.soda.Expressions;
import com.espertech.esper.common.client.soda.FilterStream;
import com.espertech.esper.common.client.soda.FromClause;
import com.espertech.esper.common.client.soda.SelectClause;
import com.espertech.esper.common.client.soda.StreamSelector;
import com.espertech.esper.common.client.soda.View;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.util.SerializableObjectCopier;
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
import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for the EPLOtherIStreamRStreamKeywords ord 0/1/9 parity
 * scenario.
 *
 * Covers three executions, one fresh runtime per case:
 *
 *   - rstream-only-om (ord 0, EPLOtherRStreamOnlyOM): 'select rstream * from
 *     SupportBean#length(3)' built through the SODA object model
 *     (SelectClause.createWildcard(StreamSelector.RSTREAM_ONLY) over a
 *     FilterStream length(3) view, named s0 via annotation). Sends a/a/b/d:
 *     the first three inserts fire nothing and the fourth expires the first
 *     'a' bean, delivered as the single newData row with oldData null.
 *   - rstream-only-compile (ord 1, EPLOtherRStreamOnlyCompile): the identical
 *     statement reached through eplToModel — zero observable difference.
 *   - rstream-output-snapshot (ord 9, EPLOtherRStreamOutputSnapshot):
 *     'select rstream * from SupportBean#time(30 minutes) output snapshot'
 *     compileDeploy + undeployAll smoke — no listener, no sends, no records.
 *
 * Events are SupportBean payloads carrying theString/intPrimitive.
 *
 * Observations are listener records only: {case, operation, statement,
 * sequence, time, new, old} emitted for the s0 statement of the two
 * rstream-only cases; the snapshot case emits none. Rows render the pinned
 * SupportBean projection {theString,intPrimitive}.
 */
public class EPLOtherIStreamRStreamKeywordsScenarioOracle {

    private static final String RSTMT = "select rstream * from SupportBean#length(3)";
    private static final String SNAPSHOT_EPL = "select rstream * from SupportBean#time(30 minutes) output snapshot";
    private static final String[] PROJECTION = new String[]{"theString", "intPrimitive"};

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLOtherIStreamRStreamKeywordsScenarioOracle <scenario.json>");
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
        config.getCommon().addEventType(SupportBean.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        // The pinned runner configures SupportExceptionHandlerFactoryRethrow,
        // so statement exceptions rethrow on the sending thread; mirror that
        // wrapper so failures surface as exceptions rather than log lines.
        config.getRuntime().getExceptionHandling().addClass(RethrowExceptionHandlerFactory.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLOtherIStreamRStreamKeywordsScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        // Callback records buffer here and are flushed after each step;
        // listener delivery is synchronous on the sending thread.
        List<JsonObject> pending = new ArrayList<>();
        try {
            int[] seq = new int[]{0};
            if ("rstream-only-om".equals(caseName) || "rstream-only-compile".equals(caseName)) {
                EPStatementObjectModel model = buildModel(caseName, config);
                EPDeployment deployment = compileDeployModel(runtime, config, model);
                for (EPStatement candidate : deployment.getStatements()) {
                    if (!"s0".equals(candidate.getName())) {
                        continue;
                    }
                    candidate.addListener((newData, oldData, statement, rt) -> {
                        if ((newData == null || newData.length == 0) && (oldData == null || oldData.length == 0)) {
                            return;
                        }
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "listener");
                        record.add("statement", statement.getName());
                        record.add("sequence", ++seq[0]);
                        record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
                        record.add("new", renderRows(newData));
                        record.add("old", renderRows(oldData));
                        pending.add(record);
                    });
                }
            }

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
                    case "send" -> sendEvent(runtime, step);
                    case "deploy" -> compileDeploy(runtime, config, step.getString("epl", ""));
                    case "undeploy-all" -> runtime.getDeploymentService().undeployAll();
                    default -> throw new IllegalStateException("unsupported op " + op);
                }
                flushPending(pending, records);
            }
            flushPending(pending, records);
        } finally {
            runtime.destroy();
            flushPending(pending, records);
        }
    }

    private static void flushPending(List<JsonObject> pending, List<JsonObject> records) {
        records.addAll(pending);
        pending.clear();
    }

    /**
     * Builds the statement object model exactly as the Java execution does:
     * rstream-only-om constructs the SODA model by hand while
     * rstream-only-compile parses the statement text via eplToModel. Both
     * serialize-copy the model, assert the toEPL round-trip and attach the
     * s0 name annotation before deployment.
     */
    private static EPStatementObjectModel buildModel(String caseName, Configuration config) throws Exception {
        EPStatementObjectModel model;
        if ("rstream-only-om".equals(caseName)) {
            model = new EPStatementObjectModel();
            model.setSelectClause(SelectClause.createWildcard(StreamSelector.RSTREAM_ONLY));
            FromClause fromClause = FromClause.create(FilterStream.create("SupportBean").addView(View.create("length", Expressions.constant(3))));
            model.setFromClause(fromClause);
        } else if ("rstream-only-compile".equals(caseName)) {
            model = EPCompilerProvider.getCompiler().eplToModel(RSTMT, config);
        } else {
            throw new IllegalStateException("no model for case: " + caseName);
        }
        model = SerializableObjectCopier.copyMayFail(model);
        if (!RSTMT.equals(model.toEPL())) {
            throw new IllegalStateException("toEPL round-trip mismatch: " + model.toEPL());
        }
        model.setAnnotations(Collections.singletonList(AnnotationPart.nameAnnotation("s0")));
        return model;
    }

    /**
     * Mirrors RegressionEnvironment.compileDeploy(EPStatementObjectModel):
     * wrap the model in a Module carrying its toEPL text, compile and deploy.
     */
    private static EPDeployment compileDeployModel(EPRuntime runtime, Configuration config, EPStatementObjectModel model) throws Exception {
        Module module = new Module();
        module.getItems().add(new ModuleItem(model));
        module.setModuleText(model.toEPL());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(module, new CompilerArguments(config));
        return runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
    }

    private static EPDeployment compileDeploy(EPRuntime runtime, Configuration config, String epl) throws Exception {
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(config));
        return runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String type = step.getString("eventType", "SupportBean");
        JsonObject payload = step.get("payload").asObject();
        if (!"SupportBean".equals(type)) {
            throw new IllegalStateException("unknown eventType: " + type);
        }
        SupportBean event = new SupportBean();
        JsonValue theStringVal = payload.get("theString");
        if (theStringVal instanceof JsonString) {
            event.setTheString(((JsonString) theStringVal).asString());
        }
        JsonValue intPrimitiveVal = payload.get("intPrimitive");
        if (intPrimitiveVal instanceof JsonNumber) {
            event.setIntPrimitive(((JsonNumber) intPrimitiveVal).asInt());
        }
        runtime.getEventService().sendEventBean(event, "SupportBean");
    }

    private static JsonArray renderRows(EventBean[] events) {
        JsonArray rows = new JsonArray();
        if (events == null) {
            return rows;
        }
        for (EventBean event : events) {
            rows.add(renderRow(event));
        }
        return rows;
    }

    private static JsonObject renderRow(EventBean event) {
        JsonObject fields = new JsonObject();
        for (String prop : new TreeSet<>(java.util.Arrays.asList(PROJECTION))) {
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
        if (value instanceof EventBean) {
            EventBean inner = (EventBean) value;
            JsonObject fields = new JsonObject();
            for (String prop : new TreeSet<>(java.util.Arrays.asList(inner.getEventType().getPropertyNames()))) {
                fields.add(prop, normalize(inner.get(prop)));
            }
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        if (value instanceof SupportBean) {
            // Render the asserted-field projection {theString,intPrimitive}
            // so the trace carries no unasserted engine defaults.
            SupportBean bean = (SupportBean) value;
            JsonObject fields = new JsonObject();
            fields.add("theString", normalize(bean.getTheString()));
            fields.add("intPrimitive", normalize(bean.getIntPrimitive()));
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        return Json.value(String.valueOf(value));
    }

    /**
     * Mirrors the pinned runner's SupportExceptionHandlerFactoryRethrow:
     * statement exceptions rethrow on the sending thread wrapped as
     * "Unexpected exception in statement '&lt;name&gt;': &lt;cause&gt;".
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
