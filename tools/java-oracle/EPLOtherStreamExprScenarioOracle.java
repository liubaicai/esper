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
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportBeanComplexProps;
import com.espertech.esper.regressionlib.support.bean.SupportBeanStaticOuter;
import com.espertech.esper.regressionlib.support.bean.SupportChainChildTwo;
import com.espertech.esper.regressionlib.support.bean.SupportChainTop;
import com.espertech.esper.regressionlib.support.bean.SupportMarketDataBean;
import com.espertech.esper.regressionlib.suite.epl.other.EPLOtherStreamExpr;
import com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.lang.reflect.Constructor;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the EPLOtherStreamExpr parity unit: all nine
 * executions (Java ordinals 0 through 8). The chained-parameterized execution
 * replays its three legs (compileDeploy, the eplToModel SODA round trip and
 * the static-methods-via-instance-navigation select); the outer-join
 * instance-method and static-method-via-alias executions assert null on the
 * absent join side; the invalid-select execution replays the three
 * tryInvalidCompile probes as compile-error records.
 *
 * Each execution is replayed in a fresh runtime. The harness-local Go mirror
 * types carry only the bean surface the Go side implements, so whole-event
 * values render the projected mirror surface: SupportMarketDataBean rows are
 * {feed, price, symbol, volume} (the engine type additionally carries the
 * never-sent, never-asserted id property, which is not part of the
 * differential surface), SupportBean rows are {intPrimitive, theString},
 * the chained child renders {text} and the default SupportBeanComplexProps
 * renders {simpleProperty}. The statement output-type surface (verbatim
 * property names in declaration order plus Java types) is asserted
 * internally against the deployed statements' event type property
 * descriptors per the Java source; it is not recorded in the trace because
 * the Go side cannot introspect Java types. Values and names are the
 * differential surface.
 */
