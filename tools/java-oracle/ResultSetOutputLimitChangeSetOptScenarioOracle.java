import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.internal.epl.output.core.OutputProcessView;
import com.espertech.esper.common.internal.statement.resource.StatementResourceHolder;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.statement.EPStatementSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.time.format.DateTimeParseException;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Set;
import java.util.regex.Pattern;

/**
 * Direct Esper 9.0.0 oracle for the ResultSetOutputLimitChangeSetOpt execution, the white-box
 * getNumChangesetRows contract of the buffered output process view.  Thirty-six rounds redeploy
 * one s0 statement over SupportBean#length(2) across output-limit hint (default, enable,
 * disable), select (plain, fully aggregated, mixed, grouped), group-by, having, and output kind
 * (last, all, first) variants.  Each round attaches a recording-nothing listener, sends E0..E4,
 * reads the changeset counter before the one-second advance (five only when the disabled hint
 * keeps the buffering delta-set view, otherwise zero), advances cumulative time by one second,
 * re-reads the drained counter, and undeploys.  Each round emits exactly two records (odd
 * sequence before the advance, even sequence after); listeners never contribute row data.  The
 * ENABLE_OUTPUTLIMIT_OPT-with-order-by compile rejection is asserted in code after the rounds
 * and emits no record.
 */
