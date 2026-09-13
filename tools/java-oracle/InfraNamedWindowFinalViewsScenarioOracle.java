import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for the final differential slice of InfraNamedWindowViews: the
 * ord-52 {@code InfraPattern} execution (lines 3372-3396) and the ord-57
 * {@code InfraNamedWindowTimeToLiveDelete} execution (lines 109-156).
 *
 * <p>Ord 52 deploys one module (create window, pattern select s0, insert
 * trigger) and sends E1/S1(2)/S1(3)/S2(4)/S1(1): three ordered new rows on
 * s0 with silence gaps at the first and the last send (the S2 completion
 * quits the whole or-expression, including the every branch). Ord 57 deploys
 * one module (win, merge, delete) over absolute virtual time 0/500/1000/2000
 * with deletes E2/E1 and emits five any-order iterator snapshots of the win
 * statement; milestones are persistence round-trips with no observable
 * effect and stay out of the trace.
 *
 * <p>Each case runs on a fresh runtime with the clock pinned at 0 before any
 * statement is deployed. The internal timer is disabled so schedules fire
 * only on the pinned advance-time steps.
 */
public final class InfraNamedWindowFinalViewsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-named-window-final-views";
    private static final String DESCRIPTION =
            "InfraNamedWindowViews final slice: the pattern consumer over the named-window insert stream with or-quit silence, and the time-to-live window with merge insert and on-delete triggers observed through any-order iterator snapshots under absolute virtual time (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-476271957d6ffdb3a878",
            "java-runtime-3c2f3a2696127c04b2a6"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraPattern",
            "InfraNamedWindowTimeToLiveDelete"
    };
    private static final String[] STATIC_IDS = {
            "java-030c8e6d456d680e8745"
    };

    private static final String CASE_PATTERN = "pattern";
    private static final String CASE_TTL = "ttl-delete";
    private static final String[] CASE_NAMES = {CASE_PATTERN, CASE_TTL};
    private static final int[] ORDINALS = {52, 57};
    private static final String[] CASE_DESCRIPTIONS = {
            "pattern consumer over the named-window insert stream: every S1 match re-arms while the single S2 match quits the whole or-expression",
            "whole-bean timetolive window with merge insert and p00 deletes observed through any-order iterator snapshots over absolute virtual time"
    };

    private static final String EPL_CREATE_PATTERN =
            "@name('create') create window MyWindowPAT#keepall as MySimpleKeyValueMap";
    private static final String EPL_S0_PATTERN =
            "@name('s0') select a.key as key, a.value as value from pattern [every a=MyWindowPAT(key='S1') or a=MyWindowPAT(key='S2')]";
    private static final String EPL_INSERT_PATTERN =
            "insert into MyWindowPAT select theString as key, longBoxed as value from SupportBean";

    private static final String EPL_CREATE_TTL =
            "@name('win') create window MyWindow#timetolive(current_timestamp() + longPrimitive) as SupportBean";
    private static final String EPL_MERGE_TTL =
            "on SupportBean merge MyWindow insert select *";
    private static final String EPL_DELETE_TTL =
            "on SupportBean_S0 delete from MyWindow where theString = p00";

    private static final String[][] DEPLOYS = {
            {"create", "s0", "insert"},
            {"win", "merge", "delete"}
    };
    private static final String[][] LISTENED = {
            {"s0"}, {}
    };
    private static final int[] EXPECTED_CASE_RECORDS = {3, 5};
    private static final int EXPECTED_RECORDS = 8;
    private static final int EXPECTED_STEPS = 30;

    private InfraNamedWindowFinalViewsScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowFinalViewsScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray steps = array(scenario.get("steps"), "steps");

        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCommon().addEventType("MySimpleKeyValueMap", simpleKeyValueSchema());
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);

        JsonArray records = new JsonArray();
        for (int index = 0; index < CASE_NAMES.length; index++) {
            int before = records.size();
            runCaseOnFreshRuntime(CASE_NAMES[index], configuration, steps, records);
            int emitted = records.size() - before;
            if (emitted != EXPECTED_CASE_RECORDS[index]) {
                throw new IllegalStateException("case " + CASE_NAMES[index] + " emitted "
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

    private static Map<String, Object> simpleKeyValueSchema() {
        Map<String, Object> schema = new LinkedHashMap<>();
        schema.put("key", String.class);
        schema.put("value", long.class);
        return schema;
    }

    private static void runCaseOnFreshRuntime(String caseName, Configuration configuration,
                                              JsonArray allSteps, JsonArray records)
            throws Exception {
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        try {
            runtime.getEventService().advanceTime(0);
            runCase(caseName, configuration, runtime, allSteps, records);
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
        }
    }

    private static void runCase(String caseName, Configuration configuration,
                                EPRuntime runtime, JsonArray allSteps, JsonArray records)
            throws Exception {
        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statementsByName = new HashMap<>();
        List<String> pendingStatements = new ArrayList<>();
        List<String> pendingEpls = new ArrayList<>();
        boolean inCase = false;
        for (JsonValue stepValue : allSteps) {
            JsonObject step = stepValue.asObject();
            if ("case".equals(string(step, "op"))) {
                inCase = caseName.equals(string(step, "case"));
                continue;
            }
            if (!inCase) {
                continue;
            }
            String operation = string(step, "op");
            if ("deploy".equals(operation)) {
                pendingStatements.add(string(step, "statement"));
                pendingEpls.add(string(step, "epl"));
                continue;
            }
            if (!pendingStatements.isEmpty()) {
                deployModule(configuration, runtime, caseName, pendingStatements, pendingEpls,
                        statementsByName, sequences, records);
                pendingStatements.clear();
                pendingEpls.clear();
            }
            switch (operation) {
                case "send":
                    sendEvent(runtime, string(step, "eventType"),
                            object(step.get("payload"), "payload"));
                    break;
                case "snapshot": {
                    EPStatement statement = statementsByName.get(string(step, "statement"));
                    if (statement == null) {
                        throw new IllegalStateException("snapshot targets unknown statement in case "
                                + caseName);
                    }
                    records.add(snapshot(runtime, statement, caseName));
                    break;
                }
                case "advance-time":
                    runtime.getEventService().advanceTime(
                            Instant.parse(string(step, "at")).toEpochMilli());
                    break;
                case "undeploy-all":
                    runtime.getDeploymentService().undeployAll();
                    statementsByName.clear();
                    break;
                default:
                    throw new IllegalStateException("unsupported step op " + operation);
            }
        }
        if (!pendingStatements.isEmpty()) {
            deployModule(configuration, runtime, caseName, pendingStatements, pendingEpls,
                    statementsByName, sequences, records);
        }
        runtime.getDeploymentService().undeployAll();
        statementsByName.clear();
    }

    private static void deployModule(Configuration configuration, EPRuntime runtime,
                                     String caseName, List<String> statementNames,
                                     List<String> epls, Map<String, EPStatement> statementsByName,
                                     Map<String, Integer> sequences, JsonArray records)
            throws Exception {
        StringBuilder moduleEpl = new StringBuilder();
        for (int index = 0; index < epls.size(); index++) {
            if (index > 0) {
                moduleEpl.append(";\n");
            }
            moduleEpl.append(epls.get(index));
        }
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler()
                .compile(moduleEpl.toString(), compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService()
                .deploy(compiled, new DeploymentOptions());
        List<EPStatement> deployed = new ArrayList<>();
        for (EPStatement statement : deployment.getStatements()) {
            deployed.add(statement);
        }
        if (deployed.size() != statementNames.size()) {
            throw new IllegalStateException("module of case " + caseName + " with statements "
                    + statementNames + " deployed " + deployed.size() + " statements");
        }
        Set<String> listened = new HashSet<>(Arrays.asList(LISTENED[caseIndex(caseName)]));
        Set<String> scenarioNames = new HashSet<>(statementNames);
        for (int index = 0; index < statementNames.size(); index++) {
            String name = statementNames.get(index);
            EPStatement statement = deployed.get(index);
            if (scenarioNames.contains(statement.getName()) && !statement.getName().equals(name)) {
                throw new IllegalStateException("module of case " + caseName + " statement "
                        + statement.getName() + " is not the pinned statement at position "
                        + index + " (" + name + ")");
            }
            statementsByName.put(name, statement);
            if (listened.contains(name)) {
                if (!statement.getName().equals(name)) {
                    throw new IllegalStateException("listened statement " + name
                            + " is unnamed in case " + caseName);
                }
                statement.addListener(listener(caseName, sequences, records, runtime));
            }
        }
    }

    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonArray newRows = rows(newEvents, new String[]{"key", "value"});
            JsonArray oldRows = rows(oldEvents, new String[]{"key", "value"});
            if (newRows.size() == 0 && oldRows.size() == 0) {
                throw new IllegalStateException("listener for statement " + statement.getName()
                        + " was invoked without a stream in case " + caseName);
            }
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
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

    private static JsonObject snapshot(EPRuntime runtime, EPStatement statement, String caseName) {
        JsonArray rows = new JsonArray();
        for (java.util.Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
            EventBean event = iterator.next();
            JsonObject item = new JsonObject();
            item.add("kind", "row");
            JsonObject fields = new JsonObject();
            fields.add("theString", normalize(event.get("theString")));
            item.add("fields", fields);
            rows.add(item);
        }
        sortRowsCanonical(rows);
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot");
        record.add("statement", statement.getName());
        record.add("sequence", 0);
        record.add("time",
                Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
        if (rows.size() > 0) {
            record.add("new", rows);
        }
        return record;
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

    private static JsonArray rows(EventBean[] events, String[] fields) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            JsonObject item = new JsonObject();
            item.add("kind", "row");
            JsonObject values = new JsonObject();
            for (String field : fields) {
                values.add(field, normalize(event.get(field)));
            }
            item.add("fields", values);
            array.add(item);
        }
        return array;
    }

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
                bean.setTheString(string(payload, "theString"));
                JsonValue longBoxed = payload.get("longBoxed");
                if (longBoxed != null) {
                    bean.setLongBoxed(longInteger(longBoxed, "longBoxed"));
                }
                JsonValue longPrimitive = payload.get("longPrimitive");
                if (longPrimitive != null) {
                    bean.setLongPrimitive(longInteger(longPrimitive, "longPrimitive"));
                }
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S0": {
                int id = 0;
                JsonValue idValue = payload.get("id");
                if (idValue != null) {
                    id = (int) longInteger(idValue, "id");
                }
                runtime.getEventService().sendEventBean(
                        new SupportBean_S0(id, string(payload, "p00")), type);
                break;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
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
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASE_NAMES.length) {
            throw new IllegalArgumentException("scenario must contain exactly two cases");
        }
        String[] observations = {"listener", "iterator"};
        int[] snapshots = {0, 5};
        String[] epls = {EPL_CREATE_PATTERN, EPL_CREATE_TTL};
        String[][] caseEpls = {
                {EPL_CREATE_PATTERN, EPL_INSERT_PATTERN, EPL_S0_PATTERN, "", ""},
                {EPL_CREATE_TTL, "", "", EPL_MERGE_TTL, EPL_DELETE_TTL}
        };
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "description", "observation", "iteratorSnapshots", "epl", "createEpl",
                    "insertEpl", "s0Epl", "mergeEpl", "deleteEpl", "deploys", "listened");
            if (!CASE_NAMES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !CASE_DESCRIPTIONS[index].equals(string(definition, "description"))
                    || !observations[index].equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != snapshots[index]
                    || !epls[index].equals(string(definition, "epl"))
                    || !caseEpls[index][0].equals(string(definition, "createEpl"))
                    || !caseEpls[index][1].equals(string(definition, "insertEpl"))
                    || !caseEpls[index][2].equals(string(definition, "s0Epl"))
                    || !caseEpls[index][3].equals(string(definition, "mergeEpl"))
                    || !caseEpls[index][4].equals(string(definition, "deleteEpl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
            validateStringArray(definition.get("deploys"), DEPLOYS[index],
                    CASE_NAMES[index] + " deploys");
            validateStringArray(definition.get("listened"), LISTENED[index],
                    CASE_NAMES[index] + " listened");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        offset = validatePatternCase(steps, offset);
        offset = validateTtlCase(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario has an unexpected step suffix");
        }
    }

    private static int validatePatternCase(JsonArray steps, int offset) {
        String name = CASE_PATTERN;
        validateCaseMarker(steps.get(offset++), name);
        validateDeploy(steps.get(offset++), name, "create", EPL_CREATE_PATTERN);
        validateDeploy(steps.get(offset++), name, "s0", EPL_S0_PATTERN);
        validateDeploy(steps.get(offset++), name, "insert", EPL_INSERT_PATTERN);
        validateSupportBean(steps.get(offset++), name, "E1", 1L);
        validateSupportBean(steps.get(offset++), name, "S1", 2L);
        validateSupportBean(steps.get(offset++), name, "S1", 3L);
        validateSupportBean(steps.get(offset++), name, "S2", 4L);
        validateSupportBean(steps.get(offset++), name, "S1", 1L);
        validateUndeploy(steps.get(offset++), name);
        return offset;
    }

    private static int validateTtlCase(JsonArray steps, int offset) {
        String name = CASE_TTL;
        validateCaseMarker(steps.get(offset++), name);
        validateAdvance(steps.get(offset++), name, "1970-01-01T00:00:00Z");
        validateDeploy(steps.get(offset++), name, "win", EPL_CREATE_TTL);
        validateDeploy(steps.get(offset++), name, "merge", EPL_MERGE_TTL);
        validateDeploy(steps.get(offset++), name, "delete", EPL_DELETE_TTL);
        validateSupportBeanLongPrim(steps.get(offset++), name, "E1", 2000L);
        validateSupportBeanLongPrim(steps.get(offset++), name, "E2", 3000L);
        validateSupportBeanLongPrim(steps.get(offset++), name, "E3", 1000L);
        validateSupportBeanLongPrim(steps.get(offset++), name, "E4", 2000L);
        validateSnapshot(steps.get(offset++), name, "win");
        validateAdvance(steps.get(offset++), name, "1970-01-01T00:00:00.500Z");
        validateS0(steps.get(offset++), name, "E2");
        validateSnapshot(steps.get(offset++), name, "win");
        validateS0(steps.get(offset++), name, "E1");
        validateSnapshot(steps.get(offset++), name, "win");
        validateAdvance(steps.get(offset++), name, "1970-01-01T00:00:01Z");
        validateSnapshot(steps.get(offset++), name, "win");
        validateAdvance(steps.get(offset++), name, "1970-01-01T00:00:02Z");
        validateSnapshot(steps.get(offset++), name, "win");
        validateUndeploy(steps.get(offset++), name);
        return offset;
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject step = object(value, "case marker");
        requireFields(step, "op", "case");
        if (!"case".equals(string(step, "op")) || !expectedCase.equals(string(step, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    private static void validateDeploy(JsonValue value, String caseName, String statement,
                                       String epl) {
        JsonObject step = object(value, "deploy step");
        requireFields(step, "op", "case", "statement", "epl");
        if (!"deploy".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !statement.equals(string(step, "statement"))
                || !epl.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("deploy step is not pinned for " + caseName + "/"
                    + statement);
        }
    }

    private static void validateSupportBean(JsonValue value, String caseName, String text,
                                            long longBoxed) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "longBoxed");
        if (!text.equals(string(payload, "theString"))
                || longInteger(payload.get("longBoxed"), "longBoxed") != longBoxed) {
            throw new IllegalArgumentException("SupportBean payload is not pinned");
        }
    }

    private static void validateSupportBeanLongPrim(JsonValue value, String caseName, String text,
                                                    long longPrimitive) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "longPrimitive");
        if (!text.equals(string(payload, "theString"))
                || longInteger(payload.get("longPrimitive"), "longPrimitive") != longPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned");
        }
    }

    private static void validateS0(JsonValue value, String caseName, String p00) {
        JsonObject step = object(value, "SupportBean_S0 step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_S0 step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
        requireFields(payload, "p00");
        if (!p00.equals(string(payload, "p00"))) {
            throw new IllegalArgumentException("SupportBean_S0 payload is not pinned");
        }
    }

    private static void validateSnapshot(JsonValue value, String caseName, String statement) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement", "mode");
        if (!"snapshot".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !statement.equals(string(step, "statement"))
                || !"any".equals(string(step, "mode"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
                    + statement);
        }
    }

    private static void validateAdvance(JsonValue value, String caseName, String at) {
        JsonObject step = object(value, "advance-time step");
        requireFields(step, "op", "case", "at");
        if (!"advance-time".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !at.equals(string(step, "at"))) {
            throw new IllegalArgumentException("advance-time step is not pinned for " + caseName);
        }
    }

    private static void validateUndeploy(JsonValue value, String caseName) {
        JsonObject step = object(value, "undeploy-all step");
        requireFields(step, "op", "case");
        if (!"undeploy-all".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("undeploy-all step is not pinned for " + caseName);
        }
    }

    private static int caseIndex(String caseName) {
        for (int index = 0; index < CASE_NAMES.length; index++) {
            if (CASE_NAMES[index].equals(caseName)) {
                return index;
            }
        }
        throw new IllegalArgumentException("unknown case " + caseName);
    }

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!names.add(member.getName())) {
                    throw new IllegalArgumentException("duplicate JSON object key: " + member.getName());
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
        try {
            return Long.parseLong(value.toString(), 10);
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
