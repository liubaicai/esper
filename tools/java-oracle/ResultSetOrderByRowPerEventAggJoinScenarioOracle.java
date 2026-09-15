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
import com.espertech.esper.runtime.client.UpdateListener;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for ResultSetOrderByRowPerEvent ordinals 2, 9 and
 * 10: {@code ResultSetRowPerEventJoinOrderFunction},
 * {@code ResultSetRowPerEventJoinMax} and {@code ResultSetAggHaving}.
 *
 * <p>Each case runs one runtime with the internal timer disabled, advances
 * virtual time to zero, compiles and deploys one statement named s0 over the
 * {@code SupportMarketDataBean#length(10) as one x SupportBeanString#length(100)
 * as two where one.symbol = two.theString} join, attaches exactly one listener
 * and replays the pinned scenario sends: {@code SupportMarketDataBean(symbol,
 * price, 0L, null)} market events first, then {@code SupportBeanString}
 * seed events.  Milestones of the Java execution are checkpoints only and are
 * not modeled.  Result rows are created per {@code SupportBeanString} delivery
 * (each seed joins with every same-symbol market event in the window), so
 * {@code output every 6 events} counts result rows, not input events.
 *
 * <p>Case {@code join-order-function} (ordinal 2) sends market events
 * IBM/2, KGB/1, CMU/3, IBM/6, CAT/6, CAT/5 followed by seeds CAT, IBM, CMU,
 * KGB, DOG.  The seeds CAT, IBM, CMU, KGB create 2, 2, 1 and 1 result rows
 * respectively (DOG matches nothing), and the sixth row - created during the
 * KGB delivery - triggers exactly one listener callback carrying six new rows
 * and no old rows.  The order key {@code volume*sum(price)} collapses to 0.0
 * for every row because the market volume is 0L, so {@code symbol} ascending
 * decides: CAT, CAT, CMU, IBM, IBM, KGB - exactly what the Java
 * {@code assertPropsPerRowNewOnly} call pins.  The Java source does NOT pin
 * the {@code sum(price)} values of this execution; every row carries the
 * join-output running sum evaluated at its own seed delivery (CAT rows 11.0,
 * IBM rows 19.0, CMU row 22.0, KGB row 23.0 with the pinned send prices), so
 * this oracle asserts only the pinned symbol order and the Double type of the
 * aggregate, while still recording the full rows for differential replay.
 *
 * <p>Case {@code join-max} (ordinal 9) sends market events IBM/3, IBM/4,
 * CMU/1, CMU/2, CAT/5, CAT/6 followed by seeds CAT, IBM, CMU.  The running
 * join-output sums at the seed deliveries are 11.0 (CAT), 18.0 (IBM) and
 * 21.0 (CMU), and the nested {@code max(sum(price))} per row is the
 * historical-prefix maximum of those sums; the running sums are monotone, so
 * every row carries the running sum of its own seed delivery.  Both rows of a
 * symbol share the value evaluated at that seed event.  Ordered by symbol:
 * CAT 11.0, CAT 11.0, CMU 21.0, CMU 21.0, IBM 18.0, IBM 18.0 - exactly what
 * the Java source pins.
 *
 * <p>Case {@code join-having} (ordinal 10) replays the join-max sends with
 * the plain {@code sum(price)} projection and {@code having sum(price) > 0}
 * (every row passes).  Ordered by symbol: CAT 11.0, CAT 11.0, CMU 21.0,
 * CMU 21.0, IBM 18.0, IBM 18.0 - exactly what the Java source pins.
 *
 * <p>The Java executions assert the delivered row order exactly (not any
 * order), and the order-by makes delivery deterministic, so the trace records
 * rows in engine delivery order without re-sorting.
 *
 * <p>The aggregated columns are Java {@code Double}: {@code price} is a
 * primitive double and {@code sum(price)} and {@code max(sum(price))} return
 * Double.  The trace emits them as JSON numbers via minimaljson's
 * {@code Json.value(double)}, which writes the shortest round-trip spelling of
 * {@code Double.toString} (for example {@code 18.0}).  Nothing in these
 * executions carries a remove stream, so no record has an {@code old} key.
 */
