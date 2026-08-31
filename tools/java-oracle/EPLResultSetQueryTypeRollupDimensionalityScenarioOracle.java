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
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;
/**
 * Java oracle for ResultSetQueryTypeRollupDimensionality rollup, cube, and
 * grouping-set scenarios.
 *
 * The legacy broad replay covers its existing 33 cases and preserves their
 * established input, runtime, and trace behavior. A separate dedicated
 * replay covers five cases from three fixed executions: the two unbound
 * grouping-set 2-level unenclosed variants, the two bound cube 3-dimension
 * variants, and the context-partition rollup case. Dedicated metadata,
 * payloads, and listener rows are validated exactly.
 *
 * Events are SupportBean payloads carrying theString/intPrimitive/
 * longPrimitive/doublePrimitive/intBoxed (fields absent from a payload keep
 * their defaults) and SupportBean_S0 payloads carrying id only.
 * Additional send payloads follow the same absent-member-default convention:
 * SupportEventWithIntArray{id, array, value} and SupportThreeArrayEvent{id,
 * value, intArray, longArray, doubleArray} decode JSON arrays into their
 * int[]/long[]/double[] members.
 */

public class EPLResultSetQueryTypeRollupDimensionalityScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String SCENARIO_ID = "rollup-dimensionality-dedicated";
    private static final String DESCRIPTION =
        "Dedicated ResultSetQueryTypeRollupDimensionality ordinals 10, 11, and 17: unbound grouping-set, bounded cube, and context-partition rollup semantics.";
    private static final String BROAD_SCENARIO_ID = "rollup-dimensionality";
    private static final String BROAD_DESCRIPTION =
        "Unbound rollup dimensionality replaying the ResultSetQueryTypeRollupDimensionality unbound family over SupportBean (and a SupportBean_S0 cartesian prime for the join variants)";
    private static final String JAVA_SOURCE =
        "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRollupDimensionality.java";
    private static final String[] DEDICATED_JAVA_RUNTIMES = {
        "java-runtime-20c08346e2644a7201e5",
        "java-runtime-994120aef6b9ff1c0e75",
        "java-runtime-71fb38d67af471287089"
    };
    private static final String[] DEDICATED_JAVA_NAMES = {
        "ResultSetQueryTypeUnboundGroupingSet2LevelUnenclosed",
        "ResultSetQueryTypeBoundCube3Dim",
        "ResultSetQueryTypeContextPartitionAlsoRollup"
    };
    private static final String[] DEDICATED_STATIC_IDS = {
        "java-04090eecaf8dbe129c67",
        "java-2bb53dfa0bac679a3c29",
        "java-e684c2e0c509d0592a0d"
    };
    private static final String[] DEDICATED_JAVA_FLAGS = {};
    private static final String[] DEDICATED_CASES = {
        "unbound-grouping-set-2level-unenclosed-a",
        "unbound-grouping-set-2level-unenclosed-b",
        "bound-cube-3dim-cube",
        "bound-cube-3dim-gs",
        "context-partition-also-rollup"
    };
    private static final int[] DEDICATED_ORDINALS = {10, 10, 11, 11, 17};
    private static final String[] DEDICATED_RUNTIMES = {
        "java-runtime-20c08346e2644a7201e5",
        "java-runtime-20c08346e2644a7201e5",
        "java-runtime-994120aef6b9ff1c0e75",
        "java-runtime-994120aef6b9ff1c0e75",
        "java-runtime-71fb38d67af471287089"
    };
    private static final String[] DEDICATED_EXECUTIONS = {
        "ResultSetQueryTypeUnboundGroupingSet2LevelUnenclosed",
        "ResultSetQueryTypeUnboundGroupingSet2LevelUnenclosed",
        "ResultSetQueryTypeBoundCube3Dim",
        "ResultSetQueryTypeBoundCube3Dim",
        "ResultSetQueryTypeContextPartitionAlsoRollup"
    };
    private static final String GROUPING_A_EPL =
        "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, sum(doublePrimitive) as c3 from SupportBean group by theString, grouping sets(intPrimitive, longPrimitive)";
    private static final String GROUPING_B_EPL =
        "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, sum(doublePrimitive) as c3 from SupportBean group by grouping sets((theString, intPrimitive), (theString, longPrimitive))";
    private static final String CUBE_EPL_PREFIX =
        "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, count(*) as c3, sum(doublePrimitive) as c4,grouping(theString) as c5,grouping(intPrimitive) as c6,grouping(longPrimitive) as c7,grouping_id(theString, intPrimitive, longPrimitive) as c8 from SupportBean#length(4) group by ";
    private static final String CUBE_EPL = CUBE_EPL_PREFIX + "cube(theString, intPrimitive, longPrimitive)";
    private static final String CUBE_GS_EPL = CUBE_EPL_PREFIX +
        "grouping sets((theString, intPrimitive, longPrimitive),(theString, intPrimitive),(theString, longPrimitive),(theString),(intPrimitive, longPrimitive),(intPrimitive),(longPrimitive),())";
    private static final String CONTEXT_EPL =
        "create context SegmentedByString partition by theString from SupportBean;\n" +
        "@name('s0') context SegmentedByString select theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean group by rollup(theString, intPrimitive)";

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLResultSetQueryTypeRollupDimensionalityScenarioOracle <scenario.json>");
            System.exit(2);
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        boolean dedicated = isDedicatedScenario(scenario);
        if (dedicated) {
            validateDedicatedScenario(scenario);
        }
        JsonArray allSteps = array(scenario.get("steps"), "steps");
        JsonArray caseDefinitions = array(scenario.get("cases"), "cases");
        List<JsonObject> records = new ArrayList<>();
        for (int caseIndex = 0; caseIndex < caseDefinitions.size(); caseIndex++) {
            JsonObject definition = object(caseDefinitions.get(caseIndex), "case definition " + caseIndex);
            runCase(allSteps, definition.getString("case", ""), records);
        }
        if (dedicated) {
            validateDedicatedTrace(records);
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", scenario.getString("id", ""));
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        JsonArray recordsArr = new JsonArray();
        for (JsonObject record : records) {
            recordsArr.add(record);
        }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }
    private static boolean isDedicatedScenario(JsonObject scenario) {
        // Dedicated validation is opt-in by the frozen scenario identifier.
        // A broad replay may legitimately contain a similarly named case or
        // runtime and must retain the legacy permissive behavior.
        return SCENARIO_ID.equals(scenario.getString("id", ""));
    }

    private static int dedicatedCaseIndex(String caseName) {
        for (int i = 0; i < DEDICATED_CASES.length; i++) {
            if (DEDICATED_CASES[i].equals(caseName)) {
                return i;
            }
        }
        return -1;
    }

    private static boolean dedicatedRuntime(String runtimeId) {
        for (String expected : DEDICATED_RUNTIMES) {
            if (expected.equals(runtimeId)) {
                return true;
            }
        }
        return false;
    }

    private static void validateDedicatedScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !SCENARIO_ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("dedicated rollup-dimensionality scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), DEDICATED_JAVA_RUNTIMES, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), DEDICATED_JAVA_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), DEDICATED_STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), DEDICATED_JAVA_FLAGS, "javaFlags");

        JsonArray definitions = array(scenario.get("cases"), "cases");
        if (definitions.size() != DEDICATED_CASES.length) {
            throw new IllegalArgumentException("dedicated scenario must contain exactly five cases");
        }
        for (int dedicatedIndex = 0; dedicatedIndex < DEDICATED_CASES.length; dedicatedIndex++) {
            JsonObject definition = object(definitions.get(dedicatedIndex),
                    "dedicated case definition " + dedicatedIndex);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation",
                    "iteratorSnapshots", "epl");
            if (!DEDICATED_CASES[dedicatedIndex].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != DEDICATED_ORDINALS[dedicatedIndex]
                    || !DEDICATED_RUNTIMES[dedicatedIndex].equals(string(definition, "runtimeId"))
                    || !DEDICATED_EXECUTIONS[dedicatedIndex].equals(string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 0
                    || !eplForDedicatedCase(dedicatedIndex).equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("dedicated case metadata mismatch: "
                        + DEDICATED_CASES[dedicatedIndex]);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        int expectedStepCount = DEDICATED_CASES.length;
        for (int dedicatedIndex = 0; dedicatedIndex < DEDICATED_CASES.length; dedicatedIndex++) {
            expectedStepCount += dedicatedSendCount(dedicatedIndex);
        }
        if (steps.size() != expectedStepCount) {
            throw new IllegalArgumentException("dedicated scenario must contain exactly " + expectedStepCount
                    + " contiguous case/send steps");
        }
        int stepIndex = 0;
        for (int dedicatedIndex = 0; dedicatedIndex < DEDICATED_CASES.length; dedicatedIndex++) {
            JsonObject marker = object(steps.get(stepIndex), "dedicated case marker " + stepIndex);
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op"))
                    || !DEDICATED_CASES[dedicatedIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("dedicated case steps are not contiguous and ordered");
            }
            stepIndex++;
            for (int sendIndex = 0; sendIndex < dedicatedSendCount(dedicatedIndex); sendIndex++) {
                JsonObject send = object(steps.get(stepIndex), "dedicated send step " + stepIndex);
                validateDedicatedSend(send, dedicatedIndex, sendIndex, stepIndex);
                stepIndex++;
            }
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String label) {
        JsonArray actual = array(value, label);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(label + " has unexpected length");
        }
        for (int i = 0; i < expected.length; i++) {
            JsonValue item = actual.get(i);
            if (item == null || !item.isString() || !expected[i].equals(item.asString())) {
                throw new IllegalArgumentException(label + " is not pinned");
            }
        }
    }

    private static int dedicatedSendCount(int dedicatedIndex) {
        if (dedicatedIndex < 2) {
            return 4;
        }
        if (dedicatedIndex < 4) {
            return 5;
        }
        return 3;
    }

    private static String eplForDedicatedCase(int dedicatedIndex) {
        return switch (dedicatedIndex) {
            case 0 -> GROUPING_A_EPL;
            case 1 -> GROUPING_B_EPL;
            case 2 -> CUBE_EPL;
            case 3 -> CUBE_GS_EPL;
            case 4 -> CONTEXT_EPL;
            default -> throw new IllegalArgumentException("unknown dedicated case index " + dedicatedIndex);
        };
    }

    private static void validateDedicatedSend(JsonObject step, int dedicatedIndex, int sendIndex, int stepIndex) {
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("dedicated step " + stepIndex + " must send SupportBean");
        }
        JsonObject payload = object(step.get("payload"), "dedicated payload " + stepIndex);
        if (dedicatedIndex == 4) {
            requireFields(payload, "theString", "intPrimitive", "longPrimitive");
            validateString(payload, "theString", "E" + (sendIndex < 2 ? "1" : "2"));
            validateInteger(payload, "intPrimitive", sendIndex == 1 ? 2 : 1);
            validateLong(payload, "longPrimitive", sendIndex == 0 ? 10L : sendIndex == 1 ? 20L : 25L);
            return;
        }
        requireFields(payload, "theString", "intPrimitive", "longPrimitive", "doublePrimitive");
        String[] strings;
        int[] ints;
        long[] longs;
        double[] doubles;
        if (dedicatedIndex < 2) {
            strings = new String[]{"E1", "E1", "E1", "E1"};
            ints = new int[]{10, 20, 10, 20};
            longs = new long[]{100L, 200L, 200L, 100L};
            doubles = new double[]{1000d, 2000d, 3000d, 4000d};
        } else {
            strings = new String[]{"E1", "E2", "E1", "E2", "E2"};
            ints = new int[]{1, 1, 2, 2, 1};
            longs = new long[]{10L, 20L, 10L, 20L, 10L};
            doubles = new double[]{100d, 200d, 300d, 400d, 500d};
        }
        validateString(payload, "theString", strings[sendIndex]);
        validateInteger(payload, "intPrimitive", ints[sendIndex]);
        validateLong(payload, "longPrimitive", longs[sendIndex]);
        validateDouble(payload, "doublePrimitive", doubles[sendIndex]);
    }

    private static void validateDedicatedTrace(List<JsonObject> records) {
        int[] expectedCounts = {4, 4, 5, 5, 3};
        int[] seen = new int[DEDICATED_CASES.length];
        for (JsonObject record : records) {
            int dedicatedIndex = dedicatedCaseIndex(record.getString("case", ""));
            if (dedicatedIndex < 0) {
                throw new IllegalStateException("dedicated trace contains an unexpected case");
            }
            requireFields(record, "case", "operation", "statement", "sequence", "time", "new");
            if (!"listener".equals(string(record, "operation"))
                    || !"s0".equals(string(record, "statement"))) {
                throw new IllegalStateException("dedicated trace contains an unexpected listener record");
            }
            int sequence = integer(record, "sequence");
            if (sequence != seen[dedicatedIndex] + 1) {
                throw new IllegalStateException("dedicated trace sequence mismatch for " + DEDICATED_CASES[dedicatedIndex]);
            }
            if (!"1970-01-01T00:00:00Z".equals(string(record, "time"))) {
                throw new IllegalStateException("dedicated trace time mismatch for " + DEDICATED_CASES[dedicatedIndex]);
            }
            JsonArray rows = array(record.get("new"), "new listener rows");
            int expectedRows = dedicatedIndex < 2 ? 2 : dedicatedIndex < 4
                    ? (sequence < 5 ? 8 : 12) : 3;
            if (rows.size() != expectedRows) {
                throw new IllegalStateException("dedicated trace row count mismatch for " + DEDICATED_CASES[dedicatedIndex]
                        + " sequence " + sequence);
            }
            JsonArray expected = expectedDedicatedRows(dedicatedIndex, sequence);
            if (!rows.toString().equals(expected.toString())) {
                int mismatch = -1;
                int limit = Math.min(rows.size(), expected.size());
                for (int rowIndex = 0; rowIndex < limit; rowIndex++) {
                    if (!rows.get(rowIndex).toString().equals(expected.get(rowIndex).toString())) {
                        mismatch = rowIndex;
                        break;
                    }
                }
                throw new IllegalStateException("dedicated trace rows mismatch for " + DEDICATED_CASES[dedicatedIndex]
                        + " sequence " + sequence + " row=" + mismatch + " actual=" + rows.get(mismatch)
                        + " expected=" + expected.get(mismatch));
            }
            seen[dedicatedIndex]++;
        }
        for (int i = 0; i < seen.length; i++) {
            if (seen[i] != expectedCounts[i]) {
                throw new IllegalStateException("dedicated trace callback count mismatch for " + DEDICATED_CASES[i]
                        + ": expected " + expectedCounts[i] + ", got " + seen[i]);
            }
        }
    }
    private static JsonArray expectedDedicatedRows(int dedicatedIndex, int sequence) {
        if (dedicatedIndex < 2) {
            String[] strings = {"E1", "E1", "E1", "E1"};
            int[] ints = {10, 20, 10, 20};
            long[] longs = {100L, 200L, 200L, 100L};
            double[] intTotals = {1000d, 2000d, 4000d, 6000d};
            double[] longTotals = {1000d, 2000d, 5000d, 5000d};
            int index = sequence - 1;
            JsonArray rows = new JsonArray();
            rows.add(row(new Object[]{strings[index], ints[index], null, intTotals[index]}));
            rows.add(row(new Object[]{strings[index], null, longs[index], longTotals[index]}));
            return rows;
        }
        if (dedicatedIndex == 4) {
            return switch (sequence) {
                case 1 -> rows(
                    row(new Object[]{"E1", 1, 10L}), row(new Object[]{"E1", null, 10L}), row(new Object[]{null, null, 10L}));
                case 2 -> rows(
                    row(new Object[]{"E1", 2, 20L}), row(new Object[]{"E1", null, 30L}), row(new Object[]{null, null, 30L}));
                case 3 -> rows(
                    row(new Object[]{"E2", 1, 25L}), row(new Object[]{"E2", null, 25L}), row(new Object[]{null, null, 25L}));
                default -> throw new IllegalStateException("unexpected context sequence " + sequence);
            };
        }
        if (sequence < 5) {
            String[] strings = sequence == 1 ? new String[]{"E1", "E1", "E1", "E1", null, null, null, null}
                    : sequence == 2 ? new String[]{"E2", "E2", "E2", "E2", null, null, null, null}
                    : sequence == 3 ? new String[]{"E1", "E1", "E1", "E1", null, null, null, null}
                    : new String[]{"E2", "E2", "E2", "E2", null, null, null, null};
            int[] ints = sequence == 1 ? new int[]{1, 1, -1, -1, 1, 1, -1, -1}
                    : sequence == 2 ? new int[]{1, 1, -1, -1, 1, 1, -1, -1}
                    : sequence == 3 ? new int[]{2, 2, -1, -1, 2, 2, -1, -1}
                    : new int[]{2, 2, -1, -1, 2, 2, -1, -1};
            long[] longs = sequence == 1 ? new long[]{10, -1, 10, -1, 10, -1, 10, -1}
                    : sequence == 2 ? new long[]{20, -1, 20, -1, 20, -1, 20, -1}
                    : sequence == 3 ? new long[]{10, -1, 10, -1, 10, -1, 10, -1}
                    : new long[]{20, -1, 20, -1, 20, -1, 20, -1};
            double[] sums = sequence == 1 ? new double[]{100, 100, 100, 100, 100, 100, 100, 100}
                    : sequence == 2 ? new double[]{200, 200, 200, 200, 200, 300, 200, 300}
                    : sequence == 3 ? new double[]{300, 300, 400, 400, 300, 300, 400, 600}
                    : new double[]{400, 400, 600, 600, 400, 700, 600, 1000};
            long[] counts = sequence == 1 ? new long[]{1, 1, 1, 1, 1, 1, 1, 1}
                    : sequence == 2 ? new long[]{1, 1, 1, 1, 1, 2, 1, 2}
                    : sequence == 3 ? new long[]{1, 1, 2, 2, 1, 1, 2, 3}
                    : new long[]{1, 1, 2, 2, 1, 2, 2, 4};
            return cubeRows(strings, ints, longs, counts, sums);
        }
        return rows(
            cubeRow("E2", 1, 10L, 1, 500d, 0, 0, 0, 0),
            cubeRow("E1", 1, 10L, 0, null, 0, 0, 0, 0),
            cubeRow("E2", 1, null, 2, 700d, 0, 0, 1, 1),
            cubeRow("E1", 1, null, 0, null, 0, 0, 1, 1),
            cubeRow("E2", null, 10L, 1, 500d, 0, 1, 0, 2),
            cubeRow("E1", null, 10L, 1, 300d, 0, 1, 0, 2),
            cubeRow("E2", null, null, 3, 1100d, 0, 1, 1, 3),
            cubeRow("E1", null, null, 1, 300d, 0, 1, 1, 3),
            cubeRow(null, 1, 10L, 1, 500d, 1, 0, 0, 4),
            cubeRow(null, 1, null, 2, 700d, 1, 0, 1, 5),
            cubeRow(null, null, 10L, 2, 800d, 1, 1, 0, 6),
            cubeRow(null, null, null, 4, 1400d, 1, 1, 1, 7));
    }

    private static JsonArray cubeRows(String[] strings, int[] ints, long[] longs, long[] counts, double[] sums) {
        JsonArray rows = new JsonArray();
        for (int i = 0; i < strings.length; i++) {
            rows.add(cubeRow(strings[i], ints[i] < 0 ? null : ints[i], longs[i] < 0 ? null : longs[i],
                    counts[i], sums[i], i / 4, (i / 2) % 2, i % 2, i));
        }
        return rows;
    }

    private static JsonObject cubeRow(Object c0, Object c1, Object c2, long c3, Double c4,
                                      int c5, int c6, int c7, int c8) {
        return row(new Object[]{c0, c1, c2, c3, c4, c5, c6, c7, c8});
    }

    private static JsonArray rows(JsonObject... values) {
        JsonArray result = new JsonArray();
        for (JsonObject value : values) {
            result.add(value);
        }
        return result;
    }

    private static JsonObject row(Object[] values) {
        JsonObject fields = new JsonObject();
        for (int i = 0; i < values.length; i++) {
            fields.add("c" + i, normalize(values[i]));
        }
        return new JsonObject().add("kind", "row").add("fields", fields);
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
                || !new HashSet<>(object.names()).equals(new HashSet<>(Arrays.asList(expectedNames)))) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be an object");
        }
        return value.asObject();
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be an array");
        }
        return value.asArray();
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isString()) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isNumber()) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        double number = value.asDouble();
        long integral = value.asLong();
        if (!Double.isFinite(number) || number != integral || integral < Integer.MIN_VALUE || integral > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        return (int) integral;
    }

    private static void validateString(JsonObject payload, String field, String expected) {
        if (!expected.equals(string(payload, field))) {
            throw new IllegalArgumentException("payload " + field + " mismatch");
        }
    }

    private static void validateInteger(JsonObject payload, String field, int expected) {
        if (integer(payload, field) != expected) {
            throw new IllegalArgumentException("payload " + field + " mismatch");
        }
    }

    private static void validateLong(JsonObject payload, String field, long expected) {
        JsonValue value = payload.get(field);
        if (value == null || !value.isNumber() || value.asDouble() != expected || value.asLong() != expected) {
            throw new IllegalArgumentException("payload " + field + " mismatch");
        }
    }

    private static void validateDouble(JsonObject payload, String field, double expected) {
        JsonValue value = payload.get(field);
        if (value == null || !value.isNumber() || Double.compare(value.asDouble(), expected) != 0) {
            throw new IllegalArgumentException("payload " + field + " mismatch");
        }
    }


    private static void runCase(JsonArray allSteps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = new Configuration();
        config.getCommon().addEventType(SupportBean.class);
        config.getCommon().addEventType(SupportBean_S0.class);
        config.getCommon().addEventType(SupportEventWithIntArray.class);
        config.getCommon().addEventType(SupportThreeArrayEvent.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLResultSetQueryTypeRollupDimensionalityScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            String[] epls = buildEPL(caseName);
            List<EPStatement> s0Statements = new ArrayList<>();
            StringBuilder moduleText = new StringBuilder();
            for (int i = 0; i < epls.length; i++) {
                if (i > 0) {
                    moduleText.append(';');
                }
                moduleText.append(epls[i]);
            }
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(moduleText.toString(), new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    s0Statements.add(candidate);
                }
            }

            int[] seq = new int[] {0};
            for (EPStatement stmt : s0Statements) {
                stmt.addListener((newData, oldData, statement, rt) -> {
                    if (newData != null && newData.length > 0) {
                        seq[0]++;
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "listener");
                        record.add("statement", statement.getName());
                        record.add("sequence", seq[0]);
                        record.add("time", java.time.Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());
                        JsonArray newArr = new JsonArray();
                        for (EventBean event : newData) {
                            JsonObject newItem = new JsonObject();
                            newItem.add("kind", "row");
                            JsonObject fields = new JsonObject();
                            for (String prop : new TreeSet<>(java.util.Arrays.asList(event.getEventType().getPropertyNames()))) {
                                fields.add(prop, normalize(event.get(prop)));
                            }
                            newItem.add("fields", fields);
                            newArr.add(newItem);
                        }
                        record.add("new", newArr);
                        if (oldData != null && oldData.length > 0) {
                            JsonArray oldArr = new JsonArray();
                            for (EventBean event : oldData) {
                                JsonObject oldItem = new JsonObject();
                                oldItem.add("kind", "row");
                                JsonObject oldFields = new JsonObject();
                                for (String prop : new TreeSet<>(java.util.Arrays.asList(event.getEventType().getPropertyNames()))) {
                                    oldFields.add(prop, normalize(event.get(prop)));
                                }
                                oldItem.add("fields", oldFields);
                                oldArr.add(oldItem);
                            }
                            record.add("old", oldArr);
                        }
                        records.add(record);
                    }
                });
            }

            boolean inCase = false;
            for (JsonValue stepVal : allSteps) {
                JsonObject step = stepVal.asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    inCase = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                if ("send".equals(op)) {
                    String type = step.getString("eventType", "SupportBean");
                    JsonObject payload = step.get("payload").asObject();
                    switch (type) {
                        case "SupportBean" -> {
                            SupportBean event = new SupportBean();
                            JsonValue theStringVal = payload.get("theString");
                            if (theStringVal instanceof JsonString) {
                                event.setTheString(((JsonString) theStringVal).asString());
                            }
                            JsonValue intPrimitiveVal = payload.get("intPrimitive");
                            if (intPrimitiveVal instanceof JsonNumber) {
                                event.setIntPrimitive(((JsonNumber) intPrimitiveVal).asInt());
                            }
                            JsonValue longPrimitiveVal = payload.get("longPrimitive");
                            if (longPrimitiveVal instanceof JsonNumber) {
                                event.setLongPrimitive(((JsonNumber) longPrimitiveVal).asLong());
                            }
                            JsonValue doublePrimitiveVal = payload.get("doublePrimitive");
                            if (doublePrimitiveVal instanceof JsonNumber) {
                                event.setDoublePrimitive(((JsonNumber) doublePrimitiveVal).asDouble());
                            }
                            JsonValue intBoxedVal = payload.get("intBoxed");
                            if (intBoxedVal instanceof JsonNumber) {
                                event.setIntBoxed(((JsonNumber) intBoxedVal).asInt());
                            }
                            runtime.getEventService().sendEventBean(event, "SupportBean");
                        }
                        case "SupportBean_S0" -> {
                            SupportBean_S0 event = new SupportBean_S0(payload.getInt("id", 0));
                            runtime.getEventService().sendEventBean(event, "SupportBean_S0");
                        }
                        case "SupportEventWithIntArray" -> {
                            SupportEventWithIntArray event = new SupportEventWithIntArray(
                                payload.getString("id", null),
                                readIntArray(payload, "array"),
                                payload.getInt("value", 0));
                            runtime.getEventService().sendEventBean(event, "SupportEventWithIntArray");
                        }
                        case "SupportThreeArrayEvent" -> {
                            SupportThreeArrayEvent event = new SupportThreeArrayEvent(
                                payload.getString("id", null),
                                payload.getInt("value", 0),
                                readIntArray(payload, "intArray"),
                                readLongArray(payload, "longArray"),
                                readDoubleArray(payload, "doubleArray"));
                            runtime.getEventService().sendEventBean(event, "SupportThreeArrayEvent");
                        }
                        default -> throw new IllegalStateException("unknown eventType: " + type);
                    }
                }
                if ("types".equals(op)) {
                    for (EPStatement candidate : deployment.getStatements()) {
                        JsonObject value = new JsonObject();
                        for (String prop : new TreeSet<>(java.util.Arrays.asList(candidate.getEventType().getPropertyNames()))) {
                            if (!prop.matches("c\\d+")) {
                                // records cover the explicitly aliased
                                // selection columns only; auto-named
                                // aggregate columns are engine-dependent
                                continue;
                            }
                            value.add(prop, candidate.getEventType().getPropertyType(prop).getSimpleName());
                        }
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "types");
                        record.add("statement", candidate.getName());
                        record.add("value", value);
                        records.add(record);
                    }
                    continue;
                }
            }

        } finally {
            runtime.destroy();
        }
    }

    private static String[] buildEPL(String caseName) {
        return switch (caseName) {
            case "unbound-rollup-2dim" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean group by rollup(theString, intPrimitive)"
            };
            case "unbound-rollup-1dim-rollup" -> new String[]{
                "@Name('s0')select theString as c0, sum(intPrimitive) as c1 from SupportBean group by rollup(theString)"
            };
            case "unbound-rollup-1dim-cube" -> new String[]{
                "@Name('s0')select theString as c0, sum(intPrimitive) as c1 from SupportBean group by cube(theString)"
            };
            case "unbound-grouping-set-2level-unenclosed-a" -> new String[]{GROUPING_A_EPL};
            case "unbound-grouping-set-2level-unenclosed-b" -> new String[]{GROUPING_B_EPL};
            case "bound-cube-3dim-cube" -> new String[]{CUBE_EPL};
            case "bound-cube-3dim-gs" -> new String[]{CUBE_GS_EPL};
            case "context-partition-also-rollup" -> new String[]{CONTEXT_EPL};
            case "unbound-rollup-unenclosed-a" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, sum(doublePrimitive) as c3 from SupportBean group by theString, rollup(intPrimitive, longPrimitive)"
            };
            case "unbound-rollup-unenclosed-b" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, sum(doublePrimitive) as c3 from SupportBean group by grouping sets((theString, intPrimitive, longPrimitive),(theString, intPrimitive),theString)"
            };
            case "unbound-rollup-unenclosed-c" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, sum(doublePrimitive) as c3 from SupportBean group by theString, grouping sets((intPrimitive, longPrimitive),(intPrimitive), ())"
            };
            case "unbound-rollup-3dim-rollup" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, count(*) as c3, sum(doublePrimitive) as c4 from SupportBean#keepall group by rollup(theString, intPrimitive, longPrimitive)"
            };
            case "unbound-rollup-3dim-gs" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, count(*) as c3, sum(doublePrimitive) as c4 from SupportBean#keepall group by grouping sets((theString, intPrimitive, longPrimitive),(theString, intPrimitive),(theString),())"
            };
            case "unbound-rollup-3dim-rollup-join" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, count(*) as c3, sum(doublePrimitive) as c4 from SupportBean#keepall, SupportBean_S0#lastevent group by rollup(theString, intPrimitive, longPrimitive)"
            };
            case "unbound-rollup-3dim-gs-join" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, count(*) as c3, sum(doublePrimitive) as c4 from SupportBean#keepall, SupportBean_S0#lastevent group by grouping sets((theString, intPrimitive, longPrimitive),(theString, intPrimitive),(theString),())"
            };
            case "unbound-cube-unenclosed-a" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, sum(doublePrimitive) as c3 from SupportBean group by theString, cube(intPrimitive, longPrimitive)"
            };
            case "unbound-cube-unenclosed-b" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, sum(doublePrimitive) as c3 from SupportBean group by grouping sets((theString, intPrimitive, longPrimitive),(theString, intPrimitive),(theString, longPrimitive),theString)"
            };
            case "unbound-cube-unenclosed-c" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, sum(doublePrimitive) as c3 from SupportBean group by theString, grouping sets((intPrimitive, longPrimitive),(intPrimitive),(longPrimitive), ())"
            };
            case "unbound-cube-4dim" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, doublePrimitive as c3, sum(intBoxed) as c4 from SupportBean group by cube(theString, intPrimitive, longPrimitive, doublePrimitive)"
            };
            case "bound-rollup" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean#length(3) group by rollup(theString, intPrimitive)"
            };
            case "bound-rollup-join" -> new String[]{
                "@Name('s0')select theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean#length(3), SupportBean_S0#lastevent group by rollup(theString, intPrimitive)"
            };
            case "unbound-rollup-2dim-batch" -> new String[]{
                "@Name('s0')select irstream theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean#length_batch(4) group by rollup(theString, intPrimitive)"
            };
            case "warray-unbound" -> new String[]{
                "@Name('s0') select array, value, count(*) as cnt from SupportEventWithIntArray group by rollup(array, value)"
            };
            case "warray-bound" -> new String[]{
                "@Name('s0') select array, value, count(*) as cnt from SupportEventWithIntArray#keepall group by rollup(array, value)"
            };
            case "warray-join" -> new String[]{
                "@Name('s0') select array, value, count(*) as cnt from SupportEventWithIntArray#keepall, SupportBean#keepall group by rollup(array, value)"
            };
            case "warray-gs" -> new String[]{
                "@Name('s0') select sum(value) as thesum from SupportThreeArrayEvent group by grouping sets((intArray), (longArray), (doubleArray))"
            };
            case "nw-cube" -> new String[]{
                "create window MyWindow#keepall as SupportBean;\n" +
                    "insert into MyWindow select * from SupportBean(intBoxed = 0);\n" +
                    "on SupportBean(intBoxed = 3) delete from MyWindow;\n" +
                    "@Name('s0')" +
                    "select irstream theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from MyWindow " +
                    "group by cube(theString, intPrimitive)"
            };
            case "nw-cube-gs" -> new String[]{
                "create window MyWindow#keepall as SupportBean;\n" +
                    "insert into MyWindow select * from SupportBean(intBoxed = 0);\n" +
                    "on SupportBean(intBoxed = 3) delete from MyWindow;\n" +
                    "@Name('s0')" +
                    "select irstream theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from MyWindow " +
                    "group by grouping sets((theString, intPrimitive),(theString),(intPrimitive),())"
            };
            case "onselect-rollup" -> new String[]{
                "create window MyWindow#keepall as SupportBean;\n" +
                    "insert into MyWindow select * from SupportBean;\n" +
                    "@name('s0') on SupportBean_S0 as s0 select mw.theString as c0, sum(mw.intPrimitive) as c1, count(*) as c2 from MyWindow mw group by rollup(mw.theString);\n"
            };
            case "out-when-term-last" -> outputWhenTerminatedEPL("", "last");
            case "out-when-term-last-opt" -> outputWhenTerminatedEPL("@Hint('ENABLE_OUTPUTLIMIT_OPT')", "last");
            case "out-when-term-last-optdis" -> outputWhenTerminatedEPL("@Hint('DISABLE_OUTPUTLIMIT_OPT')", "last");
            case "out-when-term-all" -> outputWhenTerminatedEPL("", "all");
            case "out-when-term-snapshot" -> outputWhenTerminatedEPL("", "snapshot");
            case "bound-gs-no-top" -> new String[]{
                "@Name('s0')" +
                    "select irstream theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean#length(4) " +
                    "group by grouping sets(theString, intPrimitive)"
            };
            case "bound-gs-top-detail" -> new String[]{
                "@Name('s0')" +
                    "select irstream theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean#length(4) " +
                    "group by grouping sets((), (theString, intPrimitive))"
            };
            case "mixed-access" -> new String[]{
                "@name('s0') select sum(intPrimitive) as c0, theString as c1, window(*) as c2 " +
                    "from SupportBean#length(2) sb group by rollup(theString) order by theString"
            };
            case "non-boxed-types" -> new String[]{
                "@name('s0') select intPrimitive as c0, doublePrimitive as c1, longPrimitive as c2, sum(shortPrimitive) " +
                    "from SupportBean group by intPrimitive, rollup(doublePrimitive, longPrimitive)",
                "@name('s1') select intPrimitive as c0, doublePrimitive as c1, longPrimitive as c2, sum(shortPrimitive) " +
                    "from SupportBean group by grouping sets ((intPrimitive, doublePrimitive, longPrimitive))",
                "@name('s2') select intPrimitive as c0, doublePrimitive as c1, longPrimitive as c2, sum(shortPrimitive) " +
                    "from SupportBean group by grouping sets ((intPrimitive, doublePrimitive, longPrimitive), (intPrimitive, doublePrimitive))",
                "@name('s3') select intPrimitive as c0, doublePrimitive as c1, longPrimitive as c2, sum(shortPrimitive) " +
                    "from SupportBean group by grouping sets ((doublePrimitive, intPrimitive), (longPrimitive, intPrimitive))"
            };
            case "groupby-computation" -> new String[]{
                "@name('s0') select longPrimitive as c0, sum(intPrimitive) as c1 " +
                    "from SupportBean group by rollup(case when longPrimitive > 0 then 1 else 0 end)"
            };
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    private static String[] outputWhenTerminatedEPL(String hint, String outputLimit) {
        return new String[]{
            "@name('ctx') create context MyContext start SupportBean_S0(id=1) end SupportBean_S0(id=0);\n" +
                hint + " @name('s0') context MyContext select theString as c0, sum(intPrimitive) as c1 " +
                "from SupportBean group by rollup(theString) output " + outputLimit + " when terminated"
        };
    }

    private static int[] readIntArray(JsonObject payload, String field) {
        JsonValue value = payload.get(field);
        if (!(value instanceof JsonArray)) {
            throw new IllegalStateException("expected JSON array for field " + field);
        }
        JsonArray array = (JsonArray) value;
        int[] result = new int[array.size()];
        for (int i = 0; i < result.length; i++) {
            result[i] = array.get(i).asInt();
        }
        return result;
    }

    private static long[] readLongArray(JsonObject payload, String field) {
        JsonValue value = payload.get(field);
        if (!(value instanceof JsonArray)) {
            throw new IllegalStateException("expected JSON array for field " + field);
        }
        JsonArray array = (JsonArray) value;
        long[] result = new long[array.size()];
        for (int i = 0; i < result.length; i++) {
            result[i] = array.get(i).asLong();
        }
        return result;
    }

    private static double[] readDoubleArray(JsonObject payload, String field) {
        JsonValue value = payload.get(field);
        if (!(value instanceof JsonArray)) {
            throw new IllegalStateException("expected JSON array for field " + field);
        }
        JsonArray array = (JsonArray) value;
        double[] result = new double[array.size()];
        for (int i = 0; i < result.length; i++) {
            result[i] = array.get(i).asDouble();
        }
        return result;
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
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
        if (value instanceof Map<?, ?>) {
            Map<?, ?> mapValue = (Map<?, ?>) value;
            TreeSet<String> keys = new TreeSet<>();
            for (Object key : mapValue.keySet()) {
                keys.add(String.valueOf(key));
            }
            JsonObject fields = new JsonObject();
            for (String key : keys) {
                fields.add(key, normalize(mapValue.get(key)));
            }
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        if (value instanceof EventBean) {
            EventBean inner = (EventBean) value;
            JsonObject fields = new JsonObject();
            for (String prop : new TreeSet<>(java.util.Arrays.asList(inner.getEventType().getPropertyNames()))) {
                fields.add(prop, normalize(inner.get(prop)));
            }
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        if (value instanceof SupportBean_S1) {
            SupportBean_S1 event = (SupportBean_S1) value;
            JsonObject fields = new JsonObject();
            fields.add("id", normalize(event.getId()));
            fields.add("p10", normalize(event.getP10()));
            fields.add("p11", normalize(event.getP11()));
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        if (value instanceof int[]) {
            JsonArray array = new JsonArray();
            for (int item : (int[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof long[]) {
            JsonArray array = new JsonArray();
            for (long item : (long[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof double[]) {
            JsonArray array = new JsonArray();
            for (double item : (double[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof String[]) {
            JsonArray array = new JsonArray();
            for (String item : (String[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof Object[]) {
            // covers window(*) results, which surface as arrays of event
            // underlyings rather than EventBean wrappers
            JsonArray array = new JsonArray();
            for (Object item : (Object[]) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof SupportBean) {
            // renders nested SupportBean underlyings (window(*)) as
            // sorted-field rows mirroring the engine property names
            SupportBean bean = (SupportBean) value;
            JsonObject fields = new JsonObject();
            fields.add("bigDecimal", normalize(bean.getBigDecimal()));
            fields.add("bigInteger", normalize(bean.getBigInteger()));
            fields.add("boolBoxed", normalize(bean.getBoolBoxed()));
            fields.add("boolPrimitive", normalize(bean.isBoolPrimitive()));
            fields.add("byteBoxed", normalize(bean.getByteBoxed()));
            fields.add("bytePrimitive", normalize(bean.getBytePrimitive()));
            fields.add("charBoxed", normalize(bean.getCharBoxed()));
            fields.add("charPrimitive", normalize(bean.getCharPrimitive()));
            fields.add("doubleBoxed", normalize(bean.getDoubleBoxed()));
            fields.add("doublePrimitive", normalize(bean.getDoublePrimitive()));
            fields.add("enumValue", normalize(bean.getEnumValue()));
            fields.add("floatBoxed", normalize(bean.getFloatBoxed()));
            fields.add("floatPrimitive", normalize(bean.getFloatPrimitive()));
            fields.add("intBoxed", normalize(bean.getIntBoxed()));
            fields.add("intPrimitive", normalize(bean.getIntPrimitive()));
            fields.add("longBoxed", normalize(bean.getLongBoxed()));
            fields.add("longPrimitive", normalize(bean.getLongPrimitive()));
            fields.add("shortBoxed", normalize(bean.getShortBoxed()));
            fields.add("shortPrimitive", normalize(bean.getShortPrimitive()));
            fields.add("theString", normalize(bean.getTheString()));
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        return Json.value(String.valueOf(value));
    }

    /** Local mirror of the pinned SupportEventWithIntArray regression bean. */
    public static class SupportEventWithIntArray {
        private final String id;
        private final int[] array;
        private final int value;

        public SupportEventWithIntArray(String id, int[] array, int value) {
            this.id = id;
            this.array = array;
            this.value = value;
        }

        public String getId() {
            return id;
        }

        public int[] getArray() {
            return array;
        }

        public int getValue() {
            return value;
        }
    }

    /** Local mirror of the pinned SupportThreeArrayEvent regression bean. */
    public static class SupportThreeArrayEvent {
        private final String id;
        private final int value;
        private final int[] intArray;
        private final long[] longArray;
        private final double[] doubleArray;

        public SupportThreeArrayEvent(String id, int value, int[] intArray, long[] longArray, double[] doubleArray) {
            this.id = id;
            this.value = value;
            this.intArray = intArray;
            this.longArray = longArray;
            this.doubleArray = doubleArray;
        }

        public String getId() {
            return id;
        }

        public int getValue() {
            return value;
        }

        public int[] getIntArray() {
            return intArray;
        }

        public long[] getLongArray() {
            return longArray;
        }

        public double[] getDoubleArray() {
            return doubleArray;
        }
    }
}
