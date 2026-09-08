import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventPropertyDescriptor;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerContext;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportMarketDataBean;
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
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the EPLOtherSelectExprStreamSelector parity
 * unit: the alias-with-properties executions (Java ordinals 8 and 9). The
 * no-join execution selects stream-dot columns (theString.* as s0/s1) beside
 * plain property aliases (intPrimitive as a/b) over a length window; the join
 * execution mixes plain properties (intPrimitive, theString), an aliased
 * column (symbol as sym) and stream-as-object columns (s0stream/s1stream)
 * over a length-keepall inner join. The wildcard and alone wildcard/alias
 * executions of the same source are covered by their own parity units.
 *
 * Each execution is replayed in a fresh runtime. Stream-as-object columns
 * arrive as EventBean wrappers over the exact sent instances (the Java source
 * asserts identity on the underlying), so the oracle unwraps and asserts
 * identity before rendering the projected mirror surface: SupportBean rows
 * are {intPrimitive, theString} and SupportMarketDataBean rows are {feed,
 * price, symbol, volume}. The statement output-type surface (property names
 * plus Java types and the Map underlying type) is asserted internally against
 * the deployed statements' event type descriptors per the Java source; it is
 * not recorded in the trace because the Go side cannot introspect Java types.
 * Values and names are the differential surface.
 */
