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

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.Comparator;
import java.util.HashMap;
import java.util.Iterator;
import java.util.List;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for InfraNWTableSubquery's delete/replace and
 * uncorrelated aggregation executions.
 */
public final class InfraNWTableSubqueryDeleteAggregateScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "infra-nwtable-subquery-delete-aggregate";
    private static final String[] CASES = {
            "delete-replace-named-window",
            "delete-replace-table",
            "aggregate-named-window",
            "aggregate-table"
    };

    private InfraNWTableSubqueryDeleteAggregateScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: InfraNWTableSubqueryDeleteAggregateScenarioOracle <scenario.json>");
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        String scenarioID = scenario.getString("id", "");
        if (!SCENARIO_ID.equals(scenarioID)) {
            throw new IllegalArgumentException("unexpected scenario id " + scenarioID);
        }
        JsonArray steps = scenario.get("steps").asArray();
        if (steps == null || steps.size() == 0) {
            throw new IllegalArgumentException("scenario steps are required");
        }
        validateCases(steps);

        JsonObject trace = new JsonObject().add("version", VERSION).add("id", scenarioID);
        JsonArray records = new JsonArray();
        trace.add("records", records);
        for (String caseName : CASES) {
            runCase(steps, caseName, records);
        }
        System.out.println(trace);
    }

    private static void validateCases(JsonArray steps) {
        if (steps.size() == 0) {
            throw new IllegalArgumentException("scenario steps are required");
        }
        int count = 0;
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            if (!"case".equals(step.getString("op", ""))) {
                continue;
            }
            count++;
            if (!isCase(step.getString("case", ""))) {
                throw new IllegalArgumentException("unsupported case " + step.getString("case", ""));
            }
        }
        if (count != CASES.length) {
            throw new IllegalArgumentException("expected exactly four cases");
        }
        for (String expected : CASES) {
            int matches = 0;
            for (int i = 0; i < steps.size(); i++) {
                JsonObject step = steps.get(i).asObject();
                if ("case".equals(step.getString("op", "")) && expected.equals(step.getString("case", ""))) {
                    matches++;
                }
            }
            if (matches != 1) {
                throw new IllegalArgumentException("expected one case " + expected);
            }
        }
    }

    private static boolean isCase(String name) {
        for (String candidate : CASES) {
            if (candidate.equals(name)) {
                return true;
            }
        }
        return false;
    }

    private static void runCase(JsonArray steps, String caseName, JsonArray records) throws Exception {
        if (caseName.startsWith("delete-replace-")) {
            runDeleteReplace(steps, caseName, records);
        } else {
            runAggregation(steps, caseName, records);
        }
    }

    private static Configuration configuration() {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        return configuration;
    }

    private static void runDeleteReplace(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        boolean namedWindow = "delete-replace-named-window".equals(caseName);
        Configuration configuration = configuration();
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intBoxed", Integer.class);
        configuration.getCommon().addEventType("SupportBean", beanType);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("infra-nwtable-subquery-delete-aggregate-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String create = namedWindow
                    ? "@name('create') @public create window MyInfra#keepall as select theString as key, intBoxed as value from SupportBean"
                    : "@name('create') @public create table MyInfra(key string primary key, value int primary key)";
            EPDeployment createDeployment = deploy(runtime, create, caseName + "-create");
            EPStatement createStatement = findStatement(createDeployment, "create");
            createStatement.addListener(new TraceWriter(records, caseName, createStatement, runtime));
            deploy(runtime, "@name('delete') on SupportBean delete from MyInfra where key = theString", caseName + "-delete");
            deploy(runtime, "insert into MyInfra select theString as key, intBoxed as value from SupportBean as s0", caseName + "-insert");

            int caseIndex = findCase(allSteps, caseName);
            int sends = 0;
            int snapshots = 0;
            for (int i = caseIndex + 1; i < allSteps.size(); i++) {
                JsonObject step = allSteps.get(i).asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    break;
                }
                if ("send".equals(op) && "SupportBean".equals(step.getString("eventType", ""))) {
                    send(runtime, step);
                    sends++;
                } else if ("snapshot".equals(op) && "create".equals(step.getString("statement", ""))) {
                    snapshot(runtime, caseName, createStatement, records);
                    snapshots++;
                } else {
                    throw new IllegalArgumentException("unsupported delete/replace step " + op);
                }
            }
            if (sends != 3 || snapshots != 3) {
                throw new IllegalArgumentException("delete/replace requires three sends and three snapshots");
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static void runAggregation(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        boolean namedWindow = "aggregate-named-window".equals(caseName);
        Configuration configuration = configuration();
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("longPrimitive", Long.class);
        Map<String, Object> marketType = new HashMap<>();
        marketType.put("symbol", String.class);
        configuration.getCommon().addEventType("SupportBean", beanType);
        configuration.getCommon().addEventType("SupportMarketDataBean", marketType);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("infra-nwtable-subquery-delete-aggregate-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String create = namedWindow
                    ? "@name('create') @public create window MyInfraUCS#keepall as select theString as a, longPrimitive as b from SupportBean"
                    : "@name('create') @public create table MyInfraUCS(a string primary key, b long)";
            deploy(runtime, create, caseName + "-create");
            deploy(runtime, "insert into MyInfraUCS select theString as a, longPrimitive as b from SupportBean", caseName + "-insert");
            EPDeployment firstDeployment = deploy(runtime,
                    "@name('selectOne') select irstream (select sum(b) from MyInfraUCS) as value, symbol from SupportMarketDataBean",
                    caseName + "-select-one");
            EPStatement selectOne = findStatement(firstDeployment, "selectOne");
            selectOne.addListener(new TraceWriter(records, caseName, selectOne, runtime));

            int caseIndex = findCase(allSteps, caseName);
            int beanSends = 0;
            int marketSends = 0;
            boolean secondDeployed = false;
            for (int i = caseIndex + 1; i < allSteps.size(); i++) {
                JsonObject step = allSteps.get(i).asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    break;
                }
                if ("deploy".equals(op)) {
                    if (!"selectTwo".equals(step.getString("statement", "")) || secondDeployed) {
                        throw new IllegalArgumentException("invalid selectTwo deployment");
                    }
                    EPDeployment secondDeployment = deploy(runtime,
                            "@name('selectTwo') select irstream (select sum(b) from MyInfraUCS) as value, symbol from SupportMarketDataBean",
                            caseName + "-select-two");
                    EPStatement selectTwo = findStatement(secondDeployment, "selectTwo");
                    selectTwo.addListener(new TraceWriter(records, caseName, selectTwo, runtime));
                    secondDeployed = true;
                } else if ("send".equals(op)) {
                    String eventType = step.getString("eventType", "");
                    send(runtime, step);
                    if ("SupportBean".equals(eventType)) {
                        beanSends++;
                    } else if ("SupportMarketDataBean".equals(eventType)) {
                        marketSends++;
                    } else {
                        throw new IllegalArgumentException("unsupported aggregate event type " + eventType);
                    }
                } else {
                    throw new IllegalArgumentException("unsupported aggregate step " + op);
                }
            }
            if (beanSends != 3 || marketSends != 4 || !secondDeployed) {
                throw new IllegalArgumentException("aggregation requires three beans, four market events and selectTwo");
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static EPDeployment deploy(EPRuntime runtime, String epl, String deploymentID) throws Exception {
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(runtime.getRuntimePath()));
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                new DeploymentOptions().setDeploymentId(deploymentID));
        if (deployment == null) {
            throw new IllegalStateException("deployment was not created for " + deploymentID);
        }
        return deployment;
    }

    private static EPStatement findStatement(EPDeployment deployment, String name) {
        for (EPStatement statement : deployment.getStatements()) {
            if (name.equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("statement " + name + " was not deployed");
    }

    private static int findCase(JsonArray steps, String caseName) {
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            if ("case".equals(step.getString("op", "")) && caseName.equals(step.getString("case", ""))) {
                return i;
            }
        }
        throw new IllegalArgumentException("case not found " + caseName);
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        if ("SupportBean".equals(eventType)) {
            event.put("theString", payload.getString("theString", null));
            if (payload.get("intBoxed") != null && !payload.get("intBoxed").isNull()) {
                event.put("intBoxed", payload.get("intBoxed").asInt());
            }
            if (payload.get("longPrimitive") != null && !payload.get("longPrimitive").isNull()) {
                event.put("longPrimitive", payload.get("longPrimitive").asLong());
            }
        } else if ("SupportMarketDataBean".equals(eventType)) {
            event.put("symbol", payload.getString("symbol", null));
        } else {
            throw new IllegalArgumentException("unsupported event type " + eventType);
        }
        runtime.getEventService().sendEventMap(event, eventType);
    }

    private static void snapshot(EPRuntime runtime, String caseName, EPStatement statement, JsonArray records) {
        List<EventBean> events = new ArrayList<>();
        Iterator<EventBean> iterator = statement.iterator();
        while (iterator.hasNext()) {
            events.add(iterator.next());
        }
        if ("delete-replace-table".equals(caseName)) {
            Collections.sort(events, Comparator.comparing(event -> String.valueOf(event.get("key"))));
        }
        JsonObject record = new JsonObject()
                .add("case", caseName)
                .add("operation", "snapshot")
                .add("statement", statement.getName())
                .add("sequence", 0)
                .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
        JsonArray rows = rows(events);
        if (rows.size() > 0) {
            record.add("new", rows);
        }
        records.add(record);
    }

    private static JsonArray rows(List<EventBean> events) {
        JsonArray output = new JsonArray();
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
            JsonArray newArray = rows(newEvents);
            JsonArray oldArray = rows(oldEvents);
            if (newArray.size() > 0) {
                record.add("new", newArray);
            }
            if (oldArray.size() > 0) {
                record.add("old", oldArray);
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            List<EventBean> list = new ArrayList<>();
            if (events != null) {
                Collections.addAll(list, events);
            }
            return InfraNWTableSubqueryDeleteAggregateScenarioOracle.rows(list);
        }
    }

    private static JsonValue normalize(Object value) {
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
