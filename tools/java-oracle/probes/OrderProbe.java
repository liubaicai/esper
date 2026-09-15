import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.runtime.client.*;
import java.util.*;

public class OrderProbe {
    public static class LocalSupportBean {
        private String theString;
        public String getTheString() { return theString; }
        public void setTheString(String v) { theString = v; }
    }

    public static void main(String[] args) throws Exception {
        Configuration config = new Configuration();
        config.getCommon().addEventType("SupportBean", LocalSupportBean.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("OrderProbe", config);
        runtime.getEventService().advanceTime(0);

        List<String> order = new ArrayList<>();
        String epl = "select rstream theString from SupportBean#time(%d sec)";

        EPCompiled c0 = EPCompilerProvider.getCompiler().compile(String.format(epl, 4), new CompilerArguments(config));
        EPCompiled c1 = EPCompilerProvider.getCompiler().compile(String.format(epl, 3), new CompilerArguments(config));
        DeploymentOptions o0 = new DeploymentOptions();
        o0.setStatementNameRuntime(env -> "s0");
        DeploymentOptions o1 = new DeploymentOptions();
        o1.setStatementNameRuntime(env -> "s1");
        EPDeployment d0 = runtime.getDeploymentService().deploy(c0, o0);
        EPDeployment d1 = runtime.getDeploymentService().deploy(c1, o1);
        EPStatement s0 = runtime.getDeploymentService().getStatement(d0.getDeploymentId(), "s0");
        EPStatement s1 = runtime.getDeploymentService().getStatement(d1.getDeploymentId(), "s1");
        s0.addListener((nd, od, st, rt) -> order.add("s0:" + nd[0].get("theString")));
        s1.addListener((nd, od, st, rt) -> order.add("s1:" + nd[0].get("theString")));

        send(runtime, "E1", 1000);
        send(runtime, "E2", 2000);
        runtime.getEventService().advanceTime(4000);
        runtime.getEventService().advanceTime(5000);
        System.out.println("ORDER " + order);
        runtime.destroy();
    }

    static void send(EPRuntime runtime, String s, long at) throws Exception {
        runtime.getEventService().advanceTime(at);
        Map<String, Object> event = new HashMap<>();
        event.put("theString", s);
        runtime.getEventService().sendEventBean(new LocalSupportBean() {{ setTheString(s); }}, "SupportBean");
    }
}