public final class EPLOtherStreamExprScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "epl-other-stream-expr";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherStreamExpr.java";
    private static final String DESCRIPTION =
            "EPLOtherStreamExpr stream method expressions: chained parameterized event-method calls with verbatim "
                    + "expression-text output names plus a SODA eplToModel round trip, static methods resolved "
                    + "through instance-property navigation, static-method where filters with stream-name, wildcard, "
                    + "and EventBean arguments, instance-method projections over left outer joins returning null on "
                    + "the absent join side, static-method-via-alias join columns, aliased and verbatim no-alias "
                    + "expression-text output names with Long/Double/String value rendering, stream-as-object join "
                    + "columns, a followed-by pattern with a static UDF filter referencing the prior tag, and "
                    + "invalid-select compile probes (Java source regression-lib/src/main/java/com/espertech/esper/"
                    + "regressionlib/suite/epl/other/EPLOtherStreamExpr.java).";

    private static final String CHAINED = "chained-parameterized";
    private static final String STREAM_FUNCTION = "stream-function";
    private static final String OUTER_JOIN = "instance-method-outer-join";
    private static final String STATIC_JOIN = "instance-method-static";
    private static final String ALIASED = "stream-instance-method-aliased";
    private static final String NO_ALIAS = "stream-instance-method-no-alias";
    private static final String JOIN_SELECT = "join-stream-select";
    private static final String PATTERN_SELECT = "pattern-stream-select";
    private static final String INVALID = "invalid-select";
    private static final String[] CASES = {
            CHAINED, STREAM_FUNCTION, OUTER_JOIN, STATIC_JOIN, ALIASED, NO_ALIAS, JOIN_SELECT, PATTERN_SELECT, INVALID
    };
    private static final int[] ORDINALS = {0, 1, 2, 3, 4, 5, 6, 7, 8};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-9277aaf4733e12e28d28",
            "java-runtime-67e9ea0d239585623711",
            "java-runtime-cdb6409058fe3e44770f",
            "java-runtime-f33922181fbbb9c2e19b",
            "java-runtime-cc45d135a75bb01736f0",
            "java-runtime-469a37a746e59d25a115",
            "java-runtime-a59b12bbe5788257c37b",
            "java-runtime-827ea8daeea9baec40cf",
            "java-runtime-027a02a57250c1d497cb"
    };
    private static final String[] EXECUTION_NAMES = {
            "EPLOtherChainedParameterized",
            "EPLOtherStreamFunction",
            "EPLOtherInstanceMethodOuterJoin",
            "EPLOtherInstanceMethodStatic",
            "EPLOtherStreamInstanceMethodAliased",
            "EPLOtherStreamInstanceMethodNoAlias",
            "EPLOtherJoinStreamSelectNoWildcard",
            "EPLOtherPatternStreamSelectNoWildcard",
            "EPLOtherInvalidSelect"
    };
    private static final String[] STATIC_IDS = {
            "java-6828e4919d88389a5b0c",
            "java-e571ee83c24576b8aba7",
            "java-bd1bca1463bae6798d0a",
            "java-ae28c394fd146baf994e",
            "java-b448cd11a74aaf56dbab",
            "java-572869619d74ba3c95be",
            "java-534aeb16b8d33707670a",
            "java-dfb3493bb3eb08a778db",
            "java-f1f45018481fb57e3382"
    };

    private static final String STATIC_METHOD_LIB = SupportStaticMethodLib.class.getName();

    // Exact Java-source concatenations, including the trailing space that the
    // EPLOtherStreamInstanceMethodAliased and -NoAlias sources carry after
    // "SupportMarketDataBean as s0 ". The MyTestEvent schema deploy is the
    // pinned split of the Java source's two-statement EPL into its own deploy.
    private static final String CHAINED_SUBEXPR = "top.getChildOne(\"abc\",10).getChildTwo(\"append\")";
    private static final String EPL_CHAINED =
            "@name('s0') select " + CHAINED_SUBEXPR + " from SupportChainTop as top";
    private static final String EPL_STATIC_OUTER =
            "@name('s0') select inside.getMyString() as val,"
                    + "inside.insideTwo.getMyOtherString() as val2 "
                    + "from SupportBeanStaticOuter";
    private static final String[] EPL_STREAM_FUNCTION = {
            "@name('s0') select * from SupportMarketDataBean as s0 where " + STATIC_METHOD_LIB + ".volumeGreaterZero(s0)",
            "@name('s0') select * from SupportMarketDataBean as s0 where " + STATIC_METHOD_LIB + ".volumeGreaterZero(*)",
            "@name('s0') select * from SupportMarketDataBean as s0 where "
                    + STATIC_METHOD_LIB + ".volumeGreaterZeroEventBean(s0)",
            "@name('s0') select * from SupportMarketDataBean as s0 where "
                    + STATIC_METHOD_LIB + ".volumeGreaterZeroEventBean(*)"
    };
    private static final String EPL_OUTER_JOIN =
            "@name('s0') select symbol, s1.getTheString() as theString from "
                    + "SupportMarketDataBean#keepall as s0 "
                    + "left outer join "
                    + "SupportBean#keepall as s1 on s0.symbol=s1.theString";
    private static final String EPL_STATIC_JOIN =
            "@name('s0') select symbol, s1.getSimpleProperty() as simpleprop, s1.makeDefaultBean() as def from "
                    + "SupportMarketDataBean#keepall as s0 "
                    + "left outer join "
                    + "SupportBeanComplexProps#keepall as s1 on s0.symbol=s1.simpleProperty";
    private static final String EPL_ALIASED =
            "@name('s0') select s0.getVolume() as volume, s0.getSymbol() as symbol, s0.getPriceTimesVolume(2) as pvf from "
                    + "SupportMarketDataBean as s0 ";
    private static final String EPL_NO_ALIAS =
            "@name('s0') select s0.getVolume(), s0.getPriceTimesVolume(3) from "
                    + "SupportMarketDataBean as s0 ";
    private static final String EPL_MYTEST_SCHEMA =
            "@public @buseventtype create schema MyTestEvent as "
                    + "com.espertech.esper.regressionlib.suite.epl.other.EPLOtherStreamExpr$MyTestEvent";
    private static final String EPL_MYTEST_SELECT =
            "@name('s0') select "
                    + "s0.getValueAsInt(s0, 'id') as c0,"
                    + "s0.getValueAsInt(*, 'id') as c1"
                    + " from MyTestEvent as s0";
    private static final String EPL_JOIN_ALIASED =
            "@name('s0') select s0 as s0stream, s1 as s1stream from "
                    + "SupportMarketDataBean#keepall as s0, "
                    + "SupportBean#keepall as s1";
    private static final String EPL_JOIN_NO_ALIAS =
            "@name('s0') select s0, s1 from "
                    + "SupportMarketDataBean#keepall as s0, "
                    + "SupportBean#keepall as s1";
    private static final String EPL_PATTERN =
            "@name('s0') select * from pattern [every e1=SupportMarketDataBean -> e2="
                    + "SupportBean(" + STATIC_METHOD_LIB + ".compareEvents(e1, e2))]";

    // Invalid-select probe EPLs (ord 8 tryInvalidCompile calls, in order) and
    // the pinned message prefixes; the first probe pins "skip" (no prefix).
    private static final String EPL_PROBE_GETSTRING = "select s0.getString(1,2,3) from SupportBean as s0";
    private static final String EPL_PROBE_ABC = "select s0.abc() from SupportBean as s0";
    private static final String EPL_PROBE_PATTERN = "select s.theString from pattern [every [2] s=SupportBean] ee";
    private static final String ERR_PROBE_ABC =
            "Failed to validate select-clause expression 's0.abc()': Failed to solve 'abc' to either an date-time "
                    + "or enumeration method, an event property or a method on the event underlying object: Failed "
                    + "to resolve method 'abc': Could not find enumeration method, date-time method, instance method "
                    + "or property named 'abc' in class '" + SupportBean.class.getName() + "' taking no parameters [";
    private static final String ERR_PROBE_PATTERN =
            "Failed to validate select-clause expression 's.theString': Failed to resolve property 's.theString' "
                    + "(property 's' is an indexed property and requires an index or enumeration method to access "
                    + "values)";

    // Deploy counts per case: 3, 4, 1, 1, 1, 3, 2, 1, 0 (16 total).
    private static final int[] DEPLOY_COUNTS = {3, 4, 1, 1, 1, 3, 2, 1, 0};
    // Send counts per case: 3, 2, 1, 2, 1, 2, 2, 2, 0 (15 total).
    private static final int[] SEND_COUNTS = {3, 2, 1, 2, 1, 2, 2, 2, 0};
    // Build-error probe counts per case (3 total, all in invalid-select).
    private static final int[] PROBE_COUNTS = {0, 0, 0, 0, 0, 0, 0, 0, 3};
    // Per-case step spans: 10, 8, 4, 5, 4, 7, 6, 5, 5 (54 total).
    private static final int[] STEP_COUNTS = {10, 8, 4, 5, 4, 7, 6, 5, 5};
    // Listener records per case: 3, 4, 1, 2, 1, 2, 2, 1, 0 (16 total).
    private static final int[] RECORD_COUNTS = {3, 4, 1, 2, 1, 2, 2, 1, 0};
    // Cumulative listener records required after each send, per case.
    private static final int[][] CUMULATIVE_RECORDS = {
            {1, 2, 3},
            {0, 4},
            {1},
            {1, 2},
            {1},
            {1, 2},
            {0, 2},
            {0, 1},
            {}
    };

    // Statement output-type tables: property name in declaration order and the
    // rendered Java type simple name, asserted against the deployed statement's
    // event type property descriptors exactly where the Java source asserts
    // them (chained legs, aliased, no-alias and join cases).
    private static final String[][] CHAINED_TYPES = {
            {CHAINED_SUBEXPR, "SupportChainChildTwo"}
    };
    private static final String[][] ALIASED_TYPES = {
            {"volume", "Long"}, {"symbol", "String"}, {"pvf", "Double"}
    };
    private static final String[][] NO_ALIAS_TYPES = {
            {"s0.getVolume()", "Long"}, {"s0.getPriceTimesVolume(3)", "Double"}
    };
    private static final String[][] MYTEST_TYPES = {{"c0", "Integer"}, {"c1", "Integer"}};
    private static final String[][] JOIN_ALIASED_TYPES = {
            {"s0stream", "SupportMarketDataBean"}, {"s1stream", "SupportBean"}
    };
    private static final String[][] JOIN_NO_ALIAS_TYPES = {
            {"s0", "SupportMarketDataBean"}, {"s1", "SupportBean"}
    };

    private EPLOtherStreamExprScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: EPLOtherStreamExprScenarioOracle <scenario.json>");
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
        if (records.size() != 19) {
            throw new IllegalStateException("expected 19 records, got " + records.size());
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
            throw new IllegalArgumentException("scenario must contain exactly nine cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(definitions.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName", "observation",
                    "iteratorSnapshots", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !expectedObservation(index).equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != 0
                    || !primaryEPL(index).equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("scenario case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != 54) {
            throw new IllegalArgumentException("scenario must contain exactly 54 steps");
        }
        int index = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            index = validateCaseBlock(steps, index, caseIndex);
        }
        if (index != steps.size()) {
            throw new IllegalArgumentException("scenario contains trailing steps");
        }
    }

    private static String expectedObservation(int caseIndex) {
        return caseIndex == 8 ? "compile-error" : "listener";
    }

    private static String primaryEPL(int caseIndex) {
        switch (caseIndex) {
            case 0:
                return EPL_CHAINED;
            case 1:
                return EPL_STREAM_FUNCTION[0];
            case 2:
                return EPL_OUTER_JOIN;
            case 3:
                return EPL_STATIC_JOIN;
            case 4:
                return EPL_ALIASED;
            case 5:
                return EPL_NO_ALIAS;
            case 6:
                return EPL_JOIN_ALIASED;
            case 7:
                return EPL_PATTERN;
            default:
                return EPL_PROBE_GETSTRING + "\n" + EPL_PROBE_ABC + "\n" + EPL_PROBE_PATTERN + "\n";
        }
    }

    private static int validateCaseBlock(JsonArray steps, int index, int caseIndex) {
        String caseName = CASES[caseIndex];
        validateCaseMarker(steps, index++, caseName);
        int deploys = 0;
        int sends = 0;
        int probes = 0;
        switch (caseIndex) {
            case 0: {
                validateDeploy(steps, index++, caseName, "s0", EPL_CHAINED, "");
                deploys++;
                validateEmptySend(steps, index++, caseName, "SupportChainTop");
                sends++;
                validateUndeployAll(steps, index++, caseName);
                validateDeploy(steps, index++, caseName, "s0-soda", EPL_CHAINED, "soda");
                deploys++;
                validateEmptySend(steps, index++, caseName, "SupportChainTop");
                sends++;
                validateUndeployAll(steps, index++, caseName);
                validateDeploy(steps, index++, caseName, "s0-static", EPL_STATIC_OUTER, "");
                deploys++;
                validateEmptySend(steps, index++, caseName, "SupportBeanStaticOuter");
                sends++;
                break;
            }
            case 1: {
                for (int variant = 0; variant < 4; variant++) {
                    validateDeploy(steps, index++, caseName, "s0" + (char) ('a' + variant),
                            EPL_STREAM_FUNCTION[variant], "");
                    deploys++;
                }
                validateMarketDataSend(steps, index++, caseName, "ACME", 0, 0);
                sends++;
                validateMarketDataSend(steps, index++, caseName, "ACME", 0, 100);
                sends++;
                break;
            }
            case 2: {
                validateDeploy(steps, index++, caseName, "s0", EPL_OUTER_JOIN, "");
                deploys++;
                validateMarketDataSend(steps, index++, caseName, "ACME", 0, 0);
                sends++;
                break;
            }
            case 3: {
                validateDeploy(steps, index++, caseName, "s0", EPL_STATIC_JOIN, "");
                deploys++;
                validateMarketDataSend(steps, index++, caseName, "ACME", 0, 0);
                sends++;
                validateComplexPropsSend(steps, index++, caseName, "ACME");
                sends++;
                break;
            }
            case 4: {
                validateDeploy(steps, index++, caseName, "s0", EPL_ALIASED, "");
                deploys++;
                validateMarketDataSend(steps, index++, caseName, "ACME", 4, 99);
                sends++;
                break;
            }
            case 5: {
                validateDeploy(steps, index++, caseName, "s0a", EPL_NO_ALIAS, "");
                deploys++;
                validateMarketDataSend(steps, index++, caseName, "ACME", 4, 2);
                sends++;
                validateDeploy(steps, index++, caseName, "s0b", EPL_MYTEST_SCHEMA, "");
                deploys++;
                validateDeploy(steps, index++, caseName, "s0c", EPL_MYTEST_SELECT, "");
                deploys++;
                validateMyTestSend(steps, index++, caseName, 10);
                sends++;
                break;
            }
            case 6: {
                validateDeploy(steps, index++, caseName, "s0a", EPL_JOIN_ALIASED, "");
                deploys++;
                validateDeploy(steps, index++, caseName, "s0b", EPL_JOIN_NO_ALIAS, "");
                deploys++;
                // Both keepall windows must be live before the market data
                // send: a keepall view is per-statement, and the Java source
                // re-sends the market data event after deploying the second
                // statement.
                validateMarketDataSend(steps, index++, caseName, "ACME", 0, 0);
                sends++;
                validateBeanSend(steps, index++, caseName, null, 0);
                sends++;
                break;
            }
            case 7: {
                validateDeploy(steps, index++, caseName, "s0", EPL_PATTERN, "");
                deploys++;
                validateMarketDataSend(steps, index++, caseName, "ACME", 0, 0);
                sends++;
                validateBeanSend(steps, index++, caseName, "ACME", 1);
                sends++;
                break;
            }
            default: {
                validateBuildError(steps, index++, caseName, "getstring-args", EPL_PROBE_GETSTRING, null);
                probes++;
                validateBuildError(steps, index++, caseName, "abc-method", EPL_PROBE_ABC, ERR_PROBE_ABC);
                probes++;
                validateBuildError(steps, index++, caseName, "pattern-indexed", EPL_PROBE_PATTERN, ERR_PROBE_PATTERN);
                probes++;
                break;
            }
        }
        validateUndeployAll(steps, index++, caseName);
        if (deploys != DEPLOY_COUNTS[caseIndex] || sends != SEND_COUNTS[caseIndex]
                || probes != PROBE_COUNTS[caseIndex]) {
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
                                       String expectedStatement, String expectedEPL, String expectedMode) {
        JsonObject step = object(steps.get(index), "deploy step " + index);
        if (expectedMode.isEmpty()) {
            requireFields(step, "op", "case", "statement", "epl");
        } else {
            requireFields(step, "op", "case", "statement", "epl", "mode");
            if (!expectedMode.equals(string(step, "mode"))) {
                throw new IllegalArgumentException("deploy step mode mismatch at step " + index);
            }
        }
        if (!"deploy".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEPL.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("deploy step mismatch at step " + index);
        }
    }

    private static void validateBuildError(JsonArray steps, int index, String caseName,
                                           String expectedStatement, String expectedEPL, String expectedError) {
        JsonObject step = object(steps.get(index), "build-error step " + index);
        if (expectedError == null) {
            requireFields(step, "op", "case", "statement", "epl");
        } else {
            requireFields(step, "op", "case", "statement", "epl", "expectError");
            if (!expectedError.equals(string(step, "expectError"))) {
                throw new IllegalArgumentException("build-error expectError mismatch at step " + index);
            }
        }
        if (!"build-error".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEPL.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("build-error step mismatch at step " + index);
        }
    }

    private static void validateMarketDataSend(JsonArray steps, int index, String caseName,
                                               String symbol, int price, long volume) {
        JsonObject step = sendStep(steps, index, caseName);
        if (!"SupportMarketDataBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("step " + index + " must send SupportMarketDataBean");
        }
        JsonObject payload = object(step.get("payload"), "SupportMarketDataBean payload " + index);
        requireFields(payload, "symbol", "price", "volume", "feed");
        if (!symbol.equals(string(payload, "symbol"))
                || longNumber(payload, "price") != price
                || longNumber(payload, "volume") != volume
                || !payload.get("feed").isNull()) {
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

    private static void validateComplexPropsSend(JsonArray steps, int index, String caseName,
                                                 String simpleProperty) {
        JsonObject step = sendStep(steps, index, caseName);
        if (!"SupportBeanComplexProps".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("step " + index + " must send SupportBeanComplexProps");
        }
        JsonObject payload = object(step.get("payload"), "SupportBeanComplexProps payload " + index);
        requireFields(payload, "simpleProperty");
        if (!simpleProperty.equals(string(payload, "simpleProperty"))) {
            throw new IllegalArgumentException("SupportBeanComplexProps payload mismatch at step " + index);
        }
    }

    private static void validateEmptySend(JsonArray steps, int index, String caseName, String eventType) {
        JsonObject step = sendStep(steps, index, caseName);
        if (!eventType.equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("step " + index + " must send " + eventType);
        }
        JsonObject payload = object(step.get("payload"), eventType + " payload " + index);
        requireFields(payload);
    }

    private static void validateMyTestSend(JsonArray steps, int index, String caseName, int id) {
        JsonObject step = sendStep(steps, index, caseName);
        if (!"MyTestEvent".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("step " + index + " must send MyTestEvent");
        }
        JsonObject payload = object(step.get("payload"), "MyTestEvent payload " + index);
        requireFields(payload, "id");
        if (integer(payload, "id") != id) {
            throw new IllegalArgumentException("MyTestEvent payload mismatch at step " + index);
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
        addCaseTypes(configuration, caseIndex);

        String runtimeURI = "parity-" + ID + "-" + RUNTIME_IDS[caseIndex];
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            CaseContext context = new CaseContext(records, caseIndex, runtime, configuration);
            int deploys = 0;
            int sends = 0;
            int probes = 0;
            for (int offset = 0; offset < stepCount; offset++) {
                JsonObject step = object(allSteps.get(stepOffset + offset), "step " + (stepOffset + offset));
                String operation = string(step, "op");
                if ("case".equals(operation)) {
                    continue;
                }
                if ("deploy".equals(operation)) {
                    deploy(context, step);
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
                } else if ("build-error".equals(operation)) {
                    buildError(context, step);
                    probes++;
                } else if ("undeploy-all".equals(operation)) {
                    runtime.getDeploymentService().undeployAll();
                } else {
                    throw new IllegalArgumentException("unsupported operation in case " + caseName + ": " + operation);
                }
            }
            if (deploys != DEPLOY_COUNTS[caseIndex] || sends != SEND_COUNTS[caseIndex]
                    || probes != PROBE_COUNTS[caseIndex]) {
                throw new IllegalStateException("case " + caseName + " replayed " + deploys + " deploys, "
                        + sends + " sends and " + probes + " probes, expected " + DEPLOY_COUNTS[caseIndex]
                        + ", " + SEND_COUNTS[caseIndex] + " and " + PROBE_COUNTS[caseIndex]);
            }
            if (context.sequence != RECORD_COUNTS[caseIndex]) {
                throw new IllegalStateException("case " + caseName + " produced " + context.sequence
                        + " listener records, expected " + RECORD_COUNTS[caseIndex]);
            }
        } finally {
            runtime.destroy();
        }
    }

    private static void addCaseTypes(Configuration configuration, int caseIndex) {
        switch (caseIndex) {
            case 0:
                configuration.getCommon().addEventType("SupportChainTop", SupportChainTop.class);
                configuration.getCommon().addEventType("SupportBeanStaticOuter", SupportBeanStaticOuter.class);
                break;
            case 1:
            case 4:
            case 5:
                configuration.getCommon().addEventType("SupportMarketDataBean", SupportMarketDataBean.class);
                break;
            case 2:
                configuration.getCommon().addEventType("SupportMarketDataBean", SupportMarketDataBean.class);
                configuration.getCommon().addEventType("SupportBean", SupportBean.class);
                break;
            case 3:
                configuration.getCommon().addEventType("SupportMarketDataBean", SupportMarketDataBean.class);
                configuration.getCommon().addEventType("SupportBeanComplexProps", SupportBeanComplexProps.class);
                break;
            case 6:
            case 7:
                configuration.getCommon().addEventType("SupportMarketDataBean", SupportMarketDataBean.class);
                configuration.getCommon().addEventType("SupportBean", SupportBean.class);
                break;
            case 8:
                configuration.getCommon().addEventType("SupportBean", SupportBean.class);
                break;
            default:
                throw new IllegalArgumentException("unsupported case index " + caseIndex);
        }
    }

    private static void deploy(CaseContext context, JsonObject step) throws Exception {
        int caseIndex = context.caseIndex;
        EPRuntime runtime = context.runtime;
        String label = string(step, "statement");
        String epl = string(step, "epl");
        String mode = step.getString("mode", "");
        String deploymentId = context.runtimeURI + "-" + label;
        EPCompiled compiled;
        if ("soda".equals(mode)) {
            // The eplToModelCompileDeploy leg: parse to the SODA object model,
            // assert the toEPL round trip like the Java execution, then
            // compile the model through a module.
            EPStatementObjectModel model = EPCompilerProvider.getCompiler().eplToModel(epl, context.config);
            if (!epl.equals(model.toEPL())) {
                throw new IllegalStateException("soda round-trip drift for " + label
                        + ": [" + model.toEPL() + "]");
            }
            com.espertech.esper.common.client.module.Module module =
                    new com.espertech.esper.common.client.module.Module();
            module.getItems().add(new com.espertech.esper.common.client.module.ModuleItem(model));
            module.setModuleText(model.toEPL());
            compiled = EPCompilerProvider.getCompiler().compile(
                    module, new CompilerArguments(runtime.getRuntimePath()));
        } else {
            compiled = EPCompilerProvider.getCompiler().compile(
                    epl, new CompilerArguments(runtime.getRuntimePath()));
        }
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                new DeploymentOptions().setDeploymentId(deploymentId));
        if (caseIndex == 5 && "s0b".equals(label)) {
            // The MyTestEvent schema statement carries an auto-generated name
            // and no listener output.
            return;
        }
        EPStatement statement = findStatement(deployment, context.caseName, label);
        switch (caseIndex) {
            case 0:
                if ("s0".equals(label)) {
                    requireEPL(context, label, "s0", epl, EPL_CHAINED);
                } else if ("s0-soda".equals(label)) {
                    requireEPL(context, label, "s0-soda", epl, EPL_CHAINED);
                } else if ("s0-static".equals(label)) {
                    requireEPL(context, label, "s0-static", epl, EPL_STATIC_OUTER);
                } else {
                    throw new IllegalStateException("case " + context.caseName + " unexpected deploy " + label);
                }
                if (!"s0-static".equals(label)) {
                    // The Java source asserts the verbatim expression-text
                    // property name and SupportChainChildTwo type on both
                    // chained legs; the static-outer leg has no type assert.
                    assertOutputType(context, statement, CHAINED_TYPES);
                }
                context.attach(statement);
                break;
            case 1:
                requireEPL(context, label, "s0" + (char) ('a' + context.deploys), epl,
                        EPL_STREAM_FUNCTION[context.deploys]);
                context.attach(statement);
                break;
            case 2:
                requireEPL(context, label, "s0", epl, EPL_OUTER_JOIN);
                context.attach(statement);
                break;
            case 3:
                requireEPL(context, label, "s0", epl, EPL_STATIC_JOIN);
                context.attach(statement);
                break;
            case 4:
                requireEPL(context, label, "s0", epl, EPL_ALIASED);
                assertOutputType(context, statement, ALIASED_TYPES);
                context.attach(statement);
                break;
            case 5:
                if ("s0a".equals(label)) {
                    requireEPL(context, label, "s0a", epl, EPL_NO_ALIAS);
                    assertOutputType(context, statement, NO_ALIAS_TYPES);
                    context.attach(statement);
                } else if ("s0c".equals(label)) {
                    requireEPL(context, label, "s0c", epl, EPL_MYTEST_SELECT);
                    assertOutputType(context, statement, MYTEST_TYPES);
                    context.attach(statement);
                } else {
                    throw new IllegalStateException("case " + context.caseName + " unexpected deploy " + label);
                }
                break;
            case 6:
                if ("s0a".equals(label)) {
                    requireEPL(context, label, "s0a", epl, EPL_JOIN_ALIASED);
                    assertOutputType(context, statement, JOIN_ALIASED_TYPES);
                    context.attach(statement);
                } else if ("s0b".equals(label)) {
                    requireEPL(context, label, "s0b", epl, EPL_JOIN_NO_ALIAS);
                    assertOutputType(context, statement, JOIN_NO_ALIAS_TYPES);
                    context.attach(statement);
                } else {
                    throw new IllegalStateException("case " + context.caseName + " unexpected deploy " + label);
                }
                break;
            case 7:
                requireEPL(context, label, "s0", epl, EPL_PATTERN);
                context.attach(statement);
                break;
            default:
                throw new IllegalArgumentException("unsupported case index " + caseIndex);
        }
        context.deploys++;
    }

    /**
     * Compiles an expected-invalid statement, mirroring env.tryInvalidCompile:
     * the compile must fail and, when the step pins a prefix, the message must
     * start with it ("skip"-pinned probes assert failure only). The record
     * value carries the asserted prefix.
     */
    private static void buildError(CaseContext context, JsonObject step) {
        String label = string(step, "statement");
        String epl = string(step, "epl");
        String expected = step.getString("expectError", "");
        String pinnedEPL;
        String pinnedError;
        switch (label) {
            case "getstring-args":
                pinnedEPL = EPL_PROBE_GETSTRING;
                pinnedError = "";
                break;
            case "abc-method":
                pinnedEPL = EPL_PROBE_ABC;
                pinnedError = ERR_PROBE_ABC;
                break;
            case "pattern-indexed":
                pinnedEPL = EPL_PROBE_PATTERN;
                pinnedError = ERR_PROBE_PATTERN;
                break;
            default:
                throw new IllegalStateException("case " + context.caseName + " unknown build-error probe " + label);
        }
        if (!pinnedEPL.equals(epl) || !pinnedError.equals(expected)) {
            throw new IllegalStateException("case " + context.caseName + " build-error probe " + label
                    + " is not pinned");
        }
        String caught;
        try {
            EPCompilerProvider.getCompiler().compile(
                    epl, new CompilerArguments(context.runtime.getRuntimePath()));
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught.equals("<no-error>")) {
            throw new IllegalStateException("build-error probe " + label + " unexpectedly compiled");
        }
        if (!expected.isEmpty() && !caught.startsWith(expected)) {
            throw new IllegalStateException("compile-error message drift for " + label
                    + ": expected prefix [" + expected + "] got [" + caught + "]");
        }
        JsonObject record = new JsonObject()
                .add("case", context.caseName)
                .add("operation", "compile-error")
                .add("statement", label)
                .add("sequence", 0);
        if (!expected.isEmpty()) {
            record.add("value", expected);
        }
        context.records.add(record);
    }

    private static void requireEPL(CaseContext context, String label, String expectedLabel,
                                   String actualEPL, String expectedEPL) {
        if (!expectedLabel.equals(label) || !expectedEPL.equals(actualEPL)) {
            throw new IllegalStateException("case " + context.caseName + " deploy " + label + " is not pinned");
        }
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

    private static void assertOutputType(CaseContext context, EPStatement statement, String[][] expected) {
        EventPropertyDescriptor[] descriptors = statement.getEventType().getPropertyDescriptors();
        if (descriptors.length != expected.length) {
            throw new IllegalStateException("case " + context.caseName + " output type has "
                    + descriptors.length + " properties, expected " + expected.length);
        }
        for (int index = 0; index < expected.length; index++) {
            EventPropertyDescriptor descriptor = descriptors[index];
            String actualType = descriptor.getPropertyType() == null
                    ? "null"
                    : descriptor.getPropertyType().getSimpleName();
            if (!expected[index][0].equals(descriptor.getPropertyName())
                    || !expected[index][1].equals(actualType)) {
                throw new IllegalStateException("case " + context.caseName + " output type mismatch at "
                        + index + ": " + descriptor.getPropertyName() + ":" + actualType + ", expected "
                        + expected[index][0] + ":" + expected[index][1]);
            }
        }
    }

    private static void send(CaseContext context, JsonObject step) throws Exception {
        String eventType = string(step, "eventType");
        JsonObject payload = object(step.get("payload"), eventType + " payload");
        Object event;
        switch (eventType) {
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
            case "SupportBean": {
                requireFields(payload, "theString", "intPrimitive");
                SupportBean bean = new SupportBean(
                        nullableString(payload, "theString"),
                        integer(payload, "intPrimitive"));
                context.lastBean = bean;
                event = bean;
                break;
            }
            case "SupportChainTop": {
                requireFields(payload);
                event = new SupportChainTop();
                break;
            }
            case "SupportBeanStaticOuter": {
                requireFields(payload);
                event = new SupportBeanStaticOuter();
                break;
            }
            case "SupportBeanComplexProps": {
                requireFields(payload, "simpleProperty");
                // The Java source sends makeDefaultBean() with simpleProperty
                // overridden to the payload value before sending.
                SupportBeanComplexProps props = SupportBeanComplexProps.makeDefaultBean();
                props.setSimpleProperty(string(payload, "simpleProperty"));
                event = props;
                break;
            }
            case "MyTestEvent": {
                requireFields(payload, "id");
                event = newMyTestEvent(integer(payload, "id"));
                break;
            }
            default:
                throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
        context.runtime.getEventService().sendEventBean(event, eventType);
    }

    private static EPLOtherStreamExpr.MyTestEvent newMyTestEvent(int id) throws Exception {
        // The Java source's nested MyTestEvent declares a private int
        // constructor; the scenario replays the same instance the source sends.
        Constructor<EPLOtherStreamExpr.MyTestEvent> constructor =
                EPLOtherStreamExpr.MyTestEvent.class.getDeclaredConstructor(int.class);
        constructor.setAccessible(true);
        return constructor.newInstance(id);
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
        private final Configuration config;
        private final String runtimeURI;
        private int deploys;
        private int sequence;
        private SupportMarketDataBean lastMarketData;
        private SupportBean lastBean;

        private CaseContext(JsonArray records, int caseIndex, EPRuntime runtime, Configuration config) {
            this.records = records;
            this.caseIndex = caseIndex;
            this.caseName = CASES[caseIndex];
            this.runtime = runtime;
            this.config = config;
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
            JsonObject fields = validateAndRender(newEvents[0], nextSequence);
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

        private JsonObject validateAndRender(EventBean event, int sequence) {
            switch (context.caseIndex) {
                case 0:
                    return renderChainedRow(event, sequence);
                case 1:
                    return renderStreamFunctionRow(event);
                case 2:
                    return renderOuterJoinRow(event);
                case 3:
                    return renderStaticJoinRow(event, sequence);
                case 4:
                    return renderAliasedRow(event);
                case 5:
                    return renderNoAliasRow(event, sequence);
                case 6:
                    return renderJoinRow(event, sequence);
                case 7:
                    return renderPatternRow(event);
                default:
                    throw new IllegalStateException("unsupported case index " + context.caseIndex);
            }
        }

        private JsonObject renderChainedRow(EventBean event, int sequence) {
            if (sequence <= 2) {
                // Both chained legs (compileDeploy and the eplToModel round
                // trip) project the verbatim expression-text column carrying
                // the SupportChainChildTwo result; the harness mirror renders
                // its asserted getText() surface.
                assertFieldNames(event, new String[]{CHAINED_SUBEXPR});
                Object result = event.get(CHAINED_SUBEXPR);
                if (!(result instanceof SupportChainChildTwo)
                        || !"abcappend".equals(((SupportChainChildTwo) result).getText())) {
                    throw new IllegalStateException("case " + context.caseName + " chained row mismatch");
                }
                JsonObject fields = new JsonObject()
                        .add(CHAINED_SUBEXPR, new JsonObject().add("text", "abcappend"));
                return fields;
            }
            assertFieldNames(event, new String[]{"val", "val2"});
            Object val = event.get("val");
            Object val2 = event.get("val2");
            if (!"hello".equals(val) || !"hello2".equals(val2)) {
                throw new IllegalStateException("case " + context.caseName + " static-outer row mismatch");
            }
            JsonObject fields = new JsonObject()
                    .add("val", normalize(val))
                    .add("val2", normalize(val2));
            return fields;
        }

        private JsonObject renderStreamFunctionRow(EventBean event) {
            // select * over the projected harness-mirror surface; the engine
            // type also carries the never-sent id property.
            Object feed = event.get("feed");
            Object price = event.get("price");
            Object symbol = event.get("symbol");
            Object volume = event.get("volume");
            if (feed != null
                    || !sameScalar(0d, price)
                    || !"ACME".equals(symbol)
                    || !sameScalar(100L, volume)) {
                throw new IllegalStateException("case " + context.caseName + " stream-function row mismatch");
            }
            JsonObject fields = new JsonObject()
                    .add("feed", normalize(feed))
                    .add("price", normalize(price))
                    .add("symbol", normalize(symbol))
                    .add("volume", normalize(volume));
            return fields;
        }

        private JsonObject renderOuterJoinRow(EventBean event) {
            // Unmatched left outer join: the s1-side instance method result
            // is null (the Java source asserts {symbol:"ACME", theString:null}).
            assertFieldNames(event, new String[]{"symbol", "theString"});
            Object symbol = event.get("symbol");
            Object theString = event.get("theString");
            if (!"ACME".equals(symbol) || theString != null) {
                throw new IllegalStateException("case " + context.caseName + " outer-join row mismatch");
            }
            JsonObject fields = new JsonObject()
                    .add("symbol", normalize(symbol))
                    .add("theString", normalize(theString));
            return fields;
        }

        private JsonObject renderStaticJoinRow(EventBean event, int sequence) {
            assertFieldNames(event, new String[]{"def", "simpleprop", "symbol"});
            Object symbol = event.get("symbol");
            Object simpleprop = event.get("simpleprop");
            Object def = event.get("def");
            if (sequence == 1) {
                // Unmatched left outer join: both s1-side method results are
                // null (assertProps symbol+simpleprop, assertNull def).
                if (!"ACME".equals(symbol) || simpleprop != null || def != null) {
                    throw new IllegalStateException("case " + context.caseName
                            + " static-join unmatched row mismatch");
                }
            } else {
                // Matched join: def is the makeDefaultBean() result; the
                // harness mirror renders its simpleProperty surface.
                if (!"ACME".equals(symbol) || !"ACME".equals(simpleprop)
                        || !(def instanceof SupportBeanComplexProps)
                        || !"simple".equals(((SupportBeanComplexProps) def).getSimpleProperty())) {
                    throw new IllegalStateException("case " + context.caseName
                            + " static-join matched row mismatch");
                }
            }
            JsonObject fields = new JsonObject()
                    .add("def", def == null
                            ? normalize(null)
                            : new JsonObject().add("simpleProperty",
                                    ((SupportBeanComplexProps) def).getSimpleProperty()))
                    .add("simpleprop", normalize(simpleprop))
                    .add("symbol", normalize(symbol));
            return fields;
        }

        private JsonObject renderAliasedRow(EventBean event) {
            assertFieldNames(event, new String[]{"pvf", "symbol", "volume"});
            Object volume = event.get("volume");
            Object symbol = event.get("symbol");
            Object pvf = event.get("pvf");
            if (!sameScalar(99L, volume) || !"ACME".equals(symbol)
                    || !(pvf instanceof Double)
                    || Double.compare(4d * 99L * 2, (Double) pvf) != 0) {
                throw new IllegalStateException("case " + context.caseName + " aliased row mismatch");
            }
            JsonObject fields = new JsonObject()
                    .add("pvf", normalize(pvf))
                    .add("symbol", normalize(symbol))
                    .add("volume", normalize(volume));
            return fields;
        }

        private JsonObject renderNoAliasRow(EventBean event, int sequence) {
            if (sequence == 1) {
                assertFieldNames(event, new String[]{"s0.getPriceTimesVolume(3)", "s0.getVolume()"});
                Object volume = event.get("s0.getVolume()");
                Object pvf = event.get("s0.getPriceTimesVolume(3)");
                if (!sameScalar(2L, volume)
                        || !(pvf instanceof Double)
                        || Double.compare(4d * 2L * 3d, (Double) pvf) != 0) {
                    throw new IllegalStateException("case " + context.caseName + " no-alias row mismatch");
                }
                JsonObject fields = new JsonObject()
                        .add("s0.getPriceTimesVolume(3)", normalize(pvf))
                        .add("s0.getVolume()", normalize(volume));
                return fields;
            }
            assertFieldNames(event, new String[]{"c0", "c1"});
            Object c0 = event.get("c0");
            Object c1 = event.get("c1");
            if (!sameScalar(10, c0) || !sameScalar(10, c1)) {
                throw new IllegalStateException("case " + context.caseName + " MyTestEvent row mismatch");
            }
            JsonObject fields = new JsonObject()
                    .add("c0", normalize(c0))
                    .add("c1", normalize(c1));
            return fields;
        }

        private JsonObject renderJoinRow(EventBean event, int sequence) {
            String[] names = sequence == 1
                    ? new String[]{"s0stream", "s1stream"}
                    : new String[]{"s0", "s1"};
            assertFieldNames(event, names);
            Object marketData = event.get(names[0]);
            Object bean = event.get(names[1]);
            if (marketData != context.lastMarketData || bean != context.lastBean) {
                throw new IllegalStateException("case " + context.caseName
                        + " join row does not carry the joined stream events by identity");
            }
            JsonObject fields = new JsonObject()
                    .add(names[0], marketDataRow(context.lastMarketData))
                    .add(names[1], beanRow(context.lastBean));
            return fields;
        }

        private JsonObject renderPatternRow(EventBean event) {
            assertFieldNames(event, new String[]{"e1", "e2"});
            Object e1 = event.get("e1");
            Object e2 = event.get("e2");
            if (e1 != context.lastMarketData || e2 != context.lastBean) {
                throw new IllegalStateException("case " + context.caseName
                        + " pattern row does not carry the tagged pattern events by identity");
            }
            JsonObject fields = new JsonObject()
                    .add("e1", marketDataRow(context.lastMarketData))
                    .add("e2", beanRow(context.lastBean));
            return fields;
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
