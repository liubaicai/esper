import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
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
import java.util.Collection;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;
import java.util.regex.Pattern;

/**
 * Direct Esper 9.0.0 oracle for ResultSetAggregationMethodWindow ordinals 1-3.
 *
 * Each case is run in a fresh runtime. The scenario's table/window EPL and
 * event sequence are replayed without changing the fixed Java execution.
 */
public final class ResultSetAggregationMethodWindowScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-window";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregationMethodWindow.java";
    private static final String[] CASES = {"table-access", "table-ident-count", "table-list-reference"};
    private static final int[] ORDINALS = {1, 2, 3};
    private static final String[] RUNTIMES = {
            "java-runtime-785f2999e48fbaa6eb77",
            "java-runtime-9ef8f9a367e788b5afec",
            "java-runtime-6652f083e2b0f3dffc04"
    };
    private static final String[] EXECUTIONS = {
            "ResultSetAggregateWindowTableAccess",
            "ResultSetAggregateWindowTableIdentWCount",
            "ResultSetAggregateWindowListReference"
    };
    private static final String[] STATIC_IDS = {
            "java-f881d0116e155d3e9063",
            "java-804f7ea3bb20de0d35e8",
            "java-f1014305fc8b701e8ea0"
    };
    private static final String DESCRIPTION =
            "ResultSetAggregationMethodWindow ordinals 1-3: table window access, first/last property access and list reference.";
    private static final String[] EPLS = {
            "create table MyTable(windowcol window(*) @type('SupportBean')); into table MyTable select window(*) as windowcol from SupportBean#length(2); @name('s0') select MyTable.windowcol.first() as c0, MyTable.windowcol.last() as c1 from SupportBean_S0",
            "create table MyTable(windowcol window(*) @type('SupportBean')); into table MyTable select window(*) as windowcol from SupportBean; @name('s0') select windowcol.first(intPrimitive) as c0, windowcol.last(intPrimitive) as c1, windowcol.countEvents() as c2 from SupportBean_S0, MyTable",
            "create table MyTable(windowcol window(*) @type('SupportBean')); into table MyTable select window(*) as windowcol from SupportBean; @name('s0') select MyTable.windowcol.listReference() as collref from SupportBean_S0"
    };
    private static final Pattern INTEGER_SYNTAX = Pattern.compile("-?(?:0|[1-9][0-9]*)");

    private ResultSetAggregationMethodWindowScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetAggregationMethodWindowScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        validateNoDuplicateKeys(parsed);
        JsonObject scenario = object(parsed, "scenario");
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        JsonArray steps = scenario.get("steps").asArray();
        for (String caseName : CASES) {
            runCase(steps, caseName, records);
        }
        System.out.println(new JsonObject().add("version", VERSION).add("id", ID).add("records", records));
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(scenario.getString("version", ""))
                || !ID.equals(scenario.getString("id", ""))
                || !DESCRIPTION.equals(scenario.getString("description", ""))
                || !JAVA_COMMIT.equals(scenario.getString("javaCommit", ""))
                || !JAVA_SOURCE.equals(scenario.getString("javaSource", ""))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        strings(scenario.get("javaRuntimes"), RUNTIMES);
        strings(scenario.get("javaNames"), EXECUTIONS);
        strings(scenario.get("javaStaticIds"), STATIC_IDS);
        strings(scenario.get("javaFlags"));

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly three cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject entry = object(cases.get(index), "case metadata");
            requireFields(entry, "case", "ordinal", "runtimeId", "executionName", "observation",
                    "iteratorSnapshots", "epl");
            if (!CASES[index].equals(entry.getString("case", ""))
                    || requireInteger(entry, "ordinal") != ORDINALS[index]
                    || !RUNTIMES[index].equals(entry.getString("runtimeId", ""))
                    || !EXECUTIONS[index].equals(entry.getString("executionName", ""))
                    || !"listener".equals(entry.getString("observation", ""))
                    || requireInteger(entry, "iteratorSnapshots") != 0
                    || !EPLS[index].equals(entry.getString("epl", ""))) {
                throw new IllegalArgumentException("case metadata is not pinned for index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        int[] expectedSends = {7, 4, 3};
        int caseIndex = -1;
        int sendIndex = 0;
        int caseCount = 0;
        for (int index = 0; index < steps.size(); index++) {
            JsonObject step = object(steps.get(index), "step " + index);
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                requireFields(step, "op", "case");
                caseIndex++;
                if (caseIndex >= CASES.length || !CASES[caseIndex].equals(step.getString("case", ""))) {
                    throw new IllegalArgumentException("case markers must appear once in source order");
                }
                if (caseIndex > 0 && sendIndex != expectedSends[caseIndex - 1]) {
                    throw new IllegalArgumentException("case send count is not pinned");
                }
                sendIndex = 0;
                caseCount++;
                continue;
            }
            if (!"send".equals(op) || caseIndex < 0) {
                throw new IllegalArgumentException("steps must contain case markers and sends only");
            }
            requireFields(step, "op", "eventType", "payload");
            JsonObject payload = object(step.get("payload"), "step payload");
            if (caseIndex == 0) {
                if (sendIndex >= expectedSends[0]) {
                    throw new IllegalArgumentException("table-access has too many sends");
                }
                if (sendIndex % 2 == 0) {
                    if (!"SupportBean_S0".equals(step.getString("eventType", ""))) {
                        throw new IllegalArgumentException("table-access trigger order is not pinned");
                    }
                    requireFields(payload, "id");
                    if (requireInteger(payload, "id") != -1) {
                        throw new IllegalArgumentException("trigger id is not pinned");
                    }
                } else {
                    if (!"SupportBean".equals(step.getString("eventType", ""))) {
                        throw new IllegalArgumentException("table-access event order is not pinned");
                    }
                    requireBean(payload, sendIndex / 2, new String[]{"E1", "E2", "E3"}, new int[]{10, 20, 0});
                }
            } else if (caseIndex == 1) {
                if (!"SupportBean".equals(step.getString("eventType", ""))) {
                    if (sendIndex != expectedSends[1] - 1 || !"SupportBean_S0".equals(step.getString("eventType", ""))) {
                        throw new IllegalArgumentException("table-ident-count event order is not pinned");
                    }
                    requireFields(payload, "id");
                    if (requireInteger(payload, "id") != -1) {
                        throw new IllegalArgumentException("trigger id is not pinned");
                    }
                } else {
                    if (sendIndex >= 3) {
                        throw new IllegalArgumentException("table-ident-count has too many bean sends");
                    }
                    requireBean(payload, sendIndex, new String[]{"E1", "E2", "E3"}, new int[]{10, 20, 30});
                }
            } else {
                if (sendIndex < 2) {
                    if (!"SupportBean".equals(step.getString("eventType", ""))) {
                        throw new IllegalArgumentException("table-list-reference bean order is not pinned");
                    }
                    requireBean(payload, sendIndex, new String[]{"E1", "E1"}, new int[]{10, 10});
                } else {
                    if (!"SupportBean_S0".equals(step.getString("eventType", ""))) {
                        throw new IllegalArgumentException("table-list-reference trigger is not pinned");
                    }
                    requireFields(payload, "id");
                    if (requireInteger(payload, "id") != -1) {
                        throw new IllegalArgumentException("trigger id is not pinned");
                    }
                }
            }
            sendIndex++;
        }
        if (caseCount != CASES.length || caseIndex != CASES.length - 1 || sendIndex != expectedSends[2]
                || steps.size() != 1 + expectedSends[0] + 1 + expectedSends[1] + 1 + expectedSends[2]) {
            throw new IllegalArgumentException("scenario steps are not pinned");
        }
    }

    private static void requireBean(JsonObject payload, int index, String[] strings, int[] ints) {
        requireFields(payload, "theString", "intPrimitive");
        if (index < 0 || index >= strings.length || !strings[index].equals(payload.getString("theString", ""))
                || requireInteger(payload, "intPrimitive") != ints[index]) {
            throw new IllegalArgumentException("SupportBean payload is not pinned");
        }
    }

    private static void runCase(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        String runtimeURI = "parity-resultset-aggregate-window-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                    eplForCase(caseName), new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(runtimeURI));
            EPStatement statement = findStatement(deployment);
            statement.addListener(new TraceWriter(records, caseName, statement, runtime));
            replay(allSteps, caseName, runtime);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static String eplForCase(String caseName) {
        for (int index = 0; index < CASES.length; index++) {
            if (CASES[index].equals(caseName)) {
                return EPLS[index];
            }
        }
        throw new IllegalArgumentException("unknown case " + caseName);
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        for (EPStatement statement : deployment.getStatements()) {
            if ("s0".equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("statement s0 was not deployed");
    }

    private static void replay(JsonArray allSteps, String caseName, EPRuntime runtime) {
        boolean active = false;
        for (int index = 0; index < allSteps.size(); index++) {
            JsonObject step = allSteps.get(index).asObject();
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                active = caseName.equals(step.getString("case", ""));
                continue;
            }
            if (!active) {
                continue;
            }
            JsonObject payload = step.get("payload").asObject();
            if ("SupportBean".equals(step.getString("eventType", ""))) {
                runtime.getEventService().sendEventBean(
                        new SupportBean(payload.getString("theString", null), payload.getInt("intPrimitive", 0)),
                        "SupportBean");
            } else {
                runtime.getEventService().sendEventBean(
                        new SupportBean_S0(payload.getInt("id", 0)), "SupportBean_S0");
            }
        }
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored, EPRuntime ignoredRuntime) {
            if (newEvents == null && oldEvents == null) {
                return;
            }
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            JsonArray oldRows = rows(oldEvents);
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                JsonObject fields = new JsonObject();
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
            if (value instanceof EventBean) {
                return normalize(((EventBean) value).getUnderlying());
            }
            if (value instanceof SupportBean) {
                SupportBean bean = (SupportBean) value;
                return supportBeanRow(bean);
            }
            if (value instanceof Collection) {
                JsonArray array = new JsonArray();
                for (Object item : (Collection<?>) value) {
                    array.add(normalize(item));
                }
                return array;
            }
            if (value instanceof Object[]) {
                JsonArray array = new JsonArray();
                for (Object item : (Object[]) value) {
                    array.add(normalize(item));
                }
                return array;
            }
            if (value instanceof Map) {
                Map<?, ?> map = (Map<?, ?>) value;
                TreeSet<String> names = new TreeSet<>();
                for (Object key : map.keySet()) {
                    names.add(String.valueOf(key));
                }
                JsonObject fields = new JsonObject();
                for (String name : names) {
                    fields.add(name, normalize(map.get(name)));
                }
                return new JsonObject().add("kind", "row").add("fields", fields);
            }
            if (value instanceof Float || value instanceof Double) {
                double number = ((Number) value).doubleValue();
                if (number == Math.rint(number) && !Double.isInfinite(number)) {
                    return Json.value((long) number);
                }
                return Json.value(number);
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

        private JsonObject supportBeanRow(SupportBean bean) {
            return new JsonObject().add("kind", "row").add("fields",
                    new JsonObject().add("intPrimitive", normalize(bean.getIntPrimitive()))
                            .add("theString", normalize(bean.getTheString())));
        }
    }

    private static void validateNoDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> seen = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!seen.add(member.getName())) {
                    throw new IllegalArgumentException("duplicate JSON object key: " + member.getName());
                }
                validateNoDuplicateKeys(member.getValue());
            }
        } else if (value.isArray()) {
            JsonArray array = value.asArray();
            for (int index = 0; index < array.size(); index++) {
                validateNoDuplicateKeys(array.get(index));
            }
        }
    }

    private static JsonObject object(JsonValue value, String name) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(name + " must be an object");
        }
        return value.asObject();
    }

    private static JsonArray array(JsonValue value, String name) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(name + " must be an array");
        }
        return value.asArray();
    }

    private static void requireFields(JsonObject object, String... expectedNames) {
        if (object == null || object.size() != expectedNames.length) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
        Set<String> expected = new HashSet<>(Arrays.asList(expectedNames));
        if (!expected.equals(new HashSet<>(object.names()))) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
    }

    private static void strings(JsonValue value, String... expected) {
        JsonArray array = array(value, "string array");
        if (array.size() != expected.length) {
            throw new IllegalArgumentException("string array length mismatch");
        }
        for (int index = 0; index < expected.length; index++) {
            if (!array.get(index).isString() || !expected[index].equals(array.get(index).asString())) {
                throw new IllegalArgumentException("string array value mismatch");
            }
        }
    }

    private static int requireInteger(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be a JSON integer");
        }
        String text = value.toString();
        if (!INTEGER_SYNTAX.matcher(text).matches()) {
            throw new IllegalArgumentException(name + " must use integer JSON syntax");
        }
        try {
            return Integer.parseInt(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(name + " is outside the Java int range", ex);
        }
    }
}
