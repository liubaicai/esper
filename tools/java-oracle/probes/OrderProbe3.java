import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.runtime.client.*;
import java.util.*;

public class OrderProbe3 {
    public static class LocalSupportBean {
        private String theString;
        public String getTheString() { return theString; }
        public void setTheString(String v) { theString = v; }
    }

    public static void main(String[] args) throws Exception {
        Configuration config = new Configuration();
        config.getCommon().addEventType("SupportBean", LocalSupportBean.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("OrderProbe3", config);
        runtime.getEventService().advanceTime(0);

        List<String> order = new ArrayList<>();
        // pattern timer:at statements; first-deployed named "pfirst", second "psecond"
        String epl = "select * from pattern[every timer:at(1, *, *, *, *)]";

        EPCompiled c = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(config));
        DeploymentOptions o1 = new DeploymentOptions(); o1.setStatementNameRuntime(env -> "pfirst");
        DeploymentOptions o2 = new DeploymentOptions(); o2.setStatementNameRuntime(env -> "psecond");
        EPDeployment d1 = runtime.getDeploymentService().deploy(c, o1);
        EPDeployment d2 = runtime.getDeploymentService().deploy(c, o2);
        runtime.getDeploymentService().getStatement(d1.getDeploymentId(), "pfirst").addListener(
            (nd, od, st, rt) -> order.add("pfirst"));
        runtime.getDeploymentService().getStatement(d2.getDeploymentId(), "psecond").addListener(
            (nd, od, st, rt) -> order.add("psecond"));

        runtime.getEventService().advanceTime(60000);
        System.out.println("ORDER3 " + order);
        runtime.destroy();
    }
}
