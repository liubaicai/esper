import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
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
import java.util.HashSet;
import java.util.List;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for ResultSetQueryTypeLocalGroupBy ordinals 17/23,
 * the context-terminated local-group snapshots:
 * ResultSetAggregateFullyVersusNotFullyAgg (four variants over
 * {@code output snapshot when terminated} in a
 * {@code start SupportBean_S0 end SupportBean_S1} context) and
 * ResultSetLocalUngroupedOrderBy (the aggregate order-by over the same context).
 *
 * <p>Both Java executions deploy one two-statement EPL per variant - the
 * {@code create context} statement plus the {@code @name('s0')} query - through
 * a single compileDeploy, subscribe one listener to s0, and then drive the
 * context with SupportBean_S0(0), three SupportBeans and SupportBean_S1(0).
 * Terminating the context emits the snapshot, which is where every asserted
 * callback of this batch arrives.  Afterwards each execution sends
 * SupportBean_S0(1) plus SupportBean_S1(1), the empty batch, and then calls
 * undeployAll.
 *
 * <p>The listener assertions here mirror the Java executions exactly:
 * fully-agg-ungrouped pins one new row through assertPropsNew ({@code c0=60},
 * Integer), fully-agg-grouped pins two rows, agg-ungrouped three rows and
 * agg-grouped three rows through assertPropsPerRowAnyOrder (the delivered order
 * is recorded, membership is pinned), and ungrouped-order-by pins five rows in
 * exact order through assertPropsPerRowLastNew.
 *
 * <p>The empty batch is left unasserted by every Java execution, and the Java
 * runtime answers it with one further invocation per case: the fully aggregated
 * ungrouped query delivers its single unasserted {@code {c0=null}} row (the one
 * empty-batch delivery of this batch, recorded) while the other four cases
 * terminate with no new rows at all (logged, no record).  The oracle therefore
 * pins two callbacks per case and the six trace records the five asserted
 * callbacks plus that one empty-batch row add up to.  Only the
 * statement-level result metadata and the no-old-data invariant are checked on
 * unasserted deliveries, and every callback is required to carry no old rows, so
 * the emitted trace has no {@code old} key at all; any callback outside the two
 * context terminations fails the run.
 *
 * <p>Each execution owns its runtime (the four ord-17 variants share one
 * runtime and deploy/undeploy per case, exactly like the single Java execution
 * that reuses one RegressionEnvironment), the runtime's internal timer is
 * disabled, and the clock stays at 1970-01-01T00:00:00Z for every record.
 */
