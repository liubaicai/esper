import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.PropertyAccessException;
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
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;

/**
 * Java oracle for the expr-class-static-method parity scenario (pinned
 * Esper 9.0.0 commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c). It replays
 * the static-call slice of ExprClassStaticMethod in a fresh runtime per case:
 * local and path-created String calls (the Java SODA twin IDs share a typed
 * replay), local and created FAF calls over a keepall named window, the
 * local-to-created cross-class call, compile-only doc samples, package-
 * qualified calls, and the valid empty-class branch of ExprClassInvalidCompile.
 *
 * Listener records preserve callback order, per-statement sequence, virtual
 * time, and normalized rows. FAF records preserve per-case execution sequence
 * and normalized query rows. Compile-only cases execute every pinned compile
 * operation during fresh-case setup and intentionally emit no listener or FAF
 * record. Ordinal 4's deployed-class-version binding and ordinal 11's compiler
 * inspection callback have no scenario case.
 */
public class ExprClassStaticMethodScenarioOracle {

    private static final String COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";

    private static final String[] CASES = {
        "local",
        "create",
        "local-faf",
        "create-faf",
        "local-and-create",
        "doc-samples",
        "invalid-compile-valid",
        "package-create",
        "package-local"
    };

    private ExprClassStaticMethodScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: ExprClassStaticMethodScenarioOracle <scenario.json>");
            System.exit(2);
        }
        JsonObject scenario = Json.parse(
            Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        JsonArray allSteps = scenario.get("steps").asArray();

        List<JsonObject> records = new ArrayList<>();
        for (String caseName : CASES) {
            if (hasCase(allSteps, caseName)) {
                runCase(allSteps, caseName, records);
            }
        }

        JsonObject root = new JsonObject()
            .add("version", "esper-parity/v1")
            .add("id", scenario.getString("id", ""))
            .add("scenario", scenario.getString("description", ""))
            .add("javaCommit", COMMIT)
            .add("java", System.getProperty("java.version"));
        JsonArray recordsArray = new JsonArray();
        for (JsonObject record : records) {
            recordsArray.add(record);
        }
        root.add("records", recordsArray);
        System.out.println(root.toString());
    }

    private static boolean hasCase(JsonArray allSteps, String wanted) {
        for (int i = 0; i < allSteps.size(); i++) {
            JsonObject step = allSteps.get(i).asObject();
            if ("case".equals(step.getString("op", "")) && wanted.equals(step.getString("case", ""))) {
                return true;
            }
        }
        return false;
    }

    private static void runCase(JsonArray allSteps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = new Configuration();
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        config.getCommon().addEventType("SupportBean", SupportBean.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-efo-" + caseName, config);
        ((EPRuntimeSPI) runtime).initialize(0L);

        CasePlan plan = plans(caseName);
        List<EPCompiled> path = new ArrayList<>();
        int deployIndex = 0;
        int fafCount = 0;
        boolean active = false;
        try {
            for (CompileOnlyStep compileOnly : plan.compileOnly()) {
                compileOnlyStep(config, path, compileOnly);
            }
            for (int i = 0; i < allSteps.size(); i++) {
                JsonObject step = allSteps.get(i).asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    active = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!active) {
                    continue;
                }
                if ("send".equals(op)) {
                    sendEvent(runtime, step);
                    continue;
                }
                if ("deploy".equals(op)) {
                    String label = step.getString("statement", "");
                    if (deployIndex >= plan.modules().length || !label.equals(plan.modules()[deployIndex].label())) {
                        throw new IllegalStateException(
                            "case " + caseName + " deploy step " + deployIndex + " does not match its pinned plan");
                    }
                    deployModule(runtime, config, path, records, caseName, deployIndex, plan.modules()[deployIndex]);
                    deployIndex++;
                    continue;
                }
				if ("faf".equals(op)) {
					if (plan.fafQuery() == null) {
						throw new IllegalStateException("case " + caseName + " has no pinned fire-and-forget query");
					}
					String label = step.getString("statement", "");
					if (!"s0".equals(label)) {
						throw new IllegalStateException(
							"case " + caseName + " faf step does not match its pinned statement, got " + label);
					}
					fafCount++;
					executeFaf(runtime, config, path, records, caseName, plan.fafQuery(), fafCount);
					continue;
				}
				throw new IllegalStateException("unknown op: " + op);
            }
			if (deployIndex != plan.modules().length) {
				throw new IllegalStateException(
					"case " + caseName + " has " + deployIndex + " deploy steps, plan expects " + plan.modules().length);
			}
			if (fafCount != plan.fafExecutions()) {
				throw new IllegalStateException(
					"case " + caseName + " has " + fafCount + " faf steps, plan expects " + plan.fafExecutions());
			}
			runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static void deployModule(EPRuntime runtime, Configuration config, List<EPCompiled> path,
                                     List<JsonObject> records, String caseName, int deployIndex,
                                     ModulePlan module) throws Exception {
        CompilerArguments args = new CompilerArguments(config);
        args.getPath().getCompileds().addAll(path);
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(module.epl(), args);
        path.add(compiled);
        DeploymentOptions options = new DeploymentOptions().setDeploymentId("parity-efo-" + caseName + "-" + deployIndex);
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, options);
        for (String observe : module.observed()) {
            EPStatement statement = findStatement(deployment, observe);
            statement.addListener(new TraceWriter(records, caseName, statement, runtime));
        }
    }

    private static void executeFaf(EPRuntime runtime, Configuration config, List<EPCompiled> path,
                                   List<JsonObject> records, String caseName, String fafEpl,
                                   int sequence) throws Exception {
        CompilerArguments args = new CompilerArguments(config);
        args.getPath().getCompileds().addAll(path);
        EPCompiled compiled = EPCompilerProvider.getCompiler().compileQuery(fafEpl, args);
        EventBean[] rows = runtime.getFireAndForgetService().executeQuery(compiled).getArray();

        JsonObject record = new JsonObject()
            .add("case", caseName)
            .add("operation", "faf")
            .add("statement", "s0")
            .add("sequence", sequence);
        JsonArray newArray = rows(rows);
        if (newArray.size() > 0) {
            record.add("new", newArray);
        }
        records.add(record);
    }

    /** Mirrors the pinned env.compile(epl[, path]) compile-only replay. */
    private static void compileOnlyStep(Configuration config, List<EPCompiled> path,
                                        CompileOnlyStep step) throws Exception {
        CompilerArguments args = new CompilerArguments(config);
        if (step.onPath()) {
            args.getPath().getCompileds().addAll(path);
        }
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(step.epl(), args);
        if (step.addToPath()) {
            path.add(compiled);
        }
    }

    private static EPStatement findStatement(EPDeployment deployment, String name) {
        for (EPStatement statement : deployment.getStatements()) {
            if (name.equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("statement " + name + " was not deployed");
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        if (!"SupportBean".equals(eventType)) {
            throw new IllegalStateException("unknown eventType: " + eventType);
        }
        JsonObject payload = step.get("payload").asObject();
        runtime.getEventService().sendEventBean(
            new SupportBean(payload.getString("theString", null), payload.getInt("intPrimitive", 0)),
            eventType);
    }

    /**
     * Pinned module transcriptions (verbatim from
     * ExprClassStaticMethod.java, including spacing and trailing newlines).
     */
    private static CasePlan plans(String caseName) {
        return switch (caseName) {
            case "local" -> new CasePlan(new ModulePlan[]{
                new ModulePlan("s0", EPL_LOCAL, new String[]{"s0"}),
            }, null, 0, new CompileOnlyStep[]{});
            case "create" -> new CasePlan(new ModulePlan[]{
                new ModulePlan("create-class", EPL_CREATE_CLASS, new String[]{}),
                new ModulePlan("s0", EPL_CREATE_QUERY, new String[]{"s0"}),
            }, null, 0, new CompileOnlyStep[]{});
            case "local-faf" -> new CasePlan(new ModulePlan[]{
                new ModulePlan("window", EPL_WINDOW_LOCAL_FAF, new String[]{}),
            }, EPL_FAF_LOCAL, 2, new CompileOnlyStep[]{});
            case "create-faf" -> new CasePlan(new ModulePlan[]{
                new ModulePlan("window", EPL_WINDOW_CREATE_FAF, new String[]{}),
            }, EPL_FAF_CREATE, 2, new CompileOnlyStep[]{});
            case "local-and-create" -> new CasePlan(new ModulePlan[]{
                new ModulePlan("classes", EPL_CLASSES_TOGETHER, new String[]{}),
                new ModulePlan("s0", EPL_TOGETHER_QUERY, new String[]{"s0"}),
            }, null, 0, new CompileOnlyStep[]{});
            case "doc-samples" -> new CasePlan(new ModulePlan[]{}, null, 0, new CompileOnlyStep[]{
                // env.compile(epl) with no path; the compiled never joins one.
                new CompileOnlyStep(EPL_DOC_FIB, false, false),
                // env.compile(eplCreate, path): compiled against the empty
                // path, then added to it.
                new CompileOnlyStep(EPL_DOC_MIDPRICE_CREATE, true, true),
                // env.compile(epl, path) resolving the created class.
                new CompileOnlyStep(EPL_DOC_MIDPRICE_QUERY, true, false),
            });
            case "invalid-compile-valid" -> new CasePlan(new ModulePlan[]{}, null, 0, new CompileOnlyStep[]{
                // Pinned case 1: empty class text compiles.
                new CompileOnlyStep(EPL_INVALID_EMPTY_CLASS, false, false),
            });
            case "package-create" -> new CasePlan(new ModulePlan[]{
                new ModulePlan("s0", EPL_PACKAGE_CREATE, new String[]{"s0"}),
            }, null, 0, new CompileOnlyStep[]{});
            case "package-local" -> new CasePlan(new ModulePlan[]{
                new ModulePlan("s0", EPL_PACKAGE_LOCAL, new String[]{"s0"}),
            }, null, 0, new CompileOnlyStep[]{});
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    // ExprClassStaticMethodLocal (lines 337-344): one module, local class.
    private static final String EPL_LOCAL =
        "@name('s0') inlined_class \"\"\"\n" +
        "    public class MyClass {\n" +
        "        public static String doIt(String parameter) {\n" +
        "            return \"|\" + parameter + \"|\";\n" +
        "        }\n" +
        "    }\n" +
        "\"\"\" " +
        "select MyClass.doIt(theString) as c0 from SupportBean\n";

    // ExprClassStaticMethodCreate (lines 302-310): class module + query.
    private static final String EPL_CREATE_CLASS =
        "@public create inlined_class \"\"\"\n" +
        "    public class MyClass {\n" +
        "        public static String doIt(String parameter) {\n" +
        "            return \"|\" + parameter + \"|\";\n" +
        "        }\n" +
        "    }\n" +
        "\"\"\"";
    private static final String EPL_CREATE_QUERY =
        "@name('s0') select MyClass.doIt(theString) as c0 from SupportBean";

    // ExprClassStaticMethodLocalFAFQuery (lines 205-217).
    private static final String EPL_WINDOW_LOCAL_FAF =
        "@public create window MyWindow#keepall as (theString string);\n" +
        "on SupportBean merge MyWindow insert select theString;\n";
    private static final String EPL_FAF_LOCAL =
        "inlined_class \"\"\"\n" +
        "    public class MyClass {\n" +
        "        public static String doIt(String parameter) {\n" +
        "            return '>' + parameter + '<';\n" +
        "        }\n" +
        "    }\n" +
        "\"\"\"\n select MyClass.doIt(theString) as c0 from MyWindow";

    // ExprClassStaticMethodCreateFAFQuery (lines 171-186).
    private static final String EPL_WINDOW_CREATE_FAF =
        "@public create inlined_class \"\"\"\n" +
        "    public class MyClass {\n" +
        "        public static String doIt(String parameter) {\n" +
        "            return \"abc\";\n" +
        "        }\n" +
        "    }\n" +
        "\"\"\";\n" +
        "@public create window MyWindow#keepall as (theString string);\n" +
        "on SupportBean merge MyWindow insert select theString;\n";
    private static final String EPL_FAF_CREATE =
        "select MyClass.doIt(theString) as c0 from MyWindow";

    // ExprClassStaticMethodLocalAndCreateClassTogether (lines 104-121).
    private static final String EPL_CLASSES_TOGETHER =
        "inlined_class \"\"\"\n" +
        "    public class MyUtil {\n" +
        "        public static String returnBubba() {\n" +
        "            return \"bubba\";\n" +
        "        }\n" +
        "    }\n" +
        "\"\"\" \n" +
        "@public create inlined_class \"\"\"\n" +
        "    public class MyClass {\n" +
        "        public static String doIt() {\n" +
        "            return \"|\" + MyUtil.returnBubba() + \"|\";\n" +
        "        }\n" +
        "    }\n" +
        "\"\"\"\n";
    private static final String EPL_TOGETHER_QUERY =
        "@name('s0') select MyClass.doIt() as c0 from SupportBean\n";

    // ExprClassDocSamples (lines 76-97).
    private static final String EPL_DOC_FIB =
        "inlined_class \"\"\"\n" +
        "  public class MyUtility {\n" +
        "    public static double fib(int n) {\n" +
        "      if (n <= 1)\n" +
        "        return n;\n" +
        "      return fib(n-1) + fib(n-2);\n" +
        "    }\n" +
        "  }\n" +
        "\"\"\"\n" +
        "select MyUtility.fib(intPrimitive) from SupportBean";
    private static final String EPL_DOC_MIDPRICE_CREATE =
        "@public create inlined_class \"\"\" \n" +
        "  public class MyUtility {\n" +
        "    public static double midPrice(double buy, double sell) {\n" +
        "      return (buy + sell) / 2;\n" +
        "    }\n" +
        "  }\n" +
        "\"\"\"";
    private static final String EPL_DOC_MIDPRICE_QUERY =
        "select MyUtility.midPrice(doublePrimitive, doubleBoxed) from SupportBean";

    // ExprClassInvalidCompile case 1 (line 237): empty class text is valid.
    private static final String EPL_INVALID_EMPTY_CLASS =
        "inlined_class \"\"\" \"\"\" select * from SupportBean";

    // ExprClassStaticMethodCreateClassWithPackageName (lines 150-159).
    private static final String EPL_PACKAGE_CREATE =
        "create inlined_class \"\"\"\n" +
        "    package mypackage;" +
        "    public class MyUtil {\n" +
        "        public static String doIt(String theString, int intPrimitive) {\n" +
        "            return theString + Integer.toString(intPrimitive);\n" +
        "        }\n" +
        "    }\n" +
        "\"\"\";\n" +
        "@name('s0') select mypackage.MyUtil.doIt(theString, intPrimitive) as c0 from SupportBean;\n";

    // ExprClassStaticMethodLocalWithPackageName (lines 131-139).
    private static final String EPL_PACKAGE_LOCAL =
        "@name('s0') inlined_class \"\"\"\n" +
        "    package mypackage;" +
        "    public class MyUtil {\n" +
        "        public static String doIt() {\n" +
        "            return \"test\";\n" +
        "        }\n" +
        "    }\n" +
        "\"\"\" \n" +
        "select mypackage.MyUtil.doIt() as c0 from SupportBean\n";

    private record ModulePlan(String label, String epl, String[] observed) {
    }

    private record CompileOnlyStep(String epl, boolean onPath, boolean addToPath) {
    }

    private record CasePlan(ModulePlan[] modules, String fafQuery, int fafExecutions,
                            CompileOnlyStep[] compileOnly) {
    }

    private static final class TraceWriter implements UpdateListener {
        private final List<JsonObject> records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(List<JsonObject> records, String caseName, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored, EPRuntime ignoredRuntime) {
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if (!hasNew && !hasOld) {
                return;
            }
            sequence++;
            String time = Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString();
            JsonObject record = new JsonObject()
                .add("case", caseName)
                .add("operation", "listener")
                .add("statement", statement.getName())
                .add("sequence", sequence)
                .add("time", time);
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
    }

    private static JsonArray rows(EventBean[] events) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            JsonObject fields = new JsonObject();
            String[] names = event.getEventType().getPropertyNames().clone();
            java.util.Arrays.sort(names);
            for (String name : names) {
                Object value;
                try {
                    value = event.get(name);
                } catch (PropertyAccessException unreadable) {
                    continue;
                }
                fields.add(name, normalize(value));
            }
            array.add(new JsonObject().add("kind", "row").add("fields", fields));
        }
        return array;
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short || value instanceof Byte) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).doubleValue());
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        return Json.value(String.valueOf(value));
    }
}
