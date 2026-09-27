import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.EventType;
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
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.lrreport.Item;
import com.espertech.esper.regressionlib.support.lrreport.LRUtil;
import com.espertech.esper.regressionlib.support.lrreport.Location;
import com.espertech.esper.regressionlib.support.lrreport.LocationReport;

import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.lang.reflect.Array;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the expr-define-locreport-555 parity
 * scenario. Mirrors ExprDefineLambdaLocReport (ord 0 direct): a single
 * compileDeploy of the three chained declared expressions — lostLuggage,
 * passengers and nearestOwner — over LocationReport, then one
 * LocationReport bean built from the pinned 21-item makeLarge() payload.
 * The listener asserts val1 = the separated luggage collection
 * [L00000, L00007, L00008] and val2 = the nearest-owner map
 * {L00000->P00008, L00007->P00001, L00008->P00001}.
 *
 * <p>Records: one {"operation":"types"} record pinning the asserted
 * val1 Collection / val2 Map select-clause surface (the type is verified
 * in-process before the pinned names are recorded) and one
 * {"operation":"listener"} record carrying the new-data row. Time is the
 * externally-advanced engine clock so the Go replay matches byte-for-byte.
 */
public final class ExprDefineLocReport555ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "expr-define-locreport-555";
    private static final String DESCRIPTION =
            "ExprDefineLambdaLocReport (ord 0): the single-module deploy declares"
                    + " lostLuggage/passengers/nearestOwner — lostLuggage(lr)"
                    + " filters L-typed items whose owning passenger sits farther than"
                    + " LRUtil.distance > 20, and nearestOwner(lr) maps each lost"
                    + " item's assetId to the minBy-distance passenger — then one"
                    + " LocationReportFactory.makeLarge() send emits"
                    + " val1=[L00000,L00007,L00008] and"
                    + " val2={L00000->P00008, L00007->P00001, L00008->P00001}."
                    + " The types record pins the asserted val1 Collection / val2"
                    + " Map surface (Java source"
                    + " regression-lib/src/main/java/com/espertech/esper/regressionlib/"
                    + "suite/expr/define/ExprDefineLambdaLocReport.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/define/"
                    + "ExprDefineLambdaLocReport.java";

    private static final String[] CASES = {"locreport"};
    private static final int[] ORDINALS = {0};
    private static final String[] RUNTIME_IDS = {"java-runtime-867dc2875052889e9df2"};
    private static final String[] EXECUTION_NAMES = {"ExprDefineLambdaLocReport"};
    private static final String[] STATIC_IDS = {"java-1333311a1ef60ea35019"};

    // Byte-exact transcription of ExprDefineLambdaLocReport.java lines 30-44:
    // the concatenation keeps the double spaces inside each expression body
    // and the two empty literals verbatim.
    private static final String EPL_MODULE =
            "@name('s0') "
                    + "expression lostLuggage {"
                    + "  lr => lr.items.where(l => l.type='L' and "
                    + "    lr.items.anyof(p => p.type='P' and p.assetId=l.assetIdPassenger"
                    + " and LRUtil.distance(l.location.x, l.location.y, p.location.x, p.location.y) > 20))"
                    + "}"
                    + "expression passengers {"
                    + "  lr => lr.items.where(l => l.type='P')"
                    + "}"
                    + ""
                    + "expression nearestOwner {"
                    + "  lr => lostLuggage(lr).toMap(key => key.assetId, "
                    + "     value => passengers(lr).minBy(p => LRUtil.distance(value.location.x,"
                    + " value.location.y, p.location.x, p.location.y)))"
                    + "}"
                    + ""
                    + "select lostLuggage(lr) as val1, nearestOwner(lr) as val2"
                    + " from LocationReport lr";

    private static final String CASE_OBSERVATION =
            "types+listener; lostLuggage(lr) filters L items whose assetIdPassenger"
                    + " passenger is farther than LRUtil.distance > 20, nearestOwner(lr)"
                    + " maps each lost assetId to the minBy-distance passenger: the"
                    + " single makeLarge() send emits val1=[L00000,L00007,L00008]"
                    + " and val2={L00000->P00008, L00007->P00001, L00008->P00001}";

    // LocationReportFactory.makeLarge() pinned as (assetId, x, y, type,
    // assetIdPassenger) rows — the item order is load-bearing for val1.
    private static final Object[][] MAKE_LARGE = {
            {"P00002", 40, 40, "P", null},
            {"L00001", 42, 41, "L", "P00002"},
            {"L00002", 43, 43, "L", "P00002"},
            {"P00001", 10, 10, "P", null},
            {"L00000", 99, 97, "L", "P00001"},
            {"P00004", 20, 20, "P", null},
            {"P00002", 40, 40, "P", null},
            {"L00003", 29, 26, "L", "P00004"},
            {"E00011", 90, 95, "P", null},
            {"A00010", 104, 101, "L", "E00011"},
            {"A00011", 96, 100, "L", "E00011"},
            {"E00010", 90, 95, "P", null},
            {"L00009", 102, 101, "L", "E00010"},
            {"P00005", 30, 30, "P", null},
            {"L00004", 26, 27, "L", "P00005"},
            {"L00005", 30, 28, "L", "P00005"},
            {"P00007", 90, 95, "P", null},
            {"L00006", 96, 100, "L", "P00007"},
            {"P00008", 100, 100, "P", null},
            {"L00007", 10, 12, "L", "P00008"},
            {"L00008", 10, 12, "L", "P00008"},
    };

    private static final String[][] LOST_LUGGAGE = {{"L00000"}, {"L00007"}, {"L00008"}};
    private static final String[][] NEAREST_OWNER = {
            {"L00000", "P00008"}, {"L00007", "P00001"}, {"L00008", "P00001"},
    };

    private static final int EXPECTED_RECORDS = 2;
    private static final int EXPECTED_STEPS = 5;

    private ExprDefineLocReport555ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ExprDefineLocReport555ScenarioOracle <scenario.json>");
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
        runCase(allSteps, records);
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

    /** Replays the single case's steps on a fresh runtime (the Java execution
     *  runs on the default suite session and ends with undeployAll). */
    private static void runCase(JsonArray allSteps, JsonArray records) throws Exception {
        String caseName = CASES[0];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(LocationReport.class);
        // Mirrors TestSuiteExprDefine.configure: the suite imports LRUtil so
        // the unqualified LRUtil.distance static call resolves in the module.
        configuration.getCommon().addImport(LRUtil.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(RUNTIME_IDS[0], configuration);
        runtime.getEventService().advanceTime(0);

        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statementsByName = new HashMap<>();
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
                switch (operation) {
                    case "deploy": {
                        CompilerArguments compilerArgs =
                                new CompilerArguments(configuration);
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                                .compile(string(step, "epl"), compilerArgs);
                        EPDeployment deployment = runtime.getDeploymentService()
                                .deploy(compiled, new DeploymentOptions());
                        for (EPStatement statement : deployment.getStatements()) {
                            statementsByName.put(statement.getName(), statement);
                            if ("s0".equals(statement.getName())) {
                                statement.addListener(
                                        listener(caseName, sequences, records, runtime));
                            }
                        }
                        break;
                    }
                    case "types":
                        typesStep(caseName, statementsByName, string(step, "statement"),
                                records, runtime);
                        break;
                    case "send":
                        sendLocationReport(runtime, string(step, "eventType"),
                                object(step.get("payload"), "payload"));
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        statementsByName.clear();
                        break;
                    default:
                        throw new IllegalArgumentException("unknown op: " + operation);
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
     * Emits the {"operation":"types"} record carrying the pinned Java type
     * names for the asserted select-clause surface: val1 Collection and
     * val2 Map. The actual event-type properties are verified before the
     * pinned names are recorded so a schema drift fails the oracle.
     */
    private static void typesStep(String caseName, Map<String, EPStatement> statements,
                                  String label, JsonArray records, EPRuntime runtime) {
        if (!CASES[0].equals(caseName) || !"s0".equals(label)) {
            throw new IllegalStateException("types statement " + label
                    + " is not pinned in case " + caseName);
        }
        EPStatement statement = statements.get(label);
        if (statement == null) {
            throw new IllegalStateException("types statement " + label
                    + " was not deployed in case " + caseName);
        }
        EventType eventType = statement.getEventType();
        String[] pinnedNames = {"val1", "val2"};
        String[] pinnedTypes = {"Collection", "Map"};
        JsonObject pinned = new JsonObject();
        for (int index = 0; index < pinnedNames.length; index++) {
            Class<?> propertyType = eventType.getPropertyType(pinnedNames[index]);
            String actual = propertyType == null ? "null" : propertyType.getSimpleName();
            if (!pinnedTypes[index].equals(actual)) {
                throw new IllegalStateException("property type drift for " + caseName
                        + "/s0." + pinnedNames[index] + ": expected " + pinnedTypes[index]
                        + " got " + actual);
            }
            pinned.add(pinnedNames[index], pinnedTypes[index]);
        }
        JsonObject value = new JsonObject();
        value.add("properties", pinned);
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "types");
        record.add("statement", statement.getName());
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(
                runtime.getEventService().getCurrentTime()).toString());
        record.add("value", value);
        records.add(record);
    }

    /**
     * Rebuilds the LocationReport bean from the pinned 21-item payload. The
     * payload must equal LocationReportFactory.makeLarge() row-for-row —
     * the item order is load-bearing for val1's ordering — so each item is
     * checked against MAKE_LARGE before the event is sent.
     */
    private static void sendLocationReport(EPRuntime runtime, String eventType,
                                           JsonObject payload) {
        if (!"LocationReport".equals(eventType)) {
            throw new IllegalArgumentException("unknown event type: " + eventType);
        }
        JsonArray items = array(payload.get("items"), "LocationReport items");
        if (items.size() != MAKE_LARGE.length) {
            throw new IllegalStateException("LocationReport payload item count "
                    + items.size() + " is not pinned");
        }
        List<Item> reconstructed = new ArrayList<>(MAKE_LARGE.length);
        for (int index = 0; index < MAKE_LARGE.length; index++) {
            JsonObject item = object(items.get(index), "LocationReport item " + index);
            requireFields(item, "assetId", "location", "type", "assetIdPassenger");
            String assetId = item.get("assetId").asString();
            JsonObject location = object(item.get("location"), "item " + index + " location");
            requireFields(location, "x", "y");
            int x = integer(location, "x");
            int y = integer(location, "y");
            String type = item.get("type").asString();
            String assetIdPassenger = item.get("assetIdPassenger").isNull()
                    ? null : item.get("assetIdPassenger").asString();
            Object[] pinned = MAKE_LARGE[index];
            if (!pinned[0].equals(assetId) || !pinned[1].equals(x) || !pinned[2].equals(y)
                    || !pinned[3].equals(type)
                    || (pinned[4] == null ? assetIdPassenger != null
                            : !pinned[4].equals(assetIdPassenger))) {
                throw new IllegalStateException("LocationReport payload item " + index
                        + " drifted from makeLarge()");
            }
            reconstructed.add(new Item(assetId, new Location(x, y), type, assetIdPassenger));
        }
        runtime.getEventService().sendEventBean(new LocationReport(reconstructed), eventType);
    }

    /**
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; new and old arrays render only when non-empty, and
     * a listener invocation without a stream is a contract violation. The
     * first new row is verified in-process against the Java execution's
     * assertions (val1 assetIds and the val2 owner map) before recording.
     */
    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonArray newRows = rows(newEvents);
            JsonArray oldRows = rows(oldEvents);
            if (newRows.size() == 0 && oldRows.size() == 0) {
                throw new IllegalStateException("listener for statement " + statement.getName()
                        + " was invoked without a stream in case " + caseName);
            }
            if ("s0".equals(statement.getName()) && sequence == 1 && newEvents != null
                    && newEvents.length == 1) {
                assertAssertions(newEvents[0]);
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

    /**
     * In-process mirror of the Java execution's assertListener block:
     * val1 carries exactly [L00000, L00007, L00008] in collection order and
     * val2 maps L00000->P00008, L00007->P00001, L00008->P00001. A drift
     * fails the oracle rather than recording a different trace.
     */
    @SuppressWarnings("unchecked")
    private static void assertAssertions(EventBean event) {
        Object val1 = event.get("val1");
        if (!(val1 instanceof Collection)) {
            throw new IllegalStateException("val1 is not a Collection: " + val1);
        }
        List<Item> lost = new ArrayList<>((Collection<Item>) val1);
        if (lost.size() != LOST_LUGGAGE.length) {
            throw new IllegalStateException("val1 size " + lost.size()
                    + " is not the pinned " + LOST_LUGGAGE.length);
        }
        for (int index = 0; index < LOST_LUGGAGE.length; index++) {
            if (!LOST_LUGGAGE[index][0].equals(lost.get(index).getAssetId())) {
                throw new IllegalStateException("val1[" + index + "] = "
                        + lost.get(index).getAssetId() + ", want " + LOST_LUGGAGE[index][0]);
            }
        }
        Object val2 = event.get("val2");
        if (!(val2 instanceof Map)) {
            throw new IllegalStateException("val2 is not a Map: " + val2);
        }
        Map<?, ?> owners = (Map<?, ?>) val2;
        if (owners.size() != NEAREST_OWNER.length) {
            throw new IllegalStateException("val2 size " + owners.size()
                    + " is not the pinned " + NEAREST_OWNER.length);
        }
        for (String[] pair : NEAREST_OWNER) {
            Object owner = owners.get(pair[0]);
            if (!(owner instanceof Item) || !pair[1].equals(((Item) owner).getAssetId())) {
                throw new IllegalStateException("val2[" + pair[0] + "] = " + owner
                        + ", want Item " + pair[1]);
            }
        }
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

    /**
     * Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, null as the tagged
     * {"state":"null"} object, EventBean fragments as nested row objects,
     * Java arrays and collections as JSON arrays, maps as JSON objects and
     * lrreport.Item beans as canonical {assetId, assetIdPassenger,
     * location{x,y}, type} objects — the same shapes the Go normalizer
     * emits for the Go fixture structs (a nil *string renders JSON null).
     */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof EventBean) {
            return row((EventBean) value);
        }
        if (value instanceof Item) {
            return item((Item) value);
        }
        if (value.getClass().isArray()) {
            JsonArray array = new JsonArray();
            int length = Array.getLength(value);
            for (int index = 0; index < length; index++) {
                array.add(normalize(Array.get(value, index)));
            }
            return array;
        }
        if (value instanceof Collection) {
            JsonArray array = new JsonArray();
            for (Object element : (Collection<?>) value) {
                array.add(normalize(element));
            }
            return array;
        }
        if (value instanceof Map) {
            JsonObject object = new JsonObject();
            for (Map.Entry<?, ?> entry : ((Map<?, ?>) value).entrySet()) {
                object.add(String.valueOf(entry.getKey()), normalize(entry.getValue()));
            }
            return object;
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

    /**
     * Canonical Item rendering: the four Java bean properties in fixed
     * order with location{x,y} nested; a null assetIdPassenger renders as
     * JSON null to match the Go *string marshal.
     */
    private static JsonObject item(Item item) {
        JsonObject object = new JsonObject();
        object.add("assetId", item.getAssetId());
        if (item.getAssetIdPassenger() == null) {
            object.add("assetIdPassenger", Json.NULL);
        } else {
            object.add("assetIdPassenger", item.getAssetIdPassenger());
        }
        JsonObject location = new JsonObject();
        location.add("x", item.getLocation().getX());
        location.add("y", item.getLocation().getY());
        object.add("location", location);
        object.add("type", item.getType());
        return object;
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
        validateStringArray(scenario.get("javaFlags"), new String[]{}, "javaFlags");

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
                    || !CASE_OBSERVATION.equals(string(definition, "observation"))
                    || !EPL_MODULE.equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        validateCaseMarker(steps.get(offset++), CASES[0]);
        validateDeploy(steps.get(offset++), CASES[0], "s0", EPL_MODULE);
        validateTypes(steps.get(offset++), CASES[0], "s0");
        validateSend(steps.get(offset++), CASES[0]);
        validateUndeployAll(steps.get(offset++), CASES[0]);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
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

    private static void validateTypes(JsonValue value, String caseName,
                                      String expectedStatement) {
        JsonObject step = object(value, "types step");
        requireFields(step, "op", "case", "statement");
        if (!"types".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("types step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateSend(JsonValue value, String caseName) {
        JsonObject step = object(value, "LocationReport step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"LocationReport".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("LocationReport step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "LocationReport payload");
        requireFields(payload, "items");
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
