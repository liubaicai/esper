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
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;
import org.apache.avro.Schema;
import org.apache.avro.generic.GenericData;

import java.io.FileReader;
import java.io.Serializable;
import java.time.Instant;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.Map;

/** Direct Esper 9.0.0 oracle for the ExprCoreTypeOfFragment replay slice. */
public final class ExprCoreTypeNameScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "expr-core-type-name";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String SOURCE_FILE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreTypeOf.java";
    private static final String EXECUTION = "ExprCoreTypeOfFragment";
    private static final String RUNTIME_ID = "java-runtime-eeeacbe2c7669e1e1207";
    private static final String[] CASES = {
            "type-name-fragment-object-array",
            "type-name-fragment-map",
            "type-name-fragment-avro",
            "type-name-fragment-json",
            "type-name-fragment-json-provided",
            "type-name-fragment-default"
    };

    private ExprCoreTypeNameScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        String scenarioFile = args.length > 0 ? args[0] : "testdata/parity/expr-core-type-name.json";
        JsonObject scenario = Json.parse(new FileReader(scenarioFile)).asObject();
        validateScenarioMetadata(scenario);
        JsonValue stepsValue = scenario.get("steps");
        if (stepsValue == null || !stepsValue.isArray()) {
            throw new IllegalArgumentException("scenario steps are required");
        }
        JsonArray steps = stepsValue.asArray();
        validateScenario(steps);

        JsonObject trace = new JsonObject()
                .add("version", VERSION)
                .add("id", SCENARIO_ID)
                .add("records", new JsonArray());
        JsonArray records = trace.get("records").asArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(steps, caseIndex, records);
        }
        System.out.println(trace);
    }

    private static void validateScenarioMetadata(JsonObject scenario) {
        requireString(scenario, "version", VERSION);
        requireString(scenario, "id", SCENARIO_ID);
        requireFields(scenario, "scenario", "version", "id", "steps");
    }

    private static void validateScenario(JsonArray steps) {
        int offset = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            if (offset >= steps.size()) {
                throw new IllegalArgumentException("scenario is missing case " + CASES[caseIndex]);
            }
            JsonObject marker = steps.get(offset++).asObject();
            if (!"case".equals(marker.getString("op", "")) ||
                    !CASES[caseIndex].equals(marker.getString("case", ""))) {
                throw new IllegalArgumentException("scenario case order mismatch at " + caseIndex);
            }
            requireFields(marker, "case", "op", "case");
            for (int sendIndex = 0; sendIndex < 3; sendIndex++) {
                if (offset >= steps.size()) {
                    throw new IllegalArgumentException("scenario is missing send for " + CASES[caseIndex]);
                }
                JsonObject step = steps.get(offset++).asObject();
                if (!"send".equals(step.getString("op", "")) ||
                        !"MySchema".equals(step.getString("eventType", ""))) {
                    throw new IllegalArgumentException("scenario send shape mismatch for " + CASES[caseIndex]);
                }
                requireFields(step, "send", "op", "eventType", "payload");
                JsonValue payloadValue = step.get("payload");
                if (payloadValue == null || !payloadValue.isObject()) {
                    throw new IllegalArgumentException("scenario payload is required for " + CASES[caseIndex]);
                }
                validatePayload(payloadValue.asObject(), caseIndex, sendIndex);
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario has trailing steps");
        }
    }

    private static void validatePayload(JsonObject payload, int caseIndex, int sendIndex) {
        requireFields(payload, "payload", "shape");
        String expected = sendIndex == 0 ? "empty" : sendIndex == 1 ? "inside" : "insidearr";
        requireString(payload, "shape", expected);
    }

    private static void requireFields(JsonObject object, String kind, String... names) {
        if (object.names().size() != names.length) {
            throw new IllegalArgumentException(kind + " has unsupported metadata");
        }
        for (String name : names) {
            if (!object.names().contains(name)) {
                throw new IllegalArgumentException(kind + " is missing field " + name);
            }
        }
    }

    private static void requireString(JsonObject object, String name, String expected) {
        JsonValue value = object.get(name);
        if (value == null || !value.isString() || !expected.equals(value.asString())) {
            throw new IllegalArgumentException("metadata mismatch for " + name);
        }
    }

    private static void requireSingleStringArray(JsonObject object, String name, String expected) {
        JsonValue value = object.get(name);
        if (value == null || !value.isArray() || value.asArray().size() != 1 ||
                !value.asArray().get(0).isString() || !expected.equals(value.asArray().get(0).asString())) {
            throw new IllegalArgumentException("metadata array mismatch for " + name);
        }
    }

    private static void runCase(JsonArray allSteps, int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().getEventMeta().getAvroSettings().setEnableAvro(true);
        String caseName = CASES[caseIndex];
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-expr-core-type-name-" + caseName, configuration);
        try {
            ((EPRuntimeSPI) runtime).initialize(0L);
            CompilerArguments compilerArguments = new CompilerArguments(configuration);
            compilerArguments.getPath().add(runtime.getRuntimePath());
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                    eplFor(caseIndex), compilerArguments);
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            EPStatement statement = findStatement(deployment, "s0");
            statement.addListener(new TraceWriter(records, caseName, statement, runtime));
            replayCase(allSteps, caseIndex, runtime, deployment);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static EPStatement findStatement(EPDeployment deployment, String name) {
        for (EPStatement candidate : deployment.getStatements()) {
            if (name.equals(candidate.getName())) {
                return candidate;
            }
        }
        throw new IllegalStateException("statement " + name + " was not deployed");
    }

    private static String eplFor(int caseIndex) {
        return annotation(caseIndex, MyLocalJsonProvidedInnerSchema.class) +
                " @public create schema InnerSchema as (key string);\n" +
                annotation(caseIndex, MyLocalJsonProvidedMySchema.class) +
                " @buseventtype @public create schema MySchema as (inside InnerSchema, insidearr InnerSchema[]);\n" +
                annotation(caseIndex, MyLocalJsonProvidedOut.class) +
                " @name('s0') select typeof(s0.inside) as t0, typeof(s0.insidearr) as t1 from MySchema as s0;\n";
    }

    private static String annotation(int caseIndex, Class<?> jsonProvidedClass) {
        switch (caseIndex) {
            case 0:
                return "@EventRepresentation('objectarray')";
            case 1:
            case 5:
                return "";
            case 2:
                return "@EventRepresentation('avro')";
            case 3:
                return "@EventRepresentation('json')";
            case 4:
                return "@JsonSchema(className='" + jsonProvidedClass.getName() + "') @EventRepresentation('json')";
            default:
                throw new IllegalArgumentException("unsupported representation index " + caseIndex);
        }
    }

    private static void replayCase(JsonArray allSteps, int caseIndex, EPRuntime runtime, EPDeployment deployment) {
        String caseName = CASES[caseIndex];
        boolean active = false;
        for (JsonValue value : allSteps) {
            JsonObject step = value.asObject();
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                active = caseName.equals(step.getString("case", ""));
                continue;
            }
            if (!active || !"send".equals(op)) {
                continue;
            }
            String shape = step.get("payload").asObject().getString("shape", "");
            send(runtime, caseIndex, shape, deployment);
        }
    }

    private static void send(EPRuntime runtime, int caseIndex, String shape, EPDeployment deployment) {
        if (caseIndex == 0) {
            if ("empty".equals(shape)) {
                runtime.getEventService().sendEventObjectArray(new Object[2], "MySchema");
            } else if ("inside".equals(shape)) {
                runtime.getEventService().sendEventObjectArray(new Object[]{new Object[2], null}, "MySchema");
            } else {
                runtime.getEventService().sendEventObjectArray(new Object[]{null, new Object[2][]}, "MySchema");
            }
            return;
        }
        if (caseIndex == 1 || caseIndex == 5) {
            if ("empty".equals(shape)) {
                runtime.getEventService().sendEventMap(new HashMap<>(), "MySchema");
            } else if ("inside".equals(shape)) {
                Map<String, Object> event = new HashMap<>();
                event.put("inside", new HashMap<String, Object>());
                runtime.getEventService().sendEventMap(event, "MySchema");
            } else {
                Map<String, Object> event = new HashMap<>();
                event.put("insidearr", new Map[0]);
                runtime.getEventService().sendEventMap(event, "MySchema");
            }
            return;
        }
        if (caseIndex == 2) {
            runtime.getEventService().sendEventAvro(buildAvro(runtime, deployment), "MySchema");
            return;
        }
        if (caseIndex == 3 || caseIndex == 4) {
            if ("empty".equals(shape)) {
                runtime.getEventService().sendEventJson("{}", "MySchema");
            } else if ("inside".equals(shape)) {
                JsonObject event = new JsonObject().add("inside", new JsonObject());
                runtime.getEventService().sendEventJson(event.toString(), "MySchema");
            } else {
                JsonObject event = new JsonObject().add("insidearr", new JsonArray().add(new JsonObject()));
                runtime.getEventService().sendEventJson(event.toString(), "MySchema");
            }
            return;
        }
        throw new IllegalArgumentException("unsupported representation index " + caseIndex);
    }

    private static GenericData.Record buildAvro(EPRuntime runtime, EPDeployment deployment) {
        String deploymentId = deployment.getDeploymentId();
        EventType myEventType = runtime.getEventTypeService().getEventType(deploymentId, "MySchema");
        EventType innerEventType = runtime.getEventTypeService().getEventType(deploymentId, "InnerSchema");
        Schema mySchema = SupportAvroUtil.getAvroSchema(myEventType);
        Schema innerSchema = SupportAvroUtil.getAvroSchema(innerEventType);
        GenericData.Record event = new GenericData.Record(mySchema);
        event.put("insidearr", Collections.emptyList());
        GenericData.Record inner = new GenericData.Record(innerSchema);
        inner.put("key", "k");
        event.put("inside", inner);
        return event;
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored, EPRuntime ignoredRuntime) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = results(newEvents);
            if (newArray.size() > 0) {
                record.add("new", newArray);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", results(oldEvents));
            }
            records.add(record);
        }

        private JsonArray results(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                output.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return output;
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            return Json.value(String.valueOf(value));
        }
    }

    public static class MyLocalJsonProvidedInnerSchema implements Serializable {
        public String key;
    }

    public static class MyLocalJsonProvidedMySchema implements Serializable {
        public MyLocalJsonProvidedInnerSchema inside;
        public MyLocalJsonProvidedInnerSchema[] insidearr;
    }

    public static class MyLocalJsonProvidedOut implements Serializable {
        public String t0;
        public String t1;
    }
}
