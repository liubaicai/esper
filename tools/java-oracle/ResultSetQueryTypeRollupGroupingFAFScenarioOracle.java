import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventType;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.fireandforget.EPFireAndForgetQueryResult;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Set;

/** Direct Esper 9.0.0 oracle for ordinal 2's fire-and-forget grouping query. */
public final class ResultSetQueryTypeRollupGroupingFAFScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "rollup-grouping-funcs-faf-dedicated";
    private static final String COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String SOURCE = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRollupGroupingFuncs.java";
    private static final String DESCRIPTION = "ResultSetQueryTypeRollupGroupingFuncs ordinal 2: fire-and-forget grouping sets over a public CarWindow.";
    private static final String RUNTIME = "java-runtime-e8f49362431d4c33baa8";
    private static final String STATIC_ID = "java-1ae66c9985dc53282af0";
    private static final String EXECUTION = "ResultSetQueryTypeFAFCarEventAndGroupingFunc";
    private static final String CASE = "faf-grouping-snapshot";
    private static final String EPL = "select name, place, sum(count), grouping(name), grouping(place), grouping_id(name,place) as gid from CarWindow group by grouping sets((name, place), name, place, ())";
    private static final String[] NAMES = {"skoda", "skoda", "bmw", "bmw", "opel", "opel"};
    private static final String[] PLACES = {"france", "germany", "france", "germany", "france", "germany"};
    private static final int[] COUNTS = {10000, 5000, 100, 1000, 7000, 7000};

    private ResultSetQueryTypeRollupGroupingFAFScenarioOracle() {}

    public static void main(String[] args) throws Exception {
        if (args.length != 1) throw new IllegalArgumentException("usage: ResultSetQueryTypeRollupGroupingFAFScenarioOracle <scenario.json>");
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        rejectDuplicates(parsed);
        JsonObject scenario = object(parsed, "scenario");
        validate(scenario);
        JsonArray records = new JsonArray();
        run(scenario.get("steps").asArray(), records);
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID).add("javaCommit", COMMIT)
                .add("java", System.getProperty("java.version")).add("records", records));
    }

    private static void run(JsonArray steps, JsonArray records) throws Exception {
        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        config.getCommon().addEventType("SupportCarEvent", SupportCarEvent.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + RUNTIME, config);
        ((com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled setup = EPCompilerProvider.getCompiler().compile(
                    "@public create window CarWindow#keepall as SupportCarEvent; insert into CarWindow select * from SupportCarEvent",
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(setup, new DeploymentOptions().setDeploymentId("car-window"));
            if (deployment == null || deployment.getStatements() == null || deployment.getStatements().length != 2) throw new IllegalStateException("CarWindow setup did not deploy exactly two statements");
            boolean active = false; int sends = 0; int fafs = 0;
            for (int i = 0; i < steps.size(); i++) {
                JsonObject step = object(steps.get(i), "step " + i); String op = string(step, "op");
                if ("case".equals(op)) { if (active) throw new IllegalArgumentException("duplicate case marker"); active = CASE.equals(string(step, "case")); if (!active) throw new IllegalArgumentException("unexpected case"); continue; }
                if (!active) throw new IllegalArgumentException("step precedes case marker");
                if ("send".equals(op)) { validateSend(step, sends); runtime.getEventService().sendEventBean(new SupportCarEvent(string(object(step.get("payload"), "payload"), "name"), string(object(step.get("payload"), "payload"), "place"), integer(object(step.get("payload"), "payload"), "count")), "SupportCarEvent"); sends++; }
                else if ("faf".equals(op)) { require(step, "op", "statement"); if (!"s0".equals(string(step, "statement")) || fafs++ != 0) throw new IllegalArgumentException("FAF statement must be unique s0"); execute(runtime, records); }
                else throw new IllegalArgumentException("unsupported operation " + op);
            }
            if (!active || sends != 6 || fafs != 1) throw new IllegalArgumentException("case requires six sends and one faf");
        } finally { runtime.getDeploymentService().undeployAll(); runtime.destroy(); }
    }

    private static void execute(EPRuntime runtime, JsonArray records) throws Exception {
        EPCompiled compiled = EPCompilerProvider.getCompiler().compileQuery("@name('s0') " + EPL, new CompilerArguments(runtime.getRuntimePath()));
        EPFireAndForgetQueryResult result = runtime.getFireAndForgetService().executeQuery(compiled);
        EventBean[] events = result.getArray();
        if (events == null || events.length != 12) throw new IllegalStateException("expected 12 FAF rows, got " + (events == null ? 0 : events.length));
        EventType type = events[0].getEventType();
        String[] expectedProperties = {"name", "place", "sum(count)", "grouping(name)", "grouping(place)", "gid"};
        Set<String> expectedSet = new HashSet<>(Arrays.asList(expectedProperties));
        if (!expectedSet.equals(new HashSet<>(Arrays.asList(type.getPropertyNames())))) throw new IllegalStateException("unexpected FAF result schema");
        String[] expectedNames = {"skoda", "skoda", "bmw", "bmw", "opel", "opel", "skoda", "bmw", "opel", null, null, null};
        String[] expectedPlaces = {"france", "germany", "france", "germany", "france", "germany", null, null, null, "france", "germany", null};
        int[] expectedSums = {10000, 5000, 100, 1000, 7000, 7000, 15000, 1100, 14000, 17100, 13000, 30100};
        int[] expectedGroupingName = {0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1};
        int[] expectedGroupingPlace = {0, 0, 0, 0, 0, 0, 1, 1, 1, 0, 0, 1};
        for (int i = 0; i < events.length; i++) {
            EventBean event = events[i];
            if (!java.util.Objects.equals(expectedNames[i], event.get("name")) || !java.util.Objects.equals(expectedPlaces[i], event.get("place"))
                    || !(event.get("sum(count)") instanceof Number) || ((Number) event.get("sum(count)")).longValue() != expectedSums[i]
                    || !Integer.valueOf(expectedGroupingName[i]).equals(event.get("grouping(name)"))
                    || !Integer.valueOf(expectedGroupingPlace[i]).equals(event.get("grouping(place)"))
                    || !Integer.valueOf(expectedGroupingName[i] * 2 + expectedGroupingPlace[i]).equals(event.get("gid"))) {
                throw new IllegalStateException("FAF row " + i + " differs from pinned result");
            }
        }
        JsonArray rows = new JsonArray();
        for (EventBean event : events) { JsonObject fields = new JsonObject(); String[] names = event.getEventType().getPropertyNames().clone(); Arrays.sort(names); for (String name : names) fields.add(name, normalize(event.get(name))); rows.add(new JsonObject().add("kind", "row").add("fields", fields)); }
        records.add(new JsonObject().add("case", CASE).add("operation", "faf").add("statement", "s0").add("sequence", 0).add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString()).add("new", rows));
    }

    private static void validate(JsonObject s) {
        require(s, "version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(s,"version")) || !ID.equals(string(s,"id")) || !DESCRIPTION.equals(string(s,"description")) || !COMMIT.equals(string(s,"javaCommit")) || !SOURCE.equals(string(s,"javaSource"))) throw new IllegalArgumentException("scenario metadata is not pinned");
        strings(s.get("javaRuntimes"), new String[]{RUNTIME}, "javaRuntimes"); strings(s.get("javaNames"), new String[]{EXECUTION}, "javaNames"); strings(s.get("javaStaticIds"), new String[]{STATIC_ID}, "javaStaticIds"); strings(s.get("javaFlags"), new String[]{"FIREANDFORGET"}, "javaFlags");
        JsonArray cs = array(s.get("cases"), "cases"); if (cs.size() != 1) throw new IllegalArgumentException("exactly one case required"); JsonObject c = object(cs.get(0), "case"); require(c,"case","ordinal","runtimeId","executionName","observation","iteratorSnapshots","epl"); if (!CASE.equals(string(c,"case")) || integer(c,"ordinal") != 2 || !RUNTIME.equals(string(c,"runtimeId")) || !EXECUTION.equals(string(c,"executionName")) || !"faf".equals(string(c,"observation")) || integer(c,"iteratorSnapshots") != 0 || !EPL.equals(string(c,"epl"))) throw new IllegalArgumentException("case metadata is not pinned");
        JsonArray st = array(s.get("steps"), "steps"); if (st.size() != 8) throw new IllegalArgumentException("exactly eight steps required");
        JsonObject marker = object(st.get(0), "case marker"); require(marker,"op","case"); if (!"case".equals(string(marker,"op")) || !CASE.equals(string(marker,"case"))) throw new IllegalArgumentException("case marker is not pinned"); for (int i=0;i<6;i++) validateSend(object(st.get(i+1),"send"), i); JsonObject faf=object(st.get(7),"faf"); require(faf,"op","statement"); if (!"faf".equals(string(faf,"op")) || !"s0".equals(string(faf,"statement"))) throw new IllegalArgumentException("faf step is not pinned");
    }
    private static void validateSend(JsonObject s, int i) { require(s,"op","eventType","payload"); JsonObject p=object(s.get("payload"),"payload"); require(p,"name","place","count"); if (!"send".equals(string(s,"op")) || !"SupportCarEvent".equals(string(s,"eventType")) || !NAMES[i].equals(string(p,"name")) || !PLACES[i].equals(string(p,"place")) || COUNTS[i] != integer(p,"count")) throw new IllegalArgumentException("send payload is not pinned"); }
    private static JsonValue normalize(Object value) { if (value == null) return new JsonObject().add("state", "null"); if (value instanceof Number) return Json.value(((Number)value).longValue()); if (value instanceof Boolean) return Json.value((Boolean)value); return Json.value(String.valueOf(value)); }
    private static void rejectDuplicates(JsonValue v) { if (v.isObject()) { Set<String> seen=new HashSet<>(); for (Member m:v.asObject()) { if(!seen.add(m.getName())) throw new IllegalArgumentException("duplicate JSON object key: " + m.getName()); rejectDuplicates(m.getValue()); } } else if (v.isArray()) for(JsonValue x:v.asArray()) rejectDuplicates(x); }
    private static void require(JsonObject o,String... names){if(o==null||o.size()!=names.length||!new HashSet<>(o.names()).equals(new HashSet<>(Arrays.asList(names))))throw new IllegalArgumentException("JSON object has unexpected fields");}
    private static String string(JsonObject o,String n){JsonValue v=o.get(n);if(v==null||!v.isString())throw new IllegalArgumentException(n+" must be a JSON string");return v.asString();}
    private static int integer(JsonObject o,String n){JsonValue v=o.get(n);if(v==null||!v.isNumber())throw new IllegalArgumentException(n+" must be an integer JSON number");try{return v.asInt();}catch(RuntimeException e){throw new IllegalArgumentException(n+" must be an integer JSON number",e);}}
    private static JsonObject object(JsonValue v,String l){if(v==null||!v.isObject())throw new IllegalArgumentException(l+" must be a JSON object");return v.asObject();}
    private static JsonArray array(JsonValue v,String l){if(v==null||!v.isArray())throw new IllegalArgumentException(l+" must be a JSON array");return v.asArray();}
    private static void strings(JsonValue v,String[] e,String l){JsonArray a=array(v,l);if(a.size()!=e.length)throw new IllegalArgumentException(l+" length is not pinned");for(int i=0;i<e.length;i++)if(!e[i].equals(string(object(new JsonObject().add("v",a.get(i)),"x"),"v")))throw new IllegalArgumentException(l+" mismatch");}
    public static final class SupportCarEvent { private final String name,place; private final int count; public SupportCarEvent(String n,String p,int c){name=n;place=p;count=c;} public String getName(){return name;} public String getPlace(){return place;} public int getCount(){return count;} }
}
