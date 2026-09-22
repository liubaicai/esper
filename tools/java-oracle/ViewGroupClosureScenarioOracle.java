import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportMarketDataBean;
import com.espertech.esper.regressionlib.support.bean.SupportSensorEvent;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Scenario oracle for the ViewGroup closure executions (ords 7, 8 and 19 —
 * the last unreferenced executions of ViewGroup.java). Replays each case on
 * one fresh runtime:
 *
 * invalid (ord 7, ViewGroupInvalid): five path-less tryInvalidCompile
 * probes pin the groupwin-declaration rejection prefixes — multiple
 * groupwin declarations, a groupwin with no child view, a groupwin that is
 * not the first declaration, a merge view combined with multiple data
 * windows (emitted as the pinned "unrepresentable" record because the Go
 * API has no merge view), and a null-typed group-window criteria
 * expression.
 *
 * length-win-weighted-avg (ord 8, ViewGroupLengthWinWeightAvg,
 * EXCLUDEWHENINSTRUMENTED+PERFORMANCE): deploys the
 * #groupwin(type)#length(10000000)#weighted_avg(measurement, confidence)
 * statement with a listener, sends a 100-event prime batch and a
 * 10000-event measured batch of SupportSensorEvent beans whose
 * measurement/confidence advance with the batch index (the suite's
 * (double) i constructor arguments), asserts the sub-second wall-clock
 * bound, and undeploys. The listener never asserts, so the trace stays
 * thin: one deployed record plus one sent marker per batch.
 *
 * escaped-property-text (ord 19, ViewGroupEscapedPropertyText,
 * SERDEREQUIRED): compiles and deploys the three-statement module —
 * create schema event as EventWithTags, insert into stream1, and the
 * groupwin(name, tags('a\.b')) length-10 select with having count(1) >= 5 —
 * then undeploys. The escaped mapped-property key 'a\.b' resolves to the
 * map key "a.b". Deployed records only.
 */
