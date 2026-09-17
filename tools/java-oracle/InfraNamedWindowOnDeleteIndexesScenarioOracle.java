import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.epl.namedwindow.core.NamedWindow;
import com.espertech.esper.common.internal.epl.namedwindow.core.NamedWindowInstance;
import com.espertech.esper.common.internal.epl.namedwindow.core.NamedWindowManagementService;
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
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the named-window on-delete implicit-index
 * parity scenario. Mirrors InfraNamedWindowOnDelete ordinals 1-4:
 * InfraStaggeredNamedWindow (DEFAULT event representation only, per the
 * approved-difference precedent), InfraCoercionKeyMultiPropIndexes,
 * InfraCoercionRangeMultiPropIndexes and
 * InfraCoercionKeyAndRangeMultiPropIndexes.
 *
 * <p>Every statement deploys as its own single-statement module against the
 * shared runtime path (the RegressionPath equivalent), so the on-delete and
 * on-select triggers resolve the @public windows. Listeners attach to the
 * createOne/createTwo/delete statements; deployed markers follow each deploy;
 * "index-count" records pin SupportInfraUtil.getIndexCountNoContext
 * (of=indexes) or getDataWindowCountNoContext (of=rows, staggered case) via
 * the create statement's deployment; "snapshot" records pin the create
 * statement iterator in insertion order (assertPropsPerRowIterator).
 */
