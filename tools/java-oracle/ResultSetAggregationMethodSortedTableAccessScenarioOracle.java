import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
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
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.io.Serializable;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.Comparator;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.regex.Pattern;

/**
 * Direct Esper 9.0.0 oracle for ResultSetAggregationMethodSorted ordinals 7-9.
 *
 * Each table-backed sorted-access execution runs in a fresh runtime. The replay
 * is deliberately pinned to the exact seed and probe/range vectors used by the
 * fixed Java regression source; the resulting listener records are normalized
 * into the language-neutral parity trace representation.
 */
public final class ResultSetAggregationMethodSortedTableAccessScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-sorted-table-access";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregationMethodSorted.java";
    private static final String DESCRIPTION =
            "ResultSetAggregationMethodSorted ordinals 7-9: table-backed sorted get/contains/counts, inclusive submap/eventsBetween ranges, and detached navigable-map projections over duplicate-key buckets.";

    private static final String GET_CASE = "get-contains-counts";
    private static final String SUBMAP_CASE = "submap-events-between";
    private static final String NAVIGABLE_CASE = "navigable-map-reference";
    private static final String[] CASES = {GET_CASE, SUBMAP_CASE, NAVIGABLE_CASE};
    private static final int[] ORDINALS = {7, 8, 9};
    private static final String[] RUNTIMES = {
            "java-runtime-d7d60fec056d6eb16659",
            "java-runtime-0c0e45c751e3cafeb454",
            "java-runtime-2117ba8232651a61a058"
    };
    private static final String[] EXECUTIONS = {
            "ResultSetAggregateSortedGetContainsCounts",
            "ResultSetAggregateSortedSubmapEventsBetween",
            "ResultSetAggregateSortedNavigableMapReference"
    };
    private static final String[] STATIC_IDS = {
            "java-ad219f9a27aebc7885fc",
            "java-4b989f8ba297c8e8d426",
            "java-db7b9e752ef9e4000cc5"
    };
    private static final String INVENTORY_ID = "java-0ded2b32c81677a9d36f";
    private static final String[] SEND_STRINGS = {"E1a", "E1b", "E4b", "E6a", "E6b", "E8", "E9"};
    private static final int[] SEND_INTS = {1, 1, 4, 6, 6, 8, 9};
    private static final Pattern INTEGER_SYNTAX = Pattern.compile("-?(?:0|[1-9][0-9]*)");

    private static final String GET_EPL =
            "create table MyTable(sortcol sorted(intPrimitive) @type('SupportBean'));\n" +
            "into table MyTable select sorted(*) as sortcol from SupportBean;\n" +
            "@name('s0') select " +
            "MyTable.sortcol.getEvent(id) as ge," +
            "MyTable.sortcol.getEvents(id) as ges," +
            "MyTable.sortcol.containsKey(id) as ck," +
            "MyTable.sortcol.countEvents() as cnte," +
            "MyTable.sortcol.countKeys() as cntk," +
            "MyTable.sortcol.getEvent(id).theString as geid," +
            "MyTable.sortcol.getEvent(id).firstOf() as gefo," +
            "MyTable.sortcol.getEvents(id).lastOf() as geslo" +
            " from SupportBean_S0";
    private static final String SUBMAP_EPL =
            "@public @buseventtype create schema MySubmapEvent as " + MySubmapEvent.class.getName() + ";\n" +
            "create table MyTable(sortcol sorted(intPrimitive) @type('SupportBean'));\n" +
            "into table MyTable select sorted(*) as sortcol from SupportBean;\n" +
            "@name('s0') select " +
            "MyTable.sortcol.eventsBetween(fromKey, fromInclusive, toKey, toInclusive) as eb," +
            "MyTable.sortcol.eventsBetween(fromKey, fromInclusive, toKey, toInclusive).lastOf() as eblastof," +
            "MyTable.sortcol.subMap(fromKey, fromInclusive, toKey, toInclusive) as sm" +
            " from MySubmapEvent";
    private static final String NAVIGABLE_EPL =
            "create table MyTable(sortcol sorted(intPrimitive) @type('SupportBean'));\n" +
            "into table MyTable select sorted(*) as sortcol from SupportBean;\n" +
            "@name('s0') select MyTable.sortcol.navigableMapReference() as nmr from SupportBean_S0";

    private ResultSetAggregationMethodSortedTableAccessScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetAggregationMethodSortedTableAccessScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be an object");
        }
        validateNoDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        JsonArray steps = scenario.get("steps").asArray();
        for (String caseName : CASES) {
            runCase(steps, caseName, records);
        }
        if (records.size() != 325) {
            throw new IllegalStateException("expected 325 listener trace records, got " + records.size());
        }
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID).add("records", records));
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(requireString(scenario, "version"))
                || !ID.equals(requireString(scenario, "id"))
                || !DESCRIPTION.equals(requireString(scenario, "description"))
                || !JAVA_COMMIT.equals(requireString(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(requireString(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        strings(scenario.get("javaRuntimes"), RUNTIMES);
        strings(scenario.get("javaNames"), EXECUTIONS);
        strings(scenario.get("javaStaticIds"), STATIC_IDS);
        strings(scenario.get("javaFlags"));

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly three cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject entry = object(cases.get(index), "case metadata");
            requireFields(entry, "case", "ordinal", "runtimeId", "executionName", "observation",
                    "iteratorSnapshots", "epl");
            if (!CASES[index].equals(requireString(entry, "case"))
                    || requireInteger(entry, "ordinal") != ORDINALS[index]
                    || !RUNTIMES[index].equals(requireString(entry, "runtimeId"))
                    || !EXECUTIONS[index].equals(requireString(entry, "executionName"))
                    || !"listener".equals(requireString(entry, "observation"))
                    || requireInteger(entry, "iteratorSnapshots") != 0
                    || !eplFor(index).equals(requireString(entry, "epl"))) {
                throw new IllegalArgumentException("scenario case metadata is not pinned for index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        int expectedSteps = 1 + SEND_STRINGS.length + 12
                + 1 + SEND_STRINGS.length + submapProbeCount()
                + 1 + SEND_STRINGS.length + 1;
        if (steps.size() != expectedSteps) {
            throw new IllegalArgumentException("scenario must contain exactly " + expectedSteps + " steps");
        }
        int index = 0;
        index = validateCaseMarker(steps, index, GET_CASE);
        for (int send = 0; send < SEND_STRINGS.length; send++) {
            validateBeanSend(object(steps.get(index++), "get/contains SupportBean send"), send);
        }
        for (int id = 0; id < 12; id++) {
            validateSupportBeanS0Send(object(steps.get(index++), "get/contains probe"), id);
        }

        index = validateCaseMarker(steps, index, SUBMAP_CASE);
        for (int send = 0; send < SEND_STRINGS.length; send++) {
            validateBeanSend(object(steps.get(index++), "submap SupportBean send"), send);
        }
        for (int start = 0; start < 12; start++) {
            for (int end = start; end < 12; end++) {
                for (boolean includeStart : new boolean[]{false, true}) {
                    for (boolean includeEnd : new boolean[]{false, true}) {
                        validateSubmapSend(object(steps.get(index++), "submap probe"),
                                start, includeStart, end, includeEnd);
                    }
                }
            }
        }

        index = validateCaseMarker(steps, index, NAVIGABLE_CASE);
        for (int send = 0; send < SEND_STRINGS.length; send++) {
            validateBeanSend(object(steps.get(index++), "navigable SupportBean send"), send);
        }
        validateSupportBeanS0Send(object(steps.get(index++), "navigable probe"), -1);
        if (index != steps.size()) {
            throw new IllegalArgumentException("scenario has trailing steps");
        }
    }

    private static int validateCaseMarker(JsonArray steps, int index, String caseName) {
        JsonObject marker = object(steps.get(index), "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(requireString(marker, "op")) || !caseName.equals(requireString(marker, "case"))) {
            throw new IllegalArgumentException("scenario case order mismatch; expected " + caseName);
        }
        return index + 1;
    }

    private static void validateBeanSend(JsonObject step, int sendIndex) {
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(requireString(step, "op"))
                || !"SupportBean".equals(requireString(step, "eventType"))) {
            throw new IllegalArgumentException("scenario SupportBean send is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        if (!SEND_STRINGS[sendIndex].equals(requireString(payload, "theString"))
                || requireInteger(payload, "intPrimitive") != SEND_INTS[sendIndex]) {
            throw new IllegalArgumentException("scenario SupportBean payload is not pinned");
        }
    }

    private static void validateSupportBeanS0Send(JsonObject step, int id) {
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(requireString(step, "op"))
                || !"SupportBean_S0".equals(requireString(step, "eventType"))) {
            throw new IllegalArgumentException("scenario SupportBean_S0 send is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
        requireFields(payload, "id");
        if (requireInteger(payload, "id") != id) {
            throw new IllegalArgumentException("scenario SupportBean_S0 id is not pinned");
        }
    }

    private static void validateSubmapSend(JsonObject step, int fromKey, boolean fromInclusive,
                                           int toKey, boolean toInclusive) {
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(requireString(step, "op"))
                || !"MySubmapEvent".equals(requireString(step, "eventType"))) {
            throw new IllegalArgumentException("scenario MySubmapEvent send is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "MySubmapEvent payload");
        requireFields(payload, "fromKey", "fromInclusive", "toKey", "toInclusive");
        if (requireInteger(payload, "fromKey") != fromKey
                || requireBoolean(payload, "fromInclusive") != fromInclusive
                || requireInteger(payload, "toKey") != toKey
                || requireBoolean(payload, "toInclusive") != toInclusive) {
            throw new IllegalArgumentException("scenario MySubmapEvent payload is not pinned");
        }
    }

    private static int submapProbeCount() {
        int count = 0;
        for (int start = 0; start < 12; start++) {
            for (int end = start; end < 12; end++) {
                count += 4;
            }
        }
        return count;
    }

    private static String eplFor(int index) {
        switch (index) {
            case 0:
                return GET_EPL;
            case 1:
                return SUBMAP_EPL;
            case 2:
                return NAVIGABLE_EPL;
            default:
                throw new IllegalArgumentException("unsupported case index " + index);
        }
    }

    private static void runCase(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);

        String runtimeURI = "parity-resultset-aggregate-sorted-table-access-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                    eplFor(indexOfCase(caseName)), new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(runtimeURI));
            EPStatement statement = findStatement(deployment);
            statement.addListener(new TraceWriter(records, caseName, statement, runtime));
            replay(allSteps, caseName, runtime);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static int indexOfCase(String caseName) {
        for (int index = 0; index < CASES.length; index++) {
            if (CASES[index].equals(caseName)) {
                return index;
            }
        }
        throw new IllegalArgumentException("unknown case " + caseName);
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        for (EPStatement statement : deployment.getStatements()) {
            if ("s0".equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("statement s0 was not deployed");
    }

    private static void replay(JsonArray allSteps, String caseName, EPRuntime runtime) {
        boolean active = false;
        for (JsonValue value : allSteps) {
            JsonObject step = value.asObject();
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                active = caseName.equals(step.getString("case", ""));
                continue;
            }
            if (!active) {
                continue;
            }
            JsonObject payload = step.get("payload").asObject();
            String eventType = step.getString("eventType", "");
            if ("SupportBean".equals(eventType)) {
                runtime.getEventService().sendEventBean(
                        new SupportBean(payload.getString("theString", null), payload.getInt("intPrimitive", 0)),
                        "SupportBean");
            } else if ("SupportBean_S0".equals(eventType)) {
                runtime.getEventService().sendEventBean(
                        new SupportBean_S0(payload.getInt("id", 0)), "SupportBean_S0");
            } else if ("MySubmapEvent".equals(eventType)) {
                runtime.getEventService().sendEventBean(
                        new MySubmapEvent(payload.getInt("fromKey", 0),
                                payload.getBoolean("fromInclusive", false),
                                payload.getInt("toKey", 0),
                                payload.getBoolean("toInclusive", false)),
                        "MySubmapEvent");
            } else {
                throw new IllegalArgumentException("unsupported replay event type " + eventType);
            }
        }
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored,
                           EPRuntime ignoredRuntime) {
            if (newEvents == null && oldEvents == null) {
                return;
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            JsonArray oldRows = rows(oldEvents);
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                output.add(eventValue(event));
            }
            return output;
        }

        private JsonObject eventValue(EventBean event) {
            JsonObject fields = new JsonObject();
            String[] names = event.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            for (String name : names) {
                fields.add(name, normalize(event.get(name)));
            }
            return new JsonObject().add("kind", "row").add("fields", fields);
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof EventBean) {
                return normalize(((EventBean) value).getUnderlying());
            }
            if (value instanceof SupportBean) {
                SupportBean bean = (SupportBean) value;
                return supportBeanRow(bean);
            }
            if (value instanceof Map) {
                return mapRow((Map<?, ?>) value);
            }
            if (value instanceof Collection) {
                JsonArray array = new JsonArray();
                for (Object item : (Collection<?>) value) {
                    array.add(normalize(item));
                }
                return array;
            }
            if (value instanceof Object[]) {
                JsonArray array = new JsonArray();
                for (Object item : (Object[]) value) {
                    array.add(normalize(item));
                }
                return array;
            }
            if (value instanceof Float || value instanceof Double) {
                double number = ((Number) value).doubleValue();
                if (number == Math.rint(number) && !Double.isInfinite(number)) {
                    return Json.value((long) number);
                }
                return Json.value(number);
            }
            if (value instanceof Integer || value instanceof Short || value instanceof Byte) {
                return Json.value(((Number) value).intValue());
            }
            if (value instanceof Number) {
                return Json.value(((Number) value).longValue());
            }
            if (value instanceof Boolean) {
                return Json.value((Boolean) value);
            }
            return Json.value(String.valueOf(value));
        }

        /** Normalize map entries without converting numeric keys before lookup. */
        private JsonValue mapRow(Map<?, ?> map) {
            List<Map.Entry<?, ?>> entries = new ArrayList<>(map.entrySet());
            entries.sort(Comparator.comparing(entry -> String.valueOf(entry.getKey())));
            JsonObject fields = new JsonObject();
            for (Map.Entry<?, ?> entry : entries) {
                fields.add(String.valueOf(entry.getKey()), normalize(entry.getValue()));
            }
            return new JsonObject().add("kind", "row").add("fields", fields);
        }

        private JsonObject supportBeanRow(SupportBean bean) {
            return new JsonObject().add("kind", "row").add("fields",
                    new JsonObject().add("intPrimitive", normalize(bean.getIntPrimitive()))
                            .add("theString", normalize(bean.getTheString())));
        }
    }

    private static void validateNoDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> seen = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!seen.add(member.getName())) {
                    throw new IllegalArgumentException("duplicate JSON object key: " + member.getName());
                }
                validateNoDuplicateKeys(member.getValue());
            }
        } else if (value.isArray()) {
            JsonArray array = value.asArray();
            for (int index = 0; index < array.size(); index++) {
                validateNoDuplicateKeys(array.get(index));
            }
        }
    }

    private static JsonObject object(JsonValue value, String description) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(description + " must be an object");
        }
        return value.asObject();
    }

    private static JsonArray array(JsonValue value, String description) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(description + " must be an array");
        }
        return value.asArray();
    }

    private static void requireFields(JsonObject object, String... expectedNames) {
        if (object == null || object.size() != expectedNames.length) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
        Set<String> expected = new HashSet<>(Arrays.asList(expectedNames));
        if (!expected.equals(new HashSet<>(object.names()))) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
    }

    private static String requireString(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isString()) {
            throw new IllegalArgumentException(name + " must be a string");
        }
        return value.asString();
    }

    private static boolean requireBoolean(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isBoolean()) {
            throw new IllegalArgumentException(name + " must be a boolean");
        }
        return value.asBoolean();
    }

    private static int requireInteger(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be a JSON integer");
        }
        String text = value.toString();
        if (!INTEGER_SYNTAX.matcher(text).matches()) {
            throw new IllegalArgumentException(name + " must use integer JSON syntax");
        }
        try {
            return Integer.parseInt(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(name + " is outside the Java int range", ex);
        }
    }

    private static void strings(JsonValue value, String... expected) {
        JsonArray array = array(value, "string array");
        if (array.size() != expected.length) {
            throw new IllegalArgumentException("string array length mismatch");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = array.get(index);
            if (item == null || !item.isString() || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException("string array value mismatch");
            }
        }
    }

    /** Local class mirror required by the fixed MySubmapEvent class-backed schema. */
    public static class MySubmapEvent implements Serializable {
        private static final long serialVersionUID = 5055643452464631039L;
        private final int fromKey;
        private final boolean fromInclusive;
        private final int toKey;
        private final boolean toInclusive;

        public MySubmapEvent(int fromKey, boolean fromInclusive, int toKey, boolean toInclusive) {
            this.fromKey = fromKey;
            this.fromInclusive = fromInclusive;
            this.toKey = toKey;
            this.toInclusive = toInclusive;
        }

        public int getFromKey() {
            return fromKey;
        }

        public boolean isFromInclusive() {
            return fromInclusive;
        }

        public int getToKey() {
            return toKey;
        }

        public boolean isToInclusive() {
            return toInclusive;
        }
    }
}
