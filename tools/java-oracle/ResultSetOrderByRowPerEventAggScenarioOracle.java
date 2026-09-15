import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportMarketDataBean;
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
 * Direct Esper 9.0.0 oracle for ResultSetOrderByRowPerEvent ordinals 3 and 5:
 * {@code ResultSetRowPerEventOrderFunction} and {@code ResultSetRowPerEventMaxSum}.
 *
 * <p>Each case runs one runtime with the internal timer disabled, advances
 * virtual time to zero, compiles and deploys one statement named s0, attaches
 * exactly one listener, sends the six pinned {@code SupportMarketDataBean}
 * events (volume 0L, feed null) and undeploys.  Because {@code output every 6
 * events} is the only output policy, the listener fires exactly once per case
 * - at the sixth send - carrying exactly six new rows and no old rows, and no
 * virtual time beyond the initial advance to zero is involved.
 *
 * <p>Case {@code order-function} selects {@code symbol, sum(price)} ordered by
 * {@code volume*sum(price), symbol}.  Every row carries the symbol of its own
 * event and the ungrouped {@code sum(price)} over the {@code #length(10)}
 * window as of that event, so the running window sums are 2, 3, 6, 12, 18, 23
 * for IBM(2), KGB(1), CMU(3), IBM(6), CAT(6), CAT(5).  The order key
 * {@code volume*sum(price)} collapses to 0.0 for every row because volume is
 * 0L, so {@code symbol} ascending decides: CAT, CAT, CMU, IBM, IBM, KGB.  The
 * two CAT rows share the full key (0.0, "CAT"); Java's object sort is stable,
 * so their engine delivery order preserves send order: sums 18.0 then 23.0.
 *
 * <p>Case {@code max-sum} selects {@code symbol, max(sum(price))} ordered by
 * {@code symbol}.  The nested {@code max(sum(price))} per row is the
 * historical-prefix maximum of the running window sum; the running sums 3, 7,
 * 8, 10, 15, 21 are monotone, so the prefix maximum equals the running sum at
 * that row's own event, and symbol order gives CAT, CAT, CMU, CMU, IBM, IBM
 * with values 15.0, 21.0, 8.0, 10.0, 3.0, 7.0 - exactly what the Java
 * {@code assertPropsPerRowNewOnly} call pins.
 *
 * <p>The Java executions assert the delivered row order exactly (not any
 * order), and the order-by makes delivery deterministic, so the trace records
 * rows in engine delivery order without re-sorting.
 *
 * <p>Both aggregated columns are Java {@code Double}: {@code price} is a
 * primitive double and {@code sum(price)} and {@code max(sum(price))} return
 * Double.  The trace emits them as JSON numbers via minimaljson's
 * {@code Json.value(double)}, which writes the shortest round-trip spelling of
 * {@code Double.toString} (for example {@code 18.0}).  Nothing in these
 * executions carries a remove stream, so no record has an {@code old} key.
 */
public final class ResultSetOrderByRowPerEventAggScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "orderby-rowperevent-agg";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderByRowPerEvent.java";
    private static final String DESCRIPTION =
            "ResultSetOrderByRowPerEvent ordinals 3 and 5: ungrouped row-per-event aggregates ordered by an order-by"
                    + " key that mixes the event's volume with sum(price), and the nested max(sum(price))"
                    + " historical-prefix aggregate, both delivered once per six events over a length(10) window.";
    private static final String OBSERVATION = "listener";
    private static final int ITERATOR_SNAPSHOTS = 0;
    private static final int CASE_SENDS = 6;
    private static final int CASE_RECORDS = 1;
    private static final int TOTAL_RECORDS = 2;

    private static final CaseDef[] CASES = {
            new CaseDef("order-function", 3, "java-runtime-e5b38082f9b30ce8a13b",
                    "ResultSetRowPerEventOrderFunction",
                    "@name('s0') select symbol, sum(price) from SupportMarketDataBean#length(10)"
                            + " output every 6 events order by volume*sum(price), symbol",
                    new String[]{"sum(price)", "symbol"},
                    new String[]{"IBM", "KGB", "CMU", "IBM", "CAT", "CAT"},
                    new double[]{2, 1, 3, 6, 6, 5},
                    new String[]{"CAT", "CAT", "CMU", "IBM", "IBM", "KGB"},
                    new double[]{18, 23, 6, 2, 12, 3}),
            new CaseDef("max-sum", 5, "java-runtime-1be3cb0efcdb94f20d5f",
                    "ResultSetRowPerEventMaxSum",
                    "@name('s0') select symbol, max(sum(price)) from SupportMarketDataBean#length(10)"
                            + " output every 6 events order by symbol",
                    new String[]{"max(sum(price))", "symbol"},
                    new String[]{"IBM", "IBM", "CMU", "CMU", "CAT", "CAT"},
                    new double[]{3, 4, 1, 2, 5, 6},
                    new String[]{"CAT", "CAT", "CMU", "CMU", "IBM", "IBM"},
                    new double[]{15, 21, 8, 10, 3, 7}),
    };

    private static final String[] STATIC_IDS = {
            "java-f583dbb6e793f0fae373", "java-55bd91f3bd2eecde7acf"
    };

    private static final String[] STEP_OPS = {
            "case",
            "send", "send", "send", "send", "send", "send",
            "case",
            "send", "send", "send", "send", "send", "send"
    };

    private ResultSetOrderByRowPerEventAggScenarioOracle() {
    }

    private static final class CaseDef {
        final String name;
        final int ordinal;
        final String runtimeId;
        final String executionName;
        final String epl;
        // Result metadata of the one statement, sorted by the runtime.
        final String[] resultFields;
        final String valueField;
        final String[] sendSymbols;
        final double[] sendPrices;
        // Engine delivery order: the order-by makes it deterministic.
        final String[] rowSymbols;
        final double[] rowValues;

        CaseDef(String name, int ordinal, String runtimeId, String executionName, String epl,
                String[] resultFields, String[] sendSymbols, double[] sendPrices,
                String[] rowSymbols, double[] rowValues) {
            this.name = name;
            this.ordinal = ordinal;
            this.runtimeId = runtimeId;
            this.executionName = executionName;
            this.epl = epl;
            this.resultFields = resultFields;
            this.valueField = resultFields[0];
            this.sendSymbols = sendSymbols;
            this.sendPrices = sendPrices;
            this.rowSymbols = rowSymbols;
            this.rowValues = rowValues;
        }
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOrderByRowPerEventAggScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        for (CaseDef def : CASES) {
            runCase(def, scenario, records);
        }
        if (records.size() != TOTAL_RECORDS) {
            throw new IllegalStateException("expected " + TOTAL_RECORDS + " trace records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(CaseDef def, JsonObject scenario, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportMarketDataBean.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + def.runtimeId, configuration);
        try {
            runtime.getEventService().advanceTime(0L);
            EPStatement statement = deploy(runtime, def);
            ListenerWriter writer = new ListenerWriter(records, statement, runtime, def);
            statement.addListener(writer);

            JsonArray steps = scenario.get("steps").asArray();
            boolean active = false;
            int markers = 0;
            int sends = 0;
            for (int stepIndex = 0; stepIndex < steps.size(); stepIndex++) {
                JsonObject step = object(steps.get(stepIndex), "step " + stepIndex);
                String operation = string(step, "op");
                if ("case".equals(operation)) {
                    if (def.name.equals(string(step, "case"))) {
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
                if (sends >= CASE_SENDS) {
                    throw new IllegalArgumentException("unexpected send at step " + stepIndex);
                }
                SupportMarketDataBean bean = bean(step, stepIndex, def, sends);
                sends++;
                runtime.getEventService().sendEventBean(bean, "SupportMarketDataBean");
            }
            if (markers != 1) {
                throw new IllegalStateException("case marker count mismatch for " + def.name);
            }
            if (sends != CASE_SENDS) {
                throw new IllegalStateException("expected " + CASE_SENDS + " sends for " + def.name
                        + ", got " + sends);
            }
            if (writer.sequence != CASE_RECORDS) {
                throw new IllegalStateException("expected " + CASE_RECORDS + " listener record for "
                        + def.name + ", got " + writer.sequence);
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static EPStatement deploy(EPRuntime runtime, CaseDef def) throws Exception {
        EPDeployment deployment = runtime.getDeploymentService().deploy(
                EPCompilerProvider.getCompiler().compile(def.epl, new CompilerArguments(runtime.getRuntimePath())),
                new DeploymentOptions().setDeploymentId(ID + "-" + def.name));
        EPStatement[] statements = deployment.getStatements();
        if (statements == null || statements.length != 1 || !"s0".equals(statements[0].getName())) {
            throw new IllegalStateException("expected exactly one statement named s0");
        }
        return statements[0];
    }

    private static SupportMarketDataBean bean(JsonObject step, int stepIndex, CaseDef def, int sendIndex) {
        requireFields(step, "op", "eventType", "payload");
        if (!"SupportMarketDataBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("unexpected event type at step " + stepIndex);
        }
        JsonObject payload = object(step.get("payload"), "payload step " + stepIndex);
        requireFields(payload, "symbol", "price", "volume");
        String symbol = string(payload, "symbol");
        if (!def.sendSymbols[sendIndex].equals(symbol)
                || Double.compare(doubleNumber(payload, "price"), def.sendPrices[sendIndex]) != 0
                || longNumber(payload, "volume") != 0L) {
            throw new IllegalArgumentException("send payload is not pinned at step " + stepIndex);
        }
        return new SupportMarketDataBean(symbol, def.sendPrices[sendIndex], 0L, null);
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
        validateStringArray(scenario.get("javaRuntimes"), new String[]{
                CASES[0].runtimeId, CASES[1].runtimeId}, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), new String[]{
                CASES[0].executionName, CASES[1].executionName}, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("expected exactly " + CASES.length + " cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            CaseDef def = CASES[index];
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation",
                    "iteratorSnapshots", "epl");
            if (!def.name.equals(string(definition, "case"))
                    || integer(definition, "ordinal") != def.ordinal
                    || !def.runtimeId.equals(string(definition, "runtimeId"))
                    || !def.executionName.equals(string(definition, "executionName"))
                    || !OBSERVATION.equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != ITERATOR_SNAPSHOTS
                    || !def.epl.equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != STEP_OPS.length) {
            throw new IllegalArgumentException("scenario must contain exactly " + STEP_OPS.length + " steps");
        }
        int caseIndex = -1;
        int sendIndex = 0;
        for (int index = 0; index < STEP_OPS.length; index++) {
            JsonObject step = object(steps.get(index), "step " + index);
            String operation = string(step, "op");
            if (!STEP_OPS[index].equals(operation)) {
                throw new IllegalArgumentException("step " + index + " operation is " + operation + ", want "
                        + STEP_OPS[index]);
            }
            if ("case".equals(operation)) {
                requireFields(step, "op", "case");
                int next = caseIndex + 1;
                if (next >= CASES.length || !CASES[next].name.equals(string(step, "case"))) {
                    throw new IllegalArgumentException("case marker is not pinned at step " + index);
                }
                caseIndex = next;
                sendIndex = 0;
            } else {
                if (caseIndex < 0) {
                    throw new IllegalArgumentException("send before any case marker at step " + index);
                }
                validateSend(step, index, CASES[caseIndex], sendIndex);
                sendIndex++;
            }
        }
        if (caseIndex != CASES.length - 1 || sendIndex != CASE_SENDS) {
            throw new IllegalArgumentException("scenario step census is not pinned");
        }
    }

    private static void validateSend(JsonObject step, int index, CaseDef def, int sendIndex) {
        requireFields(step, "op", "eventType", "payload");
        if (!"SupportMarketDataBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send event type is not pinned at step " + index);
        }
        JsonObject payload = object(step.get("payload"), "payload step " + index);
        requireFields(payload, "symbol", "price", "volume");
        if (!def.sendSymbols[sendIndex].equals(string(payload, "symbol"))
                || Double.compare(doubleNumber(payload, "price"), def.sendPrices[sendIndex]) != 0
                || longNumber(payload, "volume") != 0L) {
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

    private static double doubleNumber(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be a JSON number");
        }
        String text = value.toString();
        if (!text.matches("-?(0|[1-9][0-9]*)(\\.[0-9]+)?")) {
            throw new IllegalArgumentException(name + " must be a plain decimal JSON number");
        }
        return Double.parseDouble(text);
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
        private final CaseDef def;
        private int sequence;

        private ListenerWriter(JsonArray records, EPStatement statement, EPRuntime runtime, CaseDef def) {
            this.records = records;
            this.statement = statement;
            this.runtime = runtime;
            this.def = def;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            int next = sequence + 1;
            try {
                if (next > CASE_RECORDS) {
                    throw new IllegalStateException("unexpected listener callback " + next + " for " + def.name);
                }
                if (newEvents == null || newEvents.length != def.rowSymbols.length) {
                    throw new IllegalStateException(def.name + " callback " + next + " must carry exactly "
                            + def.rowSymbols.length + " new rows, got "
                            + (newEvents == null ? "no new data" : Integer.toString(newEvents.length)));
                }
                if (oldEvents != null && oldEvents.length != 0) {
                    throw new IllegalStateException(def.name + " callback " + next + " delivered "
                            + oldEvents.length + " old rows");
                }
                long now = runtime.getEventService().getCurrentTime();
                if (now != 0L) {
                    throw new IllegalStateException(def.name + " callback " + next + " is at " + now + ", want 0");
                }
                assertBatch(newEvents, next);
                sequence = next;
                records.add(new JsonObject().add("case", def.name).add("operation", "listener")
                        .add("statement", statement.getName()).add("sequence", sequence)
                        .add("time", Instant.ofEpochMilli(now).toString())
                        .add("new", rows(newEvents)));
            } catch (RuntimeException ex) {
                // The runtime swallows listener exceptions, so surface the
                // failing assertion before it is lost.
                ex.printStackTrace(System.err);
                throw ex;
            }
        }

        /**
         * Mirrors the Java assertPropsPerRowNewOnly of the execution: every
         * row carries the pinned result metadata and the batch contains the
         * pinned (symbol, aggregate) pairs in the exact delivered order.
         */
        private void assertBatch(EventBean[] batch, int callback) {
            for (EventBean row : batch) {
                String[] names = row.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                if (!Arrays.equals(names, def.resultFields)) {
                    throw new IllegalStateException(def.name + " callback " + callback
                            + " result field metadata is not pinned: " + Arrays.toString(names));
                }
            }
            for (int index = 0; index < batch.length; index++) {
                assertRow(batch[index], index, callback);
            }
        }

        private void assertRow(EventBean row, int index, int callback) {
            if (!def.rowSymbols[index].equals(row.get("symbol"))) {
                throw new IllegalStateException(def.name + " callback " + callback + " row " + index
                        + " symbol is " + row.get("symbol") + ", want " + def.rowSymbols[index]);
            }
            Object value = row.get(def.valueField);
            if (!(value instanceof Double)) {
                throw new IllegalStateException(def.name + " callback " + callback + " row " + index + " "
                        + def.valueField + " = " + value + " ("
                        + (value == null ? "null" : value.getClass().getSimpleName()) + "), want Double");
            }
            if (Double.compare(((Double) value).doubleValue(), def.rowValues[index]) != 0) {
                throw new IllegalStateException(def.name + " callback " + callback + " row " + index + " "
                        + def.valueField + " is " + value + ", want " + def.rowValues[index]);
            }
        }
    }
}
