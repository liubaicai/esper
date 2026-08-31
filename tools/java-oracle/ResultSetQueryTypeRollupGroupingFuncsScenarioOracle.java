import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.internal.util.SerializableObjectCopier;
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
import java.util.Arrays;
import java.util.HashSet;
import java.util.Set;

/** Direct Esper 9.0.0 oracle for the pinned grouping-functions documentation sample. */
public final class ResultSetQueryTypeRollupGroupingFuncsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "rollup-grouping-funcs-dedicated";
    private static final String COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String SOURCE = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRollupGroupingFuncs.java";
    private static final String DESCRIPTION = "ResultSetQueryTypeRollupGroupingFuncs ordinal 0: three compilation paths for the SupportCarEvent grouping-function documentation sample.";
    private static final String RUNTIME = "java-runtime-52a4f853b6052fb47437";
    private static final String STATIC_ID = "java-1ae66c9985dc53282af0";
    private static final String EXECUTION = "ResultSetQueryTypeDocSampleCarEventAndGroupingFunc";
    private static final String EPL = "@name('s0') select name, place, sum(count), grouping(name), grouping(place), grouping_id(name,place) as gid from SupportCarEvent group by grouping sets((name, place), name, place, ())";
    private static final String[] CASES = {"doc-sample-plain", "doc-sample-audit", "doc-sample-model"};

    private ResultSetQueryTypeRollupGroupingFuncsScenarioOracle() {}

    public static void main(String[] args) throws Exception {
        if (args.length != 1) throw new IllegalArgumentException("usage: ResultSetQueryTypeRollupGroupingFuncsScenarioOracle <scenario.json>");
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) throw new IllegalArgumentException("scenario must be a JSON object");
        rejectDuplicates(parsed);
        JsonObject scenario = parsed.asObject();
        validate(scenario);
        JsonArray records = new JsonArray();
        for (int i = 0; i < CASES.length; i++) runCase(scenario.get("steps").asArray(), CASES[i], i, records);
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID).add("javaCommit", COMMIT)
                .add("java", System.getProperty("java.version")).add("records", records));
    }

    private static void runCase(JsonArray steps, String caseName, int index, JsonArray records) throws Exception {
        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        config.getCommon().addEventType("SupportCarEvent", SupportCarEvent.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + RUNTIME, config);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            String text = index == 1 ? "@Audit " + EPL : EPL;
            EPCompiled compiled;
            if (index == 2) {
                EPStatementObjectModel model = SerializableObjectCopier.copyMayFail(EPCompilerProvider.getCompiler().eplToModel(text, config));
                Module module = new Module(); module.getItems().add(new ModuleItem(model)); module.setModuleText(model.toEPL());
                compiled = EPCompilerProvider.getCompiler().compile(module, new CompilerArguments(runtime.getRuntimePath()));
            } else compiled = EPCompilerProvider.getCompiler().compile(text, new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions().setDeploymentId(caseName));
            EPStatement statement = find(deployment);
            TraceWriter writer = new TraceWriter(records, caseName, statement, runtime);
            statement.addListener(writer);
            replay(steps, caseName, runtime);
            if (writer.sequence != 2) throw new IllegalStateException("expected two listener callbacks, got " + writer.sequence);
            runtime.getDeploymentService().undeployAll();
        } finally { runtime.destroy(); }
    }

    private static EPStatement find(EPDeployment deployment) {
        if (deployment.getStatements() == null || deployment.getStatements().length != 1 || !"s0".equals(deployment.getStatements()[0].getName())) throw new IllegalStateException("expected exactly one statement named s0");
        return deployment.getStatements()[0];
    }
    private static void replay(JsonArray steps, String wanted, EPRuntime runtime) {
        boolean active = false;
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = object(steps.get(i), "step " + i); String op = string(step, "op");
            if ("case".equals(op)) { active = wanted.equals(string(step, "case")); continue; }
            if (!active || !"send".equals(op)) { if (active) throw new IllegalArgumentException("unsupported operation at step " + i); continue; }
            require(step, "op", "eventType", "payload");
            if (!"SupportCarEvent".equals(string(step, "eventType"))) throw new IllegalArgumentException("unexpected event type");
            JsonObject p = object(step.get("payload"), "payload"); require(p, "name", "place", "count");
            runtime.getEventService().sendEventBean(new SupportCarEvent(string(p,"name"), string(p,"place"), integer(p,"count")), "SupportCarEvent");
        }
    }

    private static void validate(JsonObject s) {
        require(s, "version","id","description","javaCommit","javaSource","javaRuntimes","javaNames","javaStaticIds","javaFlags","cases","steps");
        if (!VERSION.equals(string(s,"version")) || !ID.equals(string(s,"id")) || !DESCRIPTION.equals(string(s,"description")) || !COMMIT.equals(string(s,"javaCommit")) || !SOURCE.equals(string(s,"javaSource"))) throw new IllegalArgumentException("scenario metadata is not pinned");
        strings(s.get("javaRuntimes"), new String[]{RUNTIME}, "javaRuntimes"); strings(s.get("javaNames"), new String[]{EXECUTION}, "javaNames"); strings(s.get("javaStaticIds"), new String[]{STATIC_ID}, "javaStaticIds"); strings(s.get("javaFlags"), new String[0], "javaFlags");
        JsonArray cs = array(s.get("cases"), "cases"); if (cs.size()!=3) throw new IllegalArgumentException("scenario must contain exactly three cases");
        for(int i=0;i<3;i++){ JsonObject c=object(cs.get(i),"case"); require(c,"case","ordinal","runtimeId","executionName","observation","iteratorSnapshots","epl"); if(!CASES[i].equals(string(c,"case"))||integer(c,"ordinal")!=0||!RUNTIME.equals(string(c,"runtimeId"))||!EXECUTION.equals(string(c,"executionName"))||!"listener".equals(string(c,"observation"))||integer(c,"iteratorSnapshots")!=0||!EPL.equals(string(c,"epl"))) throw new IllegalArgumentException("case metadata is not pinned"); }
        JsonArray st=array(s.get("steps"),"steps"); if(st.size()!=9) throw new IllegalArgumentException("scenario must contain exactly nine steps");
        for(int i=0;i<3;i++){ JsonObject m=object(st.get(i*3),"marker"); require(m,"op","case"); if(!"case".equals(string(m,"op"))||!CASES[i].equals(string(m,"case"))) throw new IllegalArgumentException("case marker is not pinned"); validateSend(st.get(i*3+1),"skoda","france",100); validateSend(st.get(i*3+2),"skoda","germany",75); }
    }
    private static void validateSend(JsonValue v,String n,String p,int c){ JsonObject s=object(v,"send"); require(s,"op","eventType","payload"); JsonObject x=object(s.get("payload"),"payload"); require(x,"name","place","count"); if(!"send".equals(string(s,"op"))||!"SupportCarEvent".equals(string(s,"eventType"))||!n.equals(string(x,"name"))||!p.equals(string(x,"place"))||integer(x,"count")!=c) throw new IllegalArgumentException("send payload is not pinned"); }
    private static void rejectDuplicates(JsonValue v){ if(v.isObject()){Set<String> seen=new HashSet<>(); for(Member m:v.asObject()){if(!seen.add(m.getName()))throw new IllegalArgumentException("duplicate JSON object key: "+m.getName()); rejectDuplicates(m.getValue());}} else if(v.isArray()) for(JsonValue x:v.asArray()) rejectDuplicates(x); }
    private static void require(JsonObject o,String... names){if(o==null||o.size()!=names.length||!new HashSet<>(o.names()).equals(new HashSet<>(Arrays.asList(names))))throw new IllegalArgumentException("JSON object has unexpected fields");}
    private static String string(JsonObject o,String n){JsonValue v=o.get(n);if(v==null||!v.isString())throw new IllegalArgumentException(n+" must be a JSON string");return v.asString();}
    private static int integer(JsonObject o,String n){JsonValue v=o.get(n);if(v==null||!v.isNumber())throw new IllegalArgumentException(n+" must be an integer JSON number");try{return v.asInt();}catch(RuntimeException e){throw new IllegalArgumentException(n+" must be an integer JSON number",e);}}
    private static JsonObject object(JsonValue v,String l){if(v==null||!v.isObject())throw new IllegalArgumentException(l+" must be a JSON object");return v.asObject();}
    private static JsonArray array(JsonValue v,String l){if(v==null||!v.isArray())throw new IllegalArgumentException(l+" must be a JSON array");return v.asArray();}
    private static void strings(JsonValue v,String[] e,String l){JsonArray a=array(v,l);if(a.size()!=e.length)throw new IllegalArgumentException(l+" length is not pinned");for(int i=0;i<e.length;i++)if(!e[i].equals(string(object(new JsonObject().add("v",a.get(i)),"x"),"v")))throw new IllegalArgumentException(l+" mismatch");}

    private static final class TraceWriter implements UpdateListener { final JsonArray records; final String caseName; final EPStatement statement; final EPRuntime runtime; int sequence;
        TraceWriter(JsonArray r,String c,EPStatement s,EPRuntime rt){records=r;caseName=c;statement=s;runtime=rt;}
        public void update(EventBean[] n,EventBean[] o,EPStatement ignored,EPRuntime ignoredRuntime){if(n==null||n.length!=4||(o!=null&&o.length!=0))throw new IllegalStateException("expected four new rows only"); long t=runtime.getEventService().getCurrentTime(); JsonArray rows=new JsonArray(); for(EventBean e:n){JsonObject f=new JsonObject();String[] names=e.getEventType().getPropertyNames().clone();Arrays.sort(names);for(String name:names)f.add(name,normalize(e.get(name)));rows.add(new JsonObject().add("kind","row").add("fields",f));} records.add(new JsonObject().add("case",caseName).add("operation","listener").add("statement",statement.getName()).add("sequence",++sequence).add("time",Instant.ofEpochMilli(t).toString()).add("new",rows)); }
        private JsonValue normalize(Object v){if(v==null)return new JsonObject().add("state","null");if(v instanceof Number)return Json.value(((Number)v).longValue());if(v instanceof Boolean)return Json.value((Boolean)v);return Json.value(String.valueOf(v));}
    }
    public static final class SupportCarEvent { private final String name,place; private final int count; public SupportCarEvent(String n,String p,int c){name=n;place=p;count=c;} public String getName(){return name;} public String getPlace(){return place;} public int getCount(){return count;} }
}
