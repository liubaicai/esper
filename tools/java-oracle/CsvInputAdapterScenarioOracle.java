import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;
import com.espertech.esperio.csv.AdapterInputSource;
import com.espertech.esperio.csv.CSVInputAdapter;
import com.espertech.esperio.csv.CSVInputAdapterSpec;

import java.io.StringReader;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

/**
 * Direct Esper 9.0.0 oracle for the CSV input adapter parity scenario:
 * CSVInputAdapter (esperio-csv, synchronous DirectSender mode with no
 * events-per-second or timestamp pacing) replays the scenario's embedded CSV
 * text into a predefined map event type while a {@code select *} statement
 * listener records every row. The Go csv connector reproduces the same
 * normalized trace from the same scenario file.
 */
public final class CsvInputAdapterScenarioOracle {
    private static final String VERSION = "esper-parity/v1";

    private CsvInputAdapterScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: CsvInputAdapterScenarioOracle <scenario.json>");
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]))).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        String scenarioID = scenario.getString("id", "");
        if (scenarioID.isBlank()) {
            throw new IllegalArgumentException("scenario id is required");
        }
        JsonArray steps = scenario.get("steps").asArray();
        if (steps == null || steps.size() == 0) {
            throw new IllegalArgumentException("scenario steps are required");
        }
        JsonObject trace = new JsonObject()
                .add("version", VERSION)
                .add("id", scenarioID);
        JsonArray records = new JsonArray();
        trace.add("records", records);

        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            if ("case".equals(step.getString("op", "")) && "csv".equals(step.getString("case", ""))) {
                runCase(step, records);
            }
        }
        System.out.println(trace);
    }

    private static void runCase(JsonObject step, JsonArray records) throws Exception {
        String csv = step.getString("source", "");
        if (csv.isEmpty()) {
            throw new IllegalArgumentException("csv case requires a source");
        }
        JsonArray orderArray = step.get("propertyOrder").asArray();
        JsonObject typeObject = step.get("propertyTypes").asObject();
        if (orderArray == null || orderArray.size() == 0) {
            throw new IllegalArgumentException("csv case requires propertyOrder");
        }
        if (typeObject == null || typeObject.size() == 0) {
            throw new IllegalArgumentException("csv case requires propertyTypes");
        }
        List<String> order = new ArrayList<String>();
        for (int i = 0; i < orderArray.size(); i++) {
            order.add(orderArray.get(i).asString());
        }

        StringBuilder epl = new StringBuilder("@public @buseventtype create map schema MyCsvEvent(");
        boolean first = true;
        for (String name : order) {
            if (!first) {
                epl.append(", ");
            }
            first = false;
            String type = typeObject.getString(name, null);
            if (type == null) {
                throw new IllegalArgumentException("csv case propertyTypes misses " + name);
            }
            epl.append(name).append(' ').append(eplType(type));
        }
        epl.append(");\n@name('s0') select * from MyCsvEvent;");

        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-csv-input-adapter-csv", configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl.toString(), new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId("parity-csv-input-adapter-csv"));
            EPStatement statement = null;
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    statement = candidate;
                    break;
                }
            }
            if (statement == null) {
                throw new IllegalStateException("statement s0 was not deployed");
            }
            TraceWriter writer = new TraceWriter(records, "csv", statement, runtime);
            statement.addListener(writer);

            CSVInputAdapterSpec spec = new CSVInputAdapterSpec(
                    new AdapterInputSource(new StringReader(csv)), "MyCsvEvent");
            spec.setPropertyOrder(order.toArray(new String[0]));
            CSVInputAdapter adapter = new CSVInputAdapter(runtime, spec);
            adapter.start();

            // Every property of the declared map schema must have been emitted
            // by the adapter so the listener trace is complete.
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static String eplType(String type) {
        if ("string".equals(type)) {
            return "string";
        }
        if ("int".equals(type)) {
            return "int";
        }
        if ("long".equals(type)) {
            return "long";
        }
        if ("double".equals(type)) {
            return "double";
        }
        if ("boolean".equals(type)) {
            return "boolean";
        }
        throw new IllegalArgumentException("unsupported csv property type " + type);
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
}
