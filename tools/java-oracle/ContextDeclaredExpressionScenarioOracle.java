import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the ContextWDeclaredExpression parity
 * scenario: declared expressions that resolve context.label inside a category
 * context (ords 0/1) and an alias-for expression used as a pattern filter
 * inside both the context-initiation pattern and a statement pattern (ord 2).
 *
 * The runner intentionally uses the public compiler, runtime and statement
 * listener APIs. It does not invoke regression assertions, so the output is
 * an independent trace artifact.
 */
public final class ContextDeclaredExpressionScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";

    private ContextDeclaredExpressionScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: ContextDeclaredExpressionScenarioOracle <scenario.json>");
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]))).asObject();
        validateScenario(scenario);
        JsonArray allSteps = scenario.get("steps").asArray();
        JsonArray records = new JsonArray();
        String[] cases = {"simple", "alias", "wfilter"};
        for (String caseName : cases) {
            if (hasCase(allSteps, caseName)) {
                runCase(allSteps, caseName, records);
            }
        }
        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", scenario.getString("id", ""));
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        root.add("records", records);
        System.out.println(root.toString());
    }

    private static void validateScenario(JsonObject scenario) {
        if (!VERSION.equals(scenario.getString("version", ""))
                || !"context-declared-expression".equals(scenario.getString("id", ""))
                || !JAVA_COMMIT.equals(scenario.getString("javaCommit", ""))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
    }

    private static boolean hasCase(JsonArray steps, String wanted) {
        for (JsonValue value : steps) {
            JsonObject step = value.asObject();
            if ("case".equals(step.getString("op", "")) && wanted.equals(step.getString("case", ""))) {
                return true;
            }
        }
        return false;
    }

    private static void runCase(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType("SupportBean",
                com.espertech.esper.common.internal.support.SupportBean.class);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            Map<String, Integer> sequences = new HashMap<>();
            List<EPCompiled> deployedModules = new ArrayList<>();
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
                    case "deploy":
                        deployStep(runtime, configuration, caseName, step, records,
                                deployedModules, sequences);
                        break;
                    case "deployed": {
                        String label = step.getString("statement", "");
                        String key = label + ":deployed";
                        int seq = sequences.getOrDefault(key, 0) + 1;
                        sequences.put(key, seq);
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "deployed");
                        record.add("statement", label);
                        record.add("sequence", seq);
                        record.add("time", Instant.ofEpochMilli(
                                runtime.getEventService().getCurrentTime()).toString());
                        records.add(record);
                        break;
                    }
                    case "send":
                        sendStep(runtime, step);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        deployedModules.clear();
                        break;
                    default:
                        throw new IllegalArgumentException("unsupported step op " + op);
                }
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static void deployStep(EPRuntime runtime, Configuration config, String caseName,
                                   JsonObject step, JsonArray records,
                                   List<EPCompiled> deployedModules,
                                   Map<String, Integer> sequences) {
        String label = step.getString("statement", "");
        String epl = step.getString("epl", "");
        try {
            CompilerArguments compilerArgs = new CompilerArguments(config);
            for (EPCompiled deployed : deployedModules) {
                compilerArgs.getPath().add(deployed);
            }
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
            deployedModules.add(compiled);
            for (EPStatement stmt : deployment.getStatements()) {
                if ("s0".equals(stmt.getName())) {
                    stmt.addListener(listener(caseName, stmt.getName(), sequences, records, runtime));
                }
            }
        } catch (Exception ex) {
            throw new IllegalStateException("deploy " + label + " failed: " + rootCauseMessage(ex), ex);
        }
    }

    private static String rootCauseMessage(Throwable error) {
        Throwable current = error;
        while (current.getCause() != null) {
            current = current.getCause();
        }
        return String.valueOf(current.getMessage());
    }

    private static void sendStep(EPRuntime runtime, JsonObject step) {
        String type = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("SupportBean".equals(type)) {
            com.espertech.esper.common.internal.support.SupportBean bean =
                    new com.espertech.esper.common.internal.support.SupportBean();
            bean.setTheString(payload.getString("theString", null));
            bean.setIntPrimitive(payload.get("intPrimitive").asInt());
            runtime.getEventService().sendEventBean(bean, type);
            return;
        }
        throw new IllegalArgumentException("unsupported event type " + type);
    }

    private static UpdateListener listener(String caseName, String statementName,
                                           Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime) {
        return (newEvents, oldEvents, statement, rt) -> {
            if ((newEvents == null || newEvents.length == 0)
                    && (oldEvents == null || oldEvents.length == 0)) {
                return;
            }
            String key = statementName + ":listener";
            int seq = sequences.getOrDefault(key, 0) + 1;
            sequences.put(key, seq);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statementName);
            record.add("sequence", seq);
            record.add("time", Instant.ofEpochMilli(
                    runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = results(newEvents);
            JsonArray oldArray = results(oldEvents);
            if (newArray.size() > 0) {
                record.add("new", newArray);
            }
            if (oldArray.size() > 0) {
                record.add("old", oldArray);
            }
            records.add(record);
        };
    }

    private static JsonArray results(EventBean[] events) {
        JsonArray output = new JsonArray();
        if (events == null) {
            return output;
        }
        for (EventBean event : events) {
            JsonObject fields = new JsonObject();
            String[] names = event.getEventType().getPropertyNames().clone();
            java.util.Arrays.sort(names);
            for (String name : names) {
                fields.add(name, normalize(event.get(name)));
            }
            output.add(new JsonObject().add("kind", "row").add("fields", fields));
        }
        return output;
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value instanceof EventBean) {
            EventBean event = (EventBean) value;
            JsonObject fields = new JsonObject();
            String[] names = event.getEventType().getPropertyNames().clone();
            java.util.Arrays.sort(names);
            for (String name : names) {
                fields.add(name, normalize(event.get(name)));
            }
            return new JsonObject().add("kind", "row").add("fields", fields);
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
        return Json.value(String.valueOf(value));
    }
}
