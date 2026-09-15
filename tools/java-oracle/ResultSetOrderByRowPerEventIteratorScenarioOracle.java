import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBeanString;
import com.espertech.esper.regressionlib.support.bean.SupportMarketDataBean;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for ResultSetOrderByRowPerEvent ordinal 0:
 * {@code ResultSetIteratorAggregateRowPerEvent} (source lines 38-76).
 *
 * <p>The case runs one runtime with the internal timer disabled, advances
 * virtual time to zero, compiles and deploys one statement named s0 over the
 * {@code SupportMarketDataBean#length(10) as one x SupportBeanString#length(100)
 * as two where one.symbol = two.theString order by symbol} join with NO output
 * policy and NO listener, and replays the pinned scenario sends:
 * {@code SupportBeanString} seeds CAT, IBM, KGB first, then
 * {@code SupportMarketDataBean(symbol, price, 0L, null)} market events
 * CAT/50, IBM/49, CAT/15, IBM/100.  The statement ITERATOR is then read - the
 * observation of this execution, mirroring the Java
 * {@code assertPropsPerRowIterator} checkpoints - once directly after those
 * four market sends and once more after a final KGB/75 market send, followed
 * by {@code undeployAll}.
 *
 * <p>Because the statement has no output policy, every join result row is
 * current and carries the running window sum at read time: each of the three
 * seeds joins with every same-symbol market event in the window, giving 2 CAT
 * rows, 2 IBM rows and (after KGB/75) 1 KGB row.  The un-grouped
 * {@code sum(price)} is evaluated over the whole window, so every row of a
 * snapshot carries the identical value: 214.0 = 50 + 49 + 15 + 100 in
 * snapshot 1 and 289.0 = 214 + 75 in snapshot 2.  The {@code order by symbol}
 * makes the iterator order deterministic - CAT rows before IBM rows before
 * KGB rows - exactly what the Java {@code assertPropsPerRowIterator} pins
 * index-by-index in exact order, so no re-sorting and no any-order handling
 * is applied.
 *
 * <p>The scenario EPL is deployed verbatim; its aggregate column carries the
 * explicit {@code as sumPrice} alias, so the row field keys sort to
 * {@code ["sumPrice","symbol"]}.
 *
 * <p>The aggregated column is a Java {@code Double}: {@code price} is a
 * primitive double and {@code sum(price)} returns Double.  The trace emits it
 * as a JSON number via minimaljson's {@code Json.value(double)}, which writes
 * the shortest round-trip spelling of {@code Double.toString} (for example
 * {@code 214.0}).  Reading the iterator never produces a remove stream, so no
 * record has an {@code old} key.
 */