public final class ResultSetOutputLimitChangeSetOptScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-output-limit-changeset-opt";
    private static final String DESCRIPTION =
            "ResultSetOutputLimitChangeSetOpt: white-box changeset counter across output-limit hint, select, "
                    + "group-by, having, and kind variants.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/"
                    + "ResultSetOutputLimitChangeSetOpt.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-9b0f27f829d9820e3792"
    };
    private static final String[] EXECUTION_NAMES = {
            "ResultSetOutputLimitChangeSetOpt"
    };
    private static final String[] STATIC_IDS = {
            "java-9f9a094322f3c6687f00"
    };
    private static final String CASE_NAME = "changeset";
    private static final int ROUND_COUNT = 36;
    private static final int SENDS_PER_ROUND = 5;
    private static final int STEPS_PER_ROUND = SENDS_PER_ROUND + 1;
    private static final int EXPECTED_STEPS = 1 + ROUND_COUNT * STEPS_PER_ROUND;
    private static final int EXPECTED_RECORDS = ROUND_COUNT * 2;
    private static final String[] SEND_STRINGS = {"E0", "E1", "E2", "E3", "E4"};
    private static final int[] SEND_INTS = {0, 1, 2, 3, 4};
    private static final String INVALID_EPL = "@Hint('ENABLE_OUTPUTLIMIT_OPT') select sum(intPrimitive) "
            + "from SupportBean output last every 4 events order by theString";
    private static final String INVALID_MESSAGE_PREFIX =
            "The ENABLE_OUTPUTLIMIT_OPT hint is not supported with order-by";

    // EPL per round, in the execution order of ResultSetOutputLimitChangeSetOpt.run.
    private static final String[] EPLS = {
            // unaggregated and ungrouped
            "@name('s0') select irstream intPrimitive from SupportBean#length(2) output last every 1 seconds",
            "@name('s0') select irstream intPrimitive from SupportBean#length(2) output last every 1 seconds order by intPrimitive",
            "@name('s0') select irstream intPrimitive from SupportBean#length(2) output all every 1 seconds",
            "@Hint('ENABLE_OUTPUTLIMIT_OPT')@name('s0') select irstream intPrimitive "
                    + "from SupportBean#length(2) output all every 1 seconds",
            "@Hint('DISABLE_OUTPUTLIMIT_OPT')@name('s0') select irstream intPrimitive "
                    + "from SupportBean#length(2) output all every 1 seconds",
            "@name('s0') select irstream intPrimitive from SupportBean#length(2) output first every 1 seconds",
            // fully-aggregated and ungrouped
            "@name('s0') select irstream count(*) from SupportBean#length(2) output last every 1 seconds",
            "@Hint('ENABLE_OUTPUTLIMIT_OPT')@name('s0') select irstream count(*) "
                    + "from SupportBean#length(2) output last every 1 seconds",
            "@Hint('DISABLE_OUTPUTLIMIT_OPT')@name('s0') select irstream count(*) "
                    + "from SupportBean#length(2) output last every 1 seconds",
            "@name('s0') select irstream count(*) from SupportBean#length(2) output all every 1 seconds",
            "@Hint('ENABLE_OUTPUTLIMIT_OPT')@name('s0') select irstream count(*) "
                    + "from SupportBean#length(2) output all every 1 seconds",
            "@Hint('DISABLE_OUTPUTLIMIT_OPT')@name('s0') select irstream count(*) "
                    + "from SupportBean#length(2) output all every 1 seconds",
            "@name('s0') select irstream count(*) from SupportBean#length(2) output first every 1 seconds",
            "@name('s0') select irstream count(*) from SupportBean#length(2) having count(*) > 0 "
                    + "output first every 1 seconds",
            // aggregated and ungrouped
            "@name('s0') select irstream theString, count(*) from SupportBean#length(2) "
                    + "output last every 1 seconds",
            "@Hint('ENABLE_OUTPUTLIMIT_OPT')@name('s0') select irstream theString, count(*) "
                    + "from SupportBean#length(2) output last every 1 seconds",
            "@Hint('DISABLE_OUTPUTLIMIT_OPT')@name('s0') select irstream theString, count(*) "
                    + "from SupportBean#length(2) output last every 1 seconds",
            "@name('s0') select irstream theString, count(*) from SupportBean#length(2) "
                    + "output all every 1 seconds",
            "@Hint('ENABLE_OUTPUTLIMIT_OPT')@name('s0') select irstream theString, count(*) "
                    + "from SupportBean#length(2) output all every 1 seconds",
            "@Hint('DISABLE_OUTPUTLIMIT_OPT')@name('s0') select irstream theString, count(*) "
                    + "from SupportBean#length(2) output all every 1 seconds",
            "@name('s0') select irstream theString, count(*) from SupportBean#length(2) "
                    + "output first every 1 seconds",
            "@name('s0') select irstream theString, count(*) from SupportBean#length(2) having count(*) > 0 "
                    + "output first every 1 seconds",
            // fully-aggregated and grouped
            "@name('s0') select irstream theString, count(*) from SupportBean#length(2) "
                    + "group by theString output last every 1 seconds",
            "@Hint('ENABLE_OUTPUTLIMIT_OPT')@name('s0') select irstream theString, count(*) "
                    + "from SupportBean#length(2) group by theString output last every 1 seconds",
            "@Hint('DISABLE_OUTPUTLIMIT_OPT')@name('s0') select irstream theString, count(*) "
                    + "from SupportBean#length(2) group by theString output last every 1 seconds",
            "@name('s0') select irstream theString, count(*) from SupportBean#length(2) "
                    + "group by theString output all every 1 seconds",
            "@Hint('ENABLE_OUTPUTLIMIT_OPT')@name('s0') select irstream theString, count(*) "
                    + "from SupportBean#length(2) group by theString output all every 1 seconds",
            "@Hint('DISABLE_OUTPUTLIMIT_OPT')@name('s0') select irstream theString, count(*) "
                    + "from SupportBean#length(2) group by theString output all every 1 seconds",
            "@name('s0') select irstream theString, count(*) from SupportBean#length(2) "
                    + "group by theString output first every 1 seconds",
            // aggregated and grouped
            "@name('s0') select irstream theString, intPrimitive, count(*) from SupportBean#length(2) "
                    + "group by theString output last every 1 seconds",
            "@Hint('ENABLE_OUTPUTLIMIT_OPT')@name('s0') select irstream theString, intPrimitive, count(*) "
                    + "from SupportBean#length(2) group by theString output last every 1 seconds",
            "@Hint('DISABLE_OUTPUTLIMIT_OPT')@name('s0') select irstream theString, intPrimitive, count(*) "
                    + "from SupportBean#length(2) group by theString output last every 1 seconds",
            "@name('s0') select irstream theString, intPrimitive, count(*) from SupportBean#length(2) "
                    + "group by theString output all every 1 seconds",
            "@Hint('ENABLE_OUTPUTLIMIT_OPT')@name('s0') select irstream theString, intPrimitive, count(*) "
                    + "from SupportBean#length(2) group by theString output all every 1 seconds",
            "@Hint('DISABLE_OUTPUTLIMIT_OPT')@name('s0') select irstream theString, intPrimitive, count(*) "
                    + "from SupportBean#length(2) group by theString output all every 1 seconds",
            "@name('s0') select irstream theString, intPrimitive, count(*) from SupportBean#length(2) "
                    + "group by theString output first every 1 seconds",
    };

    // Expected getNumChangesetRows per round after the five sends and before the advance;
    // five only when the disabled hint keeps OutputProcessViewConditionDefault buffering,
    // zero for the last-all-unordered, first, and per-group-drained null-condition views.
    private static final int[] EXPECTED_COUNTERS_BEFORE = {
            0, 0, 0, 0, 5, 0,
            0, 0, 5, 0, 0, 5, 0, 0,
            0, 0, 5, 0, 0, 5, 0, 0,
            0, 0, 5, 0, 0, 5, 0,
            0, 0, 5, 0, 0, 5, 0,
    };
    private static final UpdateListener NO_OP_LISTENER =
            (newEvents, oldEvents, statement, runtime) -> {
                // attached exactly as the regression execution does; rows are not recorded
            };
    private static final Pattern INTEGER_SYNTAX = Pattern.compile("-?(?:0|[1-9][0-9]*)");

    private ResultSetOutputLimitChangeSetOptScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOutputLimitChangeSetOptScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + CASE_NAME, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            JsonArray allSteps = scenario.get("steps").asArray();
            for (int round = 0; round < ROUND_COUNT; round++) {
                runRound(round, runtime, allSteps, records);
            }
            assertInvalidCompileRejected(runtime);
        } finally {
            runtime.destroy();
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS + " changeset records, got "
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

    private static void runRound(int round, EPRuntime runtime, JsonArray allSteps, JsonArray records)
            throws Exception {
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                EPLS[round], new CompilerArguments(runtime.getRuntimePath()));
        try {
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(ID + "-round-" + round));
            EPStatement statement = findStatement(deployment, round);
            statement.addListener(NO_OP_LISTENER);
            int base = 1 + round * STEPS_PER_ROUND;
            for (int sendIndex = 0; sendIndex < SENDS_PER_ROUND; sendIndex++) {
                sendEvent(runtime, allSteps.get(base + sendIndex), round, sendIndex);
            }

            int counterBefore = readNumChangesetRows(statement);
            int expectedBefore = EXPECTED_COUNTERS_BEFORE[round];
            if (counterBefore != expectedBefore) {
                throw new IllegalStateException("round " + round + " changeset counter before output expected "
                        + expectedBefore + ", got " + counterBefore);
            }
            records.add(changesetRecord(statement, round * 2 + 1,
                    runtime.getEventService().getCurrentTime(), counterBefore));

            advanceTime(allSteps.get(base + SENDS_PER_ROUND), runtime, round);
            int counterAfter = readNumChangesetRows(statement);
            if (counterAfter != 0) {
                throw new IllegalStateException("round " + round + " changeset counter after output expected 0, got "
                        + counterAfter);
            }
            records.add(changesetRecord(statement, round * 2 + 2,
                    runtime.getEventService().getCurrentTime(), counterAfter));
        } finally {
            runtime.getDeploymentService().undeployAll();
        }
    }

    private static EPStatement findStatement(EPDeployment deployment, int round) {
        EPStatement result = null;
        for (EPStatement candidate : deployment.getStatements()) {
            if (!"s0".equals(candidate.getName())) {
                continue;
            }
            if (result != null) {
                throw new IllegalStateException("round " + round + " deployed multiple s0 statements");
            }
            result = candidate;
        }
        if (result == null) {
            throw new IllegalStateException("round " + round + " did not deploy statement s0");
        }
        return result;
    }

    private static int readNumChangesetRows(EPStatement statement) {
        EPStatementSPI spi = (EPStatementSPI) statement;
        StatementResourceHolder resources = spi.getStatementContext().getStatementCPCacheService()
                .getStatementResourceService().getResourcesUnpartitioned();
        OutputProcessView outputProcessView = (OutputProcessView) resources.getFinalView();
        return outputProcessView.getNumChangesetRows();
    }

    private static JsonObject changesetRecord(EPStatement statement, int sequence, long time, int value) {
        JsonObject record = new JsonObject();
        record.add("case", CASE_NAME);
        record.add("operation", "changeset");
        record.add("statement", statement.getName());
        record.add("sequence", sequence);
        record.add("time", Instant.ofEpochMilli(time).toString());
        record.add("value", value);
        return record;
    }

    private static void assertInvalidCompileRejected(EPRuntime runtime) {
        try {
            EPCompilerProvider.getCompiler().compile(INVALID_EPL,
                    new CompilerArguments(runtime.getRuntimePath()));
        } catch (EPCompileException ex) {
            if (ex.getMessage() == null || !ex.getMessage().startsWith(INVALID_MESSAGE_PREFIX)) {
                throw new IllegalStateException("invalid-compile rejection message is not pinned", ex);
            }
            return;
        }
        throw new IllegalStateException("the " + INVALID_MESSAGE_PREFIX + " statement compiled successfully");
    }

    private static void advanceTime(JsonValue stepValue, EPRuntime runtime, int round) {
        JsonObject step = object(stepValue, "step");
        requireFields(step, "op", "at");
        if (!"advance-time".equals(string(step, "op"))) {
            throw new IllegalArgumentException("round " + round + " output step is not an advance-time");
        }
        String expectedAt = Instant.ofEpochMilli((round + 1) * 1000L).toString();
        if (!expectedAt.equals(string(step, "at"))) {
            throw new IllegalArgumentException("round " + round + " advance-time value is not pinned");
        }
        Instant instant;
        try {
            instant = Instant.parse(string(step, "at"));
        } catch (DateTimeParseException ex) {
            throw new IllegalArgumentException("advance-time value is not an instant", ex);
        }
        runtime.getEventService().advanceTime(instant.toEpochMilli());
    }

    private static void sendEvent(EPRuntime runtime, JsonValue stepValue, int round, int sendIndex) {
        JsonObject step = object(stepValue, "step");
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op")) || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("round " + round + " step " + sendIndex
                    + " is not a pinned SupportBean send");
        }
        JsonObject payload = object(step.get("payload"), "event payload");
        requireFields(payload, "theString", "intPrimitive");
        String expectedString = SEND_STRINGS[sendIndex];
        int expectedIntPrimitive = SEND_INTS[sendIndex];
        if (!expectedString.equals(string(payload, "theString"))
                || longInteger(payload, "intPrimitive") != expectedIntPrimitive) {
            throw new IllegalArgumentException("round " + round + " step " + sendIndex
                    + " payload is not pinned");
        }
        runtime.getEventService().sendEventBean(new SupportBean(expectedString, expectedIntPrimitive),
                "SupportBean");
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
        if (cases.size() != 1) {
            throw new IllegalArgumentException("scenario must contain exactly one case");
        }
        JsonObject definition = object(cases.get(0), "case definition");
        requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                "observation", "iteratorSnapshots", "epl");
        if (!CASE_NAME.equals(string(definition, "case"))
                || integer(definition, "ordinal") != 0
                || !RUNTIME_IDS[0].equals(string(definition, "runtimeId"))
                || !EXECUTION_NAMES[0].equals(string(definition, "executionName"))
                || !"listener".equals(string(definition, "observation"))
                || integer(definition, "iteratorSnapshots") != 0
                || !EPLS[0].equals(string(definition, "epl"))) {
            throw new IllegalArgumentException("case metadata is not pinned");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS + " steps");
        }
        int offset = 0;
        validateCaseMarker(steps.get(offset++), CASE_NAME);
        for (int round = 0; round < ROUND_COUNT; round++) {
            for (int sendIndex = 0; sendIndex < SENDS_PER_ROUND; sendIndex++) {
                validateBeanStep(steps.get(offset++), SEND_STRINGS[sendIndex], SEND_INTS[sendIndex]);
            }
            validateAdvanceTimeStep(steps.get(offset++), Instant.ofEpochMilli((round + 1) * 1000L).toString());
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

    private static void validateBeanStep(JsonValue value, String expectedString,
                                         int expectedIntPrimitive) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned");
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        if (!expectedString.equals(string(payload, "theString"))
                || longInteger(payload, "intPrimitive") != expectedIntPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned");
        }
    }

    private static void validateAdvanceTimeStep(JsonValue value, String expectedInstant) {
        JsonObject step = object(value, "advance-time step");
        requireFields(step, "op", "at");
        if (!"advance-time".equals(string(step, "op"))) {
            throw new IllegalArgumentException("advance-time step is not pinned");
        }
        if (!expectedInstant.equals(string(step, "at"))) {
            throw new IllegalArgumentException("advance-time value is not pinned");
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
        long value = longInteger(object, name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " is outside the Java int range");
        }
        return (int) value;
    }

    private static long longInteger(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be a JSON integer");
        }
        String text = value.toString();
        if (!INTEGER_SYNTAX.matcher(text).matches()) {
            throw new IllegalArgumentException(name + " must use integer JSON syntax");
        }
        try {
            return Long.parseLong(text, 10);
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

    /** Local mirror of the pinned SupportBean regression bean. */
    public static final class SupportBean {
        private String theString;
        private int intPrimitive;

        public SupportBean(String theString, int intPrimitive) {
            this.theString = theString;
            this.intPrimitive = intPrimitive;
        }

        public String getTheString() {
            return theString;
        }

        public void setTheString(String theString) {
            this.theString = theString;
        }

        public int getIntPrimitive() {
            return intPrimitive;
        }

        public void setIntPrimitive(int intPrimitive) {
            this.intPrimitive = intPrimitive;
        }
    }
}
