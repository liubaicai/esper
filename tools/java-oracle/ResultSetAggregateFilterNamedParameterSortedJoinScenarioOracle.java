import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
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
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;

/**
 * Direct Esper 9.0.0 oracle for ResultSetAggregateFilterNamedParameter
 * ordinals 15, 17 and 18: the joined sorted-aggregate executions and the
 * two-key sorted multicriteria execution. Each execution is deployed and
 * replayed in a fresh runtime. The kickoff SupportBean_S1 send joins an
 * empty SupportBean window and produces no listener record. SupportBean
 * payloads pin doublePrimitive=-1, the Java two-arg sendEvent default; the
 * multicriteria execution pins explicit doubles.
 */
public final class ResultSetAggregateFilterNamedParameterSortedJoinScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-filter-named-parameter-sorted-join";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFilterNamedParameter.java";
    private static final String DESCRIPTION =
            "ResultSetAggregateFilterNamedParameter sorted-aggregate join executions: maxby/minby/maxbyever/minbyever "
                    + "with theString value projections and sorted event-array columns under named filter parameters "
                    + "over a last-event join with length(4) or keepall SupportBean windows, plus two-key "
                    + "sorted(intPrimitive, doublePrimitive) multicriteria ordering with null rendered for empty "
                    + "filtered sets (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/"
                    + "resultset/aggregate/ResultSetAggregateFilterNamedParameter.java).";

    private static final String SORTED_JOIN_BOUND = "sorted-join-bound";
    private static final String SORTED_JOIN_UNBOUND = "sorted-join-unbound";
    private static final String SORTED_MULTICRITERIA = "sorted-multicriteria";
    private static final String[] CASES = {SORTED_JOIN_BOUND, SORTED_JOIN_UNBOUND, SORTED_MULTICRITERIA};
    private static final int[] ORDINALS = {15, 17, 18};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-6b6e0d2290261cb8cd93",
            "java-runtime-236d99a9d77ed3932510",
            "java-runtime-398a780be4d755650285"
    };
    private static final String[] EXECUTION_NAMES = {
            "ResultSetAggregateAccessAggSortedBound{join=true}",
            "ResultSetAggregateAccessAggSortedUnbound{join=true}",
            "ResultSetAggregateAccessAggSortedMulticriteria"
    };
    private static final String[] STATIC_IDS = {
            "java-ea6830fd215ba36a098b",
            "java-d726134be3446919675e",
            "java-a26ba8e7bb8f4e6c9f2d"
    };

    // Exact Java-source concatenations (no spaces after the commas that join
    // the select expressions).
    private static final String[] EPLS = {
            "@name('s0') select maxby(intPrimitive, filter:theString like 'A%').theString as aMaxby,"
                    + "minby(intPrimitive, filter:theString like 'A%').theString as aMinby,"
                    + "sorted(intPrimitive, filter:theString like 'A%') as aSorted,"
                    + "maxby(intPrimitive, filter:theString like 'B%').theString as bMaxby,"
                    + "minby(intPrimitive, filter:theString like 'B%').theString as bMinby,"
                    + "sorted(intPrimitive, filter:theString like 'B%') as bSorted"
                    + " from SupportBean_S1#lastevent, SupportBean#length(4)",
            "@name('s0') select maxby(intPrimitive, filter:theString like 'A%').theString as aMaxby,"
                    + "maxbyever(intPrimitive, filter:theString like 'A%').theString as aMaxbyever,"
                    + "minby(intPrimitive, filter:theString like 'A%').theString as aMinby,"
                    + "minbyever(intPrimitive, filter:theString like 'A%').theString as aMinbyever"
                    + " from SupportBean_S1#lastevent, SupportBean#keepall",
            "@name('s0') select sorted(intPrimitive, doublePrimitive, filter:theString like 'A%') as aSorted,"
                    + "sorted(intPrimitive, doublePrimitive, filter:theString like 'B%') as bSorted"
                    + " from SupportBean#keepall"
    };

    // Per case: case marker, one s0 deploy, sends in Java order, undeploy-all.
    private static final int[] STEP_COUNTS = {13, 9, 7};
    private static final int[] SEND_COUNTS = {10, 6, 4};
    private static final int[] RECORD_COUNTS = {9, 5, 4};

    private static final String[] BOUND_STRINGS = {"B1", "A10", "B2", "A5", "A15", "X3", "X4", "X5", "X6"};
    private static final int[] BOUND_INTS = {1, 10, 2, 5, 15, 3, 4, 5, 6};
    private static final double[] BOUND_DOUBLES = {-1, -1, -1, -1, -1, -1, -1, -1, -1};
    private static final String[] UNBOUND_STRINGS = {"B1", "A10", "A5", "A15", "B1000"};
    private static final int[] UNBOUND_INTS = {1, 10, 5, 15, 1000};
    private static final double[] UNBOUND_DOUBLES = {-1, -1, -1, -1, -1};
    private static final String[] MULTI_STRINGS = {"B1", "A1", "B2", "A2"};
    private static final int[] MULTI_INTS = {1, 100, 1, 100};
    private static final double[] MULTI_DOUBLES = {10, 2, 4, 3};

    // Sorted output-field names per case.
    private static final String[][] SORTED_FIELDS = {
            {"aMaxby", "aMinby", "aSorted", "bMaxby", "bMinby", "bSorted"},
            {"aMaxby", "aMaxbyever", "aMinby", "aMinbyever"},
            {"aSorted", "bSorted"}
    };

    // Expected listener values in sorted-field order. maxby/minby projections
    // are String or null; aSorted/bSorted cells are sorted event arrays
    // rendered from {intPrimitive, doublePrimitive, theString} triples. The
    // bound Java-source order is aMaxby,aMinby,aSorted,bMaxby,bMinby,bSorted;
    // the unbound Java-source order aMaxby,aMaxbyever,aMinby,aMinbyever is
    // already sorted.
    private static final Object[][] BOUND_EXPECTED = {
            {null, null, null, "B1", "B1", rows(row(1, -1, "B1"))},
            {"A10", "A10", rows(row(10, -1, "A10")), "B1", "B1", rows(row(1, -1, "B1"))},
            {"A10", "A10", rows(row(10, -1, "A10")), "B2", "B1",
                    rows(row(1, -1, "B1"), row(2, -1, "B2"))},
            {"A10", "A5", rows(row(5, -1, "A5"), row(10, -1, "A10")), "B2", "B1",
                    rows(row(1, -1, "B1"), row(2, -1, "B2"))},
            {"A15", "A5", rows(row(5, -1, "A5"), row(10, -1, "A10"), row(15, -1, "A15")), "B2", "B2",
                    rows(row(2, -1, "B2"))},
            {"A15", "A5", rows(row(5, -1, "A5"), row(15, -1, "A15")), "B2", "B2",
                    rows(row(2, -1, "B2"))},
            {"A15", "A5", rows(row(5, -1, "A5"), row(15, -1, "A15")), null, null, null},
            {"A15", "A15", rows(row(15, -1, "A15")), null, null, null},
            {null, null, null, null, null, null}
    };
    private static final Object[][] UNBOUND_EXPECTED = {
            {null, null, null, null},
            {"A10", "A10", "A10", "A10"},
            {"A10", "A10", "A5", "A5"},
            {"A15", "A15", "A5", "A5"},
            {"A15", "A15", "A5", "A5"}
    };
    // Two-key sorted(intPrimitive, doublePrimitive) event arrays: null or
    // {intPrimitive, doublePrimitive, theString} triples ascending on both
    // keys, the int tie between B1 and B2 broken by doublePrimitive 4 < 10.
    private static final Object[][] MULTI_EXPECTED = {
            {null, rows(row(1, 10, "B1"))},
            {rows(row(100, 2, "A1")), rows(row(1, 10, "B1"))},
            {rows(row(100, 2, "A1")), rows(row(1, 4, "B2"), row(1, 10, "B1"))},
            {rows(row(100, 2, "A1"), row(100, 3, "A2")),
                    rows(row(1, 4, "B2"), row(1, 10, "B1"))}
    };

    private static Object[] row(int intPrimitive, double doublePrimitive, String theString) {
        return new Object[]{intPrimitive, doublePrimitive, theString};
    }

    private static Object[][] rows(Object[]... rowTriples) {
        return rowTriples;
    }

    private ResultSetAggregateFilterNamedParameterSortedJoinScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetAggregateFilterNamedParameterSortedJoinScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        JsonArray steps = scenario.get("steps").asArray();
        int stepOffset = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(steps, stepOffset, STEP_COUNTS[caseIndex], caseIndex, records);
            stepOffset += STEP_COUNTS[caseIndex];
        }
        if (stepOffset != steps.size()) {
            throw new IllegalArgumentException("scenario contains trailing steps");
        }
        if (records.size() != 18) {
            throw new IllegalStateException("expected 18 listener records, got " + records.size());
        }

        JsonObject trace = new JsonObject()
                .add("version", VERSION)
                .add("id", ID)
                .add("javaCommit", JAVA_COMMIT)
                .add("java", System.getProperty("java.version"))
                .add("records", records);
        System.out.println(trace);
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

        JsonArray definitions = array(scenario.get("cases"), "cases");
        if (definitions.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly three cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(definitions.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation",
                    "iteratorSnapshots", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !"listener".equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 0
                    || !EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("scenario case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 29) {
            throw new IllegalArgumentException("scenario must contain exactly 29 steps");
        }
        int index = 0;
        index = validateCaseBlock(steps, index, 0);
        index = validateCaseBlock(steps, index, 1);
        index = validateCaseBlock(steps, index, 2);
        if (index != steps.size()) {
            throw new IllegalArgumentException("scenario contains trailing steps");
        }
    }

    private static int validateCaseBlock(JsonArray steps, int index, int caseIndex) {
        String caseName = CASES[caseIndex];
        validateCaseMarker(steps, index++, caseName);
        validateDeploy(steps, index++, caseIndex);
        int sends = 0;
        if (caseIndex == 0 || caseIndex == 1) {
            validateS1Send(steps, index++, caseName);
            sends++;
        }
        String[] strings;
        int[] ints;
        double[] doubles;
        if (caseIndex == 0) {
            strings = BOUND_STRINGS;
            ints = BOUND_INTS;
            doubles = BOUND_DOUBLES;
        } else if (caseIndex == 1) {
            strings = UNBOUND_STRINGS;
            ints = UNBOUND_INTS;
            doubles = UNBOUND_DOUBLES;
        } else {
            strings = MULTI_STRINGS;
            ints = MULTI_INTS;
            doubles = MULTI_DOUBLES;
        }
        for (int event = 0; event < strings.length; event++) {
            validateBeanSend(steps, index++, caseName, strings[event], ints[event], doubles[event]);
            sends++;
        }
        validateUndeployAll(steps, index++, caseName);
        if (sends != SEND_COUNTS[caseIndex]) {
            throw new IllegalArgumentException("case " + caseName + " must contain exactly "
                    + SEND_COUNTS[caseIndex] + " sends");
        }
        return index;
    }

    private static void validateCaseMarker(JsonArray steps, int index, String expectedCase) {
        JsonObject marker = object(steps.get(index), "case marker " + index);
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker mismatch at step " + index);
        }
    }

    private static void validateDeploy(JsonArray steps, int index, int caseIndex) {
        JsonObject step = object(steps.get(index), "deploy step " + index);
        requireFields(step, "op", "case", "statement", "epl");
        if (!"deploy".equals(string(step, "op"))
                || !CASES[caseIndex].equals(string(step, "case"))
                || !"s0".equals(string(step, "statement"))
                || !EPLS[caseIndex].equals(string(step, "epl"))) {
            throw new IllegalArgumentException("deploy step mismatch at step " + index);
        }
    }

    private static void validateS1Send(JsonArray steps, int index, String caseName) {
        JsonObject step = sendStep(steps, index, caseName);
        if (!"SupportBean_S1".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("step " + index + " must send SupportBean_S1");
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S1 payload " + index);
        requireFields(payload, "id", "doublePrimitive");
        if (integer(payload, "id") != 0
                || longNumber(payload, "doublePrimitive") != -1L) {
            throw new IllegalArgumentException("SupportBean_S1 payload mismatch at step " + index);
        }
    }

    private static void validateBeanSend(JsonArray steps, int index, String caseName,
                                         String expectedString, int expectedInt, double expectedDouble) {
        JsonObject step = sendStep(steps, index, caseName);
        if (!"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("step " + index + " must send SupportBean");
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload " + index);
        requireFields(payload, "theString", "intPrimitive", "doublePrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || integer(payload, "intPrimitive") != expectedInt
                || longNumber(payload, "doublePrimitive") != (long) expectedDouble) {
            throw new IllegalArgumentException("SupportBean payload mismatch at step " + index);
        }
    }

    private static JsonObject sendStep(JsonArray steps, int index, String caseName) {
        JsonObject step = object(steps.get(index), "send step " + index);
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("step " + index + " is not a pinned send step");
        }
        return step;
    }

    private static void validateUndeployAll(JsonArray steps, int index, String caseName) {
        JsonObject step = object(steps.get(index), "undeploy-all step " + index);
        requireFields(step, "op", "case");
        if (!"undeploy-all".equals(string(step, "op")) || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("undeploy-all step mismatch at step " + index);
        }
    }

    private static void runCase(JsonArray allSteps, int stepOffset, int stepCount,
                                int caseIndex, JsonArray records) throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        beanType.put("doublePrimitive", Double.class);
        configuration.getCommon().addEventType("SupportBean", beanType);
        Map<String, Object> s1Type = new HashMap<>();
        s1Type.put("id", Integer.class);
        s1Type.put("doublePrimitive", Double.class);
        configuration.getCommon().addEventType("SupportBean_S1", s1Type);

        String runtimeURI = "parity-" + ID + "-" + RUNTIME_IDS[caseIndex];
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPDeployment deployment = null;
            TraceWriter writer = null;
            int sends = 0;
            for (int offset = 0; offset < stepCount; offset++) {
                JsonObject step = object(allSteps.get(stepOffset + offset), "step " + (stepOffset + offset));
                String operation = string(step, "op");
                if ("case".equals(operation)) {
                    continue;
                }
                if ("deploy".equals(operation)) {
                    EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                            EPLS[caseIndex], new CompilerArguments(runtime.getRuntimePath()));
                    deployment = runtime.getDeploymentService().deploy(compiled,
                            new DeploymentOptions().setDeploymentId(runtimeURI));
                    EPStatement statement = findStatement(deployment, caseName);
                    writer = new TraceWriter(records, caseIndex, statement, runtime);
                    statement.addListener(writer);
                } else if ("send".equals(operation)) {
                    if (deployment == null) {
                        throw new IllegalStateException("case " + caseName + " send before deploy");
                    }
                    send(runtime, step);
                    sends++;
                } else if ("undeploy-all".equals(operation)) {
                    runtime.getDeploymentService().undeployAll();
                } else {
                    throw new IllegalArgumentException("unsupported operation in case " + caseName + ": " + operation);
                }
            }
            if (sends != SEND_COUNTS[caseIndex]) {
                throw new IllegalStateException("case " + caseName + " replayed " + sends
                        + " sends, expected " + SEND_COUNTS[caseIndex]);
            }
            if (writer == null || writer.sequence != RECORD_COUNTS[caseIndex]) {
                throw new IllegalStateException("case " + caseName + " produced "
                        + (writer == null ? 0 : writer.sequence)
                        + " listener records, expected " + RECORD_COUNTS[caseIndex]);
            }
        } finally {
            runtime.destroy();
        }
    }

    private static EPStatement findStatement(EPDeployment deployment, String caseName) {
        EPStatement[] statements = deployment.getStatements();
        if (statements == null) {
            throw new IllegalStateException("case " + caseName + " deployed no statements");
        }
        for (EPStatement statement : statements) {
            if ("s0".equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("statement s0 was not deployed for case " + caseName);
    }

    private static void send(EPRuntime runtime, JsonObject step) {
        String eventType = string(step, "eventType");
        JsonObject payload = object(step.get("payload"), eventType + " payload");
        Map<String, Object> event = new HashMap<>();
        if ("SupportBean".equals(eventType)) {
            requireFields(payload, "theString", "intPrimitive", "doublePrimitive");
            event.put("theString", string(payload, "theString"));
            event.put("intPrimitive", integer(payload, "intPrimitive"));
            event.put("doublePrimitive", (double) longNumber(payload, "doublePrimitive"));
        } else if ("SupportBean_S1".equals(eventType)) {
            requireFields(payload, "id", "doublePrimitive");
            event.put("id", integer(payload, "id"));
            event.put("doublePrimitive", (double) longNumber(payload, "doublePrimitive"));
        } else {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
        runtime.getEventService().sendEventMap(event, eventType);
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final int caseIndex;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, int caseIndex, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.caseIndex = caseIndex;
            this.caseName = CASES[caseIndex];
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if (!hasNew || hasOld || newEvents.length != 1) {
                throw new IllegalStateException("case " + caseName
                        + " listener callback must contain one new-only row");
            }
            if (sequence >= RECORD_COUNTS[caseIndex]) {
                throw new IllegalStateException("case " + caseName + " produced too many callbacks");
            }
            long nextSequence = sequence + 1;
            validateRow(newEvents[0], nextSequence);
            long now = runtime.getEventService().getCurrentTime();
            if (now != 0L) {
                throw new IllegalStateException("case " + caseName + " callback time at sequence "
                        + nextSequence + " was " + now + ", expected 0");
            }
            sequence = nextSequence;
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", sequence)
                    .add("time", Instant.ofEpochMilli(now).toString())
                    .add("new", rows(newEvents));
            records.add(record);
        }

        private void validateRow(EventBean event, long nextSequence) {
            String[] expectedFields = SORTED_FIELDS[caseIndex];
            String[] actualFields = event.getEventType().getPropertyNames().clone();
            Arrays.sort(actualFields);
            if (!Arrays.equals(actualFields, expectedFields)) {
                throw new IllegalStateException("case " + caseName + " field metadata mismatch: "
                        + Arrays.toString(actualFields));
            }
            Object[] expected = expectedRow(nextSequence);
            for (int field = 0; field < expectedFields.length; field++) {
                validateCell(expectedFields[field], expected[field], event.get(expectedFields[field]));
            }
        }

        private Object[] expectedRow(long nextSequence) {
            int offset = (int) nextSequence - 1;
            if (caseIndex == 0) {
                return BOUND_EXPECTED[offset];
            }
            if (caseIndex == 1) {
                return UNBOUND_EXPECTED[offset];
            }
            return MULTI_EXPECTED[offset];
        }

        private void validateCell(String field, Object expected, Object actual) {
            if (expected instanceof Object[][]) {
                validateSortedEventArray(field, (Object[][]) expected, actual);
                return;
            }
            if (expected == null) {
                if (actual != null) {
                    throw new IllegalStateException("case " + caseName + " field " + field + " expected null");
                }
                return;
            }
            if (!sameScalar(expected, actual)) {
                throw new IllegalStateException("case " + caseName + " value mismatch at field " + field);
            }
        }

        private void validateSortedEventArray(String field, Object[][] expectedRows, Object actual) {
            if (!(actual instanceof Object[])) {
                throw new IllegalStateException("case " + caseName + " field " + field
                        + " must be an event array");
            }
            Object[] actualRows = (Object[]) actual;
            if (actualRows.length != expectedRows.length) {
                throw new IllegalStateException("case " + caseName + " field " + field
                        + " row count mismatch: " + actualRows.length);
            }
            for (int row = 0; row < expectedRows.length; row++) {
                int expectedInt = (Integer) expectedRows[row][0];
                double expectedDouble = (Double) expectedRows[row][1];
                String expectedString = (String) expectedRows[row][2];
                Object actualRow = actualRows[row];
                Object actualInt;
                Object actualDouble;
                Object actualString;
                if (actualRow instanceof EventBean) {
                    actualInt = ((EventBean) actualRow).get("intPrimitive");
                    actualDouble = ((EventBean) actualRow).get("doublePrimitive");
                    actualString = ((EventBean) actualRow).get("theString");
                } else if (actualRow instanceof Map) {
                    actualInt = ((Map<?, ?>) actualRow).get("intPrimitive");
                    actualDouble = ((Map<?, ?>) actualRow).get("doublePrimitive");
                    actualString = ((Map<?, ?>) actualRow).get("theString");
                } else {
                    throw new IllegalStateException("case " + caseName + " field " + field
                            + " row " + row + " is not an event row");
                }
                if (!sameScalar(expectedInt, actualInt)
                        || !sameScalar(expectedDouble, actualDouble)
                        || !expectedString.equals(actualString)) {
                    throw new IllegalStateException("case " + caseName + " field " + field
                            + " row " + row + " mismatch");
                }
            }
        }

        private boolean sameScalar(Object expected, Object actual) {
            if (expected == null) {
                return actual == null;
            }
            if (expected instanceof Object[] && actual instanceof Object[]) {
                Object[] expectedRows = (Object[]) expected;
                Object[] actualRows = (Object[]) actual;
                if (expectedRows.length != actualRows.length) {
                    return false;
                }
                for (int index = 0; index < expectedRows.length; index++) {
                    if (!sameScalar(expectedRows[index], actualRows[index])) {
                        return false;
                    }
                }
                return true;
            }
            if (actual instanceof Number && expected instanceof Number) {
                return Double.compare(((Number) actual).doubleValue(), ((Number) expected).doubleValue()) == 0;
            }
            return expected.equals(actual);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            for (EventBean event : events) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                output.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return output;
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof Map<?, ?>) {
                JsonObject fields = new JsonObject();
                TreeSet<String> keys = new TreeSet<>();
                for (Object key : ((Map<?, ?>) value).keySet()) {
                    keys.add(String.valueOf(key));
                }
                for (String key : keys) {
                    fields.add(key, normalize(((Map<?, ?>) value).get(key)));
                }
                return new JsonObject().add("kind", "row").add("fields", fields);
            }
            if (value instanceof EventBean[]) {
                JsonArray output = new JsonArray();
                for (EventBean event : (EventBean[]) value) {
                    JsonObject fields = new JsonObject();
                    String[] names = event.getEventType().getPropertyNames().clone();
                    Arrays.sort(names);
                    for (String name : names) {
                        fields.add(name, normalize(event.get(name)));
                    }
                    output.add(new JsonObject().add("kind", "row").add("fields", fields));
                }
                return output;
            }
            if (value instanceof Object[]) {
                JsonArray output = new JsonArray();
                for (Object item : (Object[]) value) {
                    output.add(normalize(item));
                }
                return output;
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

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isString()) {
            throw new IllegalArgumentException(name + " must be a JSON string");
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

    private static long longNumber(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isNumber()) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        try {
            double number = value.asDouble();
            long integral = value.asLong();
            if (!Double.isFinite(number) || number != integral) {
                throw new IllegalArgumentException(name + " must be an integer JSON number");
            }
            return integral;
        } catch (RuntimeException ex) {
            throw new IllegalArgumentException(name + " must be an integer JSON number", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        JsonArray actual = array(value, name);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(name + " must contain exactly " + expected.length + " values");
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
}
