import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportThreeArrayEvent;
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
import java.util.Arrays;
import java.util.HashSet;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for ResultSetQueryTypeLocalGroupBy ordinals
 * 18/19/24/26/27, the local-group key representation and readback batch:
 * ResultSetLocalUngroupedSameKey (two scalar local keys on an object-array
 * event), ResultSetLocalGroupedSameKey (the same two local keys under an outer
 * {@code group by g1}, so the local groups are shared across outer groups),
 * ResultSetLocalEnumMethods (the {@code group_by:()} statement-wide level and
 * the {@code window(*)/first(*)} accessor methods resolved per local group),
 * ResultSetLocalMultikeyWArray (int[], long[] and double[] keys compared by
 * deep content, plus the compound key) and ResultSetLocalUngroupedOnlyWGroupBy
 * (an outer group-by whose aggregation is the empty local group).
 *
 * <p>Each Java execution compiles and deploys one two-statement EPL (the
 * {@code create objectarray schema} plus the query) or one single-statement
 * EPL, subscribes one listener to s0 after deploy, sends its events and
 * asserts every callback with assertPropsNew, then undeploys.  The oracle
 * mirrors that shape exactly, once per case in its own runtime: compile,
 * deploy, add listener, replay the case's sends, assert each callback and
 * record it, undeploy and destroy.
 *
 * <p>Every send of this batch produces exactly one listener callback carrying
 * exactly one new row, so the trace records all twenty callbacks in order.
 * Nothing in this batch carries a remove stream, so no record has an
 * {@code old} key, and the internal timer is disabled while the clock stays at
 * 1970-01-01T00:00:00Z for every record.
 *
 * <p>The ord-24 case is the one event-valued projection of this batch:
 * {@code window(*, group_by:()).firstOf()} and
 * {@code window(*, group_by:theString).firstOf()} both deliver the sent
 * SupportBean, which the trace renders as a full normalized row object using
 * the same SupportBean property set and spelling as the other oracle
 * scenarios, while c2/c3 (the scalar window firstOf values) and c4/c5 (the
 * first(*) intPrimitive accessors) are Integer 10.
 */
