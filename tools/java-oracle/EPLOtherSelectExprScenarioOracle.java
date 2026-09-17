import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventPropertyDescriptor;
import com.espertech.esper.common.client.annotation.Description;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.lang.annotation.Annotation;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the Draft 4.445 'epl-other-select-expr' unit:
 * EPLOtherSelectExpr ordinals 0-5.  Each scenario case replays inside its own
 * runtime; there is no virtual time, no SODA phase, no invalid-compile probe
 * and no execution flags.
 *
 * Ordinal 0 (EPLOtherPrecedenceNoColumnName) runs three deploy/send/undeploy
 * cycles of 'select <expr> from SupportBean': '3*2+1' and '(3*2)+1' both
 * produce a column named 3*2+1 (the parenthesized form drops its parens in
 * the generated name) with value 7, while '3*(2+1)' keeps its parens and
 * produces 9.  The generated column name is asserted against the deployed
 * statement's event type before the send, exactly like the suite.
 *
 * Ordinal 1 (EPLOtherGraphSelect) deploys the suite's two statements in one
 * path: an unnamed '@public insert into MyStream select nested from
 * SupportBeanComplexProps' (deploy label "insert", no listener) and the named
 * select that navigates the nested fragment.  One default bean produces one
 * row carrying nestedValue and nestedNestedValue.
 *
 * Ordinal 2 (EPLOtherKeywordsAllowed) deploys the 31-keyword-property select
 * over SupportBeanKeywords; a "types" step pins the property names in exact
 * select order (all Integer) before the send delivers the all-ones row.  The
 * second deploy selects 'escape as stddev, count(*) as count, last'; a second
 * "types" step pins count(*) as Long before the send delivers
 * stddev=1, count=1L, last=1.
 *
 * Ordinal 3 (EPLOtherEscapeString) runs six escape-literal match cycles
 * (A'B via double quotes, backslash-escaped and unicode-escaped single
 * quotes; A"B via single quotes, backslash-escaped and unicode-escaped
 * double quotes), then the annotation phase deploying
 * {@code @Name('A\'B') @Description("A\"B")} whose statement name and
 * Description annotation are asserted at deploy and acknowledged by a
 * "deployed" record, then the constants select ('volume', "sleep",
 * "A"), then four filter matches ('John\'s', 'John's', and two LIKE
 * forms of Quote "Hello").
 *
 * Ordinal 4 (EPLOtherGetEventType) deploys the filtered length-window select
 * with alias-less aliases and sends no events: a "deployed" record
 * acknowledges the deployment and a "types" record pins the four property
 * names/types (the suite asserts the names in any order).
 *
 * Ordinal 5 (EPLOtherWindowStats) deploys the same select with 'as' aliases,
 * sends a and b (boolBoxed=false, no delivery) then c (boolBoxed=true) which
 * produces the single row theString=c, aBool=true, 3*intPrimitive=9,
 * result=30f.
 *
 * Event types mirror the pinned harness with reduced surfaces: SupportBean
 * exposes only the five properties the executions send or select (theString,
 * intPrimitive, boolBoxed, floatPrimitive, floatBoxed); SupportBeanComplexProps
 * is reduced to the nested fragment chain (nested.nestedValue,
 * nested.nestedNested.nestedNestedValue); SupportBeanKeywords exposes the 31
 * selected keyword properties, each returning int 1.
 */
