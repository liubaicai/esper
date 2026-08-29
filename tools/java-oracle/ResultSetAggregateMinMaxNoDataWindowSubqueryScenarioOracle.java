import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
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
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/** Direct Esper oracle for ResultSetAggregateMinMax ordinal 0. */
public final class ResultSetAggregateMinMaxNoDataWindowSubqueryScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-minmax-no-data-window-subquery";
    private static final String COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String SOURCE = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateMinMax.java";
    private static final String STATIC_ID = "java-347fe4c3739663bf45c4";
    private static final String RUNTIME_ID = "java-runtime-f1fedff9a25cca57fefa";
    private static final String CASE = "no-data-window-subquery";
    private static final String NAME = "ResultSetAggregateMinMaxNoDataWindowSubquery";
    private static final String EPL = "@name('s0') select max(intPrimitive) as maxi, min(intPrimitive) as mini," +
            "(select max(id) from SupportBean_S0#lastevent) as max0, (select min(id) from SupportBean_S0#lastevent) as min0" +
            " from SupportBean";
    private static final String DESCRIPTION = "ResultSetAggregateMinMax ordinal 0 NoDataWindowSubquery: unbounded SupportBean max/min plus scalar last-event SupportBean_S0 max/min. S0 updates alone produce no callback; four SupportBean events produce new-only rows.";

    public static void main(String[] args) throws Exception {
        if (args.length != 1) { System.err.println("usage: ... <scenario.json>"); System.exit(2); }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) throw new IllegalArgumentException("scenario must be an object");
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> bean = new HashMap<>();
        bean.put("theString", String.class); bean.put("intPrimitive", Integer.class);
        configuration.getCommon().addEventType("SupportBean", bean);
        Map<String, Object> s0 = new HashMap<>();
        s0.put("id", Integer.class);
        configuration.getCommon().addEventType("SupportBean_S0", s0);
        String runtimeURI = "parity-" + ID + "-" + RUNTIME_ID;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(EPL, new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions().setDeploymentId(runtimeURI));
            EPStatement statement = findStatement(deployment);
            TraceWriter writer = new TraceWriter(records, statement, runtime);
            statement.addListener(writer);
            replay(scenario.get("steps").asArray(), runtime);
            if (writer.sequence != 4) throw new IllegalStateException("expected four callbacks, got " + writer.sequence);
            runtime.getDeploymentService().undeployAll();
        } finally { runtime.destroy(); }
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID).add("javaCommit", COMMIT)
                .add("java", System.getProperty("java.version")).add("records", records));
    }

    private static void validateScenario(JsonObject s) {
        requireFields(s, "version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(s, "version")) || !ID.equals(string(s, "id")) ||
                !DESCRIPTION.equals(string(s, "description")) || !COMMIT.equals(string(s, "javaCommit")) ||
                !SOURCE.equals(string(s, "javaSource")))
            throw new IllegalArgumentException("scenario metadata mismatch");
        validateStrings(s.get("javaRuntimes"), new String[]{RUNTIME_ID}, "javaRuntimes");
        validateStrings(s.get("javaNames"), new String[]{NAME}, "javaNames");
        validateStrings(s.get("javaStaticIds"), new String[]{STATIC_ID}, "javaStaticIds");
        validateStrings(s.get("javaFlags"), new String[]{"EXCLUDEWHENINSTRUMENTED"}, "javaFlags");
        JsonArray cases = array(s.get("cases"), "cases");
        if (cases.size() != 1) throw new IllegalArgumentException("exactly one case required");
        JsonObject c = object(cases.get(0), "case");
        requireFields(c, "case", "ordinal", "runtimeId", "executionName", "observation", "epl");
        if (!CASE.equals(string(c, "case")) || integer(c, "ordinal") != 0 ||
                !RUNTIME_ID.equals(string(c, "runtimeId")) || !NAME.equals(string(c, "executionName")) ||
                !"listener".equals(string(c, "observation")) || !EPL.equals(string(c, "epl")))
            throw new IllegalArgumentException("case metadata mismatch");
        JsonArray steps = array(s.get("steps"), "steps");
        if (steps.size() != 7) throw new IllegalArgumentException("exactly seven steps required");
        JsonObject marker = object(steps.get(0), "marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !CASE.equals(string(marker, "case"))) throw new IllegalArgumentException("case marker mismatch");
        validateBean(steps, 1, "E1", 3); validateBean(steps, 2, "E2", 4);
        validateS0(steps, 3, 2); validateBean(steps, 4, "E3", 4);
        validateS0(steps, 5, 1); validateBean(steps, 6, "E4", 5);
    }
    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!names.add(member.getName())) throw new IllegalArgumentException("duplicate JSON object key: " + member.getName());
                rejectDuplicateKeys(member.getValue());
            }
        } else if (value.isArray()) {
            for (JsonValue item : value.asArray()) rejectDuplicateKeys(item);
        }
    }
    private static void requireFields(JsonObject object, String... expectedNames) {
        if (object == null || object.size() != expectedNames.length ||
                !new HashSet<>(object.names()).equals(new HashSet<>(Arrays.asList(expectedNames))))
            throw new IllegalArgumentException("JSON object has unexpected fields");
    }
    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isString()) throw new IllegalArgumentException(name + " must be a JSON string");
        return value.asString();
    }
    private static int integer(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isNumber()) throw new IllegalArgumentException(name + " must be an integer JSON number");
        try {
            double number = value.asDouble();
            long integral = value.asLong();
            if (!Double.isFinite(number) || number != integral || integral < Integer.MIN_VALUE || integral > Integer.MAX_VALUE)
                throw new IllegalArgumentException(name + " must be an integer JSON number");
            return (int) integral;
        } catch (RuntimeException ex) {
            throw new IllegalArgumentException(name + " must be an integer JSON number", ex);
        }
    }
    private static void validateStrings(JsonValue v, String[] expected, String label) {
        JsonArray a = array(v, label); if (a.size() != expected.length) throw new IllegalArgumentException(label + " length");
        for (int i=0;i<expected.length;i++) if (a.get(i) == null || !a.get(i).isString() || !expected[i].equals(a.get(i).asString())) throw new IllegalArgumentException(label + " mismatch");
    }
    private static JsonArray array(JsonValue v, String label) { if (v == null || !v.isArray()) throw new IllegalArgumentException(label + " must be array"); return v.asArray(); }
    private static JsonObject object(JsonValue v, String label) { if (v == null || !v.isObject()) throw new IllegalArgumentException(label + " must be object"); return v.asObject(); }
    private static void validateBean(JsonArray a, int i, String text, int value) {
        JsonObject x = object(a.get(i), "step"); requireFields(x, "op", "eventType", "payload"); JsonObject p = object(x.get("payload"), "payload"); requireFields(p, "theString", "intPrimitive");
        if (!"send".equals(string(x, "op")) || !"SupportBean".equals(string(x, "eventType")) ||
                !text.equals(string(p, "theString")) || integer(p, "intPrimitive") != value)
            throw new IllegalArgumentException("SupportBean step mismatch at " + i);
    }
    private static void validateS0(JsonArray a, int i, int id) {
        JsonObject x=object(a.get(i),"step"); requireFields(x, "op", "eventType", "payload"); JsonObject p=object(x.get("payload"),"payload"); requireFields(p, "id");
        if (!"send".equals(string(x, "op")) || !"SupportBean_S0".equals(string(x, "eventType")) || integer(p, "id") != id)
            throw new IllegalArgumentException("SupportBean_S0 step mismatch at " + i);
    }
    private static EPStatement findStatement(EPDeployment d) { for (EPStatement s : d.getStatements()) if ("s0".equals(s.getName())) return s; throw new IllegalStateException("s0 not deployed"); }
    private static void replay(JsonArray steps, EPRuntime r) {
        for (int i=1;i<steps.size();i++) { JsonObject x=steps.get(i).asObject(); JsonObject p=x.get("payload").asObject(); Map<String,Object> e=new HashMap<>();
            if ("SupportBean".equals(x.getString("eventType",""))) { e.put("theString",p.getString("theString",null)); e.put("intPrimitive",p.get("intPrimitive").asInt()); }
            else { e.put("id",p.get("id").asInt()); } r.getEventService().sendEventMap(e,x.getString("eventType","")); }
    }
    private static final class TraceWriter implements UpdateListener {
        final JsonArray records; final EPStatement statement; final EPRuntime runtime; long sequence;
        TraceWriter(JsonArray r, EPStatement s, EPRuntime rt) { records=r; statement=s; runtime=rt; }
        public void update(EventBean[] n, EventBean[] o, EPStatement ignored, EPRuntime ignoredRuntime) {
            if (n==null || n.length!=1 || (o!=null && o.length>0)) throw new IllegalStateException("callback must be one new-only row");
            sequence++;
            Object[] expected = sequence == 1 ? new Object[]{3,3,null,null} : sequence == 2 ? new Object[]{4,3,null,null} : sequence == 3 ? new Object[]{4,3,2,2} : sequence == 4 ? new Object[]{5,3,1,1} : null;
            if (expected == null) throw new IllegalStateException("unexpected callback sequence " + sequence);
            String[] names=n[0].getEventType().getPropertyNames().clone(); Arrays.sort(names);
            if(!Arrays.equals(names,new String[]{"max0","maxi","min0","mini"})) throw new IllegalStateException("field metadata mismatch");
            if (!same(n[0].get("maxi"), expected[0]) || !same(n[0].get("mini"), expected[1]) || !same(n[0].get("max0"), expected[2]) || !same(n[0].get("min0"), expected[3])) throw new IllegalStateException("callback values mismatch at sequence " + sequence);
            JsonObject rec=new JsonObject().add("case",CASE).add("operation","listener").add("statement",statement.getName()).add("sequence",sequence).add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString()).add("new",rows(n)); records.add(rec);
        }
        boolean same(Object actual, Object expected) { return actual == null ? expected == null : expected != null && ((Number)actual).longValue() == ((Number)expected).longValue(); }
        JsonArray rows(EventBean[] es) { JsonArray a=new JsonArray(); for(EventBean e:es){ JsonObject f=new JsonObject(); String[] names=e.getEventType().getPropertyNames().clone(); Arrays.sort(names); if(!Arrays.equals(names,new String[]{"max0","maxi","min0","mini"})) throw new IllegalStateException("field metadata mismatch"); for(String name:names) f.add(name, normalize(e.get(name))); a.add(new JsonObject().add("kind","row").add("fields",f)); } return a; }
        JsonValue normalize(Object v) { if(v==null)return new JsonObject().add("state","null"); if(v instanceof Number)return Json.value(((Number)v).longValue()); return Json.value(String.valueOf(v)); }
    }
}