public final class InfraNamedWindowOnDeleteIndexesScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-namedwindow-on-delete-indexes";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowOnDelete.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-0dcb2b72f505c7931a45",
            "java-runtime-c4c336036fdd92d803f7",
            "java-runtime-a58ae70ca908579af50a",
            "java-runtime-2d6d018e91b5664f3734"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraStaggeredNamedWindow",
            "InfraCoercionKeyMultiPropIndexes",
            "InfraCoercionRangeMultiPropIndexes",
            "InfraCoercionKeyAndRangeMultiPropIndexes"
    };
    private static final String[] STATIC_IDS = {
            "java-06bf0eb71230b3119293",
            "java-06bf0eb71230b3119293",
            "java-06bf0eb71230b3119293",
            "java-06bf0eb71230b3119293"
    };
    private static final String[] JAVA_FLAGS = {"STATICHOOK"};
    private static final String[] CASES = {
            "staggered", "coercion-key", "coercion-range", "coercion-key-range"};
    private static final int[] ORDINALS = {1, 2, 3, 4};

    // Verbatim transcriptions of InfraNamedWindowOnDelete.java: staggered
    // lines 476-495 (DEFAULT representation contributes no annotation text,
    // leaving the leading space on createTwo), coercion-key lines 169-212,
    // 274 and 289-293, coercion-range lines 304-365, coercion-key-range
    // lines 387-428 (including the trailing spaces on d2/d3).
    private static final String EPL_STAG_CREATE_ONE =
            "@name('createOne') @public create window MyWindowSTAG#keepall as select "
                    + "theString as a1, intPrimitive as b1 from SupportBean";
    private static final String EPL_STAG_CREATE_TWO =
            " @name('createTwo') @public create window MyWindowSTAGTwo#keepall as select "
                    + "theString as a2, intPrimitive as b2 from SupportBean";
    private static final String EPL_STAG_DELETE =
            "@name('delete') on MyWindowSTAG delete from MyWindowSTAGTwo where a1 = a2";
    private static final String EPL_STAG_INSERT =
            "@name('insert') insert into MyWindowSTAG select theString as a1, "
                    + "intPrimitive as b1 from SupportBean(intPrimitive > 0)";
    private static final String EPL_STAG_INSERT_TWO =
            "@name('insertTwo') insert into MyWindowSTAGTwo select theString as a2, "
                    + "intPrimitive as b2 from SupportBean(intPrimitive < 0)";

    private static final String EPL_CK_CREATE_ONE =
            "@name('createOne') @public create window MyWindowCK#keepall as select "
                    + "theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed "
                    + "from SupportBean";
    private static final String EPL_CK_D1 =
            "@name('d1') on SupportBean(theString='DB') as s0 delete from MyWindowCK as win "
                    + "where win.intPrimitive = s0.doubleBoxed";
    private static final String EPL_CK_D2 =
            "@name('d2') on SupportBean(theString='DP') as s0 delete from MyWindowCK as win "
                    + "where win.intPrimitive = s0.doublePrimitive";
    private static final String EPL_CK_D3 =
            "@name('d3') on SupportBean(theString='IB') as s0 delete from MyWindowCK "
                    + "where MyWindowCK.intPrimitive = s0.intBoxed";
    private static final String EPL_CK_D4 =
            "@name('d4') on SupportBean(theString='IPDP') as s0 delete from MyWindowCK as win "
                    + "where win.intPrimitive = s0.intPrimitive and win.doublePrimitive = s0.doublePrimitive";
    private static final String EPL_CK_D5 =
            "@name('d5') on SupportBean(theString='IPDP2') as s0 delete from MyWindowCK as win "
                    + "where win.doublePrimitive = s0.doublePrimitive and win.intPrimitive = s0.intPrimitive";
    private static final String EPL_CK_D6 =
            "@name('d6') on SupportBean(theString='IPDPIB') as s0 delete from MyWindowCK as win "
                    + "where win.doublePrimitive = s0.doublePrimitive and win.intPrimitive = s0.intPrimitive "
                    + "and win.intBoxed = s0.intBoxed";
    private static final String EPL_CK_D7 =
            "@name('d7') on SupportBean(theString='CAST') as s0 delete from MyWindowCK as win "
                    + "where win.intBoxed = s0.intPrimitive and win.doublePrimitive = s0.doubleBoxed "
                    + "and win.intPrimitive = s0.intBoxed";
    private static final String EPL_CK_INSERT =
            "insert into MyWindowCK select theString, intPrimitive, intBoxed, doublePrimitive, "
                    + "doubleBoxed from SupportBean(theString like 'E%')";
    private static final String EPL_CK_D0 =
            "@name('d0') on SupportBean(theString='LAST') as s0 delete from MyWindowCK as win "
                    + "where win.intPrimitive = s0.intPrimitive and win.doublePrimitive = s0.doublePrimitive";
    private static final String EPL_CK_CREATE_TWO =
            "@name('createTwo') @public create window WinOne#keepall as SupportBean";
    private static final String EPL_CK_SELECT_ONE =
            "on SupportBean_ST0 select * from WinOne where theString = key0";
    private static final String EPL_CK_SELECT_TWO =
            "on SupportBean_ST0 select * from WinOne where theString = key0 and intPrimitive = p00";

    private static final String EPL_CR_CREATE_ONE =
            "@name('createOne') @public create window MyWindowCR#keepall as select "
                    + "theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed "
                    + "from SupportBean";
    private static final String EPL_CR_INSERT =
            "insert into MyWindowCR select theString, intPrimitive, intBoxed, doublePrimitive, "
                    + "doubleBoxed from SupportBean";
    private static final String EPL_CR_D0 =
            "@name('d0') on SupportBeanTwo as s2 delete from MyWindowCR as win "
                    + "where win.intPrimitive between s2.doublePrimitiveTwo and s2.doubleBoxedTwo";
    private static final String EPL_CR_D1 =
            "@name('d1') on SupportBeanTwo as s2 delete from MyWindowCR as win "
                    + "where win.intPrimitive between s2.intPrimitiveTwo and s2.intBoxedTwo";
    private static final String EPL_CR_D2 =
            "@name('d2') on SupportBeanTwo as s2 delete from MyWindowCR as win "
                    + "where win.intPrimitive between s2.intPrimitiveTwo and s2.intBoxedTwo "
                    + "and win.doublePrimitive between s2.intPrimitiveTwo and s2.intBoxedTwo";
    private static final String EPL_CR_D3 =
            "@name('d3') on SupportBeanTwo as s2 delete from MyWindowCR as win "
                    + "where win.doublePrimitive between s2.intPrimitiveTwo and s2.intPrimitiveTwo "
                    + "and win.intPrimitive between s2.intPrimitiveTwo and s2.intPrimitiveTwo";
    private static final String EPL_CR_D4 =
            "@name('d4') on SupportBeanTwo as s2 delete from MyWindowCR as win "
                    + "where win.intPrimitive <= doublePrimitiveTwo";
    private static final String EPL_CR_D5 =
            "@name('d5') on SupportBeanTwo as s2 delete from MyWindowCR as win "
                    + "where win.intPrimitive not between s2.intPrimitiveTwo and s2.intBoxedTwo";

    private static final String EPL_CKR_CREATE_ONE =
            "@name('createOne') @public create window MyWindowCKR#keepall as select "
                    + "theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed "
                    + "from SupportBean";
    private static final String EPL_CKR_INSERT =
            "insert into MyWindowCKR select theString, intPrimitive, intBoxed, doublePrimitive, "
                    + "doubleBoxed from SupportBean";
    private static final String EPL_CKR_D0 =
            "@name('d0') on SupportBeanTwo delete from MyWindowCKR "
                    + "where theString = stringTwo and intPrimitive between doublePrimitiveTwo "
                    + "and doubleBoxedTwo";
    private static final String EPL_CKR_D1 =
            "@name('d1') on SupportBeanTwo delete from MyWindowCKR "
                    + "where theString = stringTwo and intPrimitive = intPrimitiveTwo "
                    + "and intBoxed between doublePrimitiveTwo and doubleBoxedTwo";
    private static final String EPL_CKR_D2 =
            "@name('d2') on SupportBeanTwo delete from MyWindowCKR "
                    + "where intBoxed between doubleBoxedTwo and doublePrimitiveTwo "
                    + "and intPrimitive = intPrimitiveTwo and theString = stringTwo ";
    private static final String EPL_CKR_D3 =
            "@name('d3') on SupportBeanTwo delete from MyWindowCKR "
                    + "where intBoxed between intBoxedTwo and intBoxedTwo "
                    + "and intPrimitive = intPrimitiveTwo and theString = stringTwo ";

    private static final Set<String> LISTENED =
            new HashSet<>(Arrays.asList("createOne", "createTwo", "delete"));
    private static final int EXPECTED_STEPS = 185;

    private InfraNamedWindowOnDeleteIndexesScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowOnDeleteIndexesScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBeanTwo.class);
        configuration.getCommon().addEventType(SupportBean_ST0.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-oracle", configuration);
        runtime.getEventService().advanceTime(0);

        JsonArray records = new JsonArray();
        try {
            for (String caseName : CASES) {
                runCase(caseName, runtime, allSteps, records);
            }
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", ID);
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        root.add("records", records);
        System.out.println(root.toString());
    }

    /** Replays one case's steps on the shared runtime; sequences restart per case. */
    private static void runCase(String caseName, EPRuntime runtime, JsonArray allSteps,
                                JsonArray records) throws Exception {
        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statements = new HashMap<>();
        // The Java executions compile every statement against one
        // accumulating RegressionPath; undeployAll starts a fresh path.
        // A fresh runtime path per deploy resolves the named window
        // through public visibility instead of the path entry and
        // produces different implicit-index inference (d4/d5 collapse).
        CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
        boolean inCase = false;
        for (JsonValue stepValue : allSteps) {
            JsonObject step = stepValue.asObject();
            String operation = string(step, "op");
            if ("case".equals(operation)) {
                inCase = caseName.equals(string(step, "case"));
                continue;
            }
            if (!inCase) {
                continue;
            }
            switch (operation) {
                case "deploy": {
                    String label = string(step, "statement");
                    EPCompiled compiled = EPCompilerProvider.getCompiler()
                            .compile(string(step, "epl"), compilerArgs);
                    EPDeployment deployment = runtime.getDeploymentService()
                            .deploy(compiled, new DeploymentOptions());
                    compilerArgs.getPath().add(compiled);
                    EPStatement[] deployed = deployment.getStatements();
                    if (deployed.length != 1) {
                        throw new IllegalStateException("deploy of " + label + " produced "
                                + deployed.length + " statements, want 1");
                    }
                    EPStatement statement = deployed[0];
                    if (LISTENED.contains(statement.getName())) {
                        statement.addListener(listener(caseName, sequences, records, runtime));
                    }
                    statements.put(label, statement);
                    break;
                }
                case "deployed": {
                    String label = string(step, "statement");
                    if (!statements.containsKey(label)) {
                        throw new IllegalStateException(
                                "deployed marker for unknown statement " + label);
                    }
                    int sequence = sequences.merge(label + ":deployed", 1, Integer::sum);
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "deployed");
                    record.add("statement", label);
                    record.add("sequence", sequence);
                    record.add("time", Instant.ofEpochMilli(
                            runtime.getEventService().getCurrentTime()).toString());
                    records.add(record);
                    break;
                }
                case "send":
                    sendEvent(runtime, string(step, "eventType"),
                            object(step.get("payload"), "payload"));
                    break;
                case "snapshot": {
                    String label = string(step, "statement");
                    EPStatement statement = statements.get(label);
                    if (statement == null) {
                        throw new IllegalStateException(
                                "snapshot targets unknown statement " + label);
                    }
                    JsonArray rows = new JsonArray();
                    for (Iterator<EventBean> iterator = statement.iterator();
                         iterator.hasNext(); ) {
                        rows.add(row(iterator.next()));
                    }
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "snapshot");
                    record.add("statement", label);
                    record.add("sequence", 0);
                    record.add("time", Instant.ofEpochMilli(
                            runtime.getEventService().getCurrentTime()).toString());
                    record.add("new", rows);
                    records.add(record);
                    break;
                }
                case "index-count": {
                    String windowName = string(step, "statement");
                    String createLabel = string(step, "create");
                    String of = string(step, "of");
                    EPStatement create = statements.get(createLabel);
                    if (create == null) {
                        throw new IllegalStateException(
                                "index-count targets unknown create statement " + createLabel);
                    }
                    NamedWindowInstance instance = namedWindowInstance(
                            runtime, create.getDeploymentId(), windowName);
                    long actual;
                    if ("rows".equals(of)) {
                        actual = instance.getCountDataWindow();
                    } else if ("indexes".equals(of)) {
                        actual = instance.getIndexDescriptors().length;
                    } else {
                        throw new IllegalArgumentException("unknown index-count kind: " + of);
                    }
                    long expected = longInteger(step.get("count"), "count");
                    if (actual != expected) {
                        throw new IllegalStateException("index-count mismatch for " + windowName
                                + " (" + of + "): expected " + expected + ", got " + actual);
                    }
                    int sequence = sequences.merge(windowName + ":index-count", 1, Integer::sum);
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "index-count");
                    record.add("statement", windowName);
                    record.add("sequence", sequence);
                    record.add("time", Instant.ofEpochMilli(
                            runtime.getEventService().getCurrentTime()).toString());
                    record.add("count", actual);
                    records.add(record);
                    break;
                }
                case "undeploy": {
                    // Mirrors undeployModuleContaining: the deployment holding
                    // the statement is removed, so every statement of that
                    // module leaves the registry.
                    String label = string(step, "statement");
                    EPStatement statement = statements.get(label);
                    if (statement == null) {
                        throw new IllegalStateException(
                                "undeploy targets unknown statement " + label);
                    }
                    String deploymentId = statement.getDeploymentId();
                    runtime.getDeploymentService().undeploy(deploymentId);
                    statements.values().removeIf(
                            registered -> deploymentId.equals(registered.getDeploymentId()));
                    break;
                }
                case "undeploy-all":
                    runtime.getDeploymentService().undeployAll();
                    statements.clear();
                    compilerArgs = new CompilerArguments(runtime.getRuntimePath());
                    break;
                default:
                    throw new IllegalArgumentException("unknown op: " + operation);
            }
        }
        runtime.getDeploymentService().undeployAll();
        statements.clear();
    }

    /** Mirrors SupportInfraUtil.getNamedWindow: the named window registered
     * under the create statement's deployment, no context partition. */
    private static NamedWindowInstance namedWindowInstance(EPRuntime runtime, String deploymentId,
                                                           String windowName) {
        EPRuntimeSPI spi = (EPRuntimeSPI) runtime;
        NamedWindowManagementService managementService =
                spi.getServicesContext().getNamedWindowManagementService();
        NamedWindow namedWindow = managementService.getNamedWindow(deploymentId, windowName);
        if (namedWindow == null) {
            throw new IllegalStateException("failed to find named window '" + windowName
                    + "' under deployment " + deploymentId);
        }
        return namedWindow.getNamedWindowInstance(null);
    }

    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime) {
        return (newEvents, oldEvents, statement, epRuntime) -> {
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time", Instant.ofEpochMilli(
                    runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            JsonArray oldRows = rows(oldEvents);
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        };
    }

    /** Canonical row rendering with sorted property names for a stable field order. */
    private static JsonArray rows(EventBean[] events) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            array.add(row(event));
        }
        return array;
    }

    private static JsonObject row(EventBean event) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        String[] names = event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        JsonObject fields = new JsonObject();
        for (String name : names) {
            fields.add(name, normalize(event.get(name)));
        }
        item.add("fields", fields);
        return item;
    }

    /** Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, and null as the tagged
     * {"state":"null"} object. */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short
                || value instanceof Byte) {
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

    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                SupportBean bean = new SupportBean();
                bean.setTheString(nullableString(payload, "theString"));
                bean.setIntPrimitive((int) longInteger(payload.get("intPrimitive"), "intPrimitive"));
                bean.setIntBoxed(nullableInt(payload.get("intBoxed")));
                bean.setDoublePrimitive(nullableDouble(payload.get("doublePrimitive"), 0d));
                bean.setDoubleBoxed(nullableBoxedDouble(payload.get("doubleBoxed")));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBeanTwo": {
                SupportBeanTwo bean = new SupportBeanTwo();
                bean.setStringTwo(nullableString(payload, "stringTwo"));
                bean.setIntPrimitiveTwo(
                        (int) longInteger(payload.get("intPrimitiveTwo"), "intPrimitiveTwo"));
                bean.setIntBoxedTwo(nullableInt(payload.get("intBoxedTwo")));
                bean.setDoublePrimitiveTwo(
                        nullableDouble(payload.get("doublePrimitiveTwo"), 0d));
                bean.setDoubleBoxedTwo(nullableBoxedDouble(payload.get("doubleBoxedTwo")));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_ST0": {
                runtime.getEventService().sendEventBean(new SupportBean_ST0(
                        nullableString(payload, "id"),
                        nullableString(payload, "key0"),
                        (int) longInteger(payload.get("p00"), "p00")), type);
                break;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
    }

    private static String nullableString(JsonObject payload, String name) {
        JsonValue value = payload.get(name);
        return value == null || value.isNull() ? null : value.asString();
    }

    private static Integer nullableInt(JsonValue value) {
        return value == null || value.isNull() ? null : (int) longInteger(value, "intBoxed");
    }

    private static Double nullableBoxedDouble(JsonValue value) {
        return value == null || value.isNull() ? null : doubleValue(value, "doubleBoxed");
    }

    private static double nullableDouble(JsonValue value, double fallback) {
        return value == null || value.isNull() ? fallback : doubleValue(value, "doublePrimitive");
    }

    private static double doubleValue(JsonValue value, String name) {
        if (value == null || !value.isNumber()) {
            throw new IllegalArgumentException("missing numeric field: " + name);
        }
        return Double.parseDouble(value.toString());
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), JAVA_FLAGS, "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + CASES.length + " cases");
        }
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        offset = validateStaggered(steps, offset);
        offset = validateCoercionKey(steps, offset);
        offset = validateCoercionRange(steps, offset);
        offset = validateCoercionKeyRange(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact staggered-case step sequence mirroring tryAssertionStaggered lines
     * 476-528: five single-statement deploys with row-count pins, four
     * SupportBean sends with iterator snapshots, then the five per-statement
     * undeploys in Java order.
     */
    private static int validateStaggered(JsonArray steps, int offset) {
        String caseName = "staggered";
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "createOne", EPL_STAG_CREATE_ONE);
        validateDeployed(steps.get(offset++), caseName, "createOne");
        validateIndexCount(steps.get(offset++), caseName, "MyWindowSTAG", "createOne", "rows", 0);
        validateDeploy(steps.get(offset++), caseName, "createTwo", EPL_STAG_CREATE_TWO);
        validateDeployed(steps.get(offset++), caseName, "createTwo");
        validateIndexCount(steps.get(offset++), caseName, "MyWindowSTAGTwo", "createTwo", "rows", 0);
        validateDeploy(steps.get(offset++), caseName, "delete", EPL_STAG_DELETE);
        validateDeployed(steps.get(offset++), caseName, "delete");
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_STAG_INSERT);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateDeploy(steps.get(offset++), caseName, "insertTwo", EPL_STAG_INSERT_TWO);
        validateDeployed(steps.get(offset++), caseName, "insertTwo");
        validateBeanSend(steps.get(offset++), caseName, "E1", -10);
        validateSnapshot(steps.get(offset++), caseName, "createTwo");
        validateIndexCount(steps.get(offset++), caseName, "MyWindowSTAGTwo", "createTwo", "rows", 1);
        validateBeanSend(steps.get(offset++), caseName, "E2", 5);
        validateSnapshot(steps.get(offset++), caseName, "createOne");
        validateIndexCount(steps.get(offset++), caseName, "MyWindowSTAG", "createOne", "rows", 1);
        validateBeanSend(steps.get(offset++), caseName, "E3", -1);
        validateSnapshot(steps.get(offset++), caseName, "createTwo");
        validateIndexCount(steps.get(offset++), caseName, "MyWindowSTAGTwo", "createTwo", "rows", 2);
        validateBeanSend(steps.get(offset++), caseName, "E3", 1);
        validateSnapshot(steps.get(offset++), caseName, "createOne");
        validateSnapshot(steps.get(offset++), caseName, "createTwo");
        validateIndexCount(steps.get(offset++), caseName, "MyWindowSTAG", "createOne", "rows", 2);
        validateIndexCount(steps.get(offset++), caseName, "MyWindowSTAGTwo", "createTwo", "rows", 1);
        validateUndeploy(steps.get(offset++), caseName, "delete");
        validateUndeploy(steps.get(offset++), caseName, "insert");
        validateUndeploy(steps.get(offset++), caseName, "insertTwo");
        validateUndeploy(steps.get(offset++), caseName, "createOne");
        validateUndeploy(steps.get(offset++), caseName, "createTwo");
        return offset;
    }

    /**
     * Exact coercion-key step sequence mirroring InfraCoercionKeyMultiPropIndexes
     * lines 167-296: createOne, seven deploy+index-count pairs (1,1,2,3,4,5,6),
     * the 'E%' insert, twenty sends with nine iterator snapshots, per-statement
     * undeploys of d1..d7, the late d0 delete, index-count 0, then phase B
     * (fresh path: WinOne plus two on-SELECT index pins) ending in undeploy-all.
     */
    private static int validateCoercionKey(JsonArray steps, int offset) {
        String caseName = "coercion-key";
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "createOne", EPL_CK_CREATE_ONE);
        validateDeployed(steps.get(offset++), caseName, "createOne");
        String[] labels = {"d1", "d2", "d3", "d4", "d5", "d6", "d7"};
        String[] epls = {EPL_CK_D1, EPL_CK_D2, EPL_CK_D3, EPL_CK_D4, EPL_CK_D5,
                EPL_CK_D6, EPL_CK_D7};
        int[] counts = {1, 1, 2, 3, 4, 5, 6};
        for (int index = 0; index < labels.length; index++) {
            validateDeploy(steps.get(offset++), caseName, labels[index], epls[index]);
            validateDeployed(steps.get(offset++), caseName, labels[index]);
            validateIndexCount(steps.get(offset++), caseName, "MyWindowCK", "createOne",
                    "indexes", counts[index]);
        }
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_CK_INSERT);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateBeanSend(steps.get(offset++), caseName, "E1", 1, 10, 100d, 1000d);
        validateBeanSend(steps.get(offset++), caseName, "E2", 2, 20, 200d, 2000d);
        validateBeanSend(steps.get(offset++), caseName, "E3", 3, 30, 300d, 3000d);
        validateBeanSend(steps.get(offset++), caseName, "E4", 4, 40, 400d, 4000d);
        validateSnapshot(steps.get(offset++), caseName, "createOne");
        validateBeanSend(steps.get(offset++), caseName, "DB", 0, 0, 0d, null);
        validateBeanSend(steps.get(offset++), caseName, "DB", 0, 0, 0d, 3d);
        validateSnapshot(steps.get(offset++), caseName, "createOne");
        validateBeanSend(steps.get(offset++), caseName, "DP", 0, 0, 5d, null);
        validateBeanSend(steps.get(offset++), caseName, "DP", 0, 0, 4d, null);
        validateSnapshot(steps.get(offset++), caseName, "createOne");
        validateBeanSend(steps.get(offset++), caseName, "IB", 0, -1, 0d, null);
        validateBeanSend(steps.get(offset++), caseName, "IB", 0, 1, 0d, null);
        validateSnapshot(steps.get(offset++), caseName, "createOne");
        validateBeanSend(steps.get(offset++), caseName, "E5", 5, 50, 500d, 5000d);
        validateBeanSend(steps.get(offset++), caseName, "E6", 6, 60, 600d, 6000d);
        validateBeanSend(steps.get(offset++), caseName, "E7", 7, 70, 700d, 7000d);
        validateBeanSend(steps.get(offset++), caseName, "IPDP", 5, 0, 500d, null);
        validateSnapshot(steps.get(offset++), caseName, "createOne");
        validateBeanSend(steps.get(offset++), caseName, "IPDP2", 6, 0, 600d, null);
        validateSnapshot(steps.get(offset++), caseName, "createOne");
        validateBeanSend(steps.get(offset++), caseName, "IPDPIB", 7, 70, 0d, null);
        validateBeanSend(steps.get(offset++), caseName, "IPDPIB", 7, 70, 700d, null);
        validateSnapshot(steps.get(offset++), caseName, "createOne");
        validateBeanSend(steps.get(offset++), caseName, "E8", 8, 80, 800d, 8000d);
        validateSnapshot(steps.get(offset++), caseName, "createOne");
        validateBeanSend(steps.get(offset++), caseName, "CAST", 80, 8, 0d, 800d);
        validateSnapshot(steps.get(offset++), caseName, "createOne");
        for (String label : labels) {
            validateUndeploy(steps.get(offset++), caseName, label);
        }
        validateDeploy(steps.get(offset++), caseName, "d0", EPL_CK_D0);
        validateDeployed(steps.get(offset++), caseName, "d0");
        validateBeanSend(steps.get(offset++), caseName, "LAST", 2, 20, 200d, 2000d);
        validateSnapshot(steps.get(offset++), caseName, "createOne");
        validateUndeploy(steps.get(offset++), caseName, "d0");
        validateIndexCount(steps.get(offset++), caseName, "MyWindowCK", "createOne",
                "indexes", 0);
        validateUndeployAll(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "createTwo", EPL_CK_CREATE_TWO);
        validateDeployed(steps.get(offset++), caseName, "createTwo");
        validateDeploy(steps.get(offset++), caseName, "select1", EPL_CK_SELECT_ONE);
        validateDeployed(steps.get(offset++), caseName, "select1");
        validateIndexCount(steps.get(offset++), caseName, "WinOne", "createTwo",
                "indexes", 1);
        validateDeploy(steps.get(offset++), caseName, "select2", EPL_CK_SELECT_TWO);
        validateDeployed(steps.get(offset++), caseName, "select2");
        validateIndexCount(steps.get(offset++), caseName, "WinOne", "createTwo",
                "indexes", 2);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact coercion-range step sequence mirroring InfraCoercionRangeMultiPropIndexes
     * lines 302-380: createOne plus unfiltered insert, six SupportBean sends,
     * then six interleaved deploy+index-count+sendTwo groups (1,2,3,4,4,4),
     * per-statement undeploys of d0..d5, index-count 0, undeploy-all.
     */
    private static int validateCoercionRange(JsonArray steps, int offset) {
        String caseName = "coercion-range";
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "createOne", EPL_CR_CREATE_ONE);
        validateDeployed(steps.get(offset++), caseName, "createOne");
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_CR_INSERT);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateBeanSend(steps.get(offset++), caseName, "E1", 1, 10, 100d, 1000d);
        validateBeanSend(steps.get(offset++), caseName, "E2", 2, 20, 200d, 2000d);
        validateBeanSend(steps.get(offset++), caseName, "E3", 3, 30, 3d, 30d);
        validateBeanSend(steps.get(offset++), caseName, "E4", 4, 40, 4d, 40d);
        validateBeanSend(steps.get(offset++), caseName, "E5", 5, 50, 500d, 5000d);
        validateBeanSend(steps.get(offset++), caseName, "E6", 6, 60, 600d, 6000d);
        String[] labels = {"d0", "d1", "d2", "d3", "d4", "d5"};
        String[] epls = {EPL_CR_D0, EPL_CR_D1, EPL_CR_D2, EPL_CR_D3, EPL_CR_D4, EPL_CR_D5};
        int[] counts = {1, 2, 3, 4, 4, 4};
        Object[][] sendTwos = {
                {"T", 0, 0, 0d, null},
                {"T", 0, 0, -1d, 1d},
                {"T", -2, 2, 0d, 0d},
                {"T", -3, 3, -3d, 3d},
                {"T", -4, 4, -4d, 4d},
                {"T", 0, 0, 5d, 1d},
                {"T", 100, 200, 0d, 0d},
        };
        int sendIndex = 0;
        for (int index = 0; index < labels.length; index++) {
            validateDeploy(steps.get(offset++), caseName, labels[index], epls[index]);
            validateDeployed(steps.get(offset++), caseName, labels[index]);
            validateIndexCount(steps.get(offset++), caseName, "MyWindowCR", "createOne",
                    "indexes", counts[index]);
            // d0 is followed by two sendTwo triggers; d1..d5 by one each.
            int sendsHere = index == 0 ? 2 : 1;
            for (int send = 0; send < sendsHere; send++) {
                Object[] args = sendTwos[sendIndex++];
                validateBeanTwoSend(steps.get(offset++), caseName, (String) args[0],
                        (Integer) args[1], (Integer) args[2], (Double) args[3],
                        (Double) args[4]);
            }
        }
        for (String label : labels) {
            validateUndeploy(steps.get(offset++), caseName, label);
        }
        validateIndexCount(steps.get(offset++), caseName, "MyWindowCR", "createOne",
                "indexes", 0);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact coercion-key-range step sequence mirroring
     * InfraCoercionKeyAndRangeMultiPropIndexes lines 386-443: createOne plus
     * unfiltered insert, four SupportBean sends, four interleaved
     * deploy+index-count+sendTwo groups (1,2,3,4 with d0 followed by two
     * triggers), per-statement undeploys of d0..d3, index-count 0,
     * undeploy-all.
     */
    private static int validateCoercionKeyRange(JsonArray steps, int offset) {
        String caseName = "coercion-key-range";
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "createOne", EPL_CKR_CREATE_ONE);
        validateDeployed(steps.get(offset++), caseName, "createOne");
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_CKR_INSERT);
        validateDeployed(steps.get(offset++), caseName, "insert");
        validateBeanSend(steps.get(offset++), caseName, "E1", 1, 10, 100d, 1000d);
        validateBeanSend(steps.get(offset++), caseName, "E2", 2, 20, 200d, 2000d);
        validateBeanSend(steps.get(offset++), caseName, "E3", 3, 30, 300d, 3000d);
        validateBeanSend(steps.get(offset++), caseName, "E4", 4, 40, 400d, 4000d);
        String[] labels = {"d0", "d1", "d2", "d3"};
        String[] epls = {EPL_CKR_D0, EPL_CKR_D1, EPL_CKR_D2, EPL_CKR_D3};
        int[] counts = {1, 2, 3, 4};
        Object[][] sendTwos = {
                {"T", 0, 0, 1d, 200d},
                {"E1", 0, 0, 1d, 200d},
                {"E2", 2, 0, 19d, 21d},
                {"E3", 3, 0, 29d, 34d},
                {"E4", 4, 40, 0d, null},
        };
        int sendIndex = 0;
        for (int index = 0; index < labels.length; index++) {
            validateDeploy(steps.get(offset++), caseName, labels[index], epls[index]);
            validateDeployed(steps.get(offset++), caseName, labels[index]);
            validateIndexCount(steps.get(offset++), caseName, "MyWindowCKR", "createOne",
                    "indexes", counts[index]);
            int sendsHere = index == 0 ? 2 : 1;
            for (int send = 0; send < sendsHere; send++) {
                Object[] args = sendTwos[sendIndex++];
                validateBeanTwoSend(steps.get(offset++), caseName, (String) args[0],
                        (Integer) args[1], (Integer) args[2], (Double) args[3],
                        (Double) args[4]);
            }
        }
        for (String label : labels) {
            validateUndeploy(steps.get(offset++), caseName, label);
        }
        validateIndexCount(steps.get(offset++), caseName, "MyWindowCKR", "createOne",
                "indexes", 0);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op"))
                || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    private static void validateDeploy(JsonValue value, String caseName, String expectedStatement,
                                       String expectedEpl) {
        JsonObject step = object(value, "deploy step");
        requireFields(step, "op", "case", "statement", "epl");
        if (!"deploy".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("deploy step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateDeployed(JsonValue value, String caseName,
                                         String expectedStatement) {
        JsonObject step = object(value, "deployed step");
        requireFields(step, "op", "case", "statement");
        if (!"deployed".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("deployed step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateIndexCount(JsonValue value, String caseName, String windowName,
                                           String create, String of, long count) {
        JsonObject step = object(value, "index-count step");
        requireFields(step, "op", "case", "statement", "create", "of", "count");
        if (!"index-count".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !windowName.equals(string(step, "statement"))
                || !create.equals(string(step, "create"))
                || !of.equals(string(step, "of"))
                || longInteger(step.get("count"), "count") != count) {
            throw new IllegalArgumentException("index-count step is not pinned for " + caseName
                    + "/" + windowName);
        }
    }

    private static void validateSnapshot(JsonValue value, String caseName,
                                         String expectedStatement) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement");
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateUndeploy(JsonValue value, String caseName,
                                         String expectedStatement) {
        JsonObject step = object(value, "undeploy step");
        requireFields(step, "op", "case", "statement");
        if (!"undeploy".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("undeploy step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateUndeployAll(JsonValue value, String caseName) {
        JsonObject step = object(value, "undeploy-all step");
        requireFields(step, "op", "case");
        if (!"undeploy-all".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("undeploy-all step is not pinned for " + caseName);
        }
    }

    /** Two-field SupportBean send (staggered case): theString + intPrimitive. */
    private static void validateBeanSend(JsonValue value, String caseName, String theString,
                                         long intPrimitive) {
        JsonObject payload = beanSendPayload(value, caseName);
        if (!theString.equals(string(payload, "theString"))
                || longInteger(payload.get("intPrimitive"), "intPrimitive") != intPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for "
                    + caseName + "/" + theString);
        }
    }

    /** Five-field SupportBean send: boxed fields may be JSON null. */
    private static void validateBeanSend(JsonValue value, String caseName, String theString,
                                         long intPrimitive, Integer intBoxed,
                                         double doublePrimitive, Double doubleBoxed) {
        JsonObject payload = beanSendPayload(value, caseName);
        if (!theString.equals(string(payload, "theString"))
                || longInteger(payload.get("intPrimitive"), "intPrimitive") != intPrimitive
                || !boxedEquals(payload.get("intBoxed"), intBoxed)
                || doubleValue(payload.get("doublePrimitive"), "doublePrimitive")
                        != doublePrimitive
                || !boxedEquals(payload.get("doubleBoxed"), doubleBoxed)) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for "
                    + caseName + "/" + theString);
        }
    }

    private static JsonObject beanSendPayload(JsonValue value, String caseName) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        return object(step.get("payload"), "SupportBean payload");
    }

    /** SupportBeanTwo send: boxed fields may be JSON null. */
    private static void validateBeanTwoSend(JsonValue value, String caseName, String stringTwo,
                                            long intPrimitiveTwo, Integer intBoxedTwo,
                                            double doublePrimitiveTwo, Double doubleBoxedTwo) {
        JsonObject step = object(value, "SupportBeanTwo step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBeanTwo".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBeanTwo step is not pinned for "
                    + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBeanTwo payload");
        if (!stringTwo.equals(string(payload, "stringTwo"))
                || longInteger(payload.get("intPrimitiveTwo"), "intPrimitiveTwo")
                        != intPrimitiveTwo
                || !boxedEquals(payload.get("intBoxedTwo"), intBoxedTwo)
                || doubleValue(payload.get("doublePrimitiveTwo"), "doublePrimitiveTwo")
                        != doublePrimitiveTwo
                || !boxedEquals(payload.get("doubleBoxedTwo"), doubleBoxedTwo)) {
            throw new IllegalArgumentException("SupportBeanTwo payload is not pinned for "
                    + caseName + "/" + stringTwo);
        }
    }

    private static boolean boxedEquals(JsonValue value, Number expected) {
        if (expected == null) {
            return value == null || value.isNull();
        }
        return value != null && value.isNumber()
                && Double.parseDouble(value.toString()) == expected.doubleValue();
    }

    private static void requireFields(JsonObject object, String... names) {
        for (String name : names) {
            if (object.get(name) == null) {
                throw new IllegalArgumentException("missing field: " + name);
            }
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        JsonArray actual = array(value, name);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(name + " length mismatch");
        }
        for (int index = 0; index < expected.length; index++) {
            if (!expected[index].equals(actual.get(index).asString())) {
                throw new IllegalArgumentException(name + " is not pinned at index " + index);
            }
        }
    }

    private static JsonObject object(JsonValue value, String name) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(name + " must be an object");
        }
        return value.asObject();
    }

    private static JsonArray array(JsonValue value, String name) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(name + " must be an array");
        }
        return value.asArray();
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isString()) {
            throw new IllegalArgumentException("missing string field: " + name);
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isNumber()) {
            throw new IllegalArgumentException("missing numeric field: " + name);
        }
        return (int) Double.parseDouble(value.toString());
    }

    private static long longInteger(JsonValue value, String name) {
        if (value == null || !value.isNumber()) {
            throw new IllegalArgumentException("missing numeric field: " + name);
        }
        return (long) Double.parseDouble(value.toString());
    }

    /** Mirrors SupportExceptionHandlerFactoryRethrow from the regression harness. */
    public static class HarnessRethrowExceptionHandlerFactory implements ExceptionHandlerFactory {
        @Override
        public ExceptionHandler getHandler(ExceptionHandlerFactoryContext context) {
            return handlerContext -> {
                throw new RuntimeException("Unexpected exception in statement '"
                        + handlerContext.getStatementName() + "': "
                        + handlerContext.getThrowable().getMessage(),
                        handlerContext.getThrowable());
            };
        }
    }

    /**
     * Local mirror of the regression SupportBeanTwo (regression-lib is not on
     * the oracle classpath); the five properties the suite's on-delete
     * triggers read, with bean-style accessors so the simple class name and
     * property names resolve identically.
     */
    public static class SupportBeanTwo {
        private String stringTwo;
        private int intPrimitiveTwo;
        private Integer intBoxedTwo;
        private double doublePrimitiveTwo;
        private Double doubleBoxedTwo;

        public String getStringTwo() {
            return stringTwo;
        }

        public void setStringTwo(String stringTwo) {
            this.stringTwo = stringTwo;
        }

        public int getIntPrimitiveTwo() {
            return intPrimitiveTwo;
        }

        public void setIntPrimitiveTwo(int intPrimitiveTwo) {
            this.intPrimitiveTwo = intPrimitiveTwo;
        }

        public Integer getIntBoxedTwo() {
            return intBoxedTwo;
        }

        public void setIntBoxedTwo(Integer intBoxedTwo) {
            this.intBoxedTwo = intBoxedTwo;
        }

        public double getDoublePrimitiveTwo() {
            return doublePrimitiveTwo;
        }

        public void setDoublePrimitiveTwo(double doublePrimitiveTwo) {
            this.doublePrimitiveTwo = doublePrimitiveTwo;
        }

        public Double getDoubleBoxedTwo() {
            return doubleBoxedTwo;
        }

        public void setDoubleBoxedTwo(Double doubleBoxedTwo) {
            this.doubleBoxedTwo = doubleBoxedTwo;
        }
    }

    /**
     * Local mirror of the regression SupportBean_ST0 (regression-lib is not on
     * the oracle classpath); the id/key0/p00 properties the phase-B on-SELECT
     * triggers reference.
     */
    public static class SupportBean_ST0 {
        private final String id;
        private final String key0;
        private final int p00;

        public SupportBean_ST0(String id, String key0, int p00) {
            this.id = id;
            this.key0 = key0;
            this.p00 = p00;
        }

        public String getId() {
            return id;
        }

        public String getKey0() {
            return key0;
        }

        public int getP00() {
            return p00;
        }
    }
}
