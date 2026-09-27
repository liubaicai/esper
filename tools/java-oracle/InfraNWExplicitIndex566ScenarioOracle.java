import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.util.StatementProperty;
import com.espertech.esper.common.client.util.StatementType;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.epl.namedwindow.core.NamedWindow;
import com.espertech.esper.common.internal.epl.namedwindow.core.NamedWindowInstance;
import com.espertech.esper.common.internal.epl.namedwindow.core.NamedWindowManagementService;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportCountAccessEvent;
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
import java.util.Arrays;
import java.util.Collections;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the named-window explicit-index parity
 * scenario. Mirrors InfraNamedWindowIndex ordinal 0 (a unique-retention
 * named window fed by insert-into, a late `create unique index` statement
 * whose STATEMENTTYPE/CREATEOBJECTNAME properties are asserted, and an
 * any-order window iterator pin) and InfraNamedWindowLateStartIndex ordinal
 * 0 (two phases over a keepall SupportCountAccessEvent window with an
 * explicit `create index I1 on AWindow(p00)`; a late-deployed unidirectional
 * join and correlated subquery, an undeploy-all phase boundary, then the
 * enable_window_subquery_indexshare twin replayed on a fresh path).
 *
 * <p>named-window-index deploys its three statements as ONE module exactly
 * like the source's single compileDeploy call; the oracle then asserts the
 * idx statement's CREATE_INDEX/CREATEOBJECTNAME properties in-process before
 * emitting the pinned "unrepresentable" record (the typed Go surface has no
 * statement-type metadata). late-start-index deploys each statement as its
 * own module sharing one path (the RegressionPath equivalent, so the @public
 * window resolves across deployments). The SupportCountAccessEvent getter
 * instrumentation is verified in-process at the same points the source
 * asserts (101 after preload, 2 after each late deploy) and each assert rides
 * a pinned "unrepresentable" record because Go has no getter-call counter.
 * "index-count" records pin the named window's getIndexDescriptors().length
 * through the create statement's deployment (SupportInfraUtil
 * .getIndexCountNoContext equivalent); "snapshot" records pin the window
 * statement iterator. The s1 listener delivery is recorded even though the
 * Java source only attaches the listener without asserting the payload.
 */
