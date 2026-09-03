import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.*;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.*;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.*;

/** Direct Esper 9.0.0 oracle for ResultSetOutputLimitRowLimit ordinal 7. */
public final class ResultSetOutputLimitRowLimitNegativeRowcountScenarioOracle {
    private static final String VERSION="esper-parity/v1", ID="resultset-output-limit-row-limit-negative-rowcount";
    private static final String DESCRIPTION="ResultSetOutputLimitRowLimit ordinal 7: grouped snapshot negative rowcount with offset.";
    private static final String JAVA_COMMIT="9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE="regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowLimit.java";
    private static final String CASE="grouped-snapshot-negative-rowcount";
    private static final int ORDINAL=7;
    private static final String RUNTIME="java-runtime-6bf4cf3ded03c9ff57fa";
    private static final String EXECUTION="ResultSetGroupedSnapshotNegativeRowcount";
    private static final String STATIC_ID="java-0109d52ee4e8b36575b9";
    private static final String EPL="@name('s0') select theString, sum(intPrimitive) as mysum from SupportBean#length(5) group by theString output snapshot every 10 seconds order by sum(intPrimitive) desc limit -1 offset 1";
    private ResultSetOutputLimitRowLimitNegativeRowcountScenarioOracle() {}

    public static void main(String[] args) throws Exception {
        if (args.length != 1) throw new IllegalArgumentException("usage: ResultSetOutputLimitRowLimitNegativeRowcountScenarioOracle <scenario.json>");
        JsonValue parsed=Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        rejectDuplicateKeys(parsed); JsonObject scenario=parsed.asObject(); validateScenario(scenario);
        Configuration cfg=new Configuration(); cfg.getRuntime().getThreading().setInternalTimerEnabled(false); cfg.getCommon().addEventType(SupportBean.class);
        EPRuntime runtime=EPRuntimeProvider.getRuntime("parity-"+ID+"-"+RUNTIME,cfg); ((EPRuntimeSPI)runtime).initialize(0L);
        JsonArray records=new JsonArray();
        try {
            JsonArray steps=scenario.get("steps").asArray();
            runtime.getEventService().advanceTime(1000L);
            EPCompiled compiled=EPCompilerProvider.getCompiler().compile(EPL,new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment=runtime.getDeploymentService().deploy(compiled,new DeploymentOptions().setDeploymentId(ID));
            EPStatement statement=findStatement(deployment); TraceWriter writer=new TraceWriter(records,statement,runtime); statement.addListener(writer);
            replay(steps,runtime,statement,writer);
            if (writer.listeners!=1 || writer.snapshots!=1) throw new IllegalStateException("expected one listener and one snapshot");
        } finally { runtime.getDeploymentService().undeployAll(); runtime.destroy(); }
        if (records.size()!=2) throw new IllegalStateException("expected exactly two trace records, got "+records.size());
        System.out.println(new JsonObject().add("version",VERSION).add("id",ID).add("javaCommit",JAVA_COMMIT).add("java",System.getProperty("java.version")).add("records",records));
    }
    private static EPStatement findStatement(EPDeployment d) { EPStatement result=null; for(EPStatement s:d.getStatements()) if("s0".equals(s.getName())) { if(result!=null) throw new IllegalStateException("multiple s0"); result=s; } if(result==null) throw new IllegalStateException("statement s0 missing"); return result; }
    private static void replay(JsonArray steps,EPRuntime runtime,EPStatement statement,TraceWriter writer) {
        boolean active=false; boolean initial=true;
        for(int i=0;i<steps.size();i++) { JsonObject step=object(steps.get(i),"step "+i); String op=string(step,"op");
            if("case".equals(op)) { requireFields(step,"op","case"); active=CASE.equals(string(step,"case")); continue; }
            if(!active) continue;
            if("advance-time".equals(op)) { requireFields(step,"op","at"); String at=string(step,"at"); if(initial && "1970-01-01T00:00:01Z".equals(at)) { initial=false; continue; } if("1970-01-01T00:00:11Z".equals(at)) { runtime.getEventService().advanceTime(11000L); continue; } throw new IllegalArgumentException("unexpected timer "+at); }
            if("snapshot".equals(op)) { requireFields(step,"op","case","statement","mode"); if(!CASE.equals(string(step,"case"))||!"s0".equals(string(step,"statement"))||!"ordered".equals(string(step,"mode"))) throw new IllegalArgumentException("snapshot metadata not pinned"); JsonArray rows=new JsonArray(); Iterator<EventBean> it=statement.iterator(); while(it.hasNext()) rows.add(row(it.next())); if(writer.snapshots==0 && rows.size()!=0) throw new IllegalStateException("initial iterator snapshot is not empty"); writer.records.add(new JsonObject().add("case",CASE).add("operation","snapshot").add("statement","s0").add("sequence",0).add("time",Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString()).add("new",rows)); writer.snapshots++; continue; }
            if(!"send".equals(op)) throw new IllegalArgumentException("unsupported operation "+op);
            requireFields(step,"op","case","eventType","payload"); if(!"SupportBean".equals(string(step,"eventType"))) throw new IllegalArgumentException("unexpected event type"); JsonObject p=object(step.get("payload"),"payload"); requireFields(p,"theString","intPrimitive"); runtime.getEventService().sendEventBean(new SupportBean(string(p,"theString"),integer(p,"intPrimitive")),"SupportBean");
        }
    }
    private static JsonObject row(EventBean e) { String[] ps=e.getEventType().getPropertyNames().clone(); Arrays.sort(ps); JsonObject f=new JsonObject(); for(String p:ps) f.add(p,normalize(e.get(p))); return new JsonObject().add("kind","row").add("fields",f); }
    private static JsonValue normalize(Object v) { if(v==null)return new JsonObject().add("state","null"); if(v instanceof Integer || v instanceof Long || v instanceof Short || v instanceof Byte)return Json.value(((Number)v).longValue()); if(v instanceof Float || v instanceof Double){double n=((Number)v).doubleValue(); return n==Math.rint(n)&&!Double.isInfinite(n)?Json.value((long)n):Json.value(n);} if(v instanceof Boolean)return Json.value((Boolean)v); if(v instanceof Character)return Json.value(String.valueOf(v)); return Json.value(String.valueOf(v)); }
    private static void validateScenario(JsonObject s) { requireFields(s,"version","id","description","javaCommit","javaSource","javaRuntimes","javaNames","javaStaticIds","javaFlags","cases","steps"); if(!VERSION.equals(string(s,"version"))||!ID.equals(string(s,"id"))||!DESCRIPTION.equals(string(s,"description"))||!JAVA_COMMIT.equals(string(s,"javaCommit"))||!JAVA_SOURCE.equals(string(s,"javaSource"))) throw new IllegalArgumentException("scenario metadata is not pinned"); validateStringArray(s.get("javaRuntimes"),new String[]{RUNTIME}); validateStringArray(s.get("javaNames"),new String[]{EXECUTION}); validateStringArray(s.get("javaStaticIds"),new String[]{STATIC_ID}); validateStringArray(s.get("javaFlags"),new String[0]); JsonArray cs=array(s.get("cases"),"cases"); if(cs.size()!=1)throw new IllegalArgumentException("exactly one case required"); JsonObject c=object(cs.get(0),"case"); requireFields(c,"case","ordinal","runtimeId","executionName","observation","iteratorSnapshots","epl"); if(!CASE.equals(string(c,"case"))||integer(c,"ordinal")!=ORDINAL||!RUNTIME.equals(string(c,"runtimeId"))||!EXECUTION.equals(string(c,"executionName"))||!"listener+iterator".equals(string(c,"observation"))||integer(c,"iteratorSnapshots")!=1||!EPL.equals(string(c,"epl")))throw new IllegalArgumentException("case metadata is not pinned"); JsonArray st=array(s.get("steps"),"steps"); if(st.size()!=8)throw new IllegalArgumentException("exactly 8 steps required");
        requireFields(object(st.get(0),"marker"),"op","case"); if(!"case".equals(string(object(st.get(0),"marker"),"op"))||!CASE.equals(string(object(st.get(0),"marker"),"case")))throw new IllegalArgumentException("case marker not pinned"); requireFields(object(st.get(1),"time"),"op","at"); if(!"advance-time".equals(string(object(st.get(1),"time"),"op"))||!"1970-01-01T00:00:01Z".equals(string(object(st.get(1),"time"),"at")))throw new IllegalArgumentException("initial time not pinned"); requireFields(object(st.get(2),"snapshot"),"op","case","statement","mode");
        String[] names={"E1","E2","E3","E1"}; int[] vals={10,5,20,30}; for(int i=0;i<4;i++){JsonObject x=object(st.get(3+i),"send"); requireFields(x,"op","case","eventType","payload"); JsonObject p=object(x.get("payload"),"payload"); requireFields(p,"theString","intPrimitive"); if(!"send".equals(string(x,"op"))||!CASE.equals(string(x,"case"))||!"SupportBean".equals(string(x,"eventType"))||!names[i].equals(string(p,"theString"))||integer(p,"intPrimitive")!=vals[i])throw new IllegalArgumentException("send not pinned");} JsonObject end=object(st.get(7),"end"); requireFields(end,"op","at"); if(!"advance-time".equals(string(end,"op"))||!"1970-01-01T00:00:11Z".equals(string(end,"at")))throw new IllegalArgumentException("final time not pinned"); }
    private static final class TraceWriter implements UpdateListener { final JsonArray records; final EPStatement statement; final EPRuntime runtime; int listeners,snapshots; TraceWriter(JsonArray r,EPStatement s,EPRuntime t){records=r;statement=s;runtime=t;} public void update(EventBean[] n,EventBean[] o,EPStatement ignored,EPRuntime ir){if(n==null||n.length!=2||o!=null&&o.length!=0)throw new IllegalStateException("unexpected listener shape"); JsonArray rows=new JsonArray(); for(EventBean e:n)rows.add(row(e)); records.add(new JsonObject().add("case",CASE).add("operation","listener").add("statement","s0").add("sequence",++listeners).add("time",Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString()).add("new",rows));} }
    private static void rejectDuplicateKeys(JsonValue v){if(v.isObject()){Set<String>s=new HashSet<>();for(Member m:v.asObject()){if(!s.add(m.getName()))throw new IllegalArgumentException("duplicate key");rejectDuplicateKeys(m.getValue());}}else if(v.isArray())for(JsonValue x:v.asArray())rejectDuplicateKeys(x);}
    private static void requireFields(JsonObject o,String... n){if(o==null||o.size()!=n.length||!new HashSet<>(o.names()).equals(new HashSet<>(Arrays.asList(n))))throw new IllegalArgumentException("JSON fields mismatch");}
    private static String string(JsonObject o,String n){JsonValue v=o.get(n);if(v==null||!v.isString())throw new IllegalArgumentException(n+" must be string");return v.asString();}
    private static int integer(JsonObject o,String n){JsonValue v=o.get(n);if(!(v instanceof JsonNumber)||!v.toString().matches("-?(0|[1-9][0-9]*)"))throw new IllegalArgumentException(n+" must be integer");return v.asInt();}
    private static JsonObject object(JsonValue v,String l){if(v==null||!v.isObject())throw new IllegalArgumentException(l+" must be object");return v.asObject();}
    private static JsonArray array(JsonValue v,String l){if(v==null||!v.isArray())throw new IllegalArgumentException(l+" must be array");return v.asArray();}
    private static void validateStringArray(JsonValue v,String[] e){JsonArray a=array(v,"array");if(a.size()!=e.length)throw new IllegalArgumentException("array mismatch");for(int i=0;i<e.length;i++)if(!a.get(i).isString()||!e[i].equals(a.get(i).asString()))throw new IllegalArgumentException("array mismatch");}
}