public final class EPLOtherSelectExprScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "epl-other-select-expr";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherSelectExpr.java";
    private static final String DESCRIPTION =
            "EPLOtherSelectExpr ordinals 0-5: three precedence cycles asserting generated column " +
            "names (ord 0), a public insert-into graph select with nested fragment navigation " +
            "(ord 1), a 31-keyword-property select plus an aliased keyword select (ord 2), six " +
            "escape-string match cycles plus an annotation phase, a constants phase, and four " +
            "filter/LIKE matches (ord 3), a no-event schema assertion over a length window " +
            "(ord 4), and a filtered length-window select where only the boolBoxed=true event " +
            "produces a row (ord 5)";

    private static final String[] CASES = {
            "precedence-no-column-name",
            "graph-select",
            "keywords-allowed",
            "escape-string",
            "get-event-type",
            "window-stats"
    };
    private static final int[] ORDINALS = {0, 1, 2, 3, 4, 5};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-7222e4dfd73a239bf53c",
            "java-runtime-a1605a2ba0d017fa91f1",
            "java-runtime-bc47c8b86afd9b19f5c6",
            "java-runtime-446476d182df93abd788",
            "java-runtime-d01d8e953908acc44955",
            "java-runtime-5371694959860be64ff6"
    };
    private static final String[] EXECUTIONS = {
            "EPLOtherPrecedenceNoColumnName",
            "EPLOtherGraphSelect",
            "EPLOtherKeywordsAllowed",
            "EPLOtherEscapeString",
            "EPLOtherGetEventType",
            "EPLOtherWindowStats"
    };
    private static final String[] OBSERVATIONS = {
            "listener; three deploy/send cycles asserting the generated column name on the " +
                    "statement event type before each send",
            "listener; unnamed @public insert-into feeds the named select that navigates the " +
                    "nested fragment",
            "listener; types step pins the 31 keyword property names in exact select order " +
                    "before the all-ones row; second deploy selects aliased keywords",
            "listener; six escape-literal matches, an annotation deploy acknowledged by a " +
                    "deployed record, a constants row, and four filter/LIKE matches",
            "deployed+types; no events are sent, the records pin the four property names " +
                    "and types of the filtered length-window select",
            "listener; only the boolBoxed=true event produces a row"
    };

    private static final String[] JAVA_RUNTIMES = RUNTIME_IDS;
    private static final String[] JAVA_NAMES = EXECUTIONS;
    private static final String[] JAVA_STATIC_IDS = {
            "java-3519a33f082112fa932c",
            "java-cb1340a82d2bd9732f19",
            "java-cb4085e892b41a9ba483",
            "java-e5e782017fcf22950ae7",
            "java-02b24f8edc7d5c4f5cba",
            "java-fdb471eeb6e137e7d462"
    };
    private static final String[] JAVA_FLAGS = {};

    // EPLOtherPrecedenceNoColumnName (ordinal 0) select expressions, byte-exact;
    // the parenthesized '(3*2)+1' drops its parens in the generated column name.
    private static final String[] PRECEDENCE_EXPRS = {"3*2+1", "(3*2)+1", "3*(2+1)"};
    private static final String[] PRECEDENCE_COLUMNS = {"3*2+1", "3*2+1", "3*(2+1)"};
    private static final int[] PRECEDENCE_VALUES = {7, 7, 9};

    // EPLOtherGraphSelect (ordinal 1) EPLs, byte-exact.
    private static final String GRAPH_INSERT_EPL =
            "@public insert into MyStream select nested from SupportBeanComplexProps";
    private static final String GRAPH_SELECT_EPL =
            "@name('s0') select nested.nestedValue, nested.nestedNested.nestedNestedValue from MyStream";

    // EPLOtherKeywordsAllowed (ordinal 2) EPLs, byte-exact; the first select
    // lists the 31 keyword properties in the suite's declaration order.
    private static final String KEYWORDS_FIELDS =
            "count,escape,every,sum,avg,max,min,coalesce,median,stddev,avedev,events,first,last," +
            "unidirectional,pattern,sql,metadatasql,prev,prior,weekday,lastweekday,cast,snapshot," +
            "variable,window,left,right,full,outer,join";
    private static final String KEYWORDS_EPL =
            "@name('s0') select " + KEYWORDS_FIELDS + " from SupportBeanKeywords";
    private static final String KEYWORDS_ALIAS_EPL =
            "@name('s0') select escape as stddev, count(*) as count, last from SupportBeanKeywords";

    // EPLOtherEscapeString (ordinal 3) EPLs, byte-exact including the literal
    // escape and unicode-escape sequences inside the filter expressions.
    private static final String[] ESCAPE_EPLS = {
            "@name('s0') select * from SupportBean(theString=\"A'B\")",
            "@name('s0') select * from SupportBean(theString='A\\'B')",
            "@name('s0') select * from SupportBean(theString='A\\u0027B')",
            "@name('s0') select * from SupportBean(theString='A\"B')",
            "@name('s0') select * from SupportBean(theString='A\\\"B')",
            "@name('s0') select * from SupportBean(theString='A\\u0022B')"
    };
    private static final String[] ESCAPE_STRINGS = {"A'B", "A'B", "A'B", "A\"B", "A\"B", "A\"B"};
    private static final String ANNOTATION_EPL =
            "@Name('A\\'B') @Description(\"A\\\"B\") select * from SupportBean";
    private static final String CONSTANTS_EPL =
            "@name('s0') select 'volume' as field1, \"sleep\" as field2, \"\\u0041\" as unicodeA " +
            "from SupportBean";
    private static final String[] MATCH_EPLS = {
            "@name('s0') select * from SupportBean(theString='John\\'s')",
            "@name('s0') select * from SupportBean(theString='John\\u0027s')",
            "@name('s0') select * from SupportBean(theString like \"Quote \\\"Hello\\\"\")",
            "@name('s0') select * from SupportBean(theString like \"Quote \\u0022Hello\\u0022\")"
    };
    private static final String[] MATCH_STRINGS =
            {"John's", "John's", "Quote \"Hello\"", "Quote \"Hello\""};

    // EPLOtherGetEventType (ordinal 4) and EPLOtherWindowStats (ordinal 5)
    // EPLs, byte-exact including the double space before 'where' produced by
    // the suite's literal concatenation.
    private static final String EVENT_TYPE_EPL =
            "@name('s0') select theString, boolBoxed aBool, 3*intPrimitive, " +
            "floatBoxed+floatPrimitive result from SupportBean#length(3)  where boolBoxed = true";
    private static final String WINDOW_STATS_EPL =
            "@name('s0') select theString, boolBoxed as aBool, 3*intPrimitive, " +
            "floatBoxed+floatPrimitive as result from SupportBean#length(3)  where boolBoxed = true";

    private EPLOtherSelectExprScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: EPLOtherSelectExprScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType("SupportBean", LocalSupportBean.class);
        configuration.getCommon().addEventType("SupportBeanComplexProps", LocalSupportBeanComplexProps.class);
        configuration.getCommon().addEventType("SupportBeanKeywords", LocalSupportBeanKeywords.class);

        String runtimeURI = "parity-" + ID + "-" + runtimeId;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            long[] sequence = {0};
            TraceWriter writer = new TraceWriter(records, caseName, runtime, sequence);
            Map<String, EPStatement> statements = new HashMap<>();
            List<EPCompiled> pathCompileds = new ArrayList<>();
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
                switch (operation) {
                    case "deploy": {
                        String label = step.getString("statement", "");
                        String epl = deployEpl(caseName, deployIndex);
                        if (!label.equals(deployLabel(epl)) || !epl.equals(step.getString("epl", ""))) {
                            throw new IllegalArgumentException("deploy is not pinned for case "
                                    + caseName + " phase " + deployIndex);
                        }
                        deployIndex++;
                        CompilerArguments arguments = new CompilerArguments(configuration);
                        if ("graph-select".equals(caseName)) {
                            // The suite compiles both statements against one
                            // RegressionPath so the second deployment resolves
                            // the @public insert-into stream MyStream.
                            arguments.getPath().addAll(pathCompileds);
                        }
                        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, arguments);
                        pathCompileds.add(compiled);
                        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                                new DeploymentOptions().setDeploymentId(
                                        ID + "-" + caseIndex + "-" + deployIndex));
                        EPStatement[] deployed = deployment.getStatements();
                        if (deployed.length != 1) {
                            throw new IllegalStateException("deployment has " + deployed.length
                                    + " statements, want 1");
                        }
                        EPStatement statement = deployed[0];
                        if ("s0".equals(label)) {
                            if (!"s0".equals(statement.getName())) {
                                throw new IllegalStateException(
                                        "statement name " + statement.getName() + ", want s0");
                            }
                            if ("precedence-no-column-name".equals(caseName)) {
                                // The suite asserts the generated column name on the
                                // statement event type before sending the event.
                                String generated = statement.getEventType().getPropertyNames()[0];
                                if (!PRECEDENCE_COLUMNS[deployIndex - 1].equals(generated)) {
                                    throw new IllegalStateException("generated column name "
                                            + generated + ", want " + PRECEDENCE_COLUMNS[deployIndex - 1]);
                                }
                            }
                            statement.addListener(writer);
                            statements.put("s0", statement);
                        } else if ("A'B".equals(label)) {
                            assertAnnotationStatement(statement);
                            statements.put("A'B", statement);
                        }
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        statements.clear();
                        break;
                    case "send":
                        sendEvent(runtime, step);
                        break;
                    case "deployed": {
                        String label = step.getString("statement", "");
                        if (!statements.containsKey(label)) {
                            throw new IllegalStateException(
                                    "deployed marker for unknown statement " + label);
                        }
                        records.add(new JsonObject()
                                .add("case", caseName)
                                .add("operation", "deployed")
                                .add("statement", label)
                                .add("sequence", ++sequence[0])
                                .add("time", Instant.ofEpochMilli(
                                        runtime.getEventService().getCurrentTime()).toString()));
                        break;
                    }
                    case "types": {
                        String label = step.getString("statement", "");
                        EPStatement statement = statements.get(label);
                        if (statement == null) {
                            throw new IllegalStateException(
                                    "types read for unknown statement " + label);
                        }
                        records.add(typesRecord(caseName, deployIndex, statement, runtime, sequence));
                        break;
                    }
                    default:
                        throw new IllegalArgumentException("unsupported step op " + operation);
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    private static String deployEpl(String caseName, int deployIndex) {
        switch (caseName) {
            case "precedence-no-column-name":
                if (deployIndex < PRECEDENCE_EXPRS.length) {
                    return "@name('s0') select " + PRECEDENCE_EXPRS[deployIndex] + " from SupportBean";
                }
                break;
            case "graph-select":
                if (deployIndex == 0) {
                    return GRAPH_INSERT_EPL;
                }
                if (deployIndex == 1) {
                    return GRAPH_SELECT_EPL;
                }
                break;
            case "keywords-allowed":
                if (deployIndex == 0) {
                    return KEYWORDS_EPL;
                }
                if (deployIndex == 1) {
                    return KEYWORDS_ALIAS_EPL;
                }
                break;
            case "escape-string":
                if (deployIndex < ESCAPE_EPLS.length) {
                    return ESCAPE_EPLS[deployIndex];
                }
                if (deployIndex == 6) {
                    return ANNOTATION_EPL;
                }
                if (deployIndex == 7) {
                    return CONSTANTS_EPL;
                }
                if (deployIndex >= 8 && deployIndex < 8 + MATCH_EPLS.length) {
                    return MATCH_EPLS[deployIndex - 8];
                }
                break;
            case "get-event-type":
                if (deployIndex == 0) {
                    return EVENT_TYPE_EPL;
                }
                break;
            case "window-stats":
                if (deployIndex == 0) {
                    return WINDOW_STATS_EPL;
                }
                break;
            default:
                throw new IllegalArgumentException("unexpected case " + caseName);
        }
        throw new IllegalArgumentException("unexpected deploy index " + deployIndex
                + " for case " + caseName);
    }

    /** Deploy labels: the named select is "s0", the annotation statement is
     * "A'B", and the unnamed insert-into is labeled "insert". */
    private static String deployLabel(String epl) {
        if (epl.contains("@name('s0')")) {
            return "s0";
        }
        if (ANNOTATION_EPL.equals(epl)) {
            return "A'B";
        }
        return "insert";
    }

    /** EPL pinned in the cases[] metadata: the first deploy text per case. */
    private static String caseEpl(String caseName) {
        return deployEpl(caseName, 0);
    }

    /**
     * Mirrors the suite's annotation assertion: the deployed statement is
     * named A'B and its second annotation is Description with value A"B.
     */
    private static void assertAnnotationStatement(EPStatement statement) {
        if (!"A'B".equals(statement.getName())) {
            throw new IllegalStateException("annotation statement name " + statement.getName());
        }
        Annotation[] annotations = statement.getAnnotations();
        if (annotations.length < 2 || !(annotations[1] instanceof Description)
                || !"A\"B".equals(((Description) annotations[1]).value())) {
            throw new IllegalStateException("annotation statement annotations are not pinned");
        }
    }

    /**
     * Emits the ordered property name/type pairs of the deployed statement's
     * event type.  The keywords case pins the 31 names in exact select order
     * (all Integer) like the suite's assertEqualsExactOrder, and its second
     * deploy pins the aliased select's three columns (count(*) is Long like
     * the suite's assertEquals(1L, ...)); the get-event-type case pins the
     * four names in any order with per-name types like the suite's
     * assertEqualsAnyOrder, emitting them in descriptor order.
     */
    private static JsonObject typesRecord(String caseName, int deployIndex,
                                          EPStatement statement, EPRuntime runtime,
                                          long[] sequence) {
        EventPropertyDescriptor[] descriptors = statement.getEventType().getPropertyDescriptors();
        JsonArray value = new JsonArray();
        if ("keywords-allowed".equals(caseName) && deployIndex == 1) {
            String[] names = KEYWORDS_FIELDS.split(",");
            if (descriptors.length != names.length) {
                throw new IllegalStateException("expected " + names.length
                        + " property descriptors, got " + descriptors.length);
            }
            for (int index = 0; index < names.length; index++) {
                if (!names[index].equals(descriptors[index].getPropertyName())
                        || descriptors[index].getPropertyType() != Integer.class) {
                    throw new IllegalStateException("descriptor " + index + " = "
                            + descriptors[index].getPropertyName() + ":"
                            + simpleTypeName(descriptors[index].getPropertyType()));
                }
                value.add(new JsonObject().add("name", names[index]).add("type", "Integer"));
            }
        } else if ("keywords-allowed".equals(caseName) && deployIndex == 2) {
            String[][] expected = {{"stddev", "Integer"}, {"count", "Long"}, {"last", "Integer"}};
            if (descriptors.length != expected.length) {
                throw new IllegalStateException("expected three property descriptors, got "
                        + descriptors.length);
            }
            for (int index = 0; index < expected.length; index++) {
                String actual = simpleTypeName(descriptors[index].getPropertyType());
                if (!expected[index][0].equals(descriptors[index].getPropertyName())
                        || !expected[index][1].equals(actual)) {
                    throw new IllegalStateException("descriptor " + index + " = "
                            + descriptors[index].getPropertyName() + ":" + actual);
                }
                value.add(new JsonObject()
                        .add("name", expected[index][0]).add("type", actual));
            }
        } else if ("get-event-type".equals(caseName)) {
            Map<String, String> expected = new HashMap<>();
            expected.put("theString", "String");
            expected.put("aBool", "Boolean");
            expected.put("3*intPrimitive", "Integer");
            expected.put("result", "Float");
            if (descriptors.length != expected.size()) {
                throw new IllegalStateException("expected four property descriptors, got "
                        + descriptors.length);
            }
            for (EventPropertyDescriptor descriptor : descriptors) {
                String token = expected.remove(descriptor.getPropertyName());
                String actual = simpleTypeName(descriptor.getPropertyType());
                if (token == null || !token.equals(actual)) {
                    throw new IllegalStateException("descriptor " + descriptor.getPropertyName()
                            + " type " + actual);
                }
                value.add(new JsonObject()
                        .add("name", descriptor.getPropertyName()).add("type", actual));
            }
        } else {
            throw new IllegalStateException("unexpected types step for case " + caseName);
        }
        return new JsonObject()
                .add("case", caseName)
                .add("operation", "types")
                .add("statement", statement.getName())
                .add("sequence", ++sequence[0])
                .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString())
                .add("value", value);
    }

    private static String simpleTypeName(Class<?> type) {
        if (type == String.class) {
            return "String";
        }
        if (type == Integer.class || type == int.class) {
            return "Integer";
        }
        if (type == Long.class || type == long.class) {
            return "Long";
        }
        if (type == Float.class || type == float.class) {
            return "Float";
        }
        if (type == Double.class || type == double.class) {
            return "Double";
        }
        if (type == Boolean.class || type == boolean.class) {
            return "Boolean";
        }
        if (type.isArray()) {
            return simpleTypeName(type.getComponentType()) + "[]";
        }
        String simple = type.getSimpleName();
        if (simple.isEmpty()) {
            throw new IllegalStateException("anonymous property type " + type);
        }
        return simple;
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (eventType) {
            case "SupportBean": {
                LocalSupportBean bean = new LocalSupportBean();
                if (payload.get("theString") != null) {
                    bean.setTheString(payload.getString("theString", null));
                }
                bean.setIntPrimitive(payload.getInt("intPrimitive", 0));
                if (payload.get("boolBoxed") != null) {
                    bean.setBoolBoxed(payload.getBoolean("boolBoxed", false));
                }
                bean.setFloatPrimitive((float) payload.getDouble("floatPrimitive", 0d));
                if (payload.get("floatBoxed") != null) {
                    bean.setFloatBoxed((float) payload.getDouble("floatBoxed", 0d));
                }
                runtime.getEventService().sendEventBean(bean, eventType);
                break;
            }
            case "SupportBeanComplexProps": {
                LocalSupportBeanComplexProps bean = new LocalSupportBeanComplexProps(
                        payload.getString("nestedValue", "nestedValue"),
                        payload.getString("nestedNestedValue", "nestedNestedValue"));
                runtime.getEventService().sendEventBean(bean, eventType);
                break;
            }
            case "SupportBeanKeywords":
                runtime.getEventService().sendEventBean(new LocalSupportBeanKeywords(), eventType);
                break;
            default:
                throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags",
                "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), JAVA_RUNTIMES, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), JAVA_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), JAVA_STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), JAVA_FLAGS, "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly six cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !caseEpl(CASES[index]).equals(string(definition, "epl"))) {
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

    private static JsonArray rowsOf(EventBean[] events) {
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

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPRuntime runtime;
        private final long[] sequence;

        private TraceWriter(JsonArray records, String caseName, EPRuntime runtime, long[] sequence) {
            this.records = records;
            this.caseName = caseName;
            this.runtime = runtime;
            this.sequence = sequence;
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
                    .add("sequence", ++sequence[0])
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rowsOf(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", rowsOf(oldEvents));
            }
            records.add(record);
        }
    }

    /** Local mirror of the pinned SupportBean members the executions use. */
    public static class LocalSupportBean {
        private String theString;
        private int intPrimitive;
        private Boolean boolBoxed;
        private float floatPrimitive;
        private Float floatBoxed;

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

        public Boolean getBoolBoxed() {
            return boolBoxed;
        }

        public void setBoolBoxed(Boolean boolBoxed) {
            this.boolBoxed = boolBoxed;
        }

        public float getFloatPrimitive() {
            return floatPrimitive;
        }

        public void setFloatPrimitive(float floatPrimitive) {
            this.floatPrimitive = floatPrimitive;
        }

        public Float getFloatBoxed() {
            return floatBoxed;
        }

        public void setFloatBoxed(Float floatBoxed) {
            this.floatBoxed = floatBoxed;
        }
    }

    /**
     * Local mirror of SupportBeanComplexProps reduced to the nested fragment
     * chain the graph-select execution navigates (nested.nestedValue and
     * nested.nestedNested.nestedNestedValue).
     */
    public static class LocalSupportBeanComplexProps {
        private final LocalNested nested;

        public LocalSupportBeanComplexProps(String nestedValue, String nestedNestedValue) {
            this.nested = new LocalNested(nestedValue, nestedNestedValue);
        }

        public LocalNested getNested() {
            return nested;
        }
    }

    /** Nested fragment bean mirroring SupportBeanSpecialGetterNested. */
    public static class LocalNested {
        private final String nestedValue;
        private final LocalNestedNested nestedNested;

        public LocalNested(String nestedValue, String nestedNestedValue) {
            this.nestedValue = nestedValue;
            this.nestedNested = new LocalNestedNested(nestedNestedValue);
        }

        public String getNestedValue() {
            return nestedValue;
        }

        public LocalNestedNested getNestedNested() {
            return nestedNested;
        }
    }

    /** Innermost fragment bean mirroring SupportBeanSpecialGetterNestedNested. */
    public static class LocalNestedNested {
        private final String nestedNestedValue;

        public LocalNestedNested(String nestedNestedValue) {
            this.nestedNestedValue = nestedNestedValue;
        }

        public String getNestedNestedValue() {
            return nestedNestedValue;
        }
    }

    /** Local mirror of SupportBeanKeywords: every selected keyword property
     * returns int 1, matching the suite's default bean. */
    public static class LocalSupportBeanKeywords {
        public int getCount() {
            return 1;
        }

        public int getEscape() {
            return 1;
        }

        public int getEvery() {
            return 1;
        }

        public int getSum() {
            return 1;
        }

        public int getAvg() {
            return 1;
        }

        public int getMax() {
            return 1;
        }

        public int getMin() {
            return 1;
        }

        public int getCoalesce() {
            return 1;
        }

        public int getMedian() {
            return 1;
        }

        public int getStddev() {
            return 1;
        }

        public int getAvedev() {
            return 1;
        }

        public int getEvents() {
            return 1;
        }

        public int getFirst() {
            return 1;
        }

        public int getLast() {
            return 1;
        }

        public int getUnidirectional() {
            return 1;
        }

        public int getPattern() {
            return 1;
        }

        public int getSql() {
            return 1;
        }

        public int getMetadatasql() {
            return 1;
        }

        public int getPrev() {
            return 1;
        }

        public int getPrior() {
            return 1;
        }

        public int getWeekday() {
            return 1;
        }

        public int getLastweekday() {
            return 1;
        }

        public int getCast() {
            return 1;
        }

        public int getSnapshot() {
            return 1;
        }

        public int getVariable() {
            return 1;
        }

        public int getWindow() {
            return 1;
        }

        public int getLeft() {
            return 1;
        }

        public int getRight() {
            return 1;
        }

        public int getFull() {
            return 1;
        }

        public int getOuter() {
            return 1;
        }

        public int getJoin() {
            return 1;
        }
    }
}