public final class ViewGroupClosureScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "view-group-closure";
    private static final String DESCRIPTION =
            "ViewGroup closure surface (ords 7, 8, 19): invalid replays ViewGroupInvalid's five "
                    + "tryInvalidCompile probes pinning the groupwin-declaration rejection prefixes "
                    + "(multiple groupwin declarations, groupwin without a child view, groupwin not "
                    + "in first position, merge with multiple data windows — unrepresentable in Go, "
                    + "and a null-typed criteria expression); length-win-weighted-avg replays "
                    + "ViewGroupLengthWinWeightAvg's performance smoke, deploying the "
                    + "groupwin(type)/length(10000000)/weighted_avg(measurement,confidence) statement "
                    + "and sending 100 prime plus 10000 measured SupportSensorEvent beans with only "
                    + "deployed/sent markers (the wall-clock assert is not trace-visible); "
                    + "escaped-property-text replays ViewGroupEscapedPropertyText's "
                    + "compile+deploy+undeploy of the three-statement module whose select groups "
                    + "stream1 by name and the escaped mapped property tags('a\\.b') under having "
                    + "count(1) >= 5. compile-error records carry the pinned Java message prefixes; "
                    + "the unrepresentable record pins the merge-view prefix; deployed records mark "
                    + "each module statement; sent records pin the batch sizes (Java source "
                    + "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/"
                    + "ViewGroup.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/"
                    + "ViewGroup.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-a72aa6eebc4ce8d21115",
            "java-runtime-e718af611543b6d40436",
            "java-runtime-d9b2e762c318369875e5"
    };
    private static final String[] EXECUTION_NAMES = {
            "ViewGroupInvalid",
            "ViewGroupLengthWinWeightAvg",
            "ViewGroupEscapedPropertyText"
    };
    private static final String[] STATIC_IDS = {
            "java-cd39f97bf5d0490c295a",
            "java-586becfef68e2842ef18",
            "java-3426d2a7294c0f93c788"
    };
    private static final String[] JAVA_FLAGS = {
            "EXCLUDEWHENINSTRUMENTED", "PERFORMANCE", "SERDEREQUIRED"
    };
    private static final String[] CASES = {
            "invalid",
            "length-win-weighted-avg",
            "escaped-property-text"
    };
    private static final int[] ORDINALS = {7, 8, 19};

    // Verbatim transcriptions of ViewGroup lines 85, 89, 93, 97, 101-102,
    // 460 and 65-71.
    private static final String EPL_PROBE_MULTIPLE =
            "select * from SupportBean#groupwin(theString)#length(1)#groupwin(theString)"
                    + "#uni(intPrimitive)";
    private static final String EPL_PROBE_NO_CHILD =
            "select avg(price), symbol from SupportMarketDataBean#length(100)#groupwin(symbol)";
    private static final String EPL_PROBE_NOT_FIRST =
            "select * from SupportBean#keepall#groupwin(theString)#length(2)";
    private static final String EPL_PROBE_MERGE =
            "select * from SupportBean#groupwin(theString)#length(2)#merge(theString)#keepall";
    private static final String EPL_PROBE_NULL_FIELD =
            "create schema MyEvent(somefield null);\n"
                    + "select * from MyEvent#groupwin(somefield)#length(2)";
    private static final String EPL_LENGTH_WIN =
            "@name('s0') select * from SupportSensorEvent#groupwin(type)#length(10000000)"
                    + "#weighted_avg(measurement, confidence)";
    private static final String EPL_MODULE =
            "create schema event as com.espertech.esper.regressionlib.suite.view.ViewGroup"
                    + "$EventWithTags;\n"
                    + "\n"
                    + "insert into stream1\n"
                    + "select name, tags from event;\n"
                    + "\n"
                    + "select name, tags('a\\.b') from stream1.std:groupwin(name, tags('a\\.b'))"
                    + ".win:length(10)\n"
                    + "having count(1) >= 5;\n";

    private static final String ERR_MULTIPLE =
            "Failed to validate data window declaration: Multiple groupwin-declarations are not "
                    + "supported";
    private static final String ERR_NO_CHILD =
            "Failed to validate data window declaration: Invalid use of the 'groupwin' view, the "
                    + "view requires one or more child views to group, or consider using the "
                    + "group-by clause";
    private static final String ERR_NOT_FIRST =
            "Failed to validate data window declaration: The 'groupwin' declaration must occur in "
                    + "the first position";
    private static final String ERR_MERGE =
            "Failed to validate data window declaration: The 'merge' declaration cannot be used in "
                    + "conjunction with multiple data windows";
    private static final String ERR_NULL_FIELD =
            "Failed to validate data window declaration: Group-window received a null-typed "
                    + "criteria expression";

    private static final String[] CASE_OBSERVATIONS = {
            "compile-error+unrepresentable; five path-less tryInvalidCompile probes pin the "
                    + "groupwin-declaration rejections: multiple groupwin declarations, groupwin "
                    + "without a child view, groupwin not in first position, the merge view over "
                    + "multiple data windows (pinned-only; no Go merge view), and the null-typed "
                    + "criteria expression",
            "deployed+sent; the groupwin(type)/length(10000000)/weighted_avg statement deploys with "
                    + "a listener, then 100 prime and 10000 measured SupportSensorEvent sends "
                    + "replay as index-sequenced batches; the suite's sub-second wall-clock assert "
                    + "is not trace-visible so only the deployed marker and per-batch sent counts "
                    + "record",
            "deployed; the three-statement module (create schema event as EventWithTags, insert "
                    + "into stream1 select name, tags, select name, tags('a\\.b') from stream1 "
                    + "groupwin(name, tags('a\\.b')) length(10) having count(1) >= 5) compiles and "
                    + "deploys, then undeploy-all retires it"
    };
    private static final String[] CASE_EPLS = {
            EPL_PROBE_MULTIPLE,
            EPL_LENGTH_WIN,
            EPL_MODULE
    };

    // Deployed-step labels each case's deploy registers, in module
    // statement order (the escaped-property-text module produces the
    // create-schema, insert-into and select statements).
    private static final Map<String, String[]> DEPLOY_LABELS = new HashMap<>();
    static {
        DEPLOY_LABELS.put("length-win-weighted-avg", new String[]{"s0"});
        DEPLOY_LABELS.put("escaped-property-text", new String[]{"schema", "insert", "select"});
    }

    private static final int EXPECTED_STEPS = 18;
    private static final int EXPECTED_RECORDS = 11;

    /**
     * Pinned per-case step keys rendered as
     * op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|mode|ids|count.
     * build-error steps carry the byte-exact probe EPL and the pinned
     * expectError prefix; every probe is path-less (compileWithoutPath=1)
     * mirroring env.tryInvalidCompile(epl, ...). The unrepresentable step
     * pins the merge-view prefix in expectError. send-batch steps pin the
     * first event's payload, the index-sequence mode and the batch count;
     * the deploy steps carry the byte-exact EPL text.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("invalid", new String[]{
                "build-error|multiple-groupwin|||" + EPL_PROBE_MULTIPLE + "||" + ERR_MULTIPLE
                        + "|1|||",
                "build-error|groupwin-no-child|||" + EPL_PROBE_NO_CHILD + "||" + ERR_NO_CHILD
                        + "|1|||",
                "build-error|groupwin-not-first|||" + EPL_PROBE_NOT_FIRST + "||" + ERR_NOT_FIRST
                        + "|1|||",
                "unrepresentable|groupwin-merge|||" + EPL_PROBE_MERGE + "||" + ERR_MERGE
                        + "|1|||",
                "build-error|groupwin-null-field|||" + EPL_PROBE_NULL_FIELD + "||" + ERR_NULL_FIELD
                        + "|1|||",
        });
        CASE_STEPS.put("length-win-weighted-avg", new String[]{
                "deploy|s0|||" + EPL_LENGTH_WIN + "||||||",
                "deployed|s0|||||||||",
                "send-batch||prime|SupportSensorEvent||{\"id\":0,\"type\":\"A\",\"device\":\"1\","
                        + "\"measurement\":0,\"confidence\":0}|||index-sequence||100",
                "send-batch||measure|SupportSensorEvent||{\"id\":0,\"type\":\"A1\",\"device\":\"1\","
                        + "\"measurement\":0,\"confidence\":0}|||index-sequence||10000",
                "undeploy-all||||||||||",
        });
        CASE_STEPS.put("escaped-property-text", new String[]{
                "deploy|module|||" + EPL_MODULE + "||||||",
                "deployed|schema|||||||||",
                "deployed|insert|||||||||",
                "deployed|select|||||||||",
                "undeploy-all||||||||||",
        });
    }

    private ViewGroupClosureScenarioOracle() {
    }
    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ViewGroupClosureScenarioOracle <scenario.json>");
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
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(caseIndex, allSteps, records);
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

    /**
     * Replays the case's steps on a fresh runtime. Deploy steps compile
     * with the runtime path (env.compileDeploy(epl, path)); the probes
     * compile without it, mirroring env.tryInvalidCompile's path-less
     * compileWCheckedEx.
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportMarketDataBean.class);
        configuration.getCommon().addEventType(SupportSensorEvent.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-" + ID + "-" + RUNTIME_IDS[caseIndex], configuration);
        runtime.getEventService().advanceTime(0);
        try {
            Set<String> deployedLabels = new HashSet<>();
            Map<String, Integer> sequences = new HashMap<>();
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
                    case "deploy":
                        deployStep(runtime, configuration, caseName, step, deployedLabels);
                        break;
                    case "deployed":
                        deployedStep(runtime, caseName, step, deployedLabels, sequences, records);
                        break;
                    case "send-batch":
                        sendBatchStep(runtime, step);
                        sentStep(caseName, step, records);
                        break;
                    case "build-error":
                        buildErrorStep(configuration, caseName, step, records);
                        break;
                    case "unrepresentable":
                        unrepresentableStep(configuration, caseName, step, records);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
                }
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
     * Compiles and deploys the step's module with the runtime path,
     * mirroring env.compileDeploy(epl, path). The deployed statement
     * labels register in module order so the deployed markers resolve;
     * the length-win-weighted-avg deploy attaches the listener the suite
     * registers (a no-op: the suite never asserts a delivery).
     */
    private static void deployStep(EPRuntime runtime, Configuration configuration, String caseName,
                                   JsonObject step, Set<String> deployedLabels) throws Exception {
        String epl = string(step, "epl");
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
        String[] labels = DEPLOY_LABELS.get(caseName);
        EPStatement[] statements = deployment.getStatements();
        if (labels == null || statements.length != labels.length) {
            throw new IllegalStateException(caseName + " module produced " + statements.length
                    + " statements, want " + (labels == null ? 0 : labels.length));
        }
        for (String label : labels) {
            deployedLabels.add(label);
        }
        if ("length-win-weighted-avg".equals(caseName)) {
            for (EPStatement statement : statements) {
                statement.addListener((newData, oldData, stmt, rt) -> {
                });
            }
        }
    }

    /**
     * Emits the deployed marker for a statement label the preceding deploy
     * registered, mirroring the per-statement deployed record.
     */
    private static void deployedStep(EPRuntime runtime, String caseName, JsonObject step,
                                     Set<String> deployedLabels, Map<String, Integer> sequences,
                                     JsonArray records) {
        String label = string(step, "statement");
        if (!deployedLabels.contains(label)) {
            throw new IllegalStateException("deployed marker for unknown statement " + label);
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
    }

    /**
     * Replays one send-batch step: count SupportSensorEvent beans whose
     * measurement/confidence advance with the batch index, mirroring the
     * suite's new SupportSensorEvent(0, type, "1", (double) i, (double) i)
     * constructor arguments (the index-sequence mode).
     */
    private static void sendBatchStep(EPRuntime runtime, JsonObject step) {
        String type = string(step, "eventType");
        if (!"SupportSensorEvent".equals(type)) {
            throw new IllegalStateException("unknown send-batch type: " + type);
        }
        if (!"index-sequence".equals(string(step, "mode"))) {
            throw new IllegalStateException("send-batch mode is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "payload");
        int id = integer(payload, "id");
        String sensorType = string(payload, "type");
        String device = string(payload, "device");
        int count = integer(step, "count");
        for (int i = 0; i < count; i++) {
            runtime.getEventService().sendEventBean(
                    new SupportSensorEvent(id, sensorType, device, (double) i, (double) i), type);
        }
    }

    /**
     * Emits the thin trace's sent marker carrying the batch name and the
     * observed send count.
     */
    private static void sentStep(String caseName, JsonObject step, JsonArray records) {
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "sent");
        record.add("name", string(step, "name"));
        record.add("count", integer(step, "count"));
        records.add(record);
    }

    /**
     * Compiles an expected-invalid probe and emits {"operation":"compile-error"}
     * carrying the pinned expectError prefix after verifying the caught
     * message starts with it (SupportMessageAssertUtil.assertMessage
     * semantics). Every probe compiles without the runtime path, mirroring
     * env.tryInvalidCompile's path-less compileWCheckedEx.
     */
    private static void buildErrorStep(Configuration configuration, String caseName,
                                       JsonObject step, JsonArray records) throws Exception {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        String epl = string(step, "epl");
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            if (!step.getBoolean("compileWithoutPath", false)) {
                throw new IllegalStateException("build-error probe " + label
                        + " is not marked compileWithoutPath");
            }
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            caught = null;
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught == null) {
            throw new IllegalStateException("build-error probe " + label
                    + " unexpectedly succeeded");
        }
        if (!caught.startsWith(expected)) {
            throw new IllegalStateException("compile-error message drift for " + label
                    + ": expected prefix [" + expected + "] got [" + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "compile-error");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", expected);
        records.add(record);
    }

    /**
     * Compiles the merge-view probe (which fails with the pinned prefix)
     * and emits the pinned "unrepresentable" record: the Go API has no
     * merge WindowSpec, so the record documents the Java rejection without
     * a Go boundary (the ContextLifecycle virtual-data-window precedent).
     */
    private static void unrepresentableStep(Configuration configuration, String caseName,
                                            JsonObject step, JsonArray records) throws Exception {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        String epl = string(step, "epl");
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            if (!step.getBoolean("compileWithoutPath", false)) {
                throw new IllegalStateException("unrepresentable step " + label
                        + " is not marked compileWithoutPath");
            }
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            caught = null;
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught == null) {
            throw new IllegalStateException("unrepresentable probe " + label
                    + " unexpectedly succeeded");
        }
        if (!caught.startsWith(expected)) {
            throw new IllegalStateException("unrepresentable message drift for " + label
                    + ": expected prefix [" + expected + "] got [" + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "unrepresentable");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", expected);
        records.add(record);
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
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
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !CASE_OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        for (String caseName : CASES) {
            validateCaseMarker(steps.get(offset++), caseName);
            String[] expected = CASE_STEPS.get(caseName);
            for (String key : expected) {
                JsonObject step = object(steps.get(offset++), "step");
                String actual = stepKey(step);
                if (!key.equals(actual)) {
                    throw new IllegalArgumentException("step is not pinned for " + caseName
                            + ": expected [" + key + "] got [" + actual + "]");
                }
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    /**
     * Renders one step as its pinned key:
     * op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|mode|ids|count
     * with the payload, ids and count rendered as compacted JSON. Unknown
     * fields are rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
                "op", "case", "statement", "name", "eventType", "epl", "payload",
                "expectError", "compileWithoutPath", "mode", "ids", "count"));
        for (String field : step.names()) {
            if (!allowed.contains(field)) {
                throw new IllegalArgumentException("step has unexpected field " + field);
            }
        }
        JsonValue payload = step.get("payload");
        String payloadText = payload == null ? "" : payload.toString();
        String cwp = step.getBoolean("compileWithoutPath", false) ? "1" : "";
        JsonValue ids = step.get("ids");
        String idsText = ids == null ? "" : ids.toString();
        JsonValue count = step.get("count");
        String countText = count == null ? "" : count.toString();
        return string(step, "op") + "|" + string(step, "statement") + "|" + string(step, "name")
                + "|" + string(step, "eventType") + "|" + string(step, "epl") + "|" + payloadText
                + "|" + string(step, "expectError") + "|" + cwp
                + "|" + string(step, "mode") + "|" + idsText + "|" + countText;
    }

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!names.add(member.getName())) {
                    throw new IllegalArgumentException(
                            "duplicate JSON object key: " + member.getName());
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
                || !new HashSet<>(object.names()).equals(new HashSet<>(Arrays.asList(expectedNames)))) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
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
        if (value == null) {
            return "";
        }
        if (!(value instanceof JsonString)) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be a JSON integer");
        }
        String text = value.toString();
        try {
            long parsed = Long.parseLong(text, 10);
            if (parsed < Integer.MIN_VALUE || parsed > Integer.MAX_VALUE) {
                throw new IllegalArgumentException(name + " is outside the Java int range");
            }
            return (int) parsed;
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(name + " is outside the Java long range", ex);
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
}