public final class ResultSetOrderByRowPerEventAggJoinScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "orderby-rowperevent-agg-join";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderByRowPerEvent.java";
    private static final String DESCRIPTION =
            "ResultSetOrderByRowPerEvent ordinals 2, 9 and 10: ungrouped row-per-event aggregates over a"
                    + " SupportMarketDataBean#length(10) x SupportBeanString#length(100) join, delivered once per six"
                    + " result rows - the order-function variant, the nested max(sum(price)) variant and the having"
                    + " variant with the plain running sum.";
    private static final String OBSERVATION = "listener";
    private static final int ITERATOR_SNAPSHOTS = 0;
    private static final int CASE_RECORDS = 1;
    private static final int TOTAL_RECORDS = 3;

    private static final CaseDef[] CASES = {
            new CaseDef("join-order-function", 2, "java-runtime-3864ed6701fd9371d2d6",
                    "ResultSetRowPerEventJoinOrderFunction",
                    "@name('s0') select symbol, sum(price) from SupportMarketDataBean#length(10) as one,"
                            + " SupportBeanString#length(100) as two where one.symbol = two.theString"
                            + " output every 6 events order by volume*sum(price), symbol",
                    new String[]{"sum(price)", "symbol"},
                    false,
                    new SendDef[]{
                            SendDef.market("IBM", 2), SendDef.market("KGB", 1), SendDef.market("CMU", 3),
                            SendDef.market("IBM", 6), SendDef.market("CAT", 6), SendDef.market("CAT", 5),
                            SendDef.seed("CAT"), SendDef.seed("IBM"), SendDef.seed("CMU"),
                            SendDef.seed("KGB"), SendDef.seed("DOG"),
                    },
                    new String[]{"CAT", "CAT", "CMU", "IBM", "IBM", "KGB"},
                    null),
            new CaseDef("join-max", 9, "java-runtime-eb2d5eb23ce35ef9d935",
                    "ResultSetRowPerEventJoinMax",
                    "@name('s0') select symbol, max(sum(price)) from SupportMarketDataBean#length(10) as one,"
                            + " SupportBeanString#length(100) as two where one.symbol = two.theString"
                            + " output every 6 events order by symbol",
                    new String[]{"max(sum(price))", "symbol"},
                    true,
                    new SendDef[]{
                            SendDef.market("IBM", 3), SendDef.market("IBM", 4), SendDef.market("CMU", 1),
                            SendDef.market("CMU", 2), SendDef.market("CAT", 5), SendDef.market("CAT", 6),
                            SendDef.seed("CAT"), SendDef.seed("IBM"), SendDef.seed("CMU"),
                    },
                    new String[]{"CAT", "CAT", "CMU", "CMU", "IBM", "IBM"},
                    new double[]{11, 11, 21, 21, 18, 18}),
            new CaseDef("join-having", 10, "java-runtime-73a76f426926d18792f2",
                    "ResultSetAggHaving",
                    "@name('s0') select symbol, sum(price) from SupportMarketDataBean#length(10) as one,"
                            + " SupportBeanString#length(100) as two where one.symbol = two.theString"
                            + " having sum(price) > 0 output every 6 events order by symbol",
                    new String[]{"sum(price)", "symbol"},
                    true,
                    new SendDef[]{
                            SendDef.market("IBM", 3), SendDef.market("IBM", 4), SendDef.market("CMU", 1),
                            SendDef.market("CMU", 2), SendDef.market("CAT", 5), SendDef.market("CAT", 6),
                            SendDef.seed("CAT"), SendDef.seed("IBM"), SendDef.seed("CMU"),
                    },
                    new String[]{"CAT", "CAT", "CMU", "CMU", "IBM", "IBM"},
                    new double[]{11, 11, 21, 21, 18, 18}),
    };

    private static final String[] STATIC_IDS = {
            "java-bb2fc1b5fb51cb943021", "java-7c35d3dcefb677a20097", "java-9c135512047209da35dd"
    };

    private static final String[] STEP_OPS = {
            "case",
            "send", "send", "send", "send", "send", "send", "send", "send", "send", "send", "send",
            "case",
            "send", "send", "send", "send", "send", "send", "send", "send", "send",
            "case",
            "send", "send", "send", "send", "send", "send", "send", "send", "send"
    };

    private ResultSetOrderByRowPerEventAggJoinScenarioOracle() {
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

    private static final class CaseDef {
        final String name;
        final int ordinal;
        final String runtimeId;
        final String executionName;
        final String epl;
        // Result metadata of the one statement, sorted by the runtime.
        final String[] resultFields;
        final String valueField;
        // True when the Java source pins the aggregate values exactly.
        final boolean valuesPinned;
        final SendDef[] sends;
        // Engine delivery order: the order-by makes it deterministic.
        final String[] rowSymbols;
        // Aligned with rowSymbols; only meaningful when valuesPinned.
        final double[] rowValues;

        CaseDef(String name, int ordinal, String runtimeId, String executionName, String epl,
                String[] resultFields, boolean valuesPinned, SendDef[] sends,
                String[] rowSymbols, double[] rowValues) {
            this.name = name;
            this.ordinal = ordinal;
            this.runtimeId = runtimeId;
            this.executionName = executionName;
            this.epl = epl;
            this.resultFields = resultFields;
            this.valueField = resultFields[0];
            this.valuesPinned = valuesPinned;
            this.sends = sends;
            this.rowSymbols = rowSymbols;
            this.rowValues = rowValues;
        }
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetOrderByRowPerEventAggJoinScenarioOracle <scenario.json>");
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
                if (sends >= def.sends.length) {
                    throw new IllegalArgumentException("unexpected send at step " + stepIndex);
                }
                sendPinned(runtime, step, stepIndex, def, sends);
                sends++;
            }
            if (markers != 1) {
                throw new IllegalStateException("case marker count mismatch for " + def.name);
            }
            if (sends != def.sends.length) {
                throw new IllegalStateException("expected " + def.sends.length + " sends for " + def.name
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
                CASES[0].runtimeId, CASES[1].runtimeId, CASES[2].runtimeId}, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), new String[]{
                CASES[0].executionName, CASES[1].executionName, CASES[2].executionName}, "javaNames");
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
        if (caseIndex != CASES.length - 1 || sendIndex != CASES[caseIndex].sends.length) {
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
         * row carries the pinned result metadata, the batch carries the pinned
         * symbols in the exact delivered order, and - only where the Java
         * source pins them - the pinned aggregate values.
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
            // The join-order-function execution pins only the symbol column;
            // its sum(price) values are derived from the live engine and are
            // recorded without being asserted.
            if (def.valuesPinned && Double.compare(((Double) value).doubleValue(), def.rowValues[index]) != 0) {
                throw new IllegalStateException(def.name + " callback " + callback + " row " + index + " "
                        + def.valueField + " is " + value + ", want " + def.rowValues[index]);
            }
        }
    }
}
