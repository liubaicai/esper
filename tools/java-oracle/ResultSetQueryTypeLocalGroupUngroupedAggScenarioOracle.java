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
 * Direct Esper 9.0.0 oracle for ResultSetQueryTypeLocalGroupBy ordinals 0/1/2/7,
 * the ungrouped local-group aggregate executions:
 * ResultSetLocalUngroupedSumSimple (zero-time advance plus five sends of the
 * four-column sum surface), ResultSetLocalUngroupedAggSQLStandard (the fifteen
 * SQL-standard aggregate selections), ResultSetLocalUngroupedAggEvent (the
 * twenty-one event-valued local aggregates over length(3)) and
 * ResultSetLocalUngroupedHaving (the local-group HAVING filter).
 *
 * All four Java executions pin new-only semantics through assertPropsNew: each
 * callback must carry exactly one new row and no old rows, so the listener here
 * throws on any callback that is not a single-new-row delivery and on any old
 * row.  Every case owns its runtime, deployment and listener, the runtime's
 * internal timer is disabled, and the clock stays at 1970-01-01T00:00:00Z for
 * every record.
 *
 * The event-valued case pins aggregate identity against the very SupportBean
 * instances that were sent (first/last/maxby/minby/maxbyever/minbyever/
 * firstever/lastever return the event itself, window returns the group's events
 * in insertion order and sorted returns them ascending by intPrimitive), and
 * renders every event-valued column as the shared twenty-field SupportBean row
 * shape.  The HAVING case pins that the listener stays silent for the first two
 * sends and fires once on the third send, with the row being that third
 * SupportBean.
 */
