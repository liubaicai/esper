import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.*;
import com.espertech.esper.compiler.client.*;
import com.espertech.esper.runtime.client.*;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;
import java.nio.charset.StandardCharsets;
import java.nio.file.*;
import java.time.Instant;
import java.util.*;

public final class ResultSetAggregateSortedNoDataWindowScenarioOracle {
 private static final String V="esper-parity/v1", ID="resultset-aggregate-sorted-no-data-window", CASE=ID;
 private static final String EXEC="ResultSetAggregateNoDataWindow";
 private static final String COMMIT="9e1b9f1cc9117fea4bf33ab043762c045d73839c";
 private static final String SOURCE="regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateSortedMinMaxBy.java";
 private static final String RUNTIME="java-runtime-af551963966a83d26468", STATIC="java-f645b6fb41fd8c63d7f0";
 private static final String EPL="@name('s0') select maxbyever(intPrimitive).theString as c0, minbyever(intPrimitive).theString as c1, maxby(intPrimitive).theString as c2, minby(intPrimitive).theString as c3 from SupportBean";
 private static final String DESC="ResultSetAggregateSortedMinMaxBy ordinal 6: unwindowed current and ever min/max-by projections.";
 private ResultSetAggregateSortedNoDataWindowScenarioOracle() {}
 public static void main(String[] a) throws Exception {
  if(a.length!=1) throw new IllegalArgumentException("usage: ... <scenario.json>");
  JsonObject s=Json.parse(Files.readString(Path.of(a[0]), StandardCharsets.UTF_8)).asObject(); validate(s);
  Configuration c=new Configuration(); c.getRuntime().getThreading().setInternalTimerEnabled(false); Map<String,Object> t=new HashMap<>(); t.put("theString",String.class); t.put("intPrimitive",Integer.class); c.getCommon().addEventType("SupportBean",t);
  EPRuntime rt=EPRuntimeProvider.getRuntime("parity-resultset-aggregate-sorted-no-data-window",c); ((EPRuntimeSPI)rt).initialize(0L); JsonArray records=new JsonArray();
  try { EPCompiled x=EPCompilerProvider.getCompiler().compile(EPL,new CompilerArguments(rt.getRuntimePath())); EPDeployment d=rt.getDeploymentService().deploy(x,new DeploymentOptions().setDeploymentId("parity-resultset-aggregate-sorted-no-data-window")); EPStatement st=find(d); st.addListener(new TraceWriter(records,st,rt)); replay(s.get("steps").asArray(),rt); if(records.size()!=4) throw new IllegalStateException("expected 4 records, got "+records.size()); validateRecords(records); rt.getDeploymentService().undeployAll(); } finally { rt.destroy(); }
  System.out.println(new JsonObject().add("version",V).add("id",ID).add("records",records));
 }
 private static void validate(JsonObject s) {
  req(s,"version","id","description","javaCommit","javaSource","javaRuntimes","javaNames","javaStaticIds","javaFlags","cases","steps");
  if(!V.equals(str(s,"version"))||!ID.equals(str(s,"id"))||!DESC.equals(str(s,"description"))||!COMMIT.equals(str(s,"javaCommit"))||!SOURCE.equals(str(s,"javaSource"))) throw new IllegalArgumentException("scenario metadata mismatch");
  strings(s.get("javaRuntimes"),RUNTIME); strings(s.get("javaNames"),EXEC); strings(s.get("javaStaticIds"),STATIC); strings(s.get("javaFlags"));
  JsonArray cs=arr(s.get("cases")); if(cs.size()!=1) throw new IllegalArgumentException("exactly one case"); JsonObject z=obj(cs.get(0)); req(z,"case","ordinal","runtimeId","executionName","observation","iteratorSnapshots","epl"); if(!CASE.equals(str(z,"case"))||integer(z,"ordinal")!=6||!RUNTIME.equals(str(z,"runtimeId"))||!EXEC.equals(str(z,"executionName"))||!"listener".equals(str(z,"observation"))||integer(z,"iteratorSnapshots")!=0||!EPL.equals(str(z,"epl"))) throw new IllegalArgumentException("case metadata mismatch");
  JsonArray st=arr(s.get("steps")); if(st.size()!=5) throw new IllegalArgumentException("exactly five steps"); JsonObject m=obj(st.get(0)); req(m,"op","case"); if(!"case".equals(str(m,"op"))||!CASE.equals(str(m,"case"))) throw new IllegalArgumentException("case marker mismatch"); String[] n={"E1","E2","E3","E4"}; int[] v={1,2,0,3}; for(int i=0;i<4;i++){ JsonObject q=obj(st.get(i+1)); req(q,"op","eventType","payload"); if(!"send".equals(str(q,"op"))||!"SupportBean".equals(str(q,"eventType"))) throw new IllegalArgumentException("send mismatch"); JsonObject p=obj(q.get("payload")); req(p,"theString","intPrimitive"); if(p.size()!=2||!n[i].equals(str(p,"theString"))||integer(p,"intPrimitive")!=v[i]) throw new IllegalArgumentException("payload mismatch"); }
 }
 private static void validateRecords(JsonArray rs){ String[][] exp={{"E1","E1","E1","E1"},{"E2","E1","E2","E1"},{"E2","E3","E2","E3"},{"E4","E3","E4","E3"}}; for(int i=0;i<4;i++){JsonObject r=rs.get(i).asObject(); req(r,"case","operation","statement","sequence","time","new"); if(!CASE.equals(str(r,"case"))||!"listener".equals(str(r,"operation"))||!"s0".equals(str(r,"statement"))||integer(r,"sequence")!=i+1||r.names().contains("old")){if(r.names().contains("old")) throw new IllegalStateException("unexpected old rows"); throw new IllegalStateException("record metadata mismatch");} JsonArray rows=r.get("new").asArray(); if(rows.size()!=1) throw new IllegalStateException("expected one new row"); JsonObject f=rows.get(0).asObject().get("fields").asObject(); if(f.size()!=4) throw new IllegalStateException("wrong fields"); for(int j=0;j<4;j++) if(!exp[i][j].equals(f.getString("c"+(j),""))) throw new IllegalStateException("wrong result row"); }}
 private static void replay(JsonArray st,EPRuntime rt){for(int i=1;i<st.size();i++){JsonObject p=st.get(i).asObject().get("payload").asObject(); Map<String,Object> e=new HashMap<>();e.put("theString",str(p,"theString"));e.put("intPrimitive",integer(p,"intPrimitive"));rt.getEventService().sendEventMap(e,"SupportBean");}}
 private static EPStatement find(EPDeployment d){for(EPStatement s:d.getStatements())if("s0".equals(s.getName()))return s;throw new IllegalStateException("statement s0 missing");}
 private static void req(JsonObject o,String... k){if(o.size()!=k.length)throw new IllegalArgumentException("wrong JSON keys");for(String x:k)if(!o.names().contains(x))throw new IllegalArgumentException("missing key "+x);}
 private static String str(JsonObject o,String k){JsonValue v=o.get(k);if(v==null||!v.isString())throw new IllegalArgumentException("string required");return v.asString();}
 private static int integer(JsonObject o,String k){JsonValue v=o.get(k);if(v==null||!v.isNumber()||v.asDouble()!=v.asLong()||v.asLong()<Integer.MIN_VALUE||v.asLong()>Integer.MAX_VALUE)throw new IllegalArgumentException("integer required");return v.asInt();}
 private static JsonObject obj(JsonValue v){if(v==null||!v.isObject())throw new IllegalArgumentException("object required");return v.asObject();} private static JsonArray arr(JsonValue v){if(v==null||!v.isArray())throw new IllegalArgumentException("array required");return v.asArray();}
 private static void strings(JsonValue v,String... expected){JsonArray a=arr(v);if(a.size()!=expected.length)throw new IllegalArgumentException("array mismatch");for(int i=0;i<expected.length;i++){JsonValue x=a.get(i);if(x==null||!x.isString()||!expected[i].equals(x.asString()))throw new IllegalArgumentException("array mismatch");}}
 private static final class TraceWriter implements UpdateListener {final JsonArray out;final EPStatement st;final EPRuntime rt;long seq;TraceWriter(JsonArray o,EPStatement s,EPRuntime r){out=o;st=s;rt=r;} public void update(EventBean[] n,EventBean[] old,EPStatement x,EPRuntime r){if(n==null&&old==null)return;JsonObject q=new JsonObject().add("case",CASE).add("operation","listener").add("statement",st.getName()).add("sequence",++seq).add("time",Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());q.add("new",rows(n));if(old!=null&&old.length>0)q.add("old",rows(old));out.add(q);} JsonArray rows(EventBean[] es){JsonArray a=new JsonArray();if(es!=null)for(EventBean e:es){JsonObject f=new JsonObject();for(String p:e.getEventType().getPropertyNames())f.add(p,Json.value(String.valueOf(e.get(p))));a.add(new JsonObject().add("kind","row").add("fields",f));}return a;}}
}
