import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.runtime.client.*;
import java.util.*;

public class OrderProbe2 {
    public static class LocalSupportBean {
        private String theString;
        public String getTheString() { return theString; }
        public void setTheString(String v) { theString = v; }
    }

    public static void main(String[] args) throws Exception {
        Configuration config = new Configuration();
        config.getCommon().addEventType("SupportBean", LocalSupportBean.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("OrderProbe2", config);
        runtime.getEventService().advanceTime(0);

        List<String> order = new ArrayList<>();
        String epl = "select rstream theString from SupportBean#time(2 sec)";

        // first-deployed named "zz", second-deployed named "aa"
        EPCompiled c = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(config));
        DeploymentOptions o1 = new DeploymentOptions(); o1.setStatementNameRuntime(env -> "zz");
        DeploymentOptions o2 = new DeploymentOptions(); o2.setStatementNameRuntime(env -> "aa");
        EPDeployment d1 = runtime.getDeploymentService().deploy(c, o1);
        EPDeployment d2 = runtime.getDeploymentService().deploy(c, o2);
        EPStatement zz = runtime.getDeploymentService().getStatement(d1.getDeploymentId(), "zz");
        EPStatement aa = runtime.getDeploymentService().getStatement(d2.getDeploymentId(), "aa");
        zz.addListener((nd, od, st, rt) -> order.add("zz:" + nd[0].get("theString")));
        aa.addListener((nd, od, st, rt) -> order.add("aa:" + nd[0].get("theString")));

        runtime.getEventService().advanceTime(1000);
        runtime.getEventService().sendEventBean(bean("E1"), "SupportBean");
        runtime.getEventService().advanceTime(3000);
        System.out.println("ORDER " + order);
        runtime.destroy();
    }

    static LocalSupportBean bean(String s) {
        LocalSupportBean b = new LocalSupportBean();
        b.setTheString(s);
        return b;
    }
}