public final class InfraNWExplicitIndex566ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-namedwindow-explicit-index-566";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite";

    private static final String CASE_INDEX = "named-window-index";
    private static final String CASE_LATE = "late-start-index";
    private static final String[] CASES = {CASE_INDEX, CASE_LATE};
    private static final int[] ORDINALS = {0, 0};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-9c952fff6ef8c649c2e4",
            "java-runtime-3bf753f4ff71d21df968",
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraNamedWindowIndex",
            "InfraNamedWindowLateStartIndex",
    };
    private static final String[] STATIC_IDS = {
            "java-152a3c2771c531a8841c",
            "java-a81367bdfb722afec857",
    };
    private static final String[] JAVA_FLAGS = {"EXCLUDEWHENINSTRUMENTED", "PERFORMANCE"};
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowIndex.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowLateStartIndex.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_S0.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportCountAccessEvent.java",
    };

    // Verbatim transcriptions of InfraNamedWindowIndex.java lines 26-28 (the
    // single-module EPL minus its statement separators) and
    // InfraNamedWindowLateStartIndex.java lines 38, 46-47, 57-58 and 70-77.
    private static final String EPL_INDEX_WINDOW =
            "@name('window') create window MyWindowOne#unique(theString) as SupportBean";
    private static final String EPL_INDEX_INSERT =
            "insert into MyWindowOne select * from SupportBean";
    private static final String EPL_INDEX_IDX =
            "@name('idx') create unique index I1 on MyWindowOne(theString)";

    private static final String EPL_LATE_CREATE =
            "@public create window AWindow#keepall as SupportCountAccessEvent";
    private static final String EPL_LATE_CREATE_SHARED =
            "@Hint('enable_window_subquery_indexshare') " + EPL_LATE_CREATE;
    private static final String EPL_LATE_INSERT =
            "insert into AWindow select * from SupportCountAccessEvent";
    private static final String EPL_LATE_INDEX = "create index I1 on AWindow(p00)";
    private static final String EPL_LATE_S0 =
            "@name('s0') select * from SupportBean_S0 as s0 unidirectional,"
                    + " AWindow(p00='x') as aw where aw.id = s0.id";
    private static final String EPL_LATE_S1 =
            "@name('s1') select (select id from AWindow(p00='x') as aw where aw.id = s0.id)"
                    + " from SupportBean_S0 as s0 unidirectional";
    private static final String EPL_LATE_S2 =
            "@name('s2') select (select id from AWindow(p00='x') as aw where aw.id = s0.id)"
                    + " from SupportBean_S0 as s0 unidirectional";

    private static final String[] CASE_OBSERVATIONS = {
            "deployed+unrepresentable+snapshot; one three-statement module (unique-retention"
                    + " MyWindowOne, wildcard insert, unique index I1 on theString): five"
                    + " SupportBean sends dedupe last-wins on theString, the idx"
                    + " statement-type/object-name assertion rides a pinned record, and the"
                    + " window iterator returns {(E0,5),(E1,4),(E2,3)} any-order",
            "deployed+listener+index-count+snapshot+unrepresentable; two phases on separate"
                    + " paths: preload create+insert+index over @public keepall AWindow (100"
                    + " E-rows + (-1,'x'), index count 1, 101-row snapshot), then late s0"
                    + " unidirectional join delivers one {s0:(-1,x),aw:(-1,x)} pair and late s1"
                    + " subquery delivers its scalar, undeploy-all boundary, indexshare-hinted"
                    + " phase B deploys the identical subquery as s2 delivering the same scalar;"
                    + " getter-count asserts (101,2,2,2) ride unrepresentable records",
    };
    private static final String[] CASE_EPLS = {
            EPL_INDEX_WINDOW + ";\n" + EPL_INDEX_INSERT + ";\n" + EPL_INDEX_IDX + ";\n",
            EPL_LATE_CREATE + "\n" + EPL_LATE_INSERT + "\n" + EPL_LATE_INDEX + "\n"
                    + EPL_LATE_S0 + "\n" + EPL_LATE_S1 + "\n" + EPL_LATE_CREATE_SHARED + "\n"
                    + EPL_LATE_INSERT + "\n" + EPL_LATE_INDEX + "\n" + EPL_LATE_S2,
    };
    private static final String[][] CASE_DEPLOYS = {
            {"window", "insert", "idx"},
            {"create", "insert", "index", "s0", "s1", "create", "insert", "index", "s2"},
    };

    /**
     * Module grouping: the scenario's deploy steps are per statement so the Go
     * runner maps each onto one catalog call or plan, while this oracle
     * reproduces the Java fan-out. named-window-index compiles all three
     * statements as one module (the window is module-private in the source);
     * late-start-index compiles every statement as its own module sharing one
     * path, matching the source's per-statement compileDeploy(path) calls.
     */
    private static final Map<String, Map<String, Integer>> MODULE_KEYS;
    private static final Map<String, Set<String>> LISTENED_STATEMENTS;

    static {
        Map<String, Map<String, Integer>> modules = new HashMap<>();
        Map<String, Integer> indexModules = new HashMap<>();
        indexModules.put("window", 0);
        indexModules.put("insert", 0);
        indexModules.put("idx", 0);
        modules.put(CASE_INDEX, indexModules);
        Map<String, Integer> lateModules = new HashMap<>();
        lateModules.put("create", 0);
        lateModules.put("insert", 1);
        lateModules.put("index", 2);
        lateModules.put("s0", 3);
        lateModules.put("s1", 4);
        lateModules.put("s2", 5);
        modules.put(CASE_LATE, lateModules);
        MODULE_KEYS = Collections.unmodifiableMap(modules);

        Map<String, Set<String>> listened = new HashMap<>();
        listened.put(CASE_INDEX, Collections.emptySet());
        listened.put(CASE_LATE, new HashSet<>(Arrays.asList("s0", "s1", "s2")));
        LISTENED_STATEMENTS = Collections.unmodifiableMap(listened);
    }

    /** Pinned notes the unrepresentable records carry; the oracle verifies the
     * Java surface in-process before emitting the same record. */
    private static final String NOTE_IDX_PROPS =
            "Java asserts the idx statement properties STATEMENTTYPE=CREATE_INDEX and"
                    + " CREATEOBJECTNAME=I1 through EPStatement.getProperty; the typed Go API"
                    + " has no statement-type metadata surface";
    private static final String NOTE_GETTER_RESET =
            "SupportCountAccessEvent.getAndResetCountGetterCalled() resets the JVM"
                    + " getter-call counter at the assert boundary; no Go instrumentation"
                    + " counterpart exists";
    private static final String NOTE_GETTER_PRELOAD =
            "Java asserts getAndResetCountGetterCalled()=101 after preloading 100 E-rows"
                    + " plus (-1,'x') through the insert-into; Go has no getter-call"
                    + " instrumentation";
    private static final String NOTE_GETTER_S0 =
            "Java asserts getAndResetCountGetterCalled()=2 after deploying the s0"
                    + " unidirectional join (the parens p00='x' filter is planned against the"
                    + " declared I1 index); Go has no getter-call instrumentation";
    private static final String NOTE_GETTER_S1 =
            "Java asserts getAndResetCountGetterCalled()=2 after deploying the s1 correlated"
                    + " subquery without index sharing; Go has no getter-call instrumentation";
    private static final String NOTE_GETTER_S2 =
            "Java asserts getAndResetCountGetterCalled()=2 after deploying the s2 correlated"
                    + " subquery with enable_window_subquery_indexshare; Go has no getter-call"
                    + " instrumentation";

    private static final Map<String, String> UNREPRESENTABLE_NOTES;
    private static final Map<String, Integer> GETTER_EXPECTED;

    static {
        Map<String, String> notes = new HashMap<>();
        notes.put("idx-props", NOTE_IDX_PROPS);
        notes.put("getter-reset", NOTE_GETTER_RESET);
        notes.put("getter-preload", NOTE_GETTER_PRELOAD);
        notes.put("getter-s0", NOTE_GETTER_S0);
        notes.put("getter-s1", NOTE_GETTER_S1);
        notes.put("getter-s2", NOTE_GETTER_S2);
        UNREPRESENTABLE_NOTES = Collections.unmodifiableMap(notes);
        Map<String, Integer> expected = new HashMap<>();
        expected.put("getter-preload", 101);
        expected.put("getter-s0", 2);
        expected.put("getter-s1", 2);
        expected.put("getter-s2", 2);
        GETTER_EXPECTED = Collections.unmodifiableMap(expected);
    }

    /** Pinned projection fields: the SupportBean iterator pin for case A and
     * the (id,p00) pin the two SupportCountAccessEvent/SupportBean_S0 joins
     * and snapshots share (the Java bean carries further unasserted props). */
    private static final String[] FIELDS_WINDOW = {"theString", "intPrimitive"};
    private static final String[] FIELDS_ACCESS = {"id", "p00"};

    private static final int[] EXPECTED_CASE_RECORDS = {5, 28};
    private static final int EXPECTED_RECORDS = 33;
    private static final int EXPECTED_STEPS = 256;

    private InfraNWExplicitIndex566ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWExplicitIndex566ScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        JsonArray records = new JsonArray();
        for (int index = 0; index < CASES.length; index++) {
            int before = records.size();
            runCase(CASES[index], allSteps, records);
            int emitted = records.size() - before;
            if (emitted != EXPECTED_CASE_RECORDS[index]) {
                for (int r = before; r < records.size(); r++) {
                    System.err.println("REC " + records.get(r).toString());
                }
                throw new IllegalStateException("case " + CASES[index] + " emitted "
                        + emitted + " records, expected " + EXPECTED_CASE_RECORDS[index]);
            }
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS + " records, got "
                    + records.size());
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", ID);
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        root.add("records", records);
        System.out.println(root.toString());
    }

    /** Replays one case's steps on a fresh runtime (one runtime per Java execution). */
    private static void runCase(String caseName, JsonArray allSteps, JsonArray records)
            throws Exception {
        Configuration configuration = new Configuration();
        switch (caseName) {
            case CASE_INDEX:
                configuration.getCommon().addEventType(SupportBean.class);
                break;
            case CASE_LATE:
                configuration.getCommon().addEventType(SupportBean_S0.class);
                configuration.getCommon().addEventType(SupportCountAccessEvent.class);
                break;
            default:
                throw new IllegalArgumentException("unknown case " + caseName);
        }
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statementsByName = new HashMap<>();
        List<String> pendingStatements = new ArrayList<>();
        List<String> pendingEpls = new ArrayList<>();
        int pendingModule = -1;
        try {
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
                if ("deploy".equals(operation)) {
                    String statementName = string(step, "statement");
                    Integer module = MODULE_KEYS.get(caseName).get(statementName);
                    if (module == null) {
                        throw new IllegalStateException("case " + caseName
                                + " deploys unknown statement " + statementName);
                    }
                    if (!pendingStatements.isEmpty() && pendingModule != module.intValue()) {
                        deployModule(runtime, caseName, pendingStatements, pendingEpls,
                                statementsByName, sequences, records);
                        pendingStatements.clear();
                        pendingEpls.clear();
                    }
                    pendingModule = module.intValue();
                    pendingStatements.add(statementName);
                    pendingEpls.add(string(step, "epl"));
                    continue;
                }
                if (!pendingStatements.isEmpty()) {
                    deployModule(runtime, caseName, pendingStatements, pendingEpls,
                            statementsByName, sequences, records);
                    pendingStatements.clear();
                    pendingEpls.clear();
                }
                switch (operation) {
                    case "deployed": {
                        String label = string(step, "statement");
                        if (!statementsByName.containsKey(label)) {
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
                        EPStatement statement = statementsByName.get(label);
                        if (statement == null) {
                            throw new IllegalStateException(
                                    "snapshot targets unknown statement " + label);
                        }
                        String[] fields = CASE_INDEX.equals(caseName)
                                ? FIELDS_WINDOW : FIELDS_ACCESS;
                        JsonArray rows = new JsonArray();
                        for (Iterator<EventBean> iterator = statement.iterator();
                             iterator.hasNext(); ) {
                            rows.add(row(iterator.next(), fields));
                        }
                        sortRowsCanonical(rows);
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "snapshot");
                        record.add("statement", label);
                        record.add("sequence", 0);
                        record.add("time", Instant.ofEpochMilli(
                                runtime.getEventService().getCurrentTime()).toString());
                        if (rows.size() > 0) {
                            record.add("new", rows);
                        }
                        records.add(record);
                        break;
                    }
                    case "index-count": {
                        String windowName = string(step, "statement");
                        String createLabel = string(step, "create");
                        String of = string(step, "of");
                        if (!"indexes".equals(of)) {
                            throw new IllegalArgumentException(
                                    "unknown index-count kind: " + of);
                        }
                        EPStatement create = statementsByName.get(createLabel);
                        if (create == null) {
                            throw new IllegalStateException(
                                    "index-count targets unknown create statement " + createLabel);
                        }
                        NamedWindowInstance instance = namedWindowInstance(
                                runtime, create.getDeploymentId(), windowName);
                        long actual = instance.getIndexDescriptors().length;
                        long expected = longInteger(step.get("count"), "count");
                        if (actual != expected) {
                            throw new IllegalStateException("index-count mismatch for "
                                    + windowName + " (indexes): expected " + expected
                                    + ", got " + actual);
                        }
                        int sequence = sequences.merge(
                                windowName + ":index-count", 1, Integer::sum);
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
                    case "unrepresentable": {
                        String label = string(step, "statement");
                        String note = UNREPRESENTABLE_NOTES.get(label);
                        if (note == null || !note.equals(string(step, "expectError"))) {
                            throw new IllegalStateException(
                                    "unrepresentable step is not pinned for " + caseName + "/"
                                            + label);
                        }
                        verifyUnrepresentable(caseName, label, statementsByName);
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "unrepresentable");
                        record.add("statement", label);
                        record.add("sequence", 0);
                        record.add("value", note);
                        records.add(record);
                        break;
                    }
                    case "undeploy-all":
                        // Phase boundary plus the trailing teardown: undeploy
                        // destroys every deployment including the named
                        // window's contents (preloadData's fresh RegressionPath
                        // equivalent).
                        runtime.getDeploymentService().undeployAll();
                        statementsByName.clear();
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
                }
            }
            if (!pendingStatements.isEmpty()) {
                deployModule(runtime, caseName, pendingStatements, pendingEpls,
                        statementsByName, sequences, records);
            }
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
        }
    }

    /**
     * Verifies the Java surface one unrepresentable record documents: the idx
     * statement's CREATE_INDEX/CREATEOBJECTNAME properties for
     * named-window-index, and the SupportCountAccessEvent getter-call counts
     * the source asserts (101 after the preload loop, 2 after each late
     * deploy) plus the pre-preload counter reset for late-start-index.
     */
    private static void verifyUnrepresentable(String caseName, String label,
                                              Map<String, EPStatement> statementsByName) {
        switch (label) {
            case "idx-props": {
                EPStatement idx = statementsByName.get("idx");
                if (idx == null) {
                    throw new IllegalStateException("idx-props without a deployed idx statement");
                }
                if (idx.getProperty(StatementProperty.STATEMENTTYPE) != StatementType.CREATE_INDEX
                        || !"I1".equals(idx.getProperty(StatementProperty.CREATEOBJECTNAME))) {
                    throw new IllegalStateException(
                            "idx statement properties diverge from the Java pins");
                }
                break;
            }
            case "getter-reset":
                SupportCountAccessEvent.getAndResetCountGetterCalled();
                break;
            default: {
                Integer expected = GETTER_EXPECTED.get(label);
                if (expected == null) {
                    throw new IllegalStateException("unknown unrepresentable label " + label
                            + " in case " + caseName);
                }
                int actual = SupportCountAccessEvent.getAndResetCountGetterCalled();
                if (actual != expected.intValue()) {
                    throw new IllegalStateException("getter-call count for " + label
                            + " = " + actual + ", expected " + expected);
                }
            }
        }
    }

    /**
     * Compiles and deploys one module: the queued statements joined with ";\n"
     * in step order (a whitespace-normalized form of the suite's module text),
     * then registers the statements by scenario label and attaches the case's
     * listeners. CompilerArguments(runtime.getRuntimePath()) carries the prior
     * deployments' public objects so the late-start-index modules resolve the
     * @public window like the source's shared RegressionPath.
     */
    private static void deployModule(EPRuntime runtime, String caseName,
                                     List<String> statementNames, List<String> epls,
                                     Map<String, EPStatement> statementsByName,
                                     Map<String, Integer> sequences, JsonArray records)
            throws Exception {
        StringBuilder moduleEpl = new StringBuilder();
        for (int index = 0; index < epls.size(); index++) {
            if (index > 0) {
                moduleEpl.append(";\n");
            }
            moduleEpl.append(epls.get(index));
        }
        CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler()
                .compile(moduleEpl.toString(), compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService()
                .deploy(compiled, new DeploymentOptions());
        EPStatement[] deployed = deployment.getStatements();
        if (deployed.length != statementNames.size()) {
            throw new IllegalStateException("module of case " + caseName + " with statements "
                    + statementNames + " deployed " + deployed.length + " statements");
        }
        Set<String> listened = LISTENED_STATEMENTS.getOrDefault(caseName, Collections.emptySet());
        for (int index = 0; index < statementNames.size(); index++) {
            String label = statementNames.get(index);
            EPStatement statement = deployed[index];
            statementsByName.put(label, statement);
            if (listened.contains(label)) {
                statement.addListener(listener(caseName, label, sequences, records, runtime));
            }
        }
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

    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                SupportBean bean = new SupportBean();
                bean.setTheString(string(payload, "theString"));
                bean.setIntPrimitive(integer(payload, "intPrimitive"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S0": {
                SupportBean_S0 bean = new SupportBean_S0(
                        integer(payload, "id"), string(payload, "p00"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportCountAccessEvent": {
                SupportCountAccessEvent event = new SupportCountAccessEvent(
                        integer(payload, "id"), string(payload, "p00"));
                runtime.getEventService().sendEventBean(event, type);
                break;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
    }

    /**
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; new and old arrays render only when non-empty. The
     * s0 join delivery renders each fragment under its stream alias with the
     * pinned (id,p00) field set; the s1/s2 scalar rows render their single
     * column verbatim.
     */
    private static UpdateListener listener(String caseName, String label,
                                           Map<String, Integer> sequences, JsonArray records,
                                           EPRuntime runtime) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            JsonArray newRows = "s0".equals(label)
                    ? joinRows(newEvents) : rows(newEvents);
            JsonArray oldRows = "s0".equals(label)
                    ? joinRows(oldEvents) : rows(oldEvents);
            if (newRows.size() == 0 && oldRows.size() == 0) {
                // Force-dispatched empty pairs carry no record, the same
                // convention the Go runner follows.
                return;
            }
            int sequence = sequences.merge(label, 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", label);
            record.add("sequence", sequence);
            record.add("time",
                    Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        };
    }

    /** Canonical row rendering projecting the pinned field set. */
    private static JsonArray rows(EventBean[] events) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            array.add(row(event, null));
        }
        return array;
    }

    /** Join-row rendering: each fragment under its stream alias carrying the
     * pinned (id,p00) field set. */
    private static JsonArray joinRows(EventBean[] events) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            JsonObject item = new JsonObject();
            item.add("kind", "row");
            JsonObject fields = new JsonObject();
            for (String alias : new String[]{"s0", "aw"}) {
                Object value = event.getFragment(alias);
                if (!(value instanceof EventBean)) {
                    throw new IllegalStateException("join row fragment " + alias
                            + " is not an event: " + event.get(alias));
                }
                fields.add(alias, row((EventBean) value, FIELDS_ACCESS));
            }
            item.add("fields", fields);
            array.add(item);
        }
        return array;
    }

    private static JsonObject row(EventBean event, String[] fields) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        String[] names = fields != null ? fields.clone()
                : event.getEventType().getPropertyNames().clone();
        Arrays.sort(names);
        JsonObject values = new JsonObject();
        for (String name : names) {
            values.add(name, normalize(event.get(name)));
        }
        item.add("fields", values);
        return item;
    }

    private static void sortRowsCanonical(JsonArray rows) {
        List<JsonObject> values = new ArrayList<>();
        for (JsonValue value : rows) {
            values.add(value.asObject());
        }
        values.sort(Comparator.comparing(value -> value.get("fields").asObject().toString()));
        for (int index = rows.size() - 1; index >= 0; index--) {
            rows.remove(index);
        }
        for (JsonObject value : values) {
            rows.add(value);
        }
    }

    /** Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, and null as the tagged
     * {"state":"null"} object the Go normalizer also emits. */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof EventBean) {
            return row((EventBean) value, null);
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

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaSourceFiles", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags",
                "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaSourceFiles"), JAVA_SOURCE_FILES, "javaSourceFiles");
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
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl", "deploys");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !CASE_OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
            validateStringArray(definition.get("deploys"), CASE_DEPLOYS[index], "deploys");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = validateIndexCase(steps, 0);
        offset = validateLateStartCase(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact named-window-index step sequence mirroring
     * InfraNamedWindowIndex lines 26-43: the three-statement module deploys,
     * the deployed markers, the pinned statement-property record, the five
     * SupportBean sends, the any-order window iterator snapshot and
     * undeploy-all.
     */
    private static int validateIndexCase(JsonArray steps, int offset) {
        validateCaseMarker(steps.get(offset++), CASE_INDEX);
        validateDeploy(steps.get(offset++), CASE_INDEX, "window", EPL_INDEX_WINDOW);
        validateDeploy(steps.get(offset++), CASE_INDEX, "insert", EPL_INDEX_INSERT);
        validateDeploy(steps.get(offset++), CASE_INDEX, "idx", EPL_INDEX_IDX);
        validateDeployed(steps.get(offset++), CASE_INDEX, "window");
        validateDeployed(steps.get(offset++), CASE_INDEX, "insert");
        validateDeployed(steps.get(offset++), CASE_INDEX, "idx");
        validateUnrepresentable(steps.get(offset++), CASE_INDEX, "idx-props", NOTE_IDX_PROPS);
        String[][] sends = {
                {"E0", "1"}, {"E2", "2"}, {"E2", "3"}, {"E1", "4"}, {"E0", "5"}};
        for (String[] send : sends) {
            validateBeanSend(steps.get(offset++), CASE_INDEX, send[0],
                    Integer.parseInt(send[1]));
        }
        validateSnapshot(steps.get(offset++), CASE_INDEX, "window", "any");
        validateUndeployAll(steps.get(offset++), CASE_INDEX);
        return offset;
    }

    /**
     * Exact late-start-index step sequence mirroring
     * InfraNamedWindowLateStartIndex lines 33-66: phase A preloads the
     * @public keepall AWindow through the insert-into (100 E-rows plus
     * (-1,'x')), deploys the late s0 unidirectional join and s1 correlated
     * subquery, then undeploy-all restarts the path; phase B repeats the
     * preload with the enable_window_subquery_indexshare hint and deploys the
     * identical subquery as s2. The getter windows must contain only the
     * deploy they assert: p00 getter reads (snapshot row rendering, listener
     * join fragments, the s1 subquery probe) are unasserted and would pollute
     * the counter, so index-count, sends and the phase-end snapshots sit
     * between an assert and the next reset.
     */
    private static int validateLateStartCase(JsonArray steps, int offset) {
        validateCaseMarker(steps.get(offset++), CASE_LATE);
        offset = validateLateStartPreload(steps, offset, EPL_LATE_CREATE);
        validateDeploy(steps.get(offset++), CASE_LATE, "s0", EPL_LATE_S0);
        validateDeployed(steps.get(offset++), CASE_LATE, "s0");
        validateUnrepresentable(steps.get(offset++), CASE_LATE, "getter-s0", NOTE_GETTER_S0);
        validateIndexCount(steps.get(offset++), CASE_LATE, "AWindow", "create", "indexes", 1);
        validateS0Send(steps.get(offset++), CASE_LATE);
        validateUnrepresentable(steps.get(offset++), CASE_LATE, "getter-reset",
                NOTE_GETTER_RESET);
        validateDeploy(steps.get(offset++), CASE_LATE, "s1", EPL_LATE_S1);
        validateDeployed(steps.get(offset++), CASE_LATE, "s1");
        validateUnrepresentable(steps.get(offset++), CASE_LATE, "getter-s1", NOTE_GETTER_S1);
        validateIndexCount(steps.get(offset++), CASE_LATE, "AWindow", "create", "indexes", 1);
        validateS0Send(steps.get(offset++), CASE_LATE);
        validateSnapshot(steps.get(offset++), CASE_LATE, "create", "any");
        validateUndeployAll(steps.get(offset++), CASE_LATE);
        offset = validateLateStartPreload(steps, offset, EPL_LATE_CREATE_SHARED);
        validateDeploy(steps.get(offset++), CASE_LATE, "s2", EPL_LATE_S2);
        validateDeployed(steps.get(offset++), CASE_LATE, "s2");
        validateUnrepresentable(steps.get(offset++), CASE_LATE, "getter-s2", NOTE_GETTER_S2);
        validateIndexCount(steps.get(offset++), CASE_LATE, "AWindow", "create", "indexes", 1);
        validateS0Send(steps.get(offset++), CASE_LATE);
        validateSnapshot(steps.get(offset++), CASE_LATE, "create", "any");
        validateUndeployAll(steps.get(offset++), CASE_LATE);
        return offset;
    }

    private static int validateLateStartPreload(JsonArray steps, int offset, String createEpl) {
        validateDeploy(steps.get(offset++), CASE_LATE, "create", createEpl);
        validateDeployed(steps.get(offset++), CASE_LATE, "create");
        validateDeploy(steps.get(offset++), CASE_LATE, "insert", EPL_LATE_INSERT);
        validateDeployed(steps.get(offset++), CASE_LATE, "insert");
        validateDeploy(steps.get(offset++), CASE_LATE, "index", EPL_LATE_INDEX);
        validateDeployed(steps.get(offset++), CASE_LATE, "index");
        validateUnrepresentable(steps.get(offset++), CASE_LATE, "getter-reset",
                NOTE_GETTER_RESET);
        for (int i = 0; i < 100; i++) {
            validateAccessSend(steps.get(offset++), CASE_LATE, i, "E" + i);
        }
        validateAccessSend(steps.get(offset++), CASE_LATE, -1, "x");
        validateUnrepresentable(steps.get(offset++), CASE_LATE, "getter-preload",
                NOTE_GETTER_PRELOAD);
        validateIndexCount(steps.get(offset++), CASE_LATE, "AWindow", "create", "indexes", 1);
        return offset;
    }

    private static void validateCaseMarker(JsonValue value, String caseName) {
        JsonObject step = object(value, "case marker");
        requireFields(step, "op", "case");
        if (!"case".equals(string(step, "op")) || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("expected case marker for " + caseName);
        }
    }

    private static void validateDeploy(JsonValue value, String caseName,
                                       String expectedStatement, String expectedEpl) {
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

    private static void validateBeanSend(JsonValue value, String caseName,
                                         String theString, int intPrimitive) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        if (!theString.equals(payload.get("theString").asString())
                || payload.get("intPrimitive").asInt() != intPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for "
                    + caseName);
        }
    }

    private static void validateS0Send(JsonValue value, String caseName) {
        JsonObject step = object(value, "SupportBean_S0 step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_S0 step is not pinned for "
                    + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
        requireFields(payload, "id", "p00");
        if (payload.get("id").asInt() != -1
                || !"x".equals(payload.get("p00").asString())) {
            throw new IllegalArgumentException("SupportBean_S0 payload is not pinned for "
                    + caseName);
        }
    }

    private static void validateAccessSend(JsonValue value, String caseName,
                                           int id, String p00) {
        JsonObject step = object(value, "SupportCountAccessEvent step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportCountAccessEvent".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportCountAccessEvent step is not pinned for "
                    + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportCountAccessEvent payload");
        requireFields(payload, "id", "p00");
        if (payload.get("id").asInt() != id
                || !p00.equals(payload.get("p00").asString())) {
            throw new IllegalArgumentException("SupportCountAccessEvent payload is not pinned"
                    + " for " + caseName + " send " + id);
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
                                         String expectedStatement, String expectedMode) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement", "mode");
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedMode.equals(string(step, "mode"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateUnrepresentable(JsonValue value, String caseName,
                                                String expectedStatement, String expectedNote) {
        JsonObject step = object(value, "unrepresentable step");
        requireFields(step, "op", "case", "statement", "expectError");
        if (!"unrepresentable".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedNote.equals(string(step, "expectError"))) {
            throw new IllegalArgumentException("unrepresentable step is not pinned for "
                    + caseName + "/" + expectedStatement);
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

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!names.add(member.getName())) {
                    throw new IllegalArgumentException("duplicate JSON object key: "
                            + member.getName());
                }
                rejectDuplicateKeys(member.getValue());
            }
        } else if (value.isArray()) {
            for (JsonValue item : value.asArray()) {
                rejectDuplicateKeys(item);
            }
        }
    }

    private static void requireFields(JsonObject object, String... expectedNames) {
        if (object == null || object.size() != expectedNames.length
                || !new HashSet<>(object.names()).equals(
                        new HashSet<>(Arrays.asList(expectedNames)))) {
            throw new IllegalArgumentException("JSON object has unexpected fields "
                    + (object == null ? "<null>" : object.names()) + ", expected "
                    + Arrays.toString(expectedNames));
        }
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be a JSON object");
        }
        return value.asObject();
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be a JSON array");
        }
        return value.asArray();
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (!(value instanceof JsonString)) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        long value = longInteger(object.get(name), name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " is outside the Java int range");
        }
        return (int) value;
    }

    private static long longInteger(JsonValue value, String label) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(label + " must be a JSON integer");
        }
        String text = value.toString();
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(label + " is outside the Java long range", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String label) {
        JsonArray actual = array(value, label);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(label + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (!(item instanceof JsonString) || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(label + " mismatch at index " + index);
            }
        }
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
}
