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
import com.espertech.esper.common.internal.support.SupportBeanComplexProps;
import com.espertech.esper.regressionlib.support.bean.SupportBean_ST0_Container;
import com.espertech.esper.regressionlib.support.bean.SupportCollection;
import com.espertech.esper.regressionlib.support.bean.SupportEventWithManyArray;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.lang.reflect.Array;
import java.math.BigDecimal;
import java.math.BigInteger;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.Collection;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the six remaining ExprEnum executions (Draft
 * 4.554) replayed as one differential chain:
 *
 * avg-scalarmore (ExprEnumAverage ord 2, ExprEnumAverageScalarMore):
 * strvals.average over SupportCollection.strvals through the
 * extractNum/extractBigDecimal single-row functions (substring(1) parsed as
 * Integer/BigDecimal) with the element, element+index and element+index+size
 * lambda footprints plus a case-when null branch; null and empty collections
 * both yield null.
 *
 * avg-invalid (ExprEnumAverage ord 3, ExprEnumAverageInvalid): three
 * tryInvalidCompile probes record the pinned Java message prefixes; all
 * compile without the runtime path, mirroring env.tryInvalidCompile's
 * path-less compileWCheckedEx.
 *
 * distinct-eventsmultikey (ExprEnumDistinct ord 2,
 * ExprEnumDistinctEventsMultikeyWArray): the wildcard keepall subquery over
 * SupportEventWithManyArray is deduplicated by the intOne int[] key (array
 * value equality) when the SupportBean trigger arrives.
 *
 * distinct-scalarmultikey (ExprEnumDistinct ord 3,
 * ExprEnumDistinctScalarMultikeyWArray): intArrayCollection.distinctOf with
 * and without the identity selector deduplicates int[] elements by array
 * value.
 *
 * allofanyof-invalid (ExprEnumAllOfAnyOf ord 2, ExprEnumAllOfAnyOfInvalid):
 * three tryInvalidCompile probes pin the int-typed and null-typed lambda
 * result rejections.
 *
 * enum-invalid (ExprEnumInvalid ord 0, direct execution): twenty
 * tryInvalidCompile probes pin the take/where/takeLast/average/firstof
 * footprint rejections and the filter-expression, UDF-lambda, subselect and
 * chained-property boundaries.
 *
 * Mirroring SupportEvalRunner, each listener case deploys "@name('s0') <case
 * EPL>" once, sends every assertion event, then undeploys. The pinned case
 * EPLs are the deployed text verbatim (the `as c0` aliases SupportEvalRunner
 * renders and the double space in "average( (v, i)" and "c0  from").
 * SupportBean and SupportBeanComplexProps are the shared common beans;
 * SupportBean_Container, SupportCollection, SupportBean_ST0 and
 * SupportBean_ST0_Container are bean classes compiled from regression-lib;
 * only SupportEventWithManyArray is a map type (3 fields) so select * stays
 * deterministic. beans/contained are declared as event-type arrays, strvals
 * as String[], intOne as int[] and intArrayCollection as int[][]. BigDecimal
 * results render as stripped
 * trailing-zero decimal strings, matching the Go trace normalizer; Java
 * Collections and (possibly primitive) arrays render as JSON arrays. The
 * TraceWriter skips null/null listener callbacks.
 */