public final class ResultSetQueryTypeLocalGroupUngroupedAggScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-querytype-local-group-ungrouped-agg";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java";
    private static final String DESCRIPTION =
            "ResultSetQueryTypeLocalGroupBy ordinals 0/1/2/7: ungrouped local-group sums, SQL-standard"
                    + " aggregate selections, event-valued local aggregates and local-group HAVING.";
    private static final String[] CASES = {
            "ungrouped-sum-simple", "ungrouped-agg-sql-standard", "ungrouped-agg-event", "ungrouped-having"
    };
    private static final int[] ORDINALS = {0, 1, 2, 7};
    private static final String[] RUNTIMES = {
            "java-runtime-a40ad8ec1c03959b5f33", "java-runtime-6bec44d03e954b1cd52c",
            "java-runtime-9ad9539a7e8f5581b192", "java-runtime-1dc3599a7c07036be603"
    };
    private static final String[] NAMES = {
            "ResultSetLocalUngroupedSumSimple", "ResultSetLocalUngroupedAggSQLStandard",
            "ResultSetLocalUngroupedAggEvent", "ResultSetLocalUngroupedHaving"
    };
    private static final String[] OBSERVATIONS = {"listener", "listener", "listener", "listener"};
    private static final int[] ITERATOR_SNAPSHOTS = {0, 0, 0, 0};
    private static final int[] CASE_RECORDS = {5, 4, 4, 1};
    private static final int[] CASE_SENDS = {5, 4, 4, 3};
    private static final int[] CASE_ADVANCES = {1, 0, 0, 0};
    private static final String[] STATIC_IDS = {
            "java-3feeb6f7d6769e71aad1", "java-d2a1aaee0aaf06dee915",
            "java-79a9c055e0f90284c89e", "java-ee7ae61064ad0a2279ed"
    };
    private static final String EPL_SIMPLE = "@Name('s0') select sum(longPrimitive,"
            + " group_by:(theString, intPrimitive)) as c0, sum(longPrimitive, group_by:(theString)) as c1,"
            + " sum(longPrimitive, group_by:(intPrimitive)) as c2, sum(longPrimitive) as c3 from SupportBean";
    private static final String EPL_SQL = "@name('s0') select intPrimitive as c0, sum(intPrimitive,"
            + " group_by:()) as sum0, sum(intPrimitive, group_by:(theString)) as sum1,avedev(intPrimitive,"
            + " group_by:(theString)) as avedev0,avg(intPrimitive, group_by:(theString)) as avg0,max(intPrimitive,"
            + " group_by:(theString)) as max0,fmax(intPrimitive, intPrimitive>0, group_by:(theString)) as fmax0,"
            + "min(intPrimitive, group_by:(theString)) as min0,fmin(intPrimitive, intPrimitive>0,"
            + " group_by:(theString)) as fmin0,maxever(intPrimitive, group_by:(theString)) as maxever0,"
            + "fmaxever(intPrimitive, intPrimitive>0, group_by:(theString)) as fmaxever0,minever(intPrimitive,"
            + " group_by:(theString)) as minever0,fminever(intPrimitive, intPrimitive>0, group_by:(theString))"
            + " as fminever0,median(intPrimitive, group_by:(theString)) as median0,Math.round(coalesce(stddev"
            + "(intPrimitive, group_by:(theString)), 0)) as stddev0 from SupportBean#keepall";
    private static final String EPL_EVENT = "@name('s0') select intPrimitive as c0, first(sb,"
            + " group_by:(theString)) as first0, first(sb, group_by:()) as first1, last(sb,"
            + " group_by:(theString)) as last0, last(sb, group_by:()) as last1, window(sb,"
            + " group_by:(theString)) as window0, window(sb, group_by:()) as window1, maxby(intPrimitive,"
            + " group_by:(theString)) as maxby0, maxby(intPrimitive, group_by:()) as maxby1,"
            + " minby(intPrimitive, group_by:(theString)) as minby0, minby(intPrimitive, group_by:()) as minby1,"
            + " sorted(intPrimitive, group_by:(theString)) as sorted0, sorted(intPrimitive, group_by:()) as"
            + " sorted1, maxbyever(intPrimitive, group_by:(theString)) as maxbyever0, maxbyever(intPrimitive,"
            + " group_by:()) as maxbyever1, minbyever(intPrimitive, group_by:(theString)) as minbyever0,"
            + " minbyever(intPrimitive, group_by:()) as minbyever1, firstever(sb, group_by:(theString)) as"
            + " firstever0, firstever(sb, group_by:()) as firstever1, lastever(sb, group_by:(theString)) as"
            + " lastever0, lastever(sb, group_by:()) as lastever1 from SupportBean#length(3) as sb";
    private static final String EPL_HAVING =
            "@name('s0') select * from SupportBean having sum(intPrimitive, group_by:theString) > 100";
    private static final String[] STEP_OPS = {
            "case", "advance-time", "send", "send", "send", "send", "send",
            "case", "send", "send", "send", "send",
            "case", "send", "send", "send", "send",
            "case", "send", "send", "send"
    };
    private static final String[] CASE_MARKERS = {
            "ungrouped-sum-simple", "ungrouped-agg-sql-standard", "ungrouped-agg-event", "ungrouped-having"
    };
    private static final String[] ADVANCE_ATS = {"1970-01-01T00:00:00Z"};
    private static final String[] SEND_STRINGS = {
            "E1", "E2", "E1", "E1", "E2",
            "E1", "E2", "E1", "E2",
            "E1", "E2", "E1", "E3",
            "E1", "E2", "E1"
    };
    private static final int[] SEND_INTS = {
            1, 2, 2, 1, 1,
            10, 20, 30, 40,
            10, 20, 15, 16,
            95, 10, 10
    };
    private static final long[] SEND_LONGS = {
            10, 11, 12, 13, 14,
            0, 0, 0, 0,
            0, 0, 0, 0,
            0, 0, 0
    };
    private static final String[] SUM_FIELDS = {"c0", "c1", "c2", "c3"};
    // The five ungrouped-sum-simple rows, exactly as the Java execution's
    // assertPropsNew pins them; sum(longPrimitive) aggregates stay Long.
    private static final long[][] SUM_ROWS = {
            {10, 10, 10, 10},
            {11, 11, 11, 21},
            {12, 22, 23, 33},
            {23, 35, 23, 46},
            {14, 25, 37, 60},
    };
    private static final String[] SQL_FIELDS = {
            "c0", "sum0", "sum1", "avedev0", "avg0", "max0", "fmax0", "min0", "fmin0", "maxever0", "fmaxever0",
            "minever0", "fminever0", "median0", "stddev0"
    };
    // The four ungrouped-agg-sql-standard rows in Java selection order: c0 and
    // the intPrimitive aggregates stay Integer, avedev/avg/median are Double and
    // stddev0 is the Math.round Long the Java execution pins.
    private static final Object[][] SQL_ROWS = {
            {10, 10, 10, 0.0d, 10.0d, 10, 10, 10, 10, 10, 10, 10, 10, 10.0d, 0L},
            {20, 30, 20, 0.0d, 20.0d, 20, 20, 20, 20, 20, 20, 20, 20, 20.0d, 0L},
            {30, 60, 40, 10.0d, 20.0d, 30, 30, 10, 10, 30, 30, 10, 10, 20.0d, 14L},
            {40, 100, 60, 10.0d, 30.0d, 40, 40, 20, 20, 40, 40, 20, 20, 30.0d, 14L},
    };
    private static final String[] EVENT_FIELDS = {
            "c0", "first0", "first1", "last0", "last1", "window0", "window1", "maxby0", "maxby1", "minby0",
            "minby1", "sorted0", "sorted1", "maxbyever0", "maxbyever1", "minbyever0", "minbyever1", "firstever0",
            "firstever1", "lastever0", "lastever1"
    };
    private static final String[] EVENT_SCALAR_FIELDS = {
            "first0", "first1", "last0", "last1", "maxby0", "maxby1", "minby0", "minby1", "maxbyever0", "maxbyever1",
            "minbyever0", "minbyever1", "firstever0", "firstever1", "lastever0", "lastever1"
    };
    // Each scalar event column as an index into the four sent SupportBeans, in
    // the order the Java execution asserts them.
    private static final int[][] EVENT_SCALAR_INDEXES = {
            {0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
            {1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 1, 0, 1, 0, 1, 1},
            {0, 0, 2, 2, 2, 1, 0, 0, 2, 1, 0, 0, 0, 0, 2, 2},
            {3, 1, 3, 3, 3, 1, 3, 2, 3, 1, 3, 0, 3, 0, 3, 3},
    };
    private static final String[] EVENT_ARRAY_FIELDS = {"sorted0", "sorted1", "window0", "window1"};
    // The event-array columns as index lists into the sent SupportBeans: window
    // keeps the group's insertion order while sorted is ascending by
    // intPrimitive, both exactly as the Java execution asserts them.
    private static final int[][][] EVENT_ARRAY_INDEXES = {
            {{0}, {0}, {0}, {0}},
            {{1}, {0, 1}, {1}, {0, 1}},
            {{0, 2}, {0, 2, 1}, {0, 2}, {0, 1, 2}},
            {{3}, {2, 3, 1}, {3}, {1, 2, 3}},
    };
    private static final int[] EVENT_C0 = {10, 20, 15, 16};
    private static final String[] BEAN_FIELDS = {
            "bigDecimal", "bigInteger", "boolBoxed", "boolPrimitive", "byteBoxed", "bytePrimitive",
            "charBoxed", "charPrimitive", "doubleBoxed", "doublePrimitive", "enumValue", "floatBoxed",
            "floatPrimitive", "intBoxed", "intPrimitive", "longBoxed", "longPrimitive", "shortBoxed",
            "shortPrimitive", "theString"
    };
    // The ungrouped-having row is the raw third-send SupportBean; the Java
    // execution asserts no values there, so the twenty bean properties are
    // pinned here as the primitive/boxed defaults of new SupportBean("E1", 10).
    private static final Object[] HAVING_VALUES = {
            null, null, null, Boolean.FALSE, null, Byte.valueOf((byte) 0), null, Character.valueOf('\u0000'),
            null, Double.valueOf(0d), null, null, Float.valueOf(0f), null, Integer.valueOf(10), null,
            Long.valueOf(0L), null, Short.valueOf((short) 0), "E1"
    };

    private ResultSetQueryTypeLocalGroupUngroupedAggScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetQueryTypeLocalGroupUngroupedAggScenarioOracle <scenario.json>");
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
        if (records.size() != 14) {
            throw new IllegalStateException("expected fourteen trace records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, JsonArray records, int index) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + RUNTIMES[index], configuration);
        runtime.getEventService().advanceTime(0L);
        int before = records.size();
        try {
            EPStatement statement = deploy(runtime, configuration, index);
            SupportBean[] sent = new SupportBean[CASE_SENDS[index]];
            ListenerWriter writer = new ListenerWriter(records, statement, runtime, sent, index);
            statement.addListener(writer);
            boolean active = false;
            int markers = 0;
            int sends = 0;
            int advances = 0;
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
                if ("send".equals(operation)) {
                    if (sends >= sent.length) {
                        throw new IllegalArgumentException("unexpected send at step " + stepIndex);
                    }
                    SupportBean bean = bean(step, stepIndex);
                    sent[sends] = bean;
                    // Any callback this send produces arrives synchronously.
                    writer.sendOrdinal = sends + 1;
                    sends++;
                    runtime.getEventService().sendEventBean(bean, "SupportBean");
                } else if ("advance-time".equals(operation)) {
                    advances++;
                    advanceTime(runtime, step);
                } else {
                    throw new IllegalArgumentException("unsupported operation at step " + stepIndex + ": " + operation);
                }
            }
            if (markers != 1) {
                throw new IllegalStateException("case marker count mismatch for " + CASES[index]);
            }
            if (sends != CASE_SENDS[index] || advances != CASE_ADVANCES[index]) {
                throw new IllegalStateException("expected " + CASE_SENDS[index] + " sends and "
                        + CASE_ADVANCES[index] + " advance-time steps for " + CASES[index] + ", got " + sends
                        + " and " + advances);
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
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static EPStatement deploy(EPRuntime runtime, Configuration configuration, int index) throws Exception {
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl(index),
                new CompilerArguments(runtime.getRuntimePath()));
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                new DeploymentOptions().setDeploymentId(ID + "-" + CASES[index]));
        return findStatement(deployment);
    }

    private static String epl(int index) {
        if (index == 0) {
            return EPL_SIMPLE;
        }
        if (index == 1) {
            return EPL_SQL;
        }
        return index == 2 ? EPL_EVENT : EPL_HAVING;
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        EPStatement[] statements = deployment.getStatements();
        if (statements == null || statements.length != 1 || !"s0".equals(statements[0].getName())) {
            throw new IllegalStateException("expected exactly one statement named s0");
        }
        return statements[0];
    }

    private static SupportBean bean(JsonObject step, int stepIndex) {
        requireFields(step, "op", "eventType", "payload");
        if (!"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("unexpected event type at step " + stepIndex);
        }
        JsonObject payload = object(step.get("payload"), "payload step " + stepIndex);
        requireFields(payload, "theString", "intPrimitive", "longPrimitive");
        SupportBean bean = new SupportBean(string(payload, "theString"), integer(payload, "intPrimitive"));
        bean.setLongPrimitive(longNumber(payload, "longPrimitive"));
        return bean;
    }

    private static void advanceTime(EPRuntime runtime, JsonObject step) {
        requireFields(step, "op", "at");
        Instant instant = Instant.parse(string(step, "at"));
        runtime.getEventService().advanceTime(instant.toEpochMilli());
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
            throw new IllegalArgumentException("expected four cases");
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
        if (steps.size() != STEP_OPS.length) {
            throw new IllegalArgumentException("scenario must contain exactly twenty-one steps");
        }
        int sendIndex = 0;
        int advanceIndex = 0;
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
            } else if ("advance-time".equals(operation)) {
                requireFields(step, "op", "at");
                if (advanceIndex >= ADVANCE_ATS.length || !ADVANCE_ATS[advanceIndex].equals(string(step, "at"))) {
                    throw new IllegalArgumentException("advance-time step is not pinned at step " + index);
                }
                advanceIndex++;
            } else {
                validateSend(step, index, SEND_STRINGS[sendIndex], SEND_INTS[sendIndex], SEND_LONGS[sendIndex]);
                sendIndex++;
            }
        }
        if (markerIndex != CASE_MARKERS.length || sendIndex != SEND_STRINGS.length
                || advanceIndex != ADVANCE_ATS.length) {
            throw new IllegalArgumentException("scenario step census is not pinned");
        }
    }

    private static void validateSend(JsonObject step, int index, String expectedString,
                                     int expectedInt, long expectedLong) {
        requireFields(step, "op", "eventType", "payload");
        if (!"SupportBean".equals(string(step, "eventType"))) {
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
        for (EventBean event : events) {
            output.add(row(event));
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
        private final SupportBean[] sent;
        private final int caseIndex;
        private int sequence;
        private int sendOrdinal;

        private ListenerWriter(JsonArray records, EPStatement statement, EPRuntime runtime, SupportBean[] sent,
                               int caseIndex) {
            this.records = records;
            this.statement = statement;
            this.runtime = runtime;
            this.sent = sent;
            this.caseIndex = caseIndex;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            try {
                int next = sequence + 1;
                if (next > CASE_RECORDS[caseIndex]) {
                    throw new IllegalStateException("unexpected listener callback after " + CASE_RECORDS[caseIndex]
                            + " records for " + CASES[caseIndex]);
                }
                // Every Java execution of this batch asserts new-only semantics
                // through assertPropsNew: one new row, never an old row.
                if (newEvents == null || newEvents.length != 1) {
                    throw new IllegalStateException(CASES[caseIndex] + " callback " + next
                            + " must carry exactly one new row");
                }
                if (oldEvents != null && oldEvents.length != 0) {
                    throw new IllegalStateException(CASES[caseIndex] + " callback " + next + " delivered "
                            + oldEvents.length + " old rows");
                }
                long now = runtime.getEventService().getCurrentTime();
                if (now != 0L) {
                    throw new IllegalStateException(CASES[caseIndex] + " callback " + next + " is at time " + now
                            + ", want 1970-01-01T00:00:00Z");
                }
                if (caseIndex == 0) {
                    assertSumRow(newEvents[0], next);
                } else if (caseIndex == 1) {
                    assertSqlRow(newEvents[0], next);
                } else if (caseIndex == 2) {
                    assertEventRow(newEvents[0], next);
                } else {
                    assertHavingRow(newEvents[0], next);
                }
                sequence = next;
                records.add(new JsonObject().add("case", CASES[caseIndex]).add("operation", "listener")
                        .add("statement", statement.getName()).add("sequence", sequence)
                        .add("time", Instant.ofEpochMilli(now).toString()).add("new", rows(newEvents)));
            } catch (RuntimeException ex) {
                // The runtime swallows listener exceptions, so surface the
                // failing assertion before it is lost.
                ex.printStackTrace(System.err);
                throw ex;
            }
        }

        private void assertSumRow(EventBean row, int sequence) {
            assertResultFields(row, SUM_FIELDS);
            long[] expected = SUM_ROWS[sequence - 1];
            for (int index = 0; index < SUM_FIELDS.length; index++) {
                assertTyped(row, SUM_FIELDS[index], expected[index], CASES[0]);
            }
        }

        private void assertSqlRow(EventBean row, int sequence) {
            assertResultFields(row, SQL_FIELDS);
            Object[] expected = SQL_ROWS[sequence - 1];
            for (int index = 0; index < SQL_FIELDS.length; index++) {
                assertTyped(row, SQL_FIELDS[index], expected[index], CASES[1]);
            }
        }

        private void assertEventRow(EventBean row, int sequence) {
            assertResultFields(row, EVENT_FIELDS);
            assertTyped(row, "c0", EVENT_C0[sequence - 1], CASES[2]);
            for (int index = 0; index < EVENT_SCALAR_FIELDS.length; index++) {
                assertSentBean(row.get(EVENT_SCALAR_FIELDS[index]), EVENT_SCALAR_INDEXES[sequence - 1][index],
                        CASES[2] + " field " + EVENT_SCALAR_FIELDS[index]);
            }
            for (int index = 0; index < EVENT_ARRAY_FIELDS.length; index++) {
                Object value = row.get(EVENT_ARRAY_FIELDS[index]);
                Object[] actual;
                if (value instanceof EventBean[] events) {
                    actual = events;
                } else if (value instanceof Object[] objects) {
                    actual = objects;
                } else {
                    throw new IllegalStateException(CASES[2] + " field " + EVENT_ARRAY_FIELDS[index]
                            + " is not an event array: " + value);
                }
                int[] expected = EVENT_ARRAY_INDEXES[sequence - 1][index];
                if (actual.length != expected.length) {
                    throw new IllegalStateException(CASES[2] + " field " + EVENT_ARRAY_FIELDS[index] + " has "
                            + actual.length + " events, want " + expected.length);
                }
                for (int item = 0; item < expected.length; item++) {
                    assertSentBean(actual[item], expected[item], CASES[2] + " field " + EVENT_ARRAY_FIELDS[index]
                            + "[" + item + "]");
                }
            }
        }

        /**
         * The HAVING execution pins that the listener stays silent for the
         * first two sends and is invoked on the third, so any callback outside
         * that third send fails the run.
         */
        private void assertHavingRow(EventBean row, int sequence) {
            if (sendOrdinal != CASE_SENDS[3] || sequence != 1) {
                throw new IllegalStateException(CASES[3] + " listener fired after send " + sendOrdinal
                        + " as record " + sequence + ", want the single record of send " + CASE_SENDS[3]);
            }
            assertSentBean(row, CASE_SENDS[3] - 1, CASES[3] + " row");
            assertResultFields(row, BEAN_FIELDS);
            for (int index = 0; index < BEAN_FIELDS.length; index++) {
                assertTyped(row, BEAN_FIELDS[index], HAVING_VALUES[index], CASES[3]);
            }
        }

        private void assertSentBean(Object value, int expectedIndex, String label) {
            SupportBean expected = sent[expectedIndex];
            if (value instanceof EventBean event) {
                if (event.getUnderlying() != expected) {
                    throw new IllegalStateException(label + " is not send " + (expectedIndex + 1)
                            + " of " + CASES[caseIndex]);
                }
                assertResultFields(event, BEAN_FIELDS);
                return;
            }
            if (value != expected) {
                throw new IllegalStateException(label + " is not send " + (expectedIndex + 1)
                        + " of " + CASES[caseIndex] + ": " + value);
            }
        }

        private void assertTyped(EventBean row, String name, Object expected, String label) {
            Object actual = row.get(name);
            if (expected == null) {
                if (actual != null) {
                    throw new IllegalStateException(label + " field " + name + " = " + actual + " ("
                            + simpleName(actual) + "), want null");
                }
                return;
            }
            if (!expected.getClass().isInstance(actual) || !expected.equals(actual)) {
                throw new IllegalStateException(label + " field " + name + " = " + actual + " ("
                        + simpleName(actual) + "), want " + expected + " (" + simpleName(expected) + ")");
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
        private static void assertResultFields(EventBean row, String[] expected) {
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