public final class ResultSetQueryTypeLocalGroupKeysScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-querytype-local-group-keys";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java";
    private static final String DESCRIPTION =
            "ResultSetQueryTypeLocalGroupBy ordinals 18/19/24/26/27: local-group key representation and readback"
                    + " - repeated scalar keys on object-array events, local groups shared across outer groups,"
                    + " the empty group_by:() statement-wide level, array-typed (int[]/long[]/double[]) keys"
                    + " compared by deep content, and the window(*)/first(*) accessor methods resolved per local"
                    + " group.";
    private static final String[] CASES = {
            "ungrouped-same-key", "grouped-same-key", "enum-methods-grouped", "multikey-w-array",
            "ungrouped-only-w-group-by"
    };
    private static final int[] ORDINALS = {18, 19, 24, 26, 27};
    private static final String[] RUNTIMES = {
            "java-runtime-890001d4334d5de6c50a",
            "java-runtime-a2b77511196632040e51",
            "java-runtime-b6a938fde543383eb73c",
            "java-runtime-81855e4095ee0ca7cadd",
            "java-runtime-e1253cd2c17a180c243a"
    };
    private static final String[] NAMES = {
            "ResultSetLocalUngroupedSameKey",
            "ResultSetLocalGroupedSameKey",
            "ResultSetLocalEnumMethods",
            "ResultSetLocalMultikeyWArray",
            "ResultSetLocalUngroupedOnlyWGroupBy"
    };
    private static final String[] STATIC_IDS = {
            "java-2c2e1d80b0046f88b67e",
            "java-b0aa55f10cfa580ccd4f",
            "java-98ac70ee0f434579c8c8",
            "java-6f7f7c3ba440787d3117",
            "java-e3b7ff9f0f3bf5872d7b"
    };
    private static final String[] OBSERVATIONS = {"listener", "listener", "listener", "listener", "listener"};
    private static final int[] ITERATOR_SNAPSHOTS = {0, 0, 0, 0, 0};
    // Every send of this batch produces exactly one callback with one new row.
    private static final int[] CASE_SENDS = {5, 5, 1, 7, 2};
    private static final int[] CASE_RECORDS = {5, 5, 1, 7, 2};
    private static final int TOTAL_RECORDS = 20;
    private static final String EPL_SAME_KEY =
            "@public @buseventtype create objectarray schema MyEventOne (d1 String, d2 String, val int);\n"
                    + "@name('s0') select sum(val, group_by: d1) as c0, sum(val, group_by: d2) as c1"
                    + " from MyEventOne";
    private static final String EPL_GROUPED_KEY =
            "@public @buseventtype create objectarray schema MyEventTwo (g1 String, d1 String, d2 String, val int);\n"
                    + "@name('s0') select sum(val) as c0, sum(val, group_by: d1) as c1,"
                    + " sum(val, group_by: d2) as c2 from MyEventTwo group by g1";
    // The Java source concatenates " first(*, group_by:theString).intPrimitive as c5 " with " from
    // SupportBean#keepall ", so the pinned EPL carries two spaces ahead of "from".
    private static final String EPL_ENUM_METHODS =
            "@name('s0') select"
                    + " window(*, group_by:()).firstOf() as c0,"
                    + " window(*, group_by:theString).firstOf() as c1,"
                    + " window(intPrimitive, group_by:()).firstOf() as c2,"
                    + " window(intPrimitive, group_by:theString).firstOf() as c3,"
                    + " first(*, group_by:()).intPrimitive as c4,"
                    + " first(*, group_by:theString).intPrimitive as c5 "
                    + " from SupportBean#keepall "
                    + "group by theString, intPrimitive";
    private static final String EPL_MULTIKEY_W_ARRAY =
            "@Name('s0') select sum(value, group_by:(intArray)) as c0, sum(value, group_by:(longArray)) as c1,"
                    + " sum(value, group_by:(doubleArray)) as c2,"
                    + " sum(value, group_by:(intArray, longArray, doubleArray)) as c3,"
                    + " sum(value) as c4 from SupportThreeArrayEvent";
    private static final String EPL_ONLY_W_GROUP_BY =
            "@name('s0') select"
                    + " first(*, group_by:()).intPrimitive as c0 "
                    + " from SupportBean#keepall "
                    + "group by theString, intPrimitive";
    // ungrouped-same-key send vectors: {d1, d2, val}.
    private static final String[] SAME_KEY_D1 = {"E1", "E1", "E2", "E3", "E3"};
    private static final String[] SAME_KEY_D2 = {"E1", "E2", "E1", "E1", "E3"};
    private static final int[] SAME_KEY_VAL = {10, 11, 12, 13, 14};
    // grouped-same-key send vectors: {g1, d1, d2, val}.
    private static final String[] GROUPED_KEY_G1 = {"E1", "E1", "E1", "X", "E1"};
    private static final String[] GROUPED_KEY_D1 = {"E1", "E1", "E2", "E1", "E2"};
    private static final String[] GROUPED_KEY_D2 = {"E1", "E2", "E1", "E1", "E3"};
    private static final int[] GROUPED_KEY_VAL = {10, 11, 12, 13, 14};
    // multikey-w-array send vectors.
    private static final String[] ARRAY_IDS = {"E1", "E2", "E3", "E4", "E5", "E6", "E7"};
    private static final int[] ARRAY_VALUES = {10, 11, 12, 13, 14, 15, 16};
    private static final int[][] ARRAY_INTS = {{1}, {2}, {3}, {1}, {1}, {3}, {2}};
    private static final long[][] ARRAY_LONGS = {{10}, {20}, {10}, {20}, {10}, {20}, {20}};
    private static final double[][] ARRAY_DOUBLES = {
            {100}, {200}, {300}, {200}, {100}, {300}, {200}
    };
    // ungrouped-only-w-group-by send vectors: SupportBean(theString, intPrimitive) with longPrimitive 0.
    private static final String[] ONLY_W_GROUP_BY_STRINGS = {"E1", "E2"};
    private static final int[] ONLY_W_GROUP_BY_INTS = {1, 2};
    // The result metadata of each query, in the order the runtime sorts it.
    private static final String[][] CASE_FIELDS = {
            {"c0", "c1"},
            {"c0", "c1", "c2"},
            {"c0", "c1", "c2", "c3", "c4", "c5"},
            {"c0", "c1", "c2", "c3", "c4"},
            {"c0"}
    };
    // The pinned scalar values of every asserted row in CASE_FIELDS order; the
    // ord-24 case is the event-valued one and is asserted separately.
    private static final int[][][] CASE_VALUES = {
            {{10, 10}, {21, 11}, {12, 22}, {13, 35}, {27, 14}},
            {{10, 10, 10}, {21, 21, 11}, {33, 12, 22}, {13, 34, 35}, {47, 26, 14}},
            null,
            {{10, 10, 10, 10, 10}, {11, 11, 11, 11, 21}, {12, 22, 12, 12, 33}, {23, 24, 24, 13, 46},
                    {37, 36, 24, 24, 60}, {27, 39, 27, 15, 75}, {27, 55, 40, 27, 91}},
            {{1}, {1}}
    };
    // The SupportBean of the ord-24 case: sent as SupportBean("E1", 10).
    private static final String ENUM_METHODS_STRING = "E1";
    private static final int ENUM_METHODS_INT = 10;

    private ResultSetQueryTypeLocalGroupKeysScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetQueryTypeLocalGroupKeysScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        JsonArray steps = scenario.get("steps").asArray();
        JsonArray records = new JsonArray();
        for (int index = 0; index < CASES.length; index++) {
            runCase(steps, records, index);
        }
        if (records.size() != TOTAL_RECORDS) {
            throw new IllegalStateException("expected " + TOTAL_RECORDS + " trace records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    /**
     * Replays one case in a fresh runtime, mirroring the Java execution that
     * compiles, deploys, listens, sends, asserts and undeploys once.
     */
    private static void runCase(JsonArray steps, JsonArray records, int index) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportThreeArrayEvent.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + RUNTIMES[index], configuration);
        runtime.getEventService().advanceTime(0L);
        int before = records.size();
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl(index),
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID + "-" + CASES[index]));
            try {
                EPStatement statement = findStatement(deployment);
                ListenerWriter writer = new ListenerWriter(records, statement, runtime, index);
                statement.addListener(writer);
                boolean active = false;
                int markers = 0;
                int sends = 0;
                for (int stepIndex = 0; stepIndex < steps.size(); stepIndex++) {
                    JsonObject step = object(steps.get(stepIndex), "step " + stepIndex);
                    String operation = string(step, "op");
                    if ("case".equals(operation)) {
                        if (CASES[index].equals(string(step, "case"))) {
                            if (++markers != 1) {
                                throw new IllegalStateException("duplicate active case marker");
                            }
                            active = true;
                        } else if (active) {
                            active = false;
                        }
                        continue;
                    }
                    if (!active) {
                        continue;
                    }
                    if (!"send".equals(operation)) {
                        throw new IllegalArgumentException(
                                "unsupported operation at step " + stepIndex + ": " + operation);
                    }
                    if (sends >= CASE_SENDS[index]) {
                        throw new IllegalArgumentException("unexpected send at step " + stepIndex);
                    }
                    send(runtime, writer, index, sends, step, stepIndex);
                    sends++;
                }
                if (markers != 1) {
                    throw new IllegalStateException("case marker count mismatch for " + CASES[index]);
                }
                if (sends != CASE_SENDS[index]) {
                    throw new IllegalStateException("expected " + CASE_SENDS[index] + " sends for " + CASES[index]
                            + ", got " + sends);
                }
                if (writer.failure != null) {
                    throw writer.failure;
                }
                if (writer.callbackCount != CASE_RECORDS[index]) {
                    throw new IllegalStateException("expected " + CASE_RECORDS[index] + " listener callbacks for "
                            + CASES[index] + ", got " + writer.callbackCount);
                }
                if (records.size() - before != CASE_RECORDS[index]) {
                    throw new IllegalStateException("expected " + CASE_RECORDS[index] + " records for "
                            + CASES[index] + ", got " + (records.size() - before));
                }
            } finally {
                runtime.getDeploymentService().undeploy(deployment.getDeploymentId());
            }
        } finally {
            runtime.destroy();
        }
    }

    private static String epl(int index) {
        if (index == 0) {
            return EPL_SAME_KEY;
        }
        if (index == 1) {
            return EPL_GROUPED_KEY;
        }
        if (index == 2) {
            return EPL_ENUM_METHODS;
        }
        return index == 3 ? EPL_MULTIKEY_W_ARRAY : EPL_ONLY_W_GROUP_BY;
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        EPStatement found = null;
        for (EPStatement candidate : deployment.getStatements()) {
            if ("s0".equals(candidate.getName())) {
                if (found != null) {
                    throw new IllegalStateException("deployment carries more than one statement named s0");
                }
                found = candidate;
            }
        }
        if (found == null) {
            throw new IllegalStateException("statement s0 not found");
        }
        return found;
    }

    private static void send(EPRuntime runtime, ListenerWriter writer, int caseIndex, int sendIndex,
                             JsonObject step, int stepIndex) {
        requireFields(step, "op", "eventType", "payload");
        if (caseIndex == 0 || caseIndex == 1) {
            String eventType = caseIndex == 0 ? "MyEventOne" : "MyEventTwo";
            if (!eventType.equals(string(step, "eventType"))) {
                throw new IllegalArgumentException("unsupported event type at step " + stepIndex);
            }
            JsonArray payload = array(step.get("payload"), "payload step " + stepIndex);
            Object[] values;
            if (caseIndex == 0) {
                values = new Object[]{payloadString(payload, 0, stepIndex), payloadString(payload, 1, stepIndex),
                        payloadInt(payload, 2, stepIndex)};
            } else {
                values = new Object[]{payloadString(payload, 0, stepIndex), payloadString(payload, 1, stepIndex),
                        payloadString(payload, 2, stepIndex), payloadInt(payload, 3, stepIndex)};
            }
            runtime.getEventService().sendEventObjectArray(values, eventType);
            return;
        }
        if (caseIndex == 2) {
            SupportBean enumMethodsBean = bean(step, stepIndex, ENUM_METHODS_STRING, ENUM_METHODS_INT, 0L);
            // The callback arrives synchronously from the send below and must
            // be able to check the delivered event against this instance.
            writer.sentBean = enumMethodsBean;
            runtime.getEventService().sendEventBean(enumMethodsBean, "SupportBean");
            return;
        }
        if (caseIndex == 3) {
            runtime.getEventService().sendEventBean(threeArrayEvent(step, stepIndex, sendIndex),
                    "SupportThreeArrayEvent");
            return;
        }
        SupportBean onlyWGroupByBean = bean(step, stepIndex, ONLY_W_GROUP_BY_STRINGS[sendIndex],
                ONLY_W_GROUP_BY_INTS[sendIndex], 0L);
        runtime.getEventService().sendEventBean(onlyWGroupByBean, "SupportBean");
    }

    private static SupportBean bean(JsonObject step, int stepIndex, String expectedString,
                                    int expectedInt, long expectedLong) {
        if (!"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("unsupported event type at step " + stepIndex);
        }
        JsonObject payload = object(step.get("payload"), "payload step " + stepIndex);
        requireFields(payload, "theString", "intPrimitive", "longPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || integer(payload, "intPrimitive") != expectedInt
                || longNumber(payload, "longPrimitive") != expectedLong) {
            throw new IllegalArgumentException("send payload is not pinned at step " + stepIndex);
        }
        SupportBean bean = new SupportBean(string(payload, "theString"), integer(payload, "intPrimitive"));
        bean.setLongPrimitive(longNumber(payload, "longPrimitive"));
        return bean;
    }

    private static SupportThreeArrayEvent threeArrayEvent(JsonObject step, int stepIndex, int sendIndex) {
        if (!"SupportThreeArrayEvent".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("unsupported event type at step " + stepIndex);
        }
        JsonObject payload = object(step.get("payload"), "payload step " + stepIndex);
        requireFields(payload, "id", "value", "intArray", "longArray", "doubleArray");
        JsonArray ints = array(payload.get("intArray"), "intArray step " + stepIndex);
        JsonArray longs = array(payload.get("longArray"), "longArray step " + stepIndex);
        JsonArray doubles = array(payload.get("doubleArray"), "doubleArray step " + stepIndex);
        if (ints.size() != ARRAY_INTS[sendIndex].length || longs.size() != ARRAY_LONGS[sendIndex].length
                || doubles.size() != ARRAY_DOUBLES[sendIndex].length) {
            throw new IllegalArgumentException("send payload array length is not pinned at step " + stepIndex);
        }
        int[] intArray = new int[ints.size()];
        for (int index = 0; index < intArray.length; index++) {
            intArray[index] = payloadInt(ints, index, stepIndex);
        }
        long[] longArray = new long[longs.size()];
        for (int index = 0; index < longArray.length; index++) {
            longArray[index] = payloadLong(longs, index, stepIndex);
        }
        double[] doubleArray = new double[doubles.size()];
        for (int index = 0; index < doubleArray.length; index++) {
            doubleArray[index] = payloadDouble(doubles, index, stepIndex);
        }
        if (!ARRAY_IDS[sendIndex].equals(string(payload, "id"))
                || integer(payload, "value") != ARRAY_VALUES[sendIndex]
                || !Arrays.equals(intArray, ARRAY_INTS[sendIndex])
                || !Arrays.equals(longArray, ARRAY_LONGS[sendIndex])
                || !Arrays.equals(doubleArray, ARRAY_DOUBLES[sendIndex])) {
            throw new IllegalArgumentException("send payload is not pinned at step " + stepIndex);
        }
        return new SupportThreeArrayEvent(string(payload, "id"), integer(payload, "value"), intArray, longArray,
                doubleArray);
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
        validateStringArray(scenario.get("javaRuntimes"), RUNTIMES, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("expected five cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation",
                    "iteratorSnapshots", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIMES[index].equals(string(definition, "runtimeId"))
                    || !NAMES[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != ITERATOR_SNAPSHOTS[index]
                    || !epl(index).equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        int stepIndex = 0;
        stepIndex = validateCaseMarker(steps, stepIndex, CASES[0]);
        for (int send = 0; send < CASE_SENDS[0]; send++) {
            validateObjectArraySend(steps, stepIndex++, "MyEventOne",
                    new String[]{SAME_KEY_D1[send], SAME_KEY_D2[send]}, SAME_KEY_VAL[send]);
        }
        stepIndex = validateCaseMarker(steps, stepIndex, CASES[1]);
        for (int send = 0; send < CASE_SENDS[1]; send++) {
            validateObjectArraySend(steps, stepIndex++, "MyEventTwo",
                    new String[]{GROUPED_KEY_G1[send], GROUPED_KEY_D1[send], GROUPED_KEY_D2[send]},
                    GROUPED_KEY_VAL[send]);
        }
        stepIndex = validateCaseMarker(steps, stepIndex, CASES[2]);
        validateBeanSend(steps, stepIndex++, ENUM_METHODS_STRING, ENUM_METHODS_INT, 0L);
        stepIndex = validateCaseMarker(steps, stepIndex, CASES[3]);
        for (int send = 0; send < CASE_SENDS[3]; send++) {
            validateThreeArraySend(steps, stepIndex++, send);
        }
        stepIndex = validateCaseMarker(steps, stepIndex, CASES[4]);
        for (int send = 0; send < CASE_SENDS[4]; send++) {
            validateBeanSend(steps, stepIndex++, ONLY_W_GROUP_BY_STRINGS[send], ONLY_W_GROUP_BY_INTS[send], 0L);
        }
        if (stepIndex != steps.size()) {
            throw new IllegalArgumentException("scenario has " + steps.size() + " steps, want " + stepIndex);
        }
    }

    private static int validateCaseMarker(JsonArray steps, int index, String expected) {
        if (index >= steps.size()) {
            throw new IllegalArgumentException("missing case marker for " + expected);
        }
        JsonObject step = object(steps.get(index), "step " + index);
        requireFields(step, "op", "case");
        if (!"case".equals(string(step, "op")) || !expected.equals(string(step, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned at step " + index);
        }
        return index + 1;
    }

    private static void validateObjectArraySend(JsonArray steps, int index, String eventType, String[] strings,
                                                int intValue) {
        JsonObject step = object(steps.get(index), "step " + index);
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !eventType.equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send event type is not pinned at step " + index);
        }
        JsonArray payload = array(step.get("payload"), "payload step " + index);
        if (payload.size() != strings.length + 1) {
            throw new IllegalArgumentException("send payload size is not pinned at step " + index);
        }
        for (int item = 0; item < strings.length; item++) {
            if (!strings[item].equals(payloadString(payload, item, index))) {
                throw new IllegalArgumentException("send payload is not pinned at step " + index);
            }
        }
        if (payloadInt(payload, strings.length, index) != intValue) {
            throw new IllegalArgumentException("send payload is not pinned at step " + index);
        }
    }

    private static void validateBeanSend(JsonArray steps, int index, String expectedString, int expectedInt,
                                         long expectedLong) {
        JsonObject step = object(steps.get(index), "step " + index);
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send event type is not pinned at step " + index);
        }
        JsonObject payload = object(step.get("payload"), "payload step " + index);
        requireFields(payload, "theString", "intPrimitive", "longPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || integer(payload, "intPrimitive") != expectedInt
                || longNumber(payload, "longPrimitive") != expectedLong) {
            throw new IllegalArgumentException("send payload is not pinned at step " + index);
        }
    }

    private static void validateThreeArraySend(JsonArray steps, int index, int send) {
        JsonObject step = object(steps.get(index), "step " + index);
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !"SupportThreeArrayEvent".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send event type is not pinned at step " + index);
        }
        JsonObject payload = object(step.get("payload"), "payload step " + index);
        requireFields(payload, "id", "value", "intArray", "longArray", "doubleArray");
        JsonArray ints = array(payload.get("intArray"), "intArray step " + index);
        JsonArray longs = array(payload.get("longArray"), "longArray step " + index);
        JsonArray doubles = array(payload.get("doubleArray"), "doubleArray step " + index);
        if (ints.size() != ARRAY_INTS[send].length || longs.size() != ARRAY_LONGS[send].length
                || doubles.size() != ARRAY_DOUBLES[send].length
                || !ARRAY_IDS[send].equals(string(payload, "id"))
                || integer(payload, "value") != ARRAY_VALUES[send]) {
            throw new IllegalArgumentException("send payload is not pinned at step " + index);
        }
        for (int item = 0; item < ints.size(); item++) {
            if (payloadInt(ints, item, index) != ARRAY_INTS[send][item]) {
                throw new IllegalArgumentException("send payload is not pinned at step " + index);
            }
        }
        for (int item = 0; item < longs.size(); item++) {
            if (payloadLong(longs, item, index) != ARRAY_LONGS[send][item]) {
                throw new IllegalArgumentException("send payload is not pinned at step " + index);
            }
        }
        for (int item = 0; item < doubles.size(); item++) {
            if (payloadDouble(doubles, item, index) != ARRAY_DOUBLES[send][item]) {
                throw new IllegalArgumentException("send payload is not pinned at step " + index);
            }
        }
    }

    private static JsonObject row(EventBean event) {
        String[] properties = event.getEventType().getPropertyNames().clone();
        Arrays.sort(properties);
        JsonObject fields = new JsonObject();
        for (String property : properties) {
            fields.add(property, normalize(event.get(property)));
        }
        return new JsonObject().add("kind", "row").add("fields", fields);
    }

    private static JsonArray rows(EventBean[] events) {
        JsonArray output = new JsonArray();
        if (events != null) {
            for (EventBean event : events) {
                output.add(row(event));
            }
        }
        return output;
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value instanceof EventBean[] events) {
            JsonArray array = new JsonArray();
            for (EventBean event : events) {
                array.add(normalize(event));
            }
            return array;
        }
        if (value instanceof EventBean event) {
            JsonObject fields = new JsonObject();
            String[] names = event.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            for (String name : names) {
                fields.add(name, normalize(event.get(name)));
            }
            return new JsonObject().add("kind", "row").add("fields", fields);
        }
        if (value instanceof Object[] objects) {
            JsonArray array = new JsonArray();
            for (Object object : objects) {
                array.add(normalize(object));
            }
            return array;
        }
        if (value instanceof SupportBean bean) {
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
            return new JsonObject().add("kind", "row").add("fields", fields);
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
        if (value instanceof Character character) {
            return Json.value(String.valueOf(character));
        }
        return Json.value(String.valueOf(value));
    }

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (com.espertech.esper.common.client.json.minimaljson.Member member : value.asObject()) {
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

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isString()) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static String payloadString(JsonArray payload, int index, int stepIndex) {
        if (index >= payload.size()) {
            throw new IllegalArgumentException("payload is missing item " + index + " at step " + stepIndex);
        }
        JsonValue value = payload.get(index);
        if (value == null || !value.isString()) {
            throw new IllegalArgumentException("payload item " + index + " must be a JSON string at step " + stepIndex);
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        long value = longNumber(object, name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        return (int) value;
    }

    private static int payloadInt(JsonArray payload, int index, int stepIndex) {
        long value = payloadLong(payload, index, stepIndex);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException("payload item " + index + " must be an integer at step " + stepIndex);
        }
        return (int) value;
    }

    private static long payloadLong(JsonArray payload, int index, int stepIndex) {
        if (index >= payload.size()) {
            throw new IllegalArgumentException("payload is missing item " + index + " at step " + stepIndex);
        }
        JsonValue value = payload.get(index);
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException("payload item " + index + " must be a number at step " + stepIndex);
        }
        String text = value.toString();
        if (!text.matches("-?(0|[1-9][0-9]*)")) {
            throw new IllegalArgumentException("payload item " + index + " must be an integer at step " + stepIndex);
        }
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException("payload item " + index + " is outside the Java long range", ex);
        }
    }

    private static double payloadDouble(JsonArray payload, int index, int stepIndex) {
        if (index >= payload.size()) {
            throw new IllegalArgumentException("payload is missing item " + index + " at step " + stepIndex);
        }
        JsonValue value = payload.get(index);
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException("payload item " + index + " must be a number at step " + stepIndex);
        }
        return Double.parseDouble(value.toString());
    }

    private static long longNumber(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        String text = value.toString();
        if (!text.matches("-?(0|[1-9][0-9]*)")) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(name + " is outside the Java long range", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        JsonArray actual = array(value, name);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(name + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (item == null || !item.isString() || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(name + " mismatch at index " + index);
            }
        }
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be a JSON array");
        }
        return value.asArray();
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be a JSON object");
        }
        return value.asObject();
    }

    private static final class ListenerWriter implements UpdateListener {
        private final JsonArray records;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private final int caseIndex;
        private int sequence;
        private int callbackCount;
        private SupportBean sentBean;
        private RuntimeException failure;

        private ListenerWriter(JsonArray records, EPStatement statement, EPRuntime runtime, int caseIndex) {
            this.records = records;
            this.statement = statement;
            this.runtime = runtime;
            this.caseIndex = caseIndex;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            try {
                int callback = callbackCount + 1;
                if (callback > CASE_RECORDS[caseIndex]) {
                    throw new IllegalStateException("unexpected listener callback " + callback + " for "
                            + CASES[caseIndex] + " (observed " + callback + " callbacks, " + records.size()
                            + " trace records overall so far)");
                }
                // Every execution of this batch asserts new-only data.
                if (oldEvents != null && oldEvents.length != 0) {
                    throw new IllegalStateException(CASES[caseIndex] + " callback " + callback + " delivered "
                            + oldEvents.length + " old rows");
                }
                long now = runtime.getEventService().getCurrentTime();
                if (now != 0L) {
                    throw new IllegalStateException(CASES[caseIndex] + " callback " + callback + " is at time " + now
                            + ", want 1970-01-01T00:00:00Z");
                }
                assertRecord(newEvents, callback);
                callbackCount = callback;
                sequence++;
                records.add(new JsonObject().add("case", CASES[caseIndex]).add("operation", "listener")
                        .add("statement", statement.getName()).add("sequence", sequence)
                        .add("time", Instant.ofEpochMilli(now).toString()).add("new", rows(newEvents)));
            } catch (RuntimeException ex) {
                // The runtime may swallow listener exceptions, so remember the
                // first failure and surface it in the main thread.
                if (failure == null) {
                    failure = ex;
                }
                ex.printStackTrace(System.err);
                throw ex;
            }
        }

        /**
         * Mirrors the Java assertPropsNew of the case: exactly one new row with
         * the pinned result metadata and the pinned values.  The ord-24 case
         * additionally pins the two event-valued projection columns against the
         * SupportBean instance that was sent.
         */
        private void assertRecord(EventBean[] newEvents, int next) {
            String label = CASES[caseIndex] + " callback " + next;
            if (newEvents == null || newEvents.length != 1) {
                throw new IllegalStateException(label + " must carry exactly one new row, got "
                        + (newEvents == null ? "no new data" : Integer.toString(newEvents.length)));
            }
            EventBean row = newEvents[0];
            assertFields(row, CASE_FIELDS[caseIndex]);
            if (caseIndex == 2) {
                assertBeanValue(row, "c0", label);
                assertBeanValue(row, "c1", label);
                assertInt(row, "c2", ENUM_METHODS_INT, label);
                assertInt(row, "c3", ENUM_METHODS_INT, label);
                assertInt(row, "c4", ENUM_METHODS_INT, label);
                assertInt(row, "c5", ENUM_METHODS_INT, label);
                return;
            }
            int[] expected = CASE_VALUES[caseIndex][next - 1];
            for (int field = 0; field < expected.length; field++) {
                assertInt(row, CASE_FIELDS[caseIndex][field], expected[field], label);
            }
        }

        private void assertBeanValue(EventBean row, String name, String label) {
            Object value = row.get(name);
            if (value instanceof EventBean event) {
                if (event.getUnderlying() != sentBean) {
                    throw new IllegalStateException(label + " field " + name + " lost the sent SupportBean event");
                }
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                if (!Arrays.equals(names, BEAN_FIELDS)) {
                    throw new IllegalStateException(label + " field " + name
                            + " SupportBean field metadata is not pinned: " + Arrays.toString(names));
                }
                return;
            }
            if (value instanceof SupportBean bean) {
                if (bean != sentBean) {
                    throw new IllegalStateException(label + " field " + name + " lost the sent SupportBean event");
                }
                return;
            }
            throw new IllegalStateException(label + " field " + name + " is not an event value: "
                    + simpleName(value));
        }

        private void assertInt(EventBean row, String name, int expected, String label) {
            Object value = row.get(name);
            if (!(value instanceof Integer) || ((Integer) value).intValue() != expected) {
                throw new IllegalStateException(label + " field " + name + " = " + value + " ("
                        + simpleName(value) + "), want " + expected + " (Integer)");
            }
        }

        private static String simpleName(Object value) {
            return value == null ? "null" : value.getClass().getSimpleName();
        }

        /**
         * Pins the result metadata as a sorted set of names: the runtime sorts
         * the result event's property names, so the pinned selection order is
         * sorted here instead of being transcribed a second time by hand.
         */
        private static void assertFields(EventBean row, String[] expected) {
            String[] names = row.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            String[] wanted = expected.clone();
            Arrays.sort(wanted);
            if (!Arrays.equals(names, wanted)) {
                throw new IllegalStateException("result field metadata is not pinned: " + Arrays.toString(names)
                        + ", want " + Arrays.toString(wanted));
            }
        }
    }

    // The SupportBean property set of an event-valued projection column, in the
    // same spelling the other oracle scenarios use.
    private static final String[] BEAN_FIELDS = {
            "bigDecimal", "bigInteger", "boolBoxed", "boolPrimitive", "byteBoxed", "bytePrimitive",
            "charBoxed", "charPrimitive", "doubleBoxed", "doublePrimitive", "enumValue", "floatBoxed",
            "floatPrimitive", "intBoxed", "intPrimitive", "longBoxed", "longPrimitive", "shortBoxed",
            "shortPrimitive", "theString"
    };
}