public final class EPLOtherSelectExprStreamSelectorScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "epl-other-select-expr-stream-selector";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/"
                    + "EPLOtherSelectExprStreamSelector.java";
    private static final String DESCRIPTION =
            "EPLOtherSelectExprStreamSelector alias-with-properties executions: stream-dot notation with alias "
                    + "(theString.* as s0/s1) beside plain property aliases (intPrimitive as a/b) over a length "
                    + "window, and mixed join select with stream-as-object columns (s0stream/s1stream), plain "
                    + "properties (intPrimitive, theString), and aliased columns (symbol as sym) over a "
                    + "length-keepall inner join (Java source regression-lib/src/main/java/com/espertech/esper/"
                    + "regressionlib/suite/epl/other/EPLOtherSelectExprStreamSelector.java).";

    private static final String NO_JOIN = "no-join-alias-props";
    private static final String JOIN = "join-alias-props";
    private static final String[] CASES = {NO_JOIN, JOIN};
    private static final int[] ORDINALS = {8, 9};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-89123cf55af0a7f5987e",
            "java-runtime-b53494cb36a6b54c2c6f"
    };
    private static final String[] EXECUTION_NAMES = {
            "EPLOtherNoJoinWithAliasWithProperties",
            "EPLOtherJoinWithAliasWithProperties"
    };
    private static final String[] STATIC_IDS = {
            "java-b5352434faea014585a8",
            "java-9682b92ad00f7295162d"
    };

    // Exact Java-source EPL strings.
    private static final String EPL_NO_JOIN =
            "@name('s0') select theString.* as s0, intPrimitive as a, theString.* as s1, "
                    + "intPrimitive as b from SupportBean#length(3) as theString";
    private static final String EPL_JOIN =
            "@name('s0') select intPrimitive, s1.* as s1stream, theString, symbol as sym, s0.* as s0stream "
                    + "from SupportBean#length(3) as s0, SupportMarketDataBean#keepall as s1";

    // Deploy counts per case: 1, 1 (2 total).
    private static final int[] DEPLOY_COUNTS = {1, 1};
    // Send counts per case: 1, 2 (3 total).
    private static final int[] SEND_COUNTS = {1, 2};
    // Per-case step spans: 4, 5 (9 total).
    private static final int[] STEP_COUNTS = {4, 5};
    // Listener records per case: 1, 1 (2 total).
    private static final int[] RECORD_COUNTS = {1, 1};
    // Cumulative listener records required after each send, per case. The join
    // execution produces no output until both join windows hold an event.
    private static final int[][] CUMULATIVE_RECORDS = {
            {1},
            {0, 1}
    };

    // Statement output-type tables: property name and the rendered Java type
    // simple name, asserted against the deployed statements' event type
    // property descriptors per the Java source (which pins count, per-name
    // types and the Map underlying type, not declaration order).
    private static final String[][] NO_JOIN_TYPES = {
            {"a", "Integer"}, {"b", "Integer"}, {"s0", "SupportBean"}, {"s1", "SupportBean"}
    };
    private static final String[][] JOIN_TYPES = {
            {"intPrimitive", "Integer"}, {"s1stream", "SupportMarketDataBean"},
            {"s0stream", "SupportBean"}, {"sym", "String"}, {"theString", "String"}
    };

    private EPLOtherSelectExprStreamSelectorScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: EPLOtherSelectExprStreamSelectorScenarioOracle <scenario.json>");
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
        if (records.size() != 2) {
            throw new IllegalStateException("expected 2 listener records, got " + records.size());
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
            throw new IllegalArgumentException("scenario must contain exactly two cases");
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
                    || !epl(index).equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("scenario case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 9) {
            throw new IllegalArgumentException("scenario must contain exactly 9 steps");
        }
        int index = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            index = validateCaseBlock(steps, index, caseIndex);
        }
        if (index != steps.size()) {
            throw new IllegalArgumentException("scenario contains trailing steps");
        }
    }

    private static String epl(int caseIndex) {
        return caseIndex == 0 ? EPL_NO_JOIN : EPL_JOIN;
    }

    private static int validateCaseBlock(JsonArray steps, int index, int caseIndex) {
        String caseName = CASES[caseIndex];
        validateCaseMarker(steps, index++, caseName);
        int deploys = 0;
        int sends = 0;
        if (caseIndex == 0) {
            validateDeploy(steps, index++, caseName, "s0", EPL_NO_JOIN);
            deploys++;
            validateBeanSend(steps, index++, caseName, "E1", 12);
            sends++;
        } else {
            validateDeploy(steps, index++, caseName, "s0", EPL_JOIN);
            deploys++;
            // The bean send alone leaves the inner join incomplete and must
            // not produce output; the market data send completes it.
            validateBeanSend(steps, index++, caseName, "E1", 13);
            sends++;
            validateMarketDataSend(steps, index++, caseName, "E2", 0, 0, "");
            sends++;
        }
        validateUndeployAll(steps, index++, caseName);
        if (deploys != DEPLOY_COUNTS[caseIndex] || sends != SEND_COUNTS[caseIndex]) {
            throw new IllegalArgumentException("case " + caseName + " step shape is not pinned");
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

    private static void validateDeploy(JsonArray steps, int index, String caseName,
                                       String expectedStatement, String expectedEPL) {
        JsonObject step = object(steps.get(index), "deploy step " + index);
        requireFields(step, "op", "case", "statement", "epl");
        if (!"deploy".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEPL.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("deploy step mismatch at step " + index);
        }
    }

    private static void validateMarketDataSend(JsonArray steps, int index, String caseName,
                                               String symbol, int price, long volume, String feed) {
        JsonObject step = sendStep(steps, index, caseName);
        if (!"SupportMarketDataBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("step " + index + " must send SupportMarketDataBean");
        }
        JsonObject payload = object(step.get("payload"), "SupportMarketDataBean payload " + index);
        requireFields(payload, "symbol", "price", "volume", "feed");
        boolean feedMatches = feed == null
                ? payload.get("feed").isNull()
                : !payload.get("feed").isNull() && feed.equals(string(payload, "feed"));
        if (!symbol.equals(string(payload, "symbol"))
                || longNumber(payload, "price") != price
                || longNumber(payload, "volume") != volume
                || !feedMatches) {
            throw new IllegalArgumentException("SupportMarketDataBean payload mismatch at step " + index);
        }
    }

    private static void validateBeanSend(JsonArray steps, int index, String caseName,
                                         String theString, int intPrimitive) {
        JsonObject step = sendStep(steps, index, caseName);
        if (!"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("step " + index + " must send SupportBean");
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload " + index);
        requireFields(payload, "theString", "intPrimitive");
        boolean stringMatches = theString == null
                ? payload.get("theString").isNull()
                : theString.equals(string(payload, "theString"));
        if (!stringMatches || integer(payload, "intPrimitive") != intPrimitive) {
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
        // Strict replay: a listener or engine exception must fail the oracle
        // instead of being swallowed by the default runtime handler.
        configuration.getRuntime().getExceptionHandling()
                .addClass(ThrowingExceptionHandlerFactory.class);
        addCaseTypes(configuration);

        String runtimeURI = "parity-" + ID + "-" + RUNTIME_IDS[caseIndex];
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            CaseContext context = new CaseContext(records, caseIndex, runtime);
            int deploys = 0;
            int sends = 0;
            for (int offset = 0; offset < stepCount; offset++) {
                JsonObject step = object(allSteps.get(stepOffset + offset), "step " + (stepOffset + offset));
                String operation = string(step, "op");
                if ("case".equals(operation)) {
                    continue;
                }
                if ("deploy".equals(operation)) {
                    deploy(context, string(step, "statement"), string(step, "epl"));
                    deploys++;
                } else if ("send".equals(operation)) {
                    send(context, step);
                    sends++;
                    int cumulative = context.sequence;
                    if (cumulative != CUMULATIVE_RECORDS[caseIndex][sends - 1]) {
                        throw new IllegalStateException("case " + caseName + " had " + cumulative
                                + " listener records after send " + sends + ", expected "
                                + CUMULATIVE_RECORDS[caseIndex][sends - 1]);
                    }
                } else if ("undeploy-all".equals(operation)) {
                    runtime.getDeploymentService().undeployAll();
                } else {
                    throw new IllegalArgumentException("unsupported operation in case " + caseName + ": " + operation);
                }
            }
            if (deploys != DEPLOY_COUNTS[caseIndex] || sends != SEND_COUNTS[caseIndex]) {
                throw new IllegalStateException("case " + caseName + " replayed " + deploys + " deploys and "
                        + sends + " sends, expected " + DEPLOY_COUNTS[caseIndex] + " and " + SEND_COUNTS[caseIndex]);
            }
            if (context.sequence != RECORD_COUNTS[caseIndex]) {
                throw new IllegalStateException("case " + caseName + " produced " + context.sequence
                        + " listener records, expected " + RECORD_COUNTS[caseIndex]);
            }
        } finally {
            runtime.destroy();
        }
    }

    private static void addCaseTypes(Configuration configuration) {
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        configuration.getCommon().addEventType("SupportMarketDataBean", SupportMarketDataBean.class);
    }

    private static void deploy(CaseContext context, String label, String epl) throws Exception {
        String expectedEPL = epl(context.caseIndex);
        if (!"s0".equals(label) || !expectedEPL.equals(epl)) {
            throw new IllegalStateException("case " + context.caseName + " deploy " + label + " is not pinned");
        }
        EPRuntime runtime = context.runtime;
        String deploymentId = context.runtimeURI + "-" + label;
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                epl, new CompilerArguments(runtime.getRuntimePath()));
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                new DeploymentOptions().setDeploymentId(deploymentId));
        EPStatement statement = findStatement(deployment, context.caseName, label);
        assertOutputType(context, statement);
        context.attach(statement);
        context.deploys++;
    }

    private static EPStatement findStatement(EPDeployment deployment, String caseName, String label) {
        EPStatement[] statements = deployment.getStatements();
        if (statements == null) {
            throw new IllegalStateException("case " + caseName + " deploy " + label + " deployed no statements");
        }
        for (EPStatement statement : statements) {
            if ("s0".equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("case " + caseName + " deploy " + label
                + " did not contain statement s0");
    }

    private static void assertOutputType(CaseContext context, EPStatement statement) {
        String[][] expected = context.caseIndex == 0 ? NO_JOIN_TYPES : JOIN_TYPES;
        EventPropertyDescriptor[] descriptors = statement.getEventType().getPropertyDescriptors();
        if (descriptors.length != expected.length) {
            throw new IllegalStateException("case " + context.caseName + " output type has "
                    + descriptors.length + " properties, expected " + expected.length);
        }
        Set<String> seen = new HashSet<>();
        for (EventPropertyDescriptor descriptor : descriptors) {
            String actualType = descriptor.getPropertyType() == null
                    ? "null"
                    : descriptor.getPropertyType().getSimpleName();
            boolean matched = false;
            for (String[] entry : expected) {
                if (entry[0].equals(descriptor.getPropertyName())) {
                    if (!entry[1].equals(actualType)) {
                        throw new IllegalStateException("case " + context.caseName + " output type mismatch at "
                                + descriptor.getPropertyName() + ": " + actualType + ", expected "
                                + entry[0] + ":" + entry[1]);
                    }
                    matched = true;
                    break;
                }
            }
            if (!matched || !seen.add(descriptor.getPropertyName())) {
                throw new IllegalStateException("case " + context.caseName + " output type has unexpected property "
                        + descriptor.getPropertyName());
            }
        }
        if (statement.getEventType().getUnderlyingType() != Map.class) {
            throw new IllegalStateException("case " + context.caseName + " output underlying type is "
                    + statement.getEventType().getUnderlyingType() + ", expected java.util.Map");
        }
    }

    private static void send(CaseContext context, JsonObject step) throws Exception {
        String eventType = string(step, "eventType");
        JsonObject payload = object(step.get("payload"), eventType + " payload");
        Object event;
        switch (eventType) {
            case "SupportBean": {
                requireFields(payload, "theString", "intPrimitive");
                SupportBean bean = new SupportBean(
                        nullableString(payload, "theString"),
                        integer(payload, "intPrimitive"));
                context.lastBean = bean;
                event = bean;
                break;
            }
            case "SupportMarketDataBean": {
                requireFields(payload, "symbol", "price", "volume", "feed");
                SupportMarketDataBean marketData = new SupportMarketDataBean(
                        string(payload, "symbol"),
                        longNumber(payload, "price"),
                        longNumber(payload, "volume"),
                        nullableString(payload, "feed"));
                context.lastMarketData = marketData;
                event = marketData;
                break;
            }
            default:
                throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
        context.runtime.getEventService().sendEventBean(event, eventType);
    }

    /**
     * Rethrows the first listener or engine exception so that a replay
     * divergence fails the oracle run instead of being logged and dropped.
     */
    public static final class ThrowingExceptionHandlerFactory implements ExceptionHandlerFactory {
        @Override
        public ExceptionHandler getHandler(ExceptionHandlerFactoryContext context) {
            return contextParam -> {
                throw new IllegalStateException("engine exception during replay",
                        contextParam.getThrowable());
            };
        }
    }

    private static final class CaseContext {
        private final JsonArray records;
        private final int caseIndex;
        private final String caseName;
        private final EPRuntime runtime;
        private final String runtimeURI;
        private int deploys;
        private int sequence;
        private SupportMarketDataBean lastMarketData;
        private SupportBean lastBean;

        private CaseContext(JsonArray records, int caseIndex, EPRuntime runtime) {
            this.records = records;
            this.caseIndex = caseIndex;
            this.caseName = CASES[caseIndex];
            this.runtime = runtime;
            this.runtimeURI = "parity-" + ID + "-" + RUNTIME_IDS[caseIndex];
        }

        private void attach(EPStatement statement) {
            statement.addListener(new TraceWriter(this, statement));
        }
    }

    private static final class TraceWriter implements UpdateListener {
        private final CaseContext context;
        private final EPStatement statement;

        private TraceWriter(CaseContext context, EPStatement statement) {
            this.context = context;
            this.statement = statement;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents,
                           EPStatement ignoredStatement, EPRuntime ignoredRuntime) {
            String caseName = context.caseName;
            boolean hasNew = newEvents != null && newEvents.length > 0;
            boolean hasOld = oldEvents != null && oldEvents.length > 0;
            if (!hasNew || hasOld || newEvents.length != 1) {
                throw new IllegalStateException("case " + caseName
                        + " listener callback must contain one new-only row");
            }
            if (context.sequence >= RECORD_COUNTS[context.caseIndex]) {
                throw new IllegalStateException("case " + caseName + " produced too many callbacks");
            }
            int nextSequence = context.sequence + 1;
            JsonObject fields = context.caseIndex == 0
                    ? renderNoJoinRow(newEvents[0])
                    : renderJoinRow(newEvents[0]);
            long now = context.runtime.getEventService().getCurrentTime();
            if (now != 0L) {
                throw new IllegalStateException("case " + caseName + " callback time at sequence "
                        + nextSequence + " was " + now + ", expected 0");
            }
            context.sequence = nextSequence;
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", nextSequence)
                    .add("time", Instant.ofEpochMilli(now).toString())
                    .add("new", new JsonArray().add(row(fields)));
            context.records.add(record);
        }

        private JsonObject renderNoJoinRow(EventBean event) {
            assertFieldNames(event, new String[]{"a", "b", "s0", "s1"});
            Object s0 = unwrap(event.get("s0"));
            Object s1 = unwrap(event.get("s1"));
            if (s0 != context.lastBean || s1 != context.lastBean) {
                throw new IllegalStateException("case " + context.caseName
                        + " no-join row does not carry the stream-dot columns of the sent bean by identity");
            }
            Object a = event.get("a");
            Object b = event.get("b");
            if (!sameScalar(context.lastBean.getIntPrimitive(), a)
                    || !sameScalar(context.lastBean.getIntPrimitive(), b)) {
                throw new IllegalStateException("case " + context.caseName + " no-join row alias mismatch");
            }
            JsonObject fields = new JsonObject()
                    .add("a", normalize(a))
                    .add("b", normalize(b))
                    .add("s0", beanRow(context.lastBean))
                    .add("s1", beanRow(context.lastBean));
            return fields;
        }

        private JsonObject renderJoinRow(EventBean event) {
            assertFieldNames(event, new String[]{"intPrimitive", "s0stream", "s1stream", "sym", "theString"});
            Object s0stream = unwrap(event.get("s0stream"));
            Object s1stream = unwrap(event.get("s1stream"));
            if (s0stream != context.lastBean || s1stream != context.lastMarketData) {
                throw new IllegalStateException("case " + context.caseName
                        + " join row does not carry the joined stream events by identity");
            }
            Object intPrimitive = event.get("intPrimitive");
            Object sym = event.get("sym");
            Object theString = event.get("theString");
            if (!sameScalar(context.lastBean.getIntPrimitive(), intPrimitive)
                    || !context.lastMarketData.getSymbol().equals(sym)
                    || !context.lastBean.getTheString().equals(theString)) {
                throw new IllegalStateException("case " + context.caseName + " join row property mismatch");
            }
            JsonObject fields = new JsonObject()
                    .add("intPrimitive", normalize(intPrimitive))
                    .add("s0stream", beanRow(context.lastBean))
                    .add("s1stream", marketDataRow(context.lastMarketData))
                    .add("sym", normalize(sym))
                    .add("theString", normalize(theString));
            return fields;
        }

        private Object unwrap(Object value) {
            // Join stream-as-object columns arrive as EventBean wrappers over
            // the exact sent instance; single-stream stream-dot columns carry
            // the instance itself. Identity is checked after unwrapping.
            if (value instanceof EventBean) {
                return ((EventBean) value).getUnderlying();
            }
            return value;
        }

        private void assertFieldNames(EventBean event, String[] expectedSorted) {
            String[] names = event.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            if (!Arrays.equals(names, expectedSorted)) {
                throw new IllegalStateException("case " + context.caseName + " field metadata mismatch: "
                        + Arrays.toString(names));
            }
        }

        private JsonObject marketDataRow(SupportMarketDataBean marketData) {
            JsonObject fields = new JsonObject()
                    .add("feed", normalize(marketData.getFeed()))
                    .add("price", normalize(marketData.getPrice()))
                    .add("symbol", normalize(marketData.getSymbol()))
                    .add("volume", normalize(marketData.getVolume()));
            return row(fields);
        }

        private JsonObject beanRow(SupportBean bean) {
            JsonObject fields = new JsonObject()
                    .add("intPrimitive", normalize(bean.getIntPrimitive()))
                    .add("theString", normalize(bean.getTheString()));
            return row(fields);
        }

        private JsonObject row(JsonObject fields) {
            return new JsonObject().add("kind", "row").add("fields", fields);
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof String) {
                return Json.value((String) value);
            }
            if (value instanceof Integer || value instanceof Short || value instanceof Byte) {
                return Json.value(((Number) value).intValue());
            }
            if (value instanceof Long) {
                return Json.value(((Long) value).longValue());
            }
            if (value instanceof Double || value instanceof Float) {
                double candidate = ((Number) value).doubleValue();
                if (!Double.isFinite(candidate) || Math.rint(candidate) != candidate) {
                    throw new IllegalStateException("non-integral number is not part of the pinned surface: "
                            + candidate);
                }
                return Json.value((long) candidate);
            }
            if (value instanceof Boolean) {
                return Json.value((Boolean) value);
            }
            throw new IllegalStateException("unsupported value type " + value.getClass().getName());
        }

        private boolean sameScalar(Object expected, Object actual) {
            if (expected == null) {
                return actual == null;
            }
            if (actual instanceof Number && expected instanceof Number) {
                return Double.compare(((Number) actual).doubleValue(), ((Number) expected).doubleValue()) == 0;
            }
            return expected.equals(actual);
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

    private static String nullableString(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || value.isNull()) {
            return null;
        }
        if (!value.isString()) {
            throw new IllegalArgumentException(name + " must be a JSON string or null");
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