public final class ResultSetQueryTypeLocalGroupContextTerminatedScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-querytype-local-group-context-terminated";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java";
    private static final String DESCRIPTION =
            "ResultSetQueryTypeLocalGroupBy ordinals 17/23: context-terminated snapshots over fully aggregated,"
                    + " ungrouped, grouped and locally grouped variants plus aggregate order-by.";
    private static final String[] CASES = {
            "fully-agg-ungrouped", "agg-ungrouped", "fully-agg-grouped", "agg-grouped", "ungrouped-order-by"
    };
    private static final int[] ORDINALS = {17, 17, 17, 17, 23};
    // The Java source has two executions: ResultSetAggregateFullyVersusNotFullyAgg
    // carries the four ord-17 variants in one RegressionEnvironment, and
    // ResultSetLocalUngroupedOrderBy is the ord-23 execution.  The scenario pins
    // the two executions as parallel deduplicated lists, so every case names its
    // execution through an index into them.
    private static final int[] CASE_EXECUTIONS = {0, 0, 0, 0, 1};
    private static final String[] RUNTIMES = {
            "java-runtime-ee681560ebeda52abbb1", "java-runtime-377240544dc6ec554ab2"
    };
    private static final String[] NAMES = {
            "ResultSetAggregateFullyVersusNotFullyAgg", "ResultSetLocalUngroupedOrderBy"
    };
    private static final String[] STATIC_IDS = {"java-d7190dd845b29e121075", "java-ae42d2957d2e50829f37"};
    private static final String[] OBSERVATIONS = {"listener", "listener", "listener", "listener", "listener"};
    private static final int[] ITERATOR_SNAPSHOTS = {0, 0, 0, 0, 0};
    // The runtime invokes the listener twice per case, once for the termination
    // of the asserted batch and once for the terminal S1 of the empty batch.
    private static final int[] CASE_CALLBACKS = {2, 2, 2, 2, 2};
    // Per-case trace census: fully-agg-ungrouped is the one case whose empty
    // batch delivers a row (the unasserted {c0=null} row of a fully aggregated
    // query), so it answers twice; the other four cases answer the empty batch
    // with no new data at all, which is logged and produces no record.
    private static final int[] CASE_RECORDS = {2, 1, 1, 1, 1};
    private static final int TOTAL_RECORDS = 6;
    // Per-case sends: S0(0), three SupportBeans, S1(0), S0(1), S1(1) for the four
    // ord-17 variants, and S0(0), five SupportBeans, S1(0), S0(1), S1(1) for the
    // ord-23 execution.
    private static final int[] CASE_SENDS = {7, 7, 7, 7, 9};
    // The send ordinal of the S1 that terminates the asserted batch.
    private static final int[] CASE_ASSERTED_TERMINATIONS = {5, 5, 5, 5, 7};
    private static final String EPL_FULLY_AGG_UNGROUPED =
            "@public create context StartS0EndS1 start SupportBean_S0 end SupportBean_S1;"
                    + "@name('s0') context StartS0EndS1 select sum(group_by:(),intPrimitive) as c0"
                    + " from SupportBean output snapshot when terminated;";
    private static final String EPL_AGG_UNGROUPED =
            "@public create context StartS0EndS1 start SupportBean_S0 end SupportBean_S1;"
                    + "@name('s0') context StartS0EndS1 select sum(group_by:theString, intPrimitive) as c0"
                    + " from SupportBean#keepall output snapshot when terminated;";
    private static final String EPL_FULLY_AGG_GROUPED =
            "@public create context StartS0EndS1 start SupportBean_S0 end SupportBean_S1;"
                    + "@name('s0') context StartS0EndS1 select sum(intPrimitive, group_by:()) as c0,"
                    + " sum(group_by:theString, intPrimitive) as c1, theString from SupportBean group by theString"
                    + " output snapshot when terminated;";
    private static final String EPL_AGG_GROUPED =
            "@public create context StartS0EndS1 start SupportBean_S0 end SupportBean_S1;"
                    + "@name('s0') context StartS0EndS1 select sum(longPrimitive, group_by:()) as c0,"
                    + " sum(longPrimitive, group_by:theString) as c1,  sum(longPrimitive, group_by:intPrimitive)"
                    + " as c2,  theString from SupportBean#keepall group by theString"
                    + " output snapshot when terminated;";
    private static final String EPL_UNGROUPED_ORDER_BY =
            "create context StartS0EndS1 start SupportBean_S0 end SupportBean_S1;"
                    + "@name('s0') context StartS0EndS1 select theString, sum(intPrimitive, group_by:theString)"
                    + " as c0  from SupportBean#keepall  output snapshot when terminated"
                    + " order by sum(intPrimitive, group_by:theString);";
    // The result metadata of each query, in the shape the runtime sorts it.
    private static final String[][] RESULT_FIELDS = {
            {"c0"},
            {"c0"},
            {"c0", "c1", "theString"},
            {"c0", "c1", "c2", "theString"},
            {"theString", "c0"}
    };
    // fully-agg-ungrouped asserts exactly one new row through assertPropsNew; the
    // sum over the empty second batch is not asserted in Java and is recorded as
    // delivered.
    private static final int FULLY_AGG_UNGROUPED_C0 = 60;
    // The three assertPropsPerRowAnyOrder variants, in the field order of
    // RESULT_FIELDS: the pinned rows are a membership set, the delivered order is
    // what this oracle records.
    private static final Object[][] AGG_UNGROUPED_ROWS = {{10}, {50}, {50}};
    private static final Object[][] FULLY_AGG_GROUPED_ROWS = {{60, 10, "E1"}, {60, 50, "E2"}};
    private static final Object[][] AGG_GROUPED_ROWS = {
            {600L, 100L, 100L, "E1"}, {600L, 500L, 200L, "E2"}, {600L, 500L, 300L, "E2"}
    };
    // ungrouped-order-by goes through assertPropsPerRowLastNew, so the five rows
    // are pinned in exactly this order: the three 40-ties keep the window
    // insertion order (E1's first event, E1's third event, E3) ahead of the two
    // 70-rows of E2.
    private static final Object[][] UNGROUPED_ORDER_BY_ROWS = {
            {"E1", 40}, {"E1", 40}, {"E3", 40}, {"E2", 70}, {"E2", 70}
    };
    private static final String[] STEP_OPS = {
            "case",
            "send", "send", "send", "send", "send", "send", "send",
            "case",
            "send", "send", "send", "send", "send", "send", "send",
            "case",
            "send", "send", "send", "send", "send", "send", "send",
            "case",
            "send", "send", "send", "send", "send", "send", "send",
            "case",
            "send", "send", "send", "send", "send", "send", "send", "send", "send"
    };
    private static final String[] CASE_MARKERS = {
            "fully-agg-ungrouped", "agg-ungrouped", "fully-agg-grouped", "agg-grouped", "ungrouped-order-by"
    };
    private static final String[] SEND_EVENT_TYPES = {
            "SupportBean_S0", "SupportBean", "SupportBean", "SupportBean", "SupportBean_S1", "SupportBean_S0",
            "SupportBean_S1",
            "SupportBean_S0", "SupportBean", "SupportBean", "SupportBean", "SupportBean_S1", "SupportBean_S0",
            "SupportBean_S1",
            "SupportBean_S0", "SupportBean", "SupportBean", "SupportBean", "SupportBean_S1", "SupportBean_S0",
            "SupportBean_S1",
            "SupportBean_S0", "SupportBean", "SupportBean", "SupportBean", "SupportBean_S1", "SupportBean_S0",
            "SupportBean_S1",
            "SupportBean_S0", "SupportBean", "SupportBean", "SupportBean", "SupportBean", "SupportBean",
            "SupportBean_S1", "SupportBean_S0", "SupportBean_S1"
    };
    // SEND_STRINGS: theString for the SupportBean sends and null for the
    // SupportBean_S0/S1 sends, which carry only id.
    private static final String[] SEND_STRINGS = {
            null, "E1", "E2", "E2", null, null, null,
            null, "E1", "E2", "E2", null, null, null,
            null, "E1", "E2", "E2", null, null, null,
            null, "E1", "E2", "E2", null, null, null,
            null, "E1", "E2", "E1", "E3", "E2", null, null, null
    };
    // SEND_INTS: intPrimitive for the SupportBean sends and id for the
    // SupportBean_S0/S1 sends.
    private static final int[] SEND_INTS = {
            0, 10, 20, 30, 0, 1, 1,
            0, 10, 20, 30, 0, 1, 1,
            0, 10, 20, 30, 0, 1, 1,
            0, 10, 20, 30, 0, 1, 1,
            0, 10, 20, 30, 40, 50, 0, 1, 1
    };
    // SEND_LONGS: longPrimitive for the SupportBean sends; the context start and
    // end events do not carry it, so those slots stay at zero.
    private static final long[] SEND_LONGS = {
            0, 100, 200, 300, 0, 0, 0,
            0, 100, 200, 300, 0, 0, 0,
            0, 100, 200, 300, 0, 0, 0,
            0, 100, 200, 300, 0, 0, 0,
            0, 0, 0, 0, 0, 0, 0, 0, 0
    };

    private ResultSetQueryTypeLocalGroupContextTerminatedScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetQueryTypeLocalGroupContextTerminatedScenarioOracle <scenario.json>");
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
        for (int execution = 0; execution < RUNTIMES.length; execution++) {
            runExecution(steps, records, execution);
        }
        if (records.size() != TOTAL_RECORDS) {
            throw new IllegalStateException("expected " + TOTAL_RECORDS + " trace records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    /**
     * Runs the cases of one Java execution in one runtime, deploy and undeploy
     * per case, exactly like the Java execution that compiles, deploys, asserts
     * and undeploys each variant inside a single RegressionEnvironment.
     */
    private static void runExecution(JsonArray steps, JsonArray records, int execution) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCommon().addEventType(SupportBean_S1.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + RUNTIMES[execution], configuration);
        runtime.getEventService().advanceTime(0L);
        try {
            for (int index = 0; index < CASES.length; index++) {
                if (CASE_EXECUTIONS[index] == execution) {
                    runCase(runtime, steps, records, index);
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static void runCase(EPRuntime runtime, JsonArray steps, JsonArray records, int index) throws Exception {
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl(index),
                new CompilerArguments(runtime.getRuntimePath()));
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                new DeploymentOptions().setDeploymentId(ID + "-" + CASES[index]));
        int before = records.size();
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
                    throw new IllegalArgumentException("unsupported operation at step " + stepIndex + ": " + operation);
                }
                if (sends >= CASE_SENDS[index]) {
                    throw new IllegalArgumentException("unexpected send at step " + stepIndex);
                }
                // Any callback this send produces arrives synchronously, so the
                // writer knows which context termination it is answering.
                writer.sendOrdinal = sends + 1;
                send(runtime, step, stepIndex);
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
            if (writer.callbackCount != CASE_CALLBACKS[index]) {
                throw new IllegalStateException("expected " + CASE_CALLBACKS[index] + " listener callbacks for "
                        + CASES[index] + ", got " + writer.callbackCount);
            }
            if (writer.sequence != CASE_RECORDS[index]) {
                throw new IllegalStateException("expected " + CASE_RECORDS[index] + " listener records for "
                        + CASES[index] + ", got " + writer.sequence);
            }
            if (records.size() - before != CASE_RECORDS[index]) {
                throw new IllegalStateException("expected " + CASE_RECORDS[index] + " records for " + CASES[index]
                        + ", got " + (records.size() - before));
            }
        } finally {
            runtime.getDeploymentService().undeploy(deployment.getDeploymentId());
        }
    }

    private static String epl(int index) {
        if (index == 0) {
            return EPL_FULLY_AGG_UNGROUPED;
        }
        if (index == 1) {
            return EPL_AGG_UNGROUPED;
        }
        if (index == 2) {
            return EPL_FULLY_AGG_GROUPED;
        }
        return index == 3 ? EPL_AGG_GROUPED : EPL_UNGROUPED_ORDER_BY;
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

    private static void send(EPRuntime runtime, JsonObject step, int stepIndex) {
        String eventType = string(step, "eventType");
        JsonObject payload = object(step.get("payload"), "payload step " + stepIndex);
        if ("SupportBean".equals(eventType)) {
            requireFields(payload, "theString", "intPrimitive", "longPrimitive");
            SupportBean bean = new SupportBean(string(payload, "theString"), integer(payload, "intPrimitive"));
            bean.setLongPrimitive(longNumber(payload, "longPrimitive"));
            runtime.getEventService().sendEventBean(bean, eventType);
            return;
        }
        if ("SupportBean_S0".equals(eventType)) {
            requireFields(payload, "id");
            runtime.getEventService().sendEventBean(new SupportBean_S0(integer(payload, "id")), eventType);
            return;
        }
        if ("SupportBean_S1".equals(eventType)) {
            requireFields(payload, "id");
            runtime.getEventService().sendEventBean(new SupportBean_S1(integer(payload, "id")), eventType);
            return;
        }
        throw new IllegalArgumentException("unsupported event type at step " + stepIndex + ": " + eventType);
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
            int execution = CASE_EXECUTIONS[index];
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIMES[execution].equals(string(definition, "runtimeId"))
                    || !NAMES[execution].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != ITERATOR_SNAPSHOTS[index]
                    || !epl(index).equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != STEP_OPS.length) {
            throw new IllegalArgumentException("scenario must contain exactly forty-two steps");
        }
        int sendIndex = 0;
        int markerIndex = 0;
        for (int index = 0; index < STEP_OPS.length; index++) {
            JsonObject step = object(steps.get(index), "step " + index);
            String operation = string(step, "op");
            if (!STEP_OPS[index].equals(operation)) {
                throw new IllegalArgumentException("step " + index + " operation is " + operation + ", want "
                        + STEP_OPS[index]);
            }
            if ("case".equals(operation)) {
                requireFields(step, "op", "case");
                if (markerIndex >= CASE_MARKERS.length || !CASE_MARKERS[markerIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("case marker is not pinned at step " + index);
                }
                markerIndex++;
            } else {
                validateSend(step, index, SEND_EVENT_TYPES[sendIndex], SEND_STRINGS[sendIndex],
                        SEND_INTS[sendIndex], SEND_LONGS[sendIndex]);
                sendIndex++;
            }
        }
        if (markerIndex != CASE_MARKERS.length || sendIndex != SEND_EVENT_TYPES.length) {
            throw new IllegalArgumentException("scenario step census is not pinned");
        }
    }

    private static void validateSend(JsonObject step, int index, String eventType, String expectedString,
                                     int expectedInt, long expectedLong) {
        requireFields(step, "op", "eventType", "payload");
        if (!eventType.equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send event type is not pinned at step " + index);
        }
        JsonObject payload = object(step.get("payload"), "payload step " + index);
        if ("SupportBean".equals(eventType)) {
            requireFields(payload, "theString", "intPrimitive", "longPrimitive");
            if (!expectedString.equals(string(payload, "theString"))
                    || integer(payload, "intPrimitive") != expectedInt
                    || longNumber(payload, "longPrimitive") != expectedLong) {
                throw new IllegalArgumentException("send payload is not pinned at step " + index);
            }
            return;
        }
        requireFields(payload, "id");
        if (integer(payload, "id") != expectedInt) {
            throw new IllegalArgumentException("send payload is not pinned at step " + index);
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

    private static int integer(JsonObject object, String name) {
        long value = longNumber(object, name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        return (int) value;
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
        private int sendOrdinal;
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
                if (callback > CASE_CALLBACKS[caseIndex]) {
                    throw new IllegalStateException("unexpected listener callback after " + CASE_CALLBACKS[caseIndex]
                            + " callbacks for " + CASES[caseIndex] + " (observed " + callback + " callbacks, "
                            + records.size() + " trace records overall so far)");
                }
                // Every execution of this batch asserts new-only data and the
                // snapshot output never carries a remove stream.
                if (oldEvents != null && oldEvents.length != 0) {
                    throw new IllegalStateException(CASES[caseIndex] + " callback " + callback + " delivered "
                            + oldEvents.length + " old rows");
                }
                long now = runtime.getEventService().getCurrentTime();
                if (now != 0L) {
                    throw new IllegalStateException(CASES[caseIndex] + " callback " + callback + " is at time " + now
                            + ", want 1970-01-01T00:00:00Z");
                }
                EventBean[] delivered;
                if (sendOrdinal == CASE_ASSERTED_TERMINATIONS[caseIndex]) {
                    assertAssertedBatch(newEvents, callback);
                    delivered = newEvents;
                } else if (sendOrdinal == CASE_SENDS[caseIndex]) {
                    delivered = emptyBatchDelivery(newEvents, callback);
                } else {
                    throw new IllegalStateException(CASES[caseIndex] + " listener fired after send " + sendOrdinal
                            + " (callback " + callback + "), want the context terminations at sends "
                            + CASE_ASSERTED_TERMINATIONS[caseIndex] + " and " + CASE_SENDS[caseIndex]);
                }
                callbackCount = callback;
                if (delivered == null) {
                    return;
                }
                sequence++;
                records.add(new JsonObject().add("case", CASES[caseIndex]).add("operation", "listener")
                        .add("statement", statement.getName()).add("sequence", sequence)
                        .add("time", Instant.ofEpochMilli(now).toString()).add("new", rows(delivered)));
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
         * The callback of the asserted context termination, mirroring the Java
         * assertion of the case: assertPropsNew pins exactly one new row,
         * assertPropsPerRowAnyOrder pins row membership, and
         * assertPropsPerRowLastNew pins the exact row order.
         */
        private void assertAssertedBatch(EventBean[] newEvents, int next) {
            String label = CASES[caseIndex] + " callback " + next;
            if (caseIndex == 0) {
                requireRows(newEvents, 1, label);
                assertFields(newEvents[0], RESULT_FIELDS[0]);
                assertTyped(newEvents[0], "c0", FULLY_AGG_UNGROUPED_C0, label);
                return;
            }
            Object[][] expected = caseIndex == 1 ? AGG_UNGROUPED_ROWS
                    : caseIndex == 2 ? FULLY_AGG_GROUPED_ROWS
                    : caseIndex == 3 ? AGG_GROUPED_ROWS : UNGROUPED_ORDER_BY_ROWS;
            requireRows(newEvents, expected.length, label);
            if (caseIndex == 4) {
                String[] fields = RESULT_FIELDS[4];
                for (int rowIndex = 0; rowIndex < expected.length; rowIndex++) {
                    assertFields(newEvents[rowIndex], fields);
                    for (int field = 0; field < fields.length; field++) {
                        assertTyped(newEvents[rowIndex], fields[field], expected[rowIndex][field],
                                label + " row " + (rowIndex + 1));
                    }
                }
                return;
            }
            boolean[] matched = new boolean[expected.length];
            for (EventBean event : newEvents) {
                assertFields(event, RESULT_FIELDS[caseIndex]);
                int hit = -1;
                for (int rowIndex = 0; rowIndex < expected.length && hit < 0; rowIndex++) {
                    if (!matched[rowIndex] && matches(event, expected[rowIndex])) {
                        hit = rowIndex;
                    }
                }
                if (hit < 0) {
                    throw new IllegalStateException(label + " carries a row that matches no pinned row: "
                            + describeRow(event) + ", want one of " + describeExpected(expected));
                }
                matched[hit] = true;
            }
        }

        /**
         * The empty batch is not asserted by any Java execution, so only the
         * result metadata of a delivered row is checked here and the values are
         * recorded exactly as delivered.  A termination that carries no new rows
         * at all is the "nothing delivered" answer the Java executions leave
         * unasserted: it is logged and answers no trace record, so a runtime that
         * emits the far side of the empty batch as an invocation without data is
         * represented by the same trace as one that emits no invocation at all.
         */
        private EventBean[] emptyBatchDelivery(EventBean[] newEvents, int callback) {
            if (newEvents == null || newEvents.length == 0) {
                System.err.println(ID + ": " + CASES[caseIndex] + " empty batch terminated without new rows"
                        + " (callback " + callback + ", no trace record)");
                return null;
            }
            for (EventBean event : newEvents) {
                assertFields(event, RESULT_FIELDS[caseIndex]);
            }
            return newEvents;
        }

        private boolean matches(EventBean event, Object[] expected) {
            String[] fields = RESULT_FIELDS[caseIndex];
            for (int index = 0; index < fields.length; index++) {
                if (!equalsTyped(event.get(fields[index]), expected[index])) {
                    return false;
                }
            }
            return true;
        }

        private void requireRows(EventBean[] newEvents, int expected, String label) {
            if (newEvents == null || newEvents.length != expected) {
                throw new IllegalStateException(label + " must carry exactly " + expected + " new rows, got "
                        + (newEvents == null ? "no new data" : newEvents.length));
            }
        }

        private void assertTyped(EventBean row, String name, Object expected, String label) {
            Object actual = row.get(name);
            if (!equalsTyped(actual, expected)) {
                throw new IllegalStateException(label + " field " + name + " = " + actual + " ("
                        + simpleName(actual) + "), want " + expected + " (" + simpleName(expected) + ")");
            }
        }

        private static boolean equalsTyped(Object actual, Object expected) {
            if (expected == null) {
                return actual == null;
            }
            return expected.getClass().isInstance(actual) && expected.equals(actual);
        }

        private String describeRow(EventBean event) {
            String[] fields = RESULT_FIELDS[caseIndex];
            List<String> rendered = new ArrayList<>();
            for (String field : fields) {
                Object value = event.get(field);
                rendered.add(field + "=" + value + " (" + simpleName(value) + ")");
            }
            return rendered.toString();
        }

        private String describeExpected(Object[][] expected) {
            String[] fields = RESULT_FIELDS[caseIndex];
            List<String> rendered = new ArrayList<>();
            for (Object[] row : expected) {
                List<String> values = new ArrayList<>();
                for (int index = 0; index < fields.length; index++) {
                    values.add(fields[index] + "=" + row[index] + " (" + simpleName(row[index]) + ")");
                }
                rendered.add(values.toString());
            }
            return rendered.toString();
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
}
