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
import java.util.Collections;
import java.util.Comparator;
import java.util.HashMap;
import java.util.Iterator;
import java.util.List;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for the selected InfraNWTableSubquery executions.
 * SceneOne covers correlated scalar reads after infrastructure preload; SelfCheck
 * covers anti-subquery insertion, deletion and reinsertion for a named window
 * and table.
 */
public final class InfraNWTableSubqueryScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "infra-nwtable-subquery";
    private static final String[] CASES = {
            "scene-one-named-window",
            "scene-one-table",
            "self-check-named-window",
            "self-check-table"
    };

    private InfraNWTableSubqueryScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException("usage: InfraNWTableSubqueryScenarioOracle <scenario.json>");
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

        JsonObject trace = new JsonObject()
                .add("version", VERSION)
                .add("id", scenarioID);
        JsonArray records = new JsonArray();
        trace.add("records", records);
        for (String caseName : CASES) {
            runCase(steps, caseName, records);
        }
        System.out.println(trace);
    }

    private static void validateCases(JsonArray steps) {
        int caseCount = 0;
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            if (!"case".equals(step.getString("op", ""))) {
                continue;
            }
            caseCount++;
            String caseName = step.getString("case", "");
            if (!isCase(caseName)) {
                throw new IllegalArgumentException("unsupported case " + caseName);
            }
        }
        if (caseCount != CASES.length) {
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

    private static boolean isCase(String caseName) {
        for (String candidate : CASES) {
            if (candidate.equals(caseName)) {
                return true;
            }
        }
        return false;
    }

    private static void runCase(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        if (caseName.startsWith("scene-one-")) {
            runSceneOne(allSteps, caseName, records);
        } else {
            runSelfCheck(allSteps, caseName, records);
        }
    }

    private static void runSceneOne(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        boolean namedWindow = "scene-one-named-window".equals(caseName);
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        Map<String, Object> s0Type = new HashMap<>();
        s0Type.put("id", Integer.class);
        s0Type.put("p00", String.class);
        configuration.getCommon().addEventType("SupportBean", beanType);
        configuration.getCommon().addEventType("SupportBean_S0", s0Type);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("infra-nwtable-subquery-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String create = namedWindow
                    ? "@Name('Create') @public create window MyInfra.win:keepall() as SupportBean"
                    : "@Name('Create') @public create table MyInfra(theString string primary key, intPrimitive int)";
            deploy(runtime, create, "infra-nwtable-subquery-" + caseName + "-create");
            String insert = "@Name('Insert') insert into MyInfra select theString, intPrimitive from SupportBean";
            deploy(runtime, insert, "infra-nwtable-subquery-" + caseName + "-insert");

            int caseIndex = findCase(allSteps, caseName);
            int seedCount = 0;
            int triggerCount = 0;
            EPStatement subquery = null;
            TraceWriter writer = null;
            for (int i = caseIndex + 1; i < allSteps.size(); i++) {
                JsonObject step = allSteps.get(i).asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    break;
                }
                if (!"send".equals(op)) {
                    throw new IllegalArgumentException("unsupported SceneOne op " + op);
                }
                String eventType = step.getString("eventType", "");
                if ("SupportBean".equals(eventType)) {
                    if (subquery != null) {
                        throw new IllegalArgumentException("SceneOne seed event follows Subq deployment");
                    }
                    send(runtime, step);
                    seedCount++;
                } else if ("SupportBean_S0".equals(eventType)) {
                    if (subquery == null) {
                        if (seedCount != 3) {
                            throw new IllegalArgumentException("SceneOne requires three seed events before Subq");
                        }
                        String epl = "@Name('Subq') select (select intPrimitive from MyInfra where theString = s0.p00) as c0 " +
                                "from SupportBean_S0 as s0";
                        EPDeployment deployment = deploy(runtime, epl,
                                "infra-nwtable-subquery-" + caseName + "-subq");
                        subquery = findStatement(deployment, "Subq");
                        writer = new TraceWriter(records, caseName, subquery, runtime);
                        subquery.addListener(writer);
                    }
                    send(runtime, step);
                    triggerCount++;
                } else {
                    throw new IllegalArgumentException("unsupported SceneOne event type " + eventType);
                }
            }
            if (seedCount != 3 || triggerCount != 2 || subquery == null || writer == null) {
                throw new IllegalArgumentException("SceneOne requires three seeds and two triggers");
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static void runSelfCheck(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        boolean namedWindow = "self-check-named-window".equals(caseName);
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intBoxed", Integer.class);
        Map<String, Object> aType = new HashMap<>();
        aType.put("id", String.class);
        configuration.getCommon().addEventType("SupportBean", beanType);
        configuration.getCommon().addEventType("SupportBean_A", aType);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("infra-nwtable-subquery-" + caseName, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String create = namedWindow
                    ? "@name('create') @public create window MyInfraSSS#keepall as select theString as key, intBoxed as value from SupportBean"
                    : "@name('create') @public create table MyInfraSSS (key string primary key, value int)";
            EPDeployment createDeployment = deploy(runtime, create,
                    "infra-nwtable-subquery-" + caseName + "-create");
            String insert = "insert into MyInfraSSS select theString as key, intBoxed as value from SupportBean as s0 " +
                    "where not exists (select * from MyInfraSSS as win where win.key = s0.theString)";
            deploy(runtime, insert, "infra-nwtable-subquery-" + caseName + "-insert");
            String delete = "@name('delete') on SupportBean_A delete from MyInfraSSS where key = id";
            deploy(runtime, delete, "infra-nwtable-subquery-" + caseName + "-delete");
            EPStatement createStatement = findStatement(createDeployment, "create");
            createStatement.addListener(new TraceWriter(records, caseName, createStatement, runtime));

            int caseIndex = findCase(allSteps, caseName);
            int beanSends = 0;
            int deleteSends = 0;
            int snapshots = 0;
            for (int i = caseIndex + 1; i < allSteps.size(); i++) {
                JsonObject step = allSteps.get(i).asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    break;
                }
                if ("send".equals(op)) {
                    String eventType = step.getString("eventType", "");
                    if ("SupportBean".equals(eventType)) {
                        beanSends++;
                    } else if ("SupportBean_A".equals(eventType)) {
                        deleteSends++;
                    } else {
                        throw new IllegalArgumentException("unsupported SelfCheck event type " + eventType);
                    }
                    send(runtime, step);
                } else if ("snapshot".equals(op)) {
                    if (!"create".equals(step.getString("statement", ""))) {
                        throw new IllegalArgumentException("unsupported SelfCheck snapshot statement");
                    }
                    snapshot(runtime, caseName, createStatement, records);
                    snapshots++;
                } else {
                    throw new IllegalArgumentException("unsupported SelfCheck op " + op);
                }
            }
            if (beanSends != 5 || deleteSends != 1 || snapshots != 6) {
                throw new IllegalArgumentException("SelfCheck requires five beans, one delete and six snapshots");
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static EPDeployment deploy(EPRuntime runtime, String epl, String deploymentID) throws Exception {
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                new CompilerArguments(runtime.getRuntimePath()));
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
            if (payload.get("intPrimitive") != null) {
                event.put("intPrimitive", payload.get("intPrimitive").asInt());
            }
            if (payload.get("intBoxed") != null && !payload.get("intBoxed").isNull()) {
                event.put("intBoxed", payload.get("intBoxed").asInt());
            }
        } else if ("SupportBean_S0".equals(eventType)) {
            event.put("id", payload.get("id").asInt());
            event.put("p00", payload.getString("p00", null));
        } else if ("SupportBean_A".equals(eventType)) {
            event.put("id", payload.getString("id", null));
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
        if ("self-check-table".equals(caseName)) {
            Collections.sort(events, Comparator.comparing(event -> String.valueOf(event.get("key"))));
        }
        JsonArray rows = rows(events);
        JsonObject record = new JsonObject()
                .add("case", caseName)
                .add("operation", "snapshot")
                .add("statement", statement.getName())
                .add("sequence", 0)
                .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
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
            java.util.Arrays.sort(names);
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
            return InfraNWTableSubqueryScenarioOracle.rows(list);
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