public final class ExprEnumRemainder554ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "expr-enum-remainder-554";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod";

    private static final String DESCRIPTION =
            "ExprEnumAverage/ExprEnumDistinct/ExprEnumAllOfAnyOf/ExprEnumInvalid remainders (Draft 4.554): avg-scalarmore replays ExprEnumAverageScalarMore — extractNum/extractBigDecimal averages over SupportCollection.strvals with element/index/size lambda footprints and a case-when null branch across four assertion sends; avg-invalid records ExprEnumAverageInvalid's three tryInvalidCompile probes; distinct-eventsmultikey replays ExprEnumDistinctEventsMultikeyWArray — a keepall-subquery wildcard collection deduplicated by the intOne array key; distinct-scalarmultikey replays ExprEnumDistinctScalarMultikeyWArray — a Collection<int[]> deduplicated by array value; allofanyof-invalid records ExprEnumAllOfAnyOfInvalid's three probes; enum-invalid records ExprEnumInvalid's twenty probes. Each listener case deploys s0 once, sends every assertion event, then undeploys; invalid cases run their probes without deploying.";

    private static final String[] CASES = {
            "avg-scalarmore", "avg-invalid", "distinct-eventsmultikey",
            "distinct-scalarmultikey", "allofanyof-invalid", "enum-invalid"};
    private static final int[] ORDINALS = {2, 3, 2, 3, 2, 0};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-4f1f81149f4970299646",
            "java-runtime-27af41592ebb2ee9651c",
            "java-runtime-717513223b2fe234ebf7",
            "java-runtime-f36948008b15e17c460c",
            "java-runtime-31307e312d939157b9d5",
            "java-runtime-6cd8336511345daaec7c"
    };
    private static final String[] EXECUTIONS = {
            "ExprEnumAverageScalarMore",
            "ExprEnumAverageInvalid",
            "ExprEnumDistinctEventsMultikeyWArray",
            "ExprEnumDistinctScalarMultikeyWArray",
            "ExprEnumAllOfAnyOfInvalid",
            "ExprEnumInvalid"
    };
    private static final String[] STATIC_IDS = {
            "java-2ba13d05d6e0ccaa16cb",
            "java-2ba13d05d6e0ccaa16cb",
            "java-8760b3322be03e680d95",
            "java-8760b3322be03e680d95",
            "java-46e96fc25ee5a674ded5",
            "java-5a117b08f2037d0e6133"
    };
    private static final String[] OBSERVATIONS = {
            "listener",
            "compile-error; three tryInvalidCompile probes pin the Java message prefixes: strvals.average() rejects the 0-parameter footprint over a string collection, beans.average() rejects the 0-parameter footprint over a collection of events and strvals.average(v => null) rejects a null-typed lambda result; all compile without the runtime path",
            "listener",
            "listener",
            "compile-error; three tryInvalidCompile probes pin the Java message prefixes: contained.allOf(x => 1) and contained.anyOf(x => 1) reject int-typed lambda results and contained.anyOf(x => null) rejects a null-typed lambda result; all compile without the runtime path",
            "compile-error; twenty tryInvalidCompile probes pin the Java message prefixes across the take/where/takeLast/average/firstof footprints, the filter-expression, UDF-lambda, subselect and chained-property boundaries; all compile without the runtime path"
    };
    private static final String[] CASE_EPLS = {
            "select strvals.average(v => extractNum(v)) as c0, strvals.average(v => extractBigDecimal(v)) as c1, strvals.average( (v, i) => extractNum(v) + i*10) as c2, strvals.average( (v, i) => extractBigDecimal(v) + i*10) as c3, strvals.average( (v, i, s) => extractNum(v) + i*10 + s*100) as c4, strvals.average( (v, i, s) => extractBigDecimal(v) + i*10 + s*100) as c5, strvals.average( (v, i, s) => case when i = 1 then null else 2 end) as c6 from SupportCollection",
            "select strvals.average() from SupportCollection",
            "select (select * from SupportEventWithManyArray#keepall).distinctOf(r => r.intOne) as c0  from SupportBean",
            "select intArrayCollection.distinctOf() as c0, intArrayCollection.distinctOf(v => v) as c1 from SupportEventWithManyArray",
            "select contained.allOf(x => 1) from SupportBean_ST0_Container",
            "select contained.take() from SupportBean_ST0_Container"
    };

    // Verbatim transcriptions of the tryInvalidCompile calls in
    // ExprEnumAverage/ExprEnumAllOfAnyOf/ExprEnumInvalid (message prefixes
    // only; JVM FQN, declared-type and trailing-EPL suffixes diverge by
    // design).
    private static final String[][] PROBE_STATEMENTS = {
            {},
            {"average-no-param", "average-beans-no-param", "average-null-lambda"},
            {},
            {},
            {"allof-int-result", "anyof-int-result", "anyof-null-lambda"},
            {"enum-take-no-param", "enum-where-primitive-array",
                    "enum-where-unknown-property", "enum-filter-products-where",
                    "enum-unknown-method", "enum-udf-lambda", "enum-static-lambda",
                    "enum-take-string-count", "enum-take-lambda-count",
                    "enum-where-four-param", "enum-where-no-param",
                    "enum-takelast-no-param", "enum-where-two-lambdas",
                    "enum-where-non-lambda", "enum-where-two-params",
                    "enum-subselect-multi-where", "enum-subselect-single-where",
                    "enum-aggregate-where", "enum-average-string-result",
                    "enum-firstof-chained"}
    };
    private static final String[][] PROBE_EPLS = {
            {},
            {"select strvals.average() from SupportCollection",
                    "select beans.average() from SupportBean_Container",
                    "select strvals.average(v => null) from SupportCollection"},
            {},
            {},
            {"select contained.allOf(x => 1) from SupportBean_ST0_Container",
                    "select contained.anyOf(x => 1) from SupportBean_ST0_Container",
                    "select contained.anyOf(x => null) from SupportBean_ST0_Container"},
            {"select contained.take() from SupportBean_ST0_Container",
                    "select arrayProperty.where(x=>x.boolPrimitive) from SupportBeanComplexProps",
                    "select contained.where(x=>x.dummy = 1) from SupportBean_ST0_Container",
                    "select * from SupportBean(products.where(p => code = '1'))",
                    "select contained.notAMethod(x=>x.boolPrimitive) from SupportBean_ST0_Container",
                    "select makeTest(x=>1) from SupportBean_ST0_Container",
                    "select SupportBean_ST0_Container.makeTest(x=>1) from SupportBean_ST0_Container",
                    "select contained.take('a') from SupportBean_ST0_Container",
                    "select contained.take(x => x.p00) from SupportBean_ST0_Container",
                    "select contained.where((x,y,z,a) => true) from SupportBean_ST0_Container",
                    "select contained.where() from SupportBean_ST0_Container",
                    "select window(intPrimitive).takeLast() from SupportBean#length(2)",
                    "select contained.where(x=>true,y=>true) from SupportBean_ST0_Container",
                    "select contained.where(1) from SupportBean_ST0_Container",
                    "select contained.where(1,2) from SupportBean_ST0_Container",
                    "select (select theString, intPrimitive from SupportBean#lastevent).where(x=>x.boolPrimitive) from SupportBean_ST0",
                    "select (select theString from SupportBean#lastevent).where(x=>x.boolPrimitive) from SupportBean_ST0",
                    "select avg(intPrimitive).where(x=>x.boolPrimitive) from SupportBean_ST0",
                    "select contained.average(x => x.id) from SupportBean_ST0_Container",
                    "select contained.firstof().dummy from SupportBean_ST0_Container"}
    };
    private static final String[][] PROBE_ERRORS = {
            {},
            {"Failed to validate select-clause expression 'strvals.average()': Invalid input for built-in enumeration method 'average' and 0-parameter footprint, expecting collection of numeric values as input, received ",
                    "Failed to validate select-clause expression 'beans.average()': Invalid input for built-in enumeration method 'average' and 0-parameter footprint, expecting collection of values (typically scalar values) as input, received collection of events of type '",
                    "Failed to validate select-clause expression 'strvals.average()': Failed to validate enumeration method 'average', expected a non-null result for expression parameter 0 but received a null-typed expression"},
            {},
            {},
            {"Failed to validate select-clause expression 'contained.allOf()': Failed to validate enumeration method 'allOf', expected a boolean-type result for expression parameter 0 but received int",
                    "Failed to validate select-clause expression 'contained.anyOf()': Failed to validate enumeration method 'anyOf', expected a boolean-type result for expression parameter 0 but received int",
                    "Failed to validate select-clause expression 'contained.anyOf()': Failed to validate enumeration method 'anyOf', expected a non-null result for expression parameter 0 but received a null-typed expression"},
            {"Failed to validate select-clause expression 'contained.take()': Parameters mismatch for enumeration method 'take', the method requires an (non-lambda) expression providing count ",
                    "Failed to validate select-clause expression 'arrayProperty.where()': Failed to validate enumeration method 'where' parameter 0: Failed to validate declared expression body expression 'x.boolPrimitive': Failed to resolve property 'x.boolPrimitive' to a stream or nested property in a stream ",
                    "Failed to validate select-clause expression 'contained.where()': Failed to validate enumeration method 'where' parameter 0: Failed to validate declared expression body expression 'x.dummy=1': Failed to resolve property 'x.dummy' to a stream or nested property in a stream ",
                    "Failed to validate filter expression 'products.where()': Failed to resolve 'products.where' to a property, single-row function, aggregation function, script, stream or class name ",
                    "Failed to validate select-clause expression 'contained.notAMethod()': Could not find event property or method named 'notAMethod' in collection of events of type '",
                    "Failed to validate select-clause expression 'makeTest()': Unrecognized lambda-expression encountered as parameter to UDF or static method 'makeTest' ",
                    "Failed to validate select-clause expression 'SupportBean_ST0_Container.makeTest()': Unrecognized lambda-expression encountered as parameter to UDF or static method 'makeTest' ",
                    "Failed to validate select-clause expression 'contained.take('a')': Failed to resolve enumeration method, date-time method or mapped property 'contained.take('a')': Failed to validate enumeration method 'take', expected a number-type result for expression parameter 0 but received String ",
                    "Failed to validate select-clause expression 'contained.take()': Parameters mismatch for enumeration method 'take', the method requires an (non-lambda) expression providing count, but receives a lambda expression ",
                    "Failed to validate select-clause expression 'contained.where()': Parameters mismatch for enumeration method 'where', the method requires a lambda expression providing predicate, but receives a 4-parameter lambda expression",
                    "Failed to validate select-clause expression 'contained.where()': Parameters mismatch for enumeration method 'where', the method has multiple footprints accepting a lambda expression providing predicate, or a 2-parameter lambda expression providing (predicate, index), or a 3-parameter lambda expression providing (predicate, index, size), but receives no parameters",
                    "Failed to validate select-clause expression 'window(intPrimitive).takeLast()': Parameters mismatch for enumeration method 'takeLast', the method requires an (non-lambda) expression providing count ",
                    "Failed to validate select-clause expression 'contained.where(,)': Parameters mismatch for enumeration method 'where', the method has multiple footprints accepting a lambda expression providing predicate, or a 2-parameter lambda expression providing (predicate, index), or a 3-parameter lambda expression providing (predicate, index, size), but receives a lambda expression and a lambda expression",
                    "Failed to validate select-clause expression 'contained.where(1)': Parameters mismatch for enumeration method 'where', the method requires a lambda expression providing predicate, but receives an (non-lambda) expression ",
                    "Failed to validate select-clause expression 'contained.where(1,2)': Parameters mismatch for enumeration method 'where', the method has multiple footprints accepting a lambda expression providing predicate, or a 2-parameter lambda expression providing (predicate, index), or a 3-parameter lambda expression providing (predicate, index, size), but receives an (non-lambda) expression and an (non-lambda) expression",
                    "Failed to validate select-clause expression 'theString.where()': Failed to validate enumeration method 'where' parameter 0: Failed to validate declared expression body expression 'x.boolPrimitive': Failed to resolve property 'x.boolPrimitive' to a stream or nested property in a stream ",
                    "Failed to validate select-clause expression 'theString.where()': Failed to validate enumeration method 'where' parameter 0: Failed to validate declared expression body expression 'x.boolPrimitive': Failed to resolve property 'x.boolPrimitive' to a stream or nested property in a stream ",
                    "Failed to validate select-clause expression 'avg(intPrimitive).where()': Failed to validate method-chain parameter expression 'intPrimitive': Property named 'intPrimitive' is not valid in any stream",
                    "Failed to validate select-clause expression 'contained.average()': Failed to validate enumeration method 'average', expected a number-type result for expression parameter 0 but received String ",
                    "Failed to validate select-clause expression 'contained.firstof().dummy': Failed to resolve method 'dummy': Could not find enumeration method, date-time method, instance method or property named 'dummy' in class '"}
    };

    private static final int EXPECTED_STEPS = 51;
    private static final int EXPECTED_RECORDS = 32;

    private ExprEnumRemainder554ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ExprEnumRemainder554ScenarioOracle <scenario.json>");
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
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS
                    + " records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", SCENARIO_ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, String caseName, String runtimeId,
                                int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBeanComplexProps.class);
        configuration.getCommon().addEventType(SupportCollection.class);
        configuration.getCommon().addEventType(SupportBean_ST0_Container.class);
        configuration.getCommon().addEventType(com.espertech.esper.regressionlib.support.bean.SupportBean_ST0.class);
        configuration.getCommon().addEventType(com.espertech.esper.regressionlib.support.bean.SupportBean_Container.class);
        // Mirroring TestSuiteExprEnum.configure: SupportEventWithManyArray
        // registers as a three-property map type so the Go fixture struct
        // pins the event schema the distinct cases read (id, intOne and
        // intArrayCollection) without the bean's other twenty-five fields.
        Map<String, Object> manyArrayType = new HashMap<>();
        manyArrayType.put("id", String.class);
        manyArrayType.put("intOne", int[].class);
        manyArrayType.put("intArrayCollection", int[][].class);
        configuration.getCommon().addEventType("SupportEventWithManyArray", manyArrayType);
        // The class import resolves `SupportBean_ST0_Container.makeTest`
        // and the plug-in single-row function resolves bare `makeTest`,
        // matching the enum suite's UDF registrations.
        configuration.getCommon().addImport(SupportBean_ST0_Container.class);
        configuration.getCompiler().addPlugInSingleRowFunction("makeTest",
                ExprEnumRemainder554ScenarioOracle.class.getName(), "makeTest");
        configuration.getCompiler().addPlugInSingleRowFunction("extractNum",
                ExprEnumRemainder554ScenarioOracle.class.getName(), "extractNum");
        configuration.getCompiler().addPlugInSingleRowFunction("extractBigDecimal",
                ExprEnumRemainder554ScenarioOracle.class.getName(), "extractBigDecimal");

        String runtimeURI = "parity-" + SCENARIO_ID + "-" + runtimeId;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            TraceWriter writer = new TraceWriter(records, caseName, runtime);
            boolean active = false;
            int probe = 0;
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
                    String epl = "@name('s0') " + CASE_EPLS[caseIndex];
                    // CompilerArguments(configuration) carries the compiler-level
                    // plug-in single-row functions (extractNum/extractBigDecimal);
                    // the runtime path does not (ContextHashScenarioOracle precedent).
                    EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                            new CompilerArguments(configuration));
                    EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                            new DeploymentOptions().setDeploymentId(SCENARIO_ID + "-" + caseIndex));
                    findStatement(deployment).addListener(writer);
                } else if ("undeploy-all".equals(operation)) {
                    runtime.getDeploymentService().undeployAll();
                } else if ("send".equals(operation)) {
                    sendEvent(runtime, step);
                } else if ("build-error".equals(operation)) {
                    buildErrorStep(runtime, configuration, caseName,
                            PROBE_STATEMENTS[caseIndex][probe], step, records);
                    probe++;
                } else {
                    throw new IllegalStateException("unsupported step op " + operation);
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }
    private static EPStatement findStatement(EPDeployment deployment) {
        for (EPStatement candidate : deployment.getStatements()) {
            if ("s0".equals(candidate.getName())) {
                return candidate;
            }
        }
        throw new IllegalStateException("statement s0 was not deployed");
    }

    /**
     * Mirrors ExprEnumMinMax.MyService.extractNum: Integer.parseInt of the
     * value after its leading 'E' marker.
     */
    public static Integer extractNum(String value) {
        return Integer.parseInt(value.substring(1));
    }

    /**
     * Mirrors ExprEnumMinMax.MyService.extractBigDecimal: new BigDecimal of
     * the value after its leading 'E' marker.
     */
    public static BigDecimal extractBigDecimal(String value) {
        return new BigDecimal(value.substring(1));
    }

    /**
     * Mirrors SupportBean_ST0_Container.makeTest: a UDF whose EPL contract is
     * the rejection of a lambda parameter (the probes never invoke it).
     */
    public static com.espertech.esper.regressionlib.support.bean.SupportBean_ST0 makeTest(String value) {
        return null;
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        if ("SupportCollection".equals(eventType)) {
            SupportCollection bean = new SupportCollection();
            JsonValue strvals = payload.get("strvals");
            if (strvals == null || strvals.isNull()) {
                bean.setStrvals(null);
            } else {
                JsonArray items = strvals.asArray();
                java.util.List<String> list = new java.util.ArrayList<>(items.size());
                for (int index = 0; index < items.size(); index++) {
                    list.add(items.get(index).asString());
                }
                bean.setStrvals(list);
            }
            runtime.getEventService().sendEventBean(bean, "SupportCollection");
        } else if ("SupportEventWithManyArray".equals(eventType)) {
            Map<String, Object> event = new HashMap<>();
            // All three keys are always present (null where the payload omits
            // a value) so the `select *` wildcard row carries the tagged
            // {"state":"null"} cell the Go fixture struct emits.
            JsonValue id = payload.get("id");
            event.put("id", (id != null && !id.isNull()) ? id.asString() : null);
            JsonValue intOne = payload.get("intOne");
            if (intOne != null && !intOne.isNull()) {
                JsonArray items = intOne.asArray();
                int[] ints = new int[items.size()];
                for (int index = 0; index < items.size(); index++) {
                    ints[index] = items.get(index).asInt();
                }
                event.put("intOne", ints);
            } else {
                event.put("intOne", null);
            }
            JsonValue collection = payload.get("intArrayCollection");
            if (collection != null && !collection.isNull()) {
                JsonArray items = collection.asArray();
                int[][] arrays = new int[items.size()][];
                for (int index = 0; index < items.size(); index++) {
                    JsonArray ints = items.get(index).asArray();
                    arrays[index] = new int[ints.size()];
                    for (int inner = 0; inner < ints.size(); inner++) {
                        arrays[index][inner] = ints.get(inner).asInt();
                    }
                }
                event.put("intArrayCollection", arrays);
            } else {
                event.put("intArrayCollection", null);
            }
            runtime.getEventService().sendEventMap(event, "SupportEventWithManyArray");
        } else if ("SupportBean".equals(eventType)) {
            SupportBean bean = new SupportBean(
                    payload.getString("theString", null),
                    payload.getInt("intPrimitive", 0));
            runtime.getEventService().sendEventBean(bean, "SupportBean");
        } else {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
    }

    /**
     * Compiles an expected-invalid probe and emits {"operation":"compile-error"}
     * carrying the pinned expectError prefix after verifying the caught
     * message starts with it (SupportMessageAssertUtil.assertMessage
     * semantics). Probes marked compileWithoutPath compile without the
     * runtime path, mirroring env.tryInvalidCompile's path-less
     * compileWCheckedEx.
     */
    private static void buildErrorStep(EPRuntime runtime, Configuration configuration,
                                       String caseName, String label, JsonObject step,
                                       JsonArray records) {
        String expected = step.getString("expectError", "");
        String epl = step.getString("epl", "");
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            if (!step.getBoolean("compileWithoutPath", false)) {
                compilerArgs.getPath().add(runtime.getRuntimePath());
            }
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught == null || caught.equals("<no-error>")) {
            throw new IllegalStateException("build-error probe " + label
                    + " unexpectedly succeeded");
        }
        if (!expected.isEmpty() && !caught.startsWith(expected)) {
            throw new IllegalStateException("compile-error message drift for " + label
                    + ": expected prefix [" + expected + "] got [" + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "compile-error");
        record.add("statement", step.getString("statement", label));
        record.add("sequence", 0);
        if (!expected.isEmpty()) {
            record.add("value", expected);
        }
        records.add(record);
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !SCENARIO_ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTIONS, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

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
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: each case marker is followed by the
     * case's pinned steps — deploy s0, the assertion sends and undeploy-all
     * for listener cases; the pinned build-error probes and undeploy-all for
     * the three invalid cases. Unknown step fields are rejected.
     */
    private static void validateSteps(JsonArray steps) {
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + EXPECTED_STEPS + " steps, got " + steps.size());
        }
        String[][] details = {
                {"deploy", "send", "send", "send", "send", "undeploy-all"},
                {"build-error", "build-error", "build-error", "undeploy-all"},
                {"deploy", "send", "send", "send", "send", "send", "undeploy-all"},
                {"deploy", "send", "undeploy-all"},
                {"build-error", "build-error", "build-error", "undeploy-all"},
                {"build-error", "build-error", "build-error", "build-error",
                        "build-error", "build-error", "build-error", "build-error",
                        "build-error", "build-error", "build-error", "build-error",
                        "build-error", "build-error", "build-error", "build-error",
                        "build-error", "build-error", "build-error", "build-error",
                        "undeploy-all"},
        };
        String[][] eventTypes = {
                {"SupportCollection", "SupportCollection", "SupportCollection",
                        "SupportCollection"},
                {},
                {"SupportEventWithManyArray", "SupportEventWithManyArray",
                        "SupportEventWithManyArray", "SupportEventWithManyArray",
                        "SupportBean"},
                {"SupportEventWithManyArray"},
                {},
                {},
        };
        int cursor = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(cursor), "case marker " + cursor);
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op")) || !CASES[caseIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker " + cursor + " is not pinned");
            }
            cursor++;
            int probes = 0;
            int sends = 0;
            for (String operation : details[caseIndex]) {
                JsonObject step = object(steps.get(cursor), "step " + cursor);
                if (!operation.equals(string(step, "op"))
                        || !CASES[caseIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("step " + cursor + " is not pinned");
                }
                switch (operation) {
                    case "deploy":
                        requireFields(step, "op", "case", "statement");
                        if (!"s0".equals(string(step, "statement"))) {
                            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
                        }
                        break;
                    case "send":
                        requireFields(step, "op", "case", "eventType", "payload");
                        if (!eventTypes[caseIndex][sends].equals(string(step, "eventType"))) {
                            throw new IllegalArgumentException("send step " + cursor + " is not pinned");
                        }
                        sends++;
                        break;
                    case "build-error":
                        requireFields(step, "op", "case", "statement", "epl", "expectError",
                                "compileWithoutPath");
                        if (!step.getBoolean("compileWithoutPath", false)
                                || !PROBE_STATEMENTS[caseIndex][probes].equals(string(step, "statement"))
                                || !PROBE_EPLS[caseIndex][probes].equals(string(step, "epl"))
                                || !PROBE_ERRORS[caseIndex][probes].equals(string(step, "expectError"))) {
                            throw new IllegalArgumentException("build-error step " + cursor + " is not pinned");
                        }
                        probes++;
                        break;
                    case "undeploy-all":
                        requireFields(step, "op", "case");
                        break;
                    default:
                        throw new IllegalArgumentException("unsupported operation at step " + cursor);
                }
                cursor++;
            }
        }
        if (cursor != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
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

    private static int integer(JsonObject object, String name) {
        long value = longNumber(object, name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        return (int) value;
    }

    private static long longNumber(JsonObject object, String name) {
        if (!(object.get(name) instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        String text = object.get(name).toString();
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
                    .add("sequence", ++sequence)
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

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof BigDecimal) {
                // BigDecimal.average yields BigDecimal(rounded-double) results
                // such as BigDecimal(3.0) whose scale is an artifact of the
                // narrowing; the Go big.Rat normalizer renders the exact
                // decimal, so trailing zeros are stripped on both sides.
                return Json.value(((BigDecimal) value).stripTrailingZeros().toPlainString());
            }
            if (value instanceof BigInteger) {
                return Json.value(value.toString());
            }
            if (value instanceof Collection) {
                JsonArray array = new JsonArray();
                for (Object item : (Collection<?>) value) {
                    array.add(normalize(item));
                }
                return array;
            }
            if (value instanceof Map) {
                // `select *` over the map-typed SupportEventWithManyArray
                // keeps the underlying HashMap; render it as a deterministic
                // JSON object (sorted keys, normalized values) so the Go row
                // shape matches instead of HashMap.toString's [I@ hashcode.
                JsonObject fields = new JsonObject();
                java.util.List<String> keys = new java.util.ArrayList<>();
                for (Object key : ((Map<?, ?>) value).keySet()) {
                    keys.add(String.valueOf(key));
                }
                java.util.Collections.sort(keys);
                for (String key : keys) {
                    fields.add(key, normalize(((Map<?, ?>) value).get(key)));
                }
                return fields;
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
            if (value.getClass().isArray()) {
                // int[]/int[][] and other Java array values (primitive arrays
                // are not Object[]) render as nested JSON arrays.
                JsonArray array = new JsonArray();
                int length = Array.getLength(value);
                for (int index = 0; index < length; index++) {
                    array.add(normalize(Array.get(value, index)));
                }
                return array;
            }
            if (value instanceof Integer || value instanceof Long || value instanceof Short
                    || value instanceof Byte) {
                return Json.value(((Number) value).longValue());
            }
            if (value instanceof Number) {
                double number = ((Number) value).doubleValue();
                if (number == Math.rint(number) && !Double.isInfinite(number)) {
                    return Json.value((long) number);
                }
                return Json.value(number);
            }
            if (value instanceof Boolean) {
                return Json.value((Boolean) value);
            }
            if (value instanceof Character character) {
                return Json.value(String.valueOf(character));
            }
            return Json.value(String.valueOf(value));
        }
    }
}
