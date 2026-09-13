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
import java.util.Collections;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for the insert-shape executions in InfraNamedWindowViews.
 *
 * <p>The fixed regression source compiles each execution as one module.  The
 * scenario keeps the source's deploy labels as ordered actions, while this
 * oracle joins each case's contiguous deploy actions back into that one module.
 * This preserves the source's statement visibility and, in particular, the
 * preemptive cascade in ordinal 56.  Every case starts on a fresh runtime with
 * virtual time at epoch zero.</p>
 */
public final class InfraNamedWindowInsertShapeScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-named-window-insert-shape";
    private static final String DESCRIPTION =
            "InfraNamedWindowViews insert-shape slice: duplicate insert delivery into one keep-all window, intersecting length/unique retention with old-stream replacement, the object-array stream-star insert whose undeclared column is an approved typed-builder difference, and preemptive on-trigger cascade across two named windows (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-1b9f5b6ccf6b49cdba09",
            "java-runtime-f739aa91028d27aa572f",
            "java-runtime-9740441818d84a3ee94d",
            "java-runtime-71402af94b27b4255ab8"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraDoubleInsertSameWindow",
            "InfraIntersection",
            "InfraSelectStreamDotStarInsert",
            "InfraOnInsertPremptiveTwoWindow"
    };
    private static final String[] STATIC_IDS = {
            "java-bed54e012c8a814aa1b2",
            "java-1aec04798b31edb42da8",
            "java-e9459e4192187d4bf9a9",
            "java-657282705cf5d824b721"
    };

    private static final String CASE_DOUBLE = "double-insert-same-window";
    private static final String CASE_INTERSECTION = "intersection";
    private static final String CASE_OBJECT_ARRAY = "select-stream-dot-star-insert";
    private static final String CASE_PREEMPTIVE = "on-insert-preemptive-two-window";
    private static final String[] CASE_NAMES = {
            CASE_DOUBLE, CASE_INTERSECTION, CASE_OBJECT_ARRAY, CASE_PREEMPTIVE
    };
    private static final int[] ORDINALS = {28, 36, 54, 56};
    private static final String[] CASE_DESCRIPTIONS = {
            "keepall named window receives two independent inserts from each SupportBean and its create/s0 listeners flatten the two rows in insert order",
            "length(2) and unique(intPrimitive) intersect so E3/intPrimitive=2 emits E3 and expires E1/E2 as old rows",
            "object-array keepall deployment accepts the stream-star c0 source in Java but the typed Go builder rejects undeclared c0 while p0-only remains expressible",
            "TypeTrigger fires both on-trigger inserts preemptively so OtherStream's s0 observes WinTwo col2=9"
    };

    private static final String EPL_CREATE_DOUBLE =
            "@name('create') create window MyWindowDISM#keepall as MySimpleKeyValueMap";
    private static final String EPL_INSERT_ONE =
            "insert into MyWindowDISM select theString as key, longBoxed+1 as value from SupportBean";
    private static final String EPL_INSERT_TWO =
            "insert into MyWindowDISM select theString as key, longBoxed+2 as value from SupportBean";
    private static final String EPL_S0_DOUBLE =
            "@name('s0') select key, value as value from MyWindowDISM";

    private static final String EPL_CREATE_INTERSECTION =
            "create window MyWindowINT#length(2)#unique(intPrimitive) as SupportBean";
    private static final String EPL_INSERT_INTERSECTION =
            "insert into MyWindowINT select * from SupportBean";
    private static final String EPL_S0_INTERSECTION =
            "@name('s0') select irstream * from MyWindowINT";

    private static final String EPL_CREATE_OBJECT_ARRAY =
            "@EventRepresentation('objectarray') create window MyNWWindowObjectArray#keepall (p0 int)";
    private static final String EPL_INSERT_OBJECT_ARRAY =
            "insert into MyNWWindowObjectArray select intPrimitive as p0, sb.* as c0 from SupportBean as sb";

    private static final String EPL_SCHEMA_PREEMPTIVE =
            "@public @buseventtype create schema TypeOne(col1 int);\n"
                    + "@public @buseventtype create schema TypeTwo(col2 int);\n"
                    + "@public @buseventtype create schema TypeTrigger(trigger int)";
    private static final String EPL_CREATE_PREEMPTIVE =
            "create window WinOne#keepall as TypeOne;\ncreate window WinTwo#keepall as TypeTwo";
    private static final String EPL_INSERT_PREEMPTIVE =
            "@name('insert-window-one') insert into WinOne(col1) select intPrimitive from SupportBean";
    private static final String EPL_S2_PREEMPTIVE =
            "@name('insert-otherstream') on TypeTrigger insert into OtherStream select col1 from WinOne";
    private static final String EPL_S3_PREEMPTIVE =
            "@name('insert-window-two') on TypeTrigger insert into WinTwo(col2) select col1 from WinOne";
    private static final String EPL_S0_PREEMPTIVE =
            "@name('s0') on OtherStream select col2 from WinTwo";

    private static final String[][] DEPLOYS = {
            {"create", "insert-one", "insert-two", "s0"},
            {"create", "insert", "s0"},
            {"create", "insert"},
            {"schema", "create", "insert", "s2", "s3", "s0"}
    };
    private static final String[][] LISTENED = {
            {"create", "s0"}, {"s0"}, {}, {"s0"}
    };
    private static final int[] EXPECTED_CASE_RECORDS = {4, 3, 0, 1};
    private static final int EXPECTED_RECORDS = 8;
    private static final int EXPECTED_STEPS = 29;

    private InfraNamedWindowInsertShapeScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowInsertShapeScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType("MySimpleKeyValueMap", simpleKeyValueSchema());
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);

        JsonArray records = new JsonArray();
        for (int index = 0; index < CASE_NAMES.length; index++) {
            int before = records.size();
            runCase(CASE_NAMES[index], RUNTIME_IDS[index], configuration, steps, records);
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
        Map<String, Object> schema = new HashMap<>();
        schema.put("key", String.class);
        schema.put("value", long.class);
        return schema;
    }

    /** Replays one case on a fresh runtime with all contiguous deploy labels joined. */
    private static void runCase(String caseName, String runtimeId, Configuration configuration,
                                JsonArray allSteps, JsonArray records) throws Exception {
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeId + "-oracle", configuration);
        Map<String, Integer> sequences = new HashMap<>();
        try {
            runtime.getEventService().advanceTime(0);
            boolean inCase = false;
            List<String> deployEpls = new ArrayList<>();
            for (JsonValue value : allSteps) {
                JsonObject step = value.asObject();
                if ("case".equals(string(step, "op"))) {
                    inCase = caseName.equals(string(step, "case"));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                String operation = string(step, "op");
                if ("deploy".equals(operation)) {
                    deployEpls.add(string(step, "epl"));
                    continue;
                }
                if (!deployEpls.isEmpty()) {
                    deployModule(configuration, runtime, caseName, deployEpls, sequences, records);
                    deployEpls.clear();
                }
                switch (operation) {
                    case "send":
                        sendEvent(runtime, string(step, "eventType"),
                                object(step.get("payload"), "payload"));
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
                }
            }
            if (!deployEpls.isEmpty()) {
                deployModule(configuration, runtime, caseName, deployEpls, sequences, records);
            }
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
        }
    }

    /** Compiles exactly one module per fixed Java execution. */
    private static void deployModule(Configuration configuration, EPRuntime runtime,
                                     String caseName, List<String> epls,
                                     Map<String, Integer> sequences, JsonArray records)
            throws Exception {
        StringBuilder module = new StringBuilder();
        for (int index = 0; index < epls.size(); index++) {
            if (index > 0) {
                module.append(";\n");
            }
            module.append(epls.get(index));
        }
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(module.toString(), compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                new DeploymentOptions());
        Set<String> listened = new HashSet<>(Arrays.asList(LISTENED[caseIndex(caseName)]));
        for (EPStatement statement : deployment.getStatements()) {
            if (listened.contains(statement.getName())) {
                statement.addListener(listener(caseName, sequences, records, runtime));
            }
        }
    }

    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            String[] fields = listenerFields(caseName, statement.getName());
            JsonArray newRows = rows(newEvents, fields);
            JsonArray oldRows = rows(oldEvents, fields);
            if (CASE_INTERSECTION.equals(caseName)) {
                sortRowsCanonical(newRows);
                sortRowsCanonical(oldRows);
            }
            if (newRows.size() == 0 && oldRows.size() == 0) {
                throw new IllegalStateException("listener " + statement.getName()
                        + " was invoked without new or old data");
            }
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time", Instant.ofEpochMilli(
                    runtime.getEventService().getCurrentTime()).toString());
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        };
    }

    private static String[] listenerFields(String caseName, String statement) {
        if (CASE_DOUBLE.equals(caseName)) {
            return new String[]{"key", "value"};
        }
        if (CASE_INTERSECTION.equals(caseName) && "s0".equals(statement)) {
            return new String[]{"theString", "intPrimitive"};
        }
        if (CASE_PREEMPTIVE.equals(caseName) && "s0".equals(statement)) {
            return new String[]{"col2"};
        }
        throw new IllegalStateException("unexpected listener " + statement + " in " + caseName);
    }

    private static JsonArray rows(EventBean[] events, String[] fields) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            JsonObject row = new JsonObject();
            row.add("kind", "row");
            JsonObject values = new JsonObject();
            for (String field : fields) {
                values.add(field, normalize(event.get(field)));
            }
            row.add("fields", values);
            array.add(row);
        }
        return array;
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

    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullValue = new JsonObject();
            nullValue.add("state", "null");
            return nullValue;
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

    private static void sendEvent(EPRuntime runtime, String eventType, JsonObject payload) {
        switch (eventType) {
            case "SupportBean": {
                SupportBean bean = new SupportBean();
                JsonValue theString = payload.get("theString");
                if (theString != null && !theString.isNull()) {
                    bean.setTheString(string(payload, "theString"));
                }
                JsonValue intPrimitive = payload.get("intPrimitive");
                if (intPrimitive != null) {
                    bean.setIntPrimitive((int) longInteger(intPrimitive, "intPrimitive"));
                }
                JsonValue longBoxed = payload.get("longBoxed");
                if (longBoxed != null && !longBoxed.isNull()) {
                    bean.setLongBoxed(longInteger(longBoxed, "longBoxed"));
                }
                runtime.getEventService().sendEventBean(bean, eventType);
                return;
            }
            case "TypeTrigger":
                // The Java EVENTSENDER execution sends a zero-field map (or an
                // empty object array when that is the engine default); trigger
                // is not read by either on-trigger statement.
                runtime.getEventService().sendEventMap(new HashMap<>(), eventType);
                return;
            default:
                throw new IllegalArgumentException("unknown event type " + eventType);
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
        validateStringArray(scenario.get("javaFlags"), new String[]{"EVENTSENDER"}, "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASE_NAMES.length) {
            throw new IllegalArgumentException("scenario must contain exactly four cases");
        }
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "description", "observation", "iteratorSnapshots", "epl", "createEpl",
                    "createEplObjectArray", "createEplMap", "createEplAvro", "insertEpl",
                    "s0Epl", "s2Epl", "s3Epl", "consumeEpl", "deleteEpl", "varEpl",
                    "onSetEpl", "schemaEpl", "updateEpl", "fafEpl", "negativeCompileEpl",
                    "deploys", "listened");
            if (!CASE_NAMES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !CASE_DESCRIPTIONS[index].equals(string(definition, "description"))
                    || !"listener".equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 0
                    || !caseEpl(index).equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
            validateCaseEpls(definition, index);
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
        offset = validateDoubleCase(steps, offset);
        offset = validateIntersectionCase(steps, offset);
        offset = validateObjectArrayCase(steps, offset);
        offset = validatePreemptiveCase(steps, offset);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario has an unexpected step suffix");
        }
    }

    private static void validateCaseEpls(JsonObject definition, int index) {
        String[] values = {
                string(definition, "createEpl"), string(definition, "createEplObjectArray"),
                string(definition, "createEplMap"), string(definition, "createEplAvro"),
                string(definition, "insertEpl"), string(definition, "s0Epl"),
                string(definition, "s2Epl"), string(definition, "s3Epl"),
                string(definition, "consumeEpl"), string(definition, "deleteEpl"),
                string(definition, "varEpl"), string(definition, "onSetEpl"),
                string(definition, "schemaEpl"), string(definition, "updateEpl"),
                string(definition, "fafEpl"), string(definition, "negativeCompileEpl")
        };
        String[] expected = caseEplSlots(index);
        if (!Arrays.equals(values, expected)) {
            throw new IllegalArgumentException("case EPL slots are not pinned at index " + index);
        }
    }

    private static String[] caseEplSlots(int index) {
        String[] empty = {"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""};
        switch (index) {
            case 0:
                return new String[]{EPL_CREATE_DOUBLE, "", "", "", EPL_INSERT_ONE,
                        EPL_S0_DOUBLE, EPL_INSERT_TWO, "", "", "", "", "", "", "", "", ""};
            case 1:
                return new String[]{EPL_CREATE_INTERSECTION, "", "", "", EPL_INSERT_INTERSECTION,
                        EPL_S0_INTERSECTION, "", "", "", "", "", "", "", "", "", ""};
            case 2:
                return new String[]{EPL_CREATE_OBJECT_ARRAY, "", "", "", EPL_INSERT_OBJECT_ARRAY,
                        "", "", "", "", "", "", "", "", "", "", ""};
            case 3:
                return new String[]{EPL_CREATE_PREEMPTIVE, "", "", "", EPL_INSERT_PREEMPTIVE,
                        EPL_S0_PREEMPTIVE, EPL_S2_PREEMPTIVE, EPL_S3_PREEMPTIVE, "", "", "",
                        "", EPL_SCHEMA_PREEMPTIVE, "", "", ""};
            default:
                return empty;
        }
    }

    private static String caseEpl(int index) {
        switch (index) {
            case 0:
                return EPL_CREATE_DOUBLE;
            case 1:
                return EPL_CREATE_INTERSECTION;
            case 2:
                return EPL_CREATE_OBJECT_ARRAY;
            case 3:
                return EPL_SCHEMA_PREEMPTIVE + "\n" + EPL_CREATE_PREEMPTIVE;
            default:
                throw new IllegalArgumentException("unknown case index " + index);
        }
    }

    private static int validateDoubleCase(JsonArray steps, int offset) {
        String name = CASE_DOUBLE;
        validateCaseMarker(steps.get(offset++), name);
        validateDeploy(steps.get(offset++), name, "create", EPL_CREATE_DOUBLE);
        validateDeploy(steps.get(offset++), name, "insert-one", EPL_INSERT_ONE);
        validateDeploy(steps.get(offset++), name, "insert-two", EPL_INSERT_TWO);
        validateDeploy(steps.get(offset++), name, "s0", EPL_S0_DOUBLE);
        validateSupportBean(steps.get(offset++), name, "E1", null, 10L);
        validateUndeploy(steps.get(offset++), name);
        return offset;
    }

    private static int validateIntersectionCase(JsonArray steps, int offset) {
        String name = CASE_INTERSECTION;
        JsonObject marker = object(steps.get(offset++), "case marker");
        requireFields(marker, "op", "case", "mode");
        if (!"case".equals(string(marker, "op")) || !name.equals(string(marker, "case"))
                || !"any".equals(string(marker, "mode"))) {
            throw new IllegalArgumentException("intersection case marker is not pinned");
        }
        validateDeploy(steps.get(offset++), name, "create", EPL_CREATE_INTERSECTION);
        validateDeploy(steps.get(offset++), name, "insert", EPL_INSERT_INTERSECTION);
        validateDeploy(steps.get(offset++), name, "s0", EPL_S0_INTERSECTION);
        validateSupportBean(steps.get(offset++), name, "E1", 1L, null);
        validateSupportBean(steps.get(offset++), name, "E2", 2L, null);
        validateSupportBean(steps.get(offset++), name, "E3", 2L, null);
        validateUndeploy(steps.get(offset++), name);
        return offset;
    }

    private static int validateObjectArrayCase(JsonArray steps, int offset) {
        String name = CASE_OBJECT_ARRAY;
        validateCaseMarker(steps.get(offset++), name);
        validateDeploy(steps.get(offset++), name, "create", EPL_CREATE_OBJECT_ARRAY);
        validateDeploy(steps.get(offset++), name, "insert", EPL_INSERT_OBJECT_ARRAY);
        validateUndeploy(steps.get(offset++), name);
        return offset;
    }

    private static int validatePreemptiveCase(JsonArray steps, int offset) {
        String name = CASE_PREEMPTIVE;
        validateCaseMarker(steps.get(offset++), name);
        validateDeploy(steps.get(offset++), name, "schema", EPL_SCHEMA_PREEMPTIVE);
        validateDeploy(steps.get(offset++), name, "create", EPL_CREATE_PREEMPTIVE);
        validateDeploy(steps.get(offset++), name, "insert", EPL_INSERT_PREEMPTIVE);
        validateDeploy(steps.get(offset++), name, "s2", EPL_S2_PREEMPTIVE);
        validateDeploy(steps.get(offset++), name, "s3", EPL_S3_PREEMPTIVE);
        validateDeploy(steps.get(offset++), name, "s0", EPL_S0_PREEMPTIVE);
        validateSupportBean(steps.get(offset++), name, "E1", 9L, null);
        JsonObject trigger = object(steps.get(offset++), "TypeTrigger step");
        requireFields(trigger, "op", "case", "eventType", "payload");
        if (!"send".equals(string(trigger, "op")) || !name.equals(string(trigger, "case"))
                || !"TypeTrigger".equals(string(trigger, "eventType"))) {
            throw new IllegalArgumentException("TypeTrigger step is not pinned");
        }
        JsonObject payload = object(trigger.get("payload"), "TypeTrigger payload");
        requireFields(payload, "trigger");
        if (longInteger(payload.get("trigger"), "trigger") != 0L) {
            throw new IllegalArgumentException("TypeTrigger payload is not pinned");
        }
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
                                            Long intPrimitive, Long longBoxed) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        if (intPrimitive == null) {
            requireFields(payload, "theString", "longBoxed");
            if (!text.equals(string(payload, "theString"))
                    || longInteger(payload.get("longBoxed"), "longBoxed") != longBoxed) {
                throw new IllegalArgumentException("SupportBean payload is not pinned");
            }
        } else {
            requireFields(payload, "theString", "intPrimitive");
            if (!text.equals(string(payload, "theString"))
                    || longInteger(payload.get("intPrimitive"), "intPrimitive") != intPrimitive) {
                throw new IllegalArgumentException("SupportBean payload is not pinned");
            }
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