public final class ResultSetOrderByRowPerEventIteratorScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "orderby-rowperevent-iterator";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderByRowPerEvent.java";
    private static final String DESCRIPTION =
            "ResultSetOrderByRowPerEvent ordinal 0: the join aggregate read through the statement iterator twice"
                    + " inside one runtime, each row carrying the current window sum, ordered by symbol.";
    private static final String OBSERVATION = "iterator";
    private static final int ITERATOR_SNAPSHOTS = 2;
    private static final int TOTAL_RECORDS = 2;

    private static final CaseDef[] CASES = {
            new CaseDef("iterator-join", 0, "java-runtime-7bef2fa8f755e74b24ac",
                    "ResultSetIteratorAggregateRowPerEvent",
                    "@name('s0') select symbol, sum(price) as sumPrice from SupportMarketDataBean#length(10) as one,"
                            + " SupportBeanString#length(100) as two where one.symbol = two.theString"
                            + " order by symbol",
                    new String[]{"sumPrice", "symbol"},
                    new SendDef[]{
                            SendDef.seed("CAT"), SendDef.seed("IBM"), SendDef.seed("KGB"),
                            SendDef.market("CAT", 50), SendDef.market("IBM", 49),
                            SendDef.market("CAT", 15), SendDef.market("IBM", 100),
                            SendDef.market("KGB", 75),
                    },
                    new SnapshotDef[]{
                            new SnapshotDef(
                                    new String[]{"CAT", "CAT", "IBM", "IBM"},
                                    new double[]{214, 214, 214, 214}),
                            new SnapshotDef(
                                    new String[]{"CAT", "CAT", "IBM", "IBM", "KGB"},
                                    new double[]{289, 289, 289, 289, 289}),
                    }),
    };

    private static final String[] STATIC_IDS = {
            "java-d88b4c5e379242ff177b"
    };

    private static final String[] STEP_OPS = {
            "case",
            "send", "send", "send", "send", "send", "send", "send",
            "snapshot",
            "send",
            "snapshot"
    };

    private ResultSetOrderByRowPerEventIteratorScenarioOracle() {
    }

    private static final class SendDef {
        final boolean marketData;
        // Market-data symbol or SupportBeanString theString.
        final String symbol;
        final double price;

        private SendDef(boolean marketData, String symbol, double price) {
            this.marketData = marketData;
            this.symbol = symbol;
            this.price = price;
        }

        static SendDef market(String symbol, double price) {
            return new SendDef(true, symbol, price);
        }

        static SendDef seed(String theString) {
            return new SendDef(false, theString, 0.0);
        }
    }

    private static final class SnapshotDef {
        // Exact iterator order pinned by assertPropsPerRowIterator.
        final String[] rowSymbols;
        // Aligned with rowSymbols; every row carries the current window sum.
        final double[] rowValues;

        SnapshotDef(String[] rowSymbols, double[] rowValues) {
            this.rowSymbols = rowSymbols;
            this.rowValues = rowValues;
        }
    }

    private static final class CaseDef {
        final String name;
        final int ordinal;
        final String runtimeId;
        final String executionName;
        final String epl;
        // Result metadata of the one statement, sorted by the runtime.
        final String[] resultFields;
        final SendDef[] sends;
        // One entry per pinned iterator read, in scenario order.
        final SnapshotDef[] snapshots;

        CaseDef(String name, int ordinal, String runtimeId, String executionName, String epl,
                String[] resultFields, SendDef[] sends, SnapshotDef[] snapshots) {
            this.name = name;
            this.ordinal = ordinal;
            this.runtimeId = runtimeId;
            this.executionName = executionName;
            this.epl = epl;
            this.resultFields = resultFields;
            this.sends = sends;
            this.snapshots = snapshots;
        }
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOrderByRowPerEventIteratorScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBeanString.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("parity-" + ID + "-" + def.runtimeId, configuration);
        try {
            runtime.getEventService().advanceTime(0L);
            EPStatement statement = deploy(runtime, def);

            JsonArray steps = scenario.get("steps").asArray();
            boolean active = false;
            int markers = 0;
            int sends = 0;
            int snapshots = 0;
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
                if ("send".equals(operation)) {
                    if (sends >= def.sends.length) {
                        throw new IllegalArgumentException("unexpected send at step " + stepIndex);
                    }
                    sendPinned(runtime, step, stepIndex, def, sends);
                    sends++;
                } else if ("snapshot".equals(operation)) {
                    if (snapshots >= def.snapshots.length) {
                        throw new IllegalArgumentException("unexpected snapshot at step " + stepIndex);
                    }
                    snapshots++;
                    takeSnapshot(statement, runtime, def, records, snapshots);
                } else {
                    throw new IllegalArgumentException(
                            "unsupported operation at step " + stepIndex + ": " + operation);
                }
            }
            if (markers != 1) {
                throw new IllegalStateException("case marker count mismatch for " + def.name);
            }
            if (sends != def.sends.length) {
                throw new IllegalStateException("expected " + def.sends.length + " sends for " + def.name
                        + ", got " + sends);
            }
            if (snapshots != def.snapshots.length) {
                throw new IllegalStateException("expected " + def.snapshots.length + " iterator snapshots for "
                        + def.name + ", got " + snapshots);
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

    private static void sendPinned(EPRuntime runtime, JsonObject step, int stepIndex, CaseDef def,
                                   int sendIndex) {
        requireFields(step, "op", "eventType", "payload");
        SendDef send = def.sends[sendIndex];
        JsonObject payload = object(step.get("payload"), "payload step " + stepIndex);
        if (send.marketData) {
            if (!"SupportMarketDataBean".equals(string(step, "eventType"))) {
                throw new IllegalArgumentException("unexpected event type at step " + stepIndex);
            }
            requireFields(payload, "symbol", "price", "volume");
            if (!send.symbol.equals(string(payload, "symbol"))
                    || Double.compare(doubleNumber(payload, "price"), send.price) != 0
                    || longNumber(payload, "volume") != 0L) {
                throw new IllegalArgumentException("send payload is not pinned at step " + stepIndex);
            }
            runtime.getEventService().sendEventBean(
                    new SupportMarketDataBean(send.symbol, send.price, 0L, null), "SupportMarketDataBean");
        } else {
            if (!"SupportBeanString".equals(string(step, "eventType"))) {
                throw new IllegalArgumentException("unexpected event type at step " + stepIndex);
            }
            requireFields(payload, "theString");
            if (!send.symbol.equals(string(payload, "theString"))) {
                throw new IllegalArgumentException("send payload is not pinned at step " + stepIndex);
            }
            runtime.getEventService().sendEventBean(new SupportBeanString(send.symbol), "SupportBeanString");
        }
    }

    /**
     * Mirrors the Java assertPropsPerRowIterator checkpoint: reads the
     * statement iterator in its own order, asserts the exact pinned row count,
     * symbols and window sums, and records one snapshot record.
     */
    private static void takeSnapshot(EPStatement statement, EPRuntime runtime, CaseDef def,
                                     JsonArray records, int sequence) {
        SnapshotDef expected = def.snapshots[sequence - 1];
        long now = runtime.getEventService().getCurrentTime();
        if (now != 0L) {
            throw new IllegalStateException(def.name + " snapshot " + sequence + " is at " + now + ", want 0");
        }
        List<EventBean> collected = new ArrayList<>();
        Iterator<EventBean> iterator = statement.iterator();
        while (iterator.hasNext()) {
            collected.add(iterator.next());
        }
        EventBean[] batch = collected.toArray(new EventBean[0]);
        if (batch.length != expected.rowSymbols.length) {
            throw new IllegalStateException(def.name + " snapshot " + sequence + " must carry exactly "
                    + expected.rowSymbols.length + " rows, got " + batch.length);
        }
        for (EventBean row : batch) {
            String[] names = row.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            if (!Arrays.equals(names, def.resultFields)) {
                throw new IllegalStateException(def.name + " snapshot " + sequence
                        + " result field metadata is not pinned: " + Arrays.toString(names));
            }
        }
        for (int index = 0; index < batch.length; index++) {
            if (!expected.rowSymbols[index].equals(batch[index].get("symbol"))) {
                throw new IllegalStateException(def.name + " snapshot " + sequence + " row " + index
                        + " symbol is " + batch[index].get("symbol") + ", want " + expected.rowSymbols[index]);
            }
            Object value = batch[index].get("sumPrice");
            if (!(value instanceof Double)) {
                throw new IllegalStateException(def.name + " snapshot " + sequence + " row " + index
                        + " sumPrice = " + value + " ("
                        + (value == null ? "null" : value.getClass().getSimpleName()) + "), want Double");
            }
            if (Double.compare(((Double) value).doubleValue(), expected.rowValues[index]) != 0) {
                throw new IllegalStateException(def.name + " snapshot " + sequence + " row " + index
                        + " sumPrice is " + value + ", want " + expected.rowValues[index]);
            }
        }
        records.add(new JsonObject().add("case", def.name).add("operation", "snapshot")
                .add("statement", statement.getName()).add("sequence", sequence)
                .add("time", Instant.ofEpochMilli(now).toString())
                .add("new", rows(batch)));
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
        validateStringArray(scenario.get("javaRuntimes"), new String[]{CASES[0].runtimeId}, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), new String[]{CASES[0].executionName}, "javaNames");
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
        int snapshotIndex = 0;
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
            } else if ("snapshot".equals(operation)) {
                if (caseIndex < 0) {
                    throw new IllegalArgumentException("snapshot before any case marker at step " + index);
                }
                requireFields(step, "op", "statement");
                if (!"s0".equals(string(step, "statement"))) {
                    throw new IllegalArgumentException("snapshot statement is not pinned at step " + index);
                }
                snapshotIndex++;
            } else {
                if (caseIndex < 0) {
                    throw new IllegalArgumentException("send before any case marker at step " + index);
                }
                validateSend(step, index, CASES[caseIndex], sendIndex);
                sendIndex++;
            }
        }
        if (caseIndex != CASES.length - 1 || sendIndex != CASES[caseIndex].sends.length
                || snapshotIndex != CASES[caseIndex].snapshots.length) {
            throw new IllegalArgumentException("scenario step census is not pinned");
        }
    }

    private static void validateSend(JsonObject step, int index, CaseDef def, int sendIndex) {
        requireFields(step, "op", "eventType", "payload");
        SendDef send = def.sends[sendIndex];
        JsonObject payload = object(step.get("payload"), "payload step " + index);
        if (send.marketData) {
            if (!"SupportMarketDataBean".equals(string(step, "eventType"))) {
                throw new IllegalArgumentException("send event type is not pinned at step " + index);
            }
            requireFields(payload, "symbol", "price", "volume");
            if (!send.symbol.equals(string(payload, "symbol"))
                    || Double.compare(doubleNumber(payload, "price"), send.price) != 0
                    || longNumber(payload, "volume") != 0L) {
                throw new IllegalArgumentException("send payload is not pinned at step " + index);
            }
        } else {
            if (!"SupportBeanString".equals(string(step, "eventType"))) {
                throw new IllegalArgumentException("send event type is not pinned at step " + index);
            }
            requireFields(payload, "theString");
            if (!send.symbol.equals(string(payload, "theString"))) {
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
}
