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
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashSet;
import java.util.List;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for ResultSetQueryTypeRollupHavingAndOrderBy
 * ordinals 0/1 (ResultSetQueryTypeHaving join=false/join=true) and
 * ResultSetQueryTypeRollupGroupingFuncs ordinal 3
 * (ResultSetQueryTypeGroupingFuncExpressionUse).  Each Java execution is one
 * scenario case; the having cases run two sequential deploy/undeploy cycles
 * (phase A rollup(theString, intPrimitive) having on the aggregate, phase B
 * rollup(theString) having on a null-key disjunction) inside one runtime,
 * matching the single Java execution.  SupportBean/SupportBean_S0 are the
 * shared regression beans; SupportCarEvent/SupportCarInfoEvent are local bean
 * classes because the regression-lib jar is not on the oracle classpath.
 * The grouping-func-expr case registers the oracle's own static myfunc as the
 * plug-in single-row function (the regression suite wires
 * GroupingSupportFunc.myfunc the same way); after every send the oracle
 * drains the UDF's per-row argument tuples into "capture" trace records with
 * fields c0..c7 so the Go runner can emit identical records.  Phase A
 * attaches a no-op listener (mirroring addListener("s0"), which the
 * select-clause UDF needs to be invoked) without recording listener rows:
 * Java's single void-UDF column has no Go-side row equivalent, the captured
 * tuples are the asserted contract.
 */
public final class ResultSetRollupHavingOrderByScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-rollup-having-orderby";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRollupHavingAndOrderBy.java";
    private static final String JAVA_SOURCE2 =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRollupGroupingFuncs.java";
    private static final String DESCRIPTION =
            "ResultSetQueryTypeRollupHavingAndOrderBy ordinals 0/1 (having over rollup, nojoin/join) plus ResultSetQueryTypeRollupGroupingFuncs ordinal 3 (grouping functions in expressions)";

    private static final String[] CASES = {
            "having-nojoin", "having-join", "grouping-func-expr"};
    private static final int[] ORDINALS = {0, 1, 3};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-23e2e441fc8898fe0303",
            "java-runtime-5c2e7accf8d815fe3ced",
            "java-runtime-7c4135247329f06e2e40"
    };
    private static final String[] EXECUTIONS = {
            "ResultSetQueryTypeHaving{join=false}",
            "ResultSetQueryTypeHaving{join=true}",
            "ResultSetQueryTypeGroupingFuncExpressionUse"
    };
    private static final String[] STATIC_IDS = {
            "java-24e47ca92d533e352474",
            "java-1ae66c9985dc53282af0"
    };
    private static final String[] OBSERVATIONS = {
            "listener", "listener", "listener+capture"
    };

    // Phase A / phase B EPL pairs for the two having executions, byte-exact
    // from ResultSetQueryTypeRollupHavingAndOrderBy (including the missing
    // space before "having" in phase A and the space before the join comma).
    private static final String HAVING_EPL_A_NOJOIN =
            "@Name('s0')select theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 " +
            "from SupportBean#keepall " +
            "group by rollup(theString, intPrimitive)" +
            "having sum(longPrimitive) > 1000";
    private static final String HAVING_EPL_A_JOIN =
            "@Name('s0')select theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 " +
            "from SupportBean#keepall " +
            ", SupportBean_S0#lastevent " +
            "group by rollup(theString, intPrimitive)" +
            "having sum(longPrimitive) > 1000";
    private static final String HAVING_EPL_B_NOJOIN =
            "@Name('s0')select theString as c0, sum(intPrimitive) as c1 " +
            "from SupportBean#keepall " +
            "group by rollup(theString) " +
            "having " +
            "(theString is null and sum(intPrimitive) > 100) " +
            "or " +
            "(theString is not null and sum(intPrimitive) > 200)";
    private static final String HAVING_EPL_B_JOIN =
            "@Name('s0')select theString as c0, sum(intPrimitive) as c1 " +
            "from SupportBean#keepall " +
            ", SupportBean_S0#lastevent " +
            "group by rollup(theString) " +
            "having " +
            "(theString is null and sum(intPrimitive) > 100) " +
            "or " +
            "(theString is not null and sum(intPrimitive) > 200)";

    // ResultSetQueryTypeGroupingFuncExpressionUse phase A is a two-statement
    // module (declared expression plus the s0 select); phase B is the
    // prev/prior rollup select.  Both byte-exact from the Java source.
    private static final String GROUPING_EPL_A =
            "create expression myExpr {x=> '|' || x.name || '|'};\n" +
            "@name('s0') select myfunc(" +
            "  name, place, sum(count), grouping(name), grouping(place), grouping_id(name, place)," +
            "  (select refId from SupportCarInfoEvent#lastevent), " +
            "  myExpr(ce)" +
            "  )" +
            "from SupportCarEvent ce group by grouping sets((name, place),name, place,())";
    private static final String GROUPING_EPL_B =
            "@name('s0') select prev(1, name) as c0, prior(1, name) as c1, name as c2, sum(count) as c3 " +
            "from SupportCarEvent#keepall ce group by rollup(name)";

    private static final List<Object[]> CAPTURED = new ArrayList<>();

    private ResultSetRollupHavingOrderByScenarioOracle() {
    }

    /**
     * Plug-in single-row function standing in for
     * ResultSetQueryTypeRollupGroupingFuncs.GroupingSupportFunc.myfunc; it
     * records every argument tuple so the send-time drain can emit them.
     */
    public static void myfunc(String name,
                              String place,
                              Integer cnt,
                              Integer grpName,
                              Integer grpPlace,
                              Integer grpId,
                              String refId,
                              String namePlusDelim) {
        CAPTURED.add(new Object[]{name, place, cnt, grpName, grpPlace, grpId, refId, namePlusDelim});
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetRollupHavingOrderByScenarioOracle <scenario.json>");
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
        for (int index = 0; index < CASES.length; index++) {
            runCase(steps, CASES[index], RUNTIME_IDS[index], index, records);
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, String caseName, String runtimeId,
                                int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCommon().addEventType(SupportCarEvent.class);
        configuration.getCommon().addEventType(SupportCarInfoEvent.class);
        configuration.getCompiler().addPlugInSingleRowFunction("myfunc",
                ResultSetRollupHavingOrderByScenarioOracle.class.getName(), "myfunc");

        String runtimeURI = "parity-" + ID + "-" + runtimeId;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            CAPTURED.clear();
            boolean active = false;
            int deployIndex = 0;
            for (int index = 0; index < steps.size(); index++) {
                JsonObject step = steps.get(index).asObject();
                String operation = step.getString("op", "");
                if ("case".equals(operation)) {
                    active = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!active) {
                    continue;
                }
                if ("deploy".equals(operation)) {
                    String statement = step.getString("statement", "");
                    if (!"s0".equals(statement)) {
                        throw new IllegalArgumentException("unexpected deploy statement " + statement);
                    }
                    int phase = deployIndex;
                    deployIndex++;
                    String epl = s0Epl(caseName, phase);
                    // CompilerArguments(configuration), not the runtime path:
                    // the plug-in single-row function registry lives on the
                    // configuration (mirroring RegressionEnvironmentBase).
                    EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                            new CompilerArguments(configuration));
                    EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                            new DeploymentOptions().setDeploymentId(ID + "-" + caseIndex + "-" + deployIndex));
                    EPStatement s0 = findStatement(deployment);
                    if (s0 == null) {
                        throw new IllegalStateException("deployment has no statement named s0");
                    }
                    if (isCapturePhase(caseName, phase)) {
                        // Mirror addListener("s0") so the select-clause UDF is
                        // invoked; the void-UDF rows themselves are not the
                        // asserted contract, only the captured tuples are.
                        s0.addListener((newEvents, oldEvents, stmt, rt) -> {
                        });
                    } else {
                        s0.addListener(writer);
                    }
                } else if ("undeploy-all".equals(operation)) {
                    runtime.getDeploymentService().undeployAll();
                } else if ("send".equals(operation)) {
                    sendEvent(runtime, step);
                    // The phase-A select-clause UDF captured one argument
                    // tuple per emitted grouping-set row; drain them into
                    // capture records right after the triggering send,
                    // mirroring assertGetAndClear at the next assertion.
                    emitCaptures(writer);
                } else {
                    throw new IllegalArgumentException("unsupported step op " + operation);
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static boolean isCapturePhase(String caseName, int deployIndex) {
        return "grouping-func-expr".equals(caseName) && deployIndex == 0;
    }

    private static void emitCaptures(TraceWriter writer) {
        for (Object[] tuple : CAPTURED) {
            JsonObject fields = new JsonObject();
            for (int column = 0; column < tuple.length; column++) {
                fields.add("c" + column, normalize(tuple[column]));
            }
            JsonObject record = new JsonObject()
                    .add("case", writer.caseName)
                    .add("operation", "capture")
                    .add("statement", "s0")
                    .add("sequence", writer.nextSequence())
                    .add("time", Instant.ofEpochMilli(writer.runtime.getEventService().getCurrentTime()).toString())
                    .add("new", new JsonArray().add(new JsonObject().add("kind", "row").add("fields", fields)));
            writer.records.add(record);
        }
        CAPTURED.clear();
    }

    private static String s0Epl(String caseName, int deployIndex) {
        if (deployIndex != 0 && deployIndex != 1) {
            throw new IllegalArgumentException("unexpected deploy index " + deployIndex);
        }
        switch (caseName) {
            case "having-nojoin":
                return deployIndex == 0 ? HAVING_EPL_A_NOJOIN : HAVING_EPL_B_NOJOIN;
            case "having-join":
                return deployIndex == 0 ? HAVING_EPL_A_JOIN : HAVING_EPL_B_JOIN;
            case "grouping-func-expr":
                return deployIndex == 0 ? GROUPING_EPL_A : GROUPING_EPL_B;
            default:
                throw new IllegalArgumentException("unexpected case " + caseName);
        }
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        for (EPStatement candidate : deployment.getStatements()) {
            if ("s0".equals(candidate.getName())) {
                return candidate;
            }
        }
        return null;
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("SupportBean".equals(eventType)) {
            SupportBean bean = new SupportBean();
            bean.setTheString(payload.getString("theString", null));
            bean.setIntPrimitive(payload.getInt("intPrimitive", 0));
            bean.setLongPrimitive(payload.getLong("longPrimitive", 0));
            runtime.getEventService().sendEventBean(bean, "SupportBean");
        } else if ("SupportBean_S0".equals(eventType)) {
            runtime.getEventService().sendEventBean(new SupportBean_S0(payload.getInt("id", 0)), "SupportBean_S0");
        } else if ("SupportCarEvent".equals(eventType)) {
            runtime.getEventService().sendEventBean(new SupportCarEvent(
                    payload.getString("name", null),
                    payload.getString("place", null),
                    payload.getInt("count", 0)), "SupportCarEvent");
        } else if ("SupportCarInfoEvent".equals(eventType)) {
            runtime.getEventService().sendEventBean(new SupportCarInfoEvent(
                    payload.getString("name", null),
                    payload.getString("place", null),
                    payload.getString("refId", null)), "SupportCarInfoEvent");
        } else {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaSource2", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags",
                "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))
                || !JAVA_SOURCE2.equals(string(scenario, "javaSource2"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTIONS, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly three cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl", "eplB");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !s0Epl(CASES[index], 0).equals(string(definition, "epl"))
                    || !s0Epl(CASES[index], 1).equals(string(definition, "eplB"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
        }
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

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.runtime = runtime;
        }

        private long nextSequence() {
            return ++sequence;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement,
                           EPRuntime ignoredRuntime) {
            if (newEvents == null && oldEvents == null) {
                return;
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", nextSequence())
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", rows(oldEvents));
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
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
    }

    /** Local stand-in for the regression-lib SupportCarEvent bean. */
    public static final class SupportCarEvent {
        private final String name;
        private final String place;
        private final int count;

        public SupportCarEvent(String name, String place, int count) {
            this.name = name;
            this.place = place;
            this.count = count;
        }

        public String getName() {
            return name;
        }

        public String getPlace() {
            return place;
        }

        public int getCount() {
            return count;
        }
    }

    /** Local stand-in for the regression-lib SupportCarInfoEvent bean. */
    public static final class SupportCarInfoEvent {
        private final String name;
        private final String place;
        private final String refId;

        public SupportCarInfoEvent(String name, String place, String refId) {
            this.name = name;
            this.place = place;
            this.refId = refId;
        }

        public String getName() {
            return name;
        }

        public String getPlace() {
            return place;
        }

        public String getRefId() {
            return refId;
        }
    }
}
