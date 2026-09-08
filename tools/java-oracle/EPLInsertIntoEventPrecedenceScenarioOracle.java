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
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

/**
 * Java oracle for EPLInsertIntoEventPrecedence constant-precedence scenarios.
 *
 * Covers the ConstantInsertInto, ConstantInfraMergeInsertInto (table),
 * ConstantInfraMergeInsertInto (named window), ConstantOnSplit,
 * NonConstInsertIntoContainedEvent, and ConstantInsertIntoOutputRate
 * runtimes by replaying deterministic event sequences and recording the
 * observable output order.
 *
 * The LvlA/LvlB/LvlC event classes and the computeEventPrecedence function
 * are oracle-local mirrors of the suite's nested classes and static method:
 * the fixed run script classpath does not include regression-lib, so the
 * EPL references these mirrors instead of the suite FQNs. Getters, fields,
 * and the function contract (param must be the HashMap underlying of the
 * output event; returns the constant) match the suite byte-for-byte.
 */
public class EPLInsertIntoEventPrecedenceScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLInsertIntoEventPrecedenceScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExecution().setPrecedenceEnabled(true);

        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-precedence-" + caseName, configuration);
        runtime.initialize();

        try {
            String epl = eplFor(caseName);
            CompilerArguments compilerArguments = new CompilerArguments(configuration);
            compilerArguments.getPath().add(runtime.getRuntimePath());
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArguments);
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId("parity-precedence-" + caseName));

            // Set up consumer listener for merge cases (c0) and insert-into
            // cases that have a named window consumer (s0).
            TraceWriter writer = null;
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName()) || "c0".equals(candidate.getName())) {
                    writer = new TraceWriter(records, caseName, candidate);
                    break;
                }
            }

            replayCase(allSteps, caseName, runtime);

            // For the constant-insertinto case, also query the named window
            // directly to capture the window iteration order (which is the
            // primary observable for precedence).
            if ("constant-insertinto".equals(caseName)) {
                captureWindowOrder(runtime, caseName, records, deployment, "window");
            }

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static void captureWindowOrder(EPRuntime runtime, String caseName, List<JsonObject> records, EPDeployment deployment, String statementName) {
        // Use the already-deployed consumer statement to iterate the named window.
        // The consumer statement "select * from MyWindow" is iterable and
        // returns the window contents in insertion order.
        EPStatement stmt = null;
        for (EPStatement candidate : deployment.getStatements()) {
            if (statementName.equals(candidate.getName())) {
                stmt = candidate;
                break;
            }
        }
        if (stmt == null) {
            return;
        }
        int seq = 0;
        for (var it = stmt.iterator(); it.hasNext(); ) {
            EventBean event = it.next();
            seq++;
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("sequence", 1000 + seq);
            record.add("operation", "window-iteration");
            String[] names = event.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            JsonObject fields = new JsonObject();
            for (String name : names) {
                Object value = event.get(name);
                fields.add(name, TraceWriter.normalize(value));
            }
            JsonArray newArr = new JsonArray();
            JsonObject evt = new JsonObject();
            evt.add("fields", fields);
            newArr.add(evt);
            record.add("new", newArr);
            records.add(record);
        }
    }

    private static String eplFor(String caseName) {
        StringBuilder sb = new StringBuilder();
        switch (caseName) {
            case "constant-insertinto":
                // ConstantInsertInto: multiple insert-into with constant precedence
                // into a named window. The window iteration order follows
                // descending precedence, with no-precedence events last.
                // The @name('window') on create window makes it iterable.
                sb.append("@name('window') create window MyWindow#keepall as (id int);\n");
                sb.append("insert into MyWindow event-precedence(4) select 4 as id from SupportBean;\n");
                sb.append("insert into MyWindow event-precedence(2) select 2 as id from SupportBean;\n");
                sb.append("insert into MyWindow select 0 as id from SupportBean;\n");
                sb.append("insert into MyWindow event-precedence(5) select 5 as id from SupportBean;\n");
                sb.append("insert into MyWindow event-precedence(1) select 1 as id from SupportBean;\n");
                sb.append("insert into MyWindow event-precedence(3) select 3 as id from SupportBean;\n");
                sb.append("@name('s0') select * from MyWindow;\n");
                break;
            case "constant-merge-table":
                // ConstantInfraMergeInsertInto(namedWindow=false)
                sb.append("create table InfraMerge(mergeid string primary key);\n");
                sb.append("create schema WindowOut(outid string);\n");
                sb.append("on SupportBean sb merge InfraMerge mw where sb.theString = mw.mergeid ");
                sb.append("when not matched ");
                sb.append("then insert into WindowOut event-precedence(0) select 'a' as outid ");
                sb.append("then insert into WindowOut select 'b' as outid ");
                sb.append("then insert into WindowOut event-precedence(1) select 'c' as outid;\n");
                sb.append("@name('c0') select * from WindowOut;\n");
                break;
            case "constant-merge-window":
                // ConstantInfraMergeInsertInto(namedWindow=true)
                sb.append("create window InfraMerge#keepall as (mergeid string);\n");
                sb.append("create schema WindowOut(outid string);\n");
                sb.append("on SupportBean sb merge InfraMerge mw where sb.theString = mw.mergeid ");
                sb.append("when not matched ");
                sb.append("then insert into WindowOut event-precedence(0) select 'a' as outid ");
                sb.append("then insert into WindowOut select 'b' as outid ");
                sb.append("then insert into WindowOut event-precedence(1) select 'c' as outid;\n");
                sb.append("@name('c0') select * from WindowOut;\n");
                break;
            case "constant-on-split":
                // ConstantOnSplit: on-split with constant precedence
                sb.append("on SupportBean \n");
                sb.append("insert into Out event-precedence(1) select 1 as id\n");
                sb.append("insert into Out event-precedence(2) select 2 as id\n");
                sb.append("insert into Out event-precedence(3) select 3 as id\n");
                sb.append("output all;\n");
                sb.append("@name('s0') select * from Out;\n");
                break;
            case "nonconst-contained-event":
                // NonConstInsertIntoContainedEvent: contained-event chains
                // with per-event precedence properties. The schema class FQN
                // mirrors the suite's LvlA.class.getName() construction.
                sb.append("@public @buseventtype create schema LvlA as ").append(LvlA.class.getName()).append(";\n");
                sb.append("insert into LvlB event-precedence(precedence) select * from LvlA[b];\n");
                sb.append("insert into LvlC event-precedence(precedence) select * from LvlB[c];\n");
                sb.append("@name('s0') select id from LvlC;\n");
                break;
            case "constant-output-rate":
                // ConstantInsertIntoOutputRate: output-rate batching with
                // constant and computed event-precedence. The computed
                // precedence mirrors the suite's FQN static call.
                sb.append("create schema Out(id int);\n");
                sb.append("insert into Out event-precedence(1) select 1 + intPrimitive * 10 as id from SupportBean output every 2 events;\n");
                sb.append("insert into Out event-precedence(2) select 2 + intPrimitive * 10 as id from SupportBean output every 2 events;\n");
                sb.append("insert into Out event-precedence(").append(EPLInsertIntoEventPrecedenceScenarioOracle.class.getName()).append(".computeEventPrecedence(3, *)) select 3 + intPrimitive * 10 as id from SupportBean output every 2 events;\n");
                sb.append("@name('s0') select * from Out;\n");
                break;
            default:
                throw new IllegalArgumentException("unknown case: " + caseName);
        }
        return sb.toString();
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
        String eventType = step.getString("type", "");
        if ("SupportBean".equals(eventType)) {
            SupportBean bean = new SupportBean();
            bean.setTheString(step.getString("theString", ""));
            bean.setIntPrimitive(step.getInt("intPrimitive", 0));
            runtime.getEventService().sendEventBean(bean, "SupportBean");
        } else if ("LvlA".equals(eventType)) {
            JsonArray bs = step.get("bs").asArray();
            LvlB[] bArr = new LvlB[bs.size()];
            for (int i = 0; i < bs.size(); i++) {
                JsonObject bObj = bs.get(i).asObject();
                JsonArray cs = bObj.get("cs").asArray();
                LvlC[] cArr = new LvlC[cs.size()];
                for (int j = 0; j < cs.size(); j++) {
                    JsonObject cObj = cs.get(j).asObject();
                    cArr[j] = new LvlC(cObj.getString("id", ""), cObj.getInt("precedence", 0));
                }
                bArr[i] = new LvlB(bObj.getInt("precedence", 0), cArr);
            }
            runtime.getEventService().sendEventBean(new LvlA(bArr), "LvlA");
        }
    }

    private static class TraceWriter implements UpdateListener {
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
            if (newEvents == null) {
                return;
            }
            for (EventBean event : newEvents) {
                sequence++;
                JsonObject record = new JsonObject();
                record.add("case", caseName);
                record.add("sequence", sequence);
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                JsonObject fields = new JsonObject();
                for (String name : names) {
                    Object value = event.get(name);
                    fields.add(name, normalize(value));
                }
                JsonArray newArr = new JsonArray();
                JsonObject evt = new JsonObject();
                evt.add("fields", fields);
                newArr.add(evt);
                record.add("new", newArr);
                records.add(record);
            }
        }

        private static JsonValue normalize(Object value) {
            if (value == null) {
                return com.espertech.esper.common.client.json.minimaljson.Json.NULL;
            }
            if (value instanceof Number || value instanceof Boolean) {
                return com.espertech.esper.common.client.json.minimaljson.Json.value(String.valueOf(value));
            }
            if (value instanceof String) {
                return com.espertech.esper.common.client.json.minimaljson.Json.value((String) value);
            }
            return com.espertech.esper.common.client.json.minimaljson.Json.value(String.valueOf(value));
        }
    }

    /**
     * Mirror of the suite's static single-row function
     * EPLInsertIntoEventPrecedence.computeEventPrecedence(int, Object).
     * The event-precedence wildcard passes the output event's underlying,
     * which is a HashMap for the map-schema Out type.
     */
    public static int computeEventPrecedence(int value, Object param) {
        if (!(param instanceof java.util.HashMap)) {
            throw new IllegalArgumentException("expected HashMap event underlying");
        }
        return value;
    }

    public static class LvlA implements java.io.Serializable {
        private final LvlB[] b;

        public LvlA(LvlB... b) {
            this.b = b;
        }

        public LvlB[] getB() {
            return b;
        }
    }

    public static class LvlB implements java.io.Serializable {
        private final int precedence;
        private final LvlC[] c;

        public LvlB(int precedence, LvlC[] c) {
            this.precedence = precedence;
            this.c = c;
        }

        public int getPrecedence() {
            return precedence;
        }

        public LvlC[] getC() {
            return c;
        }
    }

    public static class LvlC implements java.io.Serializable {
        private final String id;
        private final int precedence;

        public LvlC(String id, int precedence) {
            this.id = id;
            this.precedence = precedence;
        }

        public String getId() {
            return id;
        }

        public int getPrecedence() {
            return precedence;
        }
    }
}
