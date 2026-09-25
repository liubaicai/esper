import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.regressionlib.support.bean.SupportMarketDataBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

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
 * Scenario oracle for the EPLOtherInvalid executions pinned by Draft 4.539 —
 * all four executions of EPLOtherInvalid.java (ords 0-3), three carrying
 * INVALIDITY. Each case replays on one fresh runtime.
 *
 * invalid-func-params (ord 0, EPLOtherInvalidFuncParams): two
 * tryInvalidCompile probes pin the count/leaving arity messages verbatim.
 * The Go side has no boundary (Count takes one argument, Leaving an
 * optional bool predicate), so the oracle verifies the rejection and
 * message in-process, then emits the pinned "unrepresentable" records.
 *
 * invalid-syntax (ord 1, EPLOtherInvalidSyntax): two getSyntaxExceptionEPL
 * probes assert the message text exactly (assertEquals) and one
 * tryInvalidCompile probe asserts the startsWith prefix. All three are
 * EPL-text surfaces with no Go AST; the oracle verifies each rejection and
 * message in-process, then emits the pinned "unrepresentable" records.
 *
 * long-type-constant (ord 2, EPLOtherLongTypeConstant): compileDeploy of
 * `select 2512570244 as value from SupportBean`, addListener("s0"), one
 * default SupportBean send delivering value=2512570244L, undeployAll.
 *
 * different-joins (ord 3, EPLOtherDifferentJoins): the 50-probe
 * compile-only validity matrix. tryInvalid asserts only that compilation
 * throws EPCompileException (no message), so rejected probes emit
 * "compile-error" records carrying the pinned 'rejected' marker,
 * unrepresentable probes emit "unrepresentable" records carrying the
 * pinned boundary note, and the six divergent probes (the <= outer-join
 * ON, the two same-source ON keys, the int>=string where and the two
 * mixed-type ON keys Go accepts) emit
 * "intentionally-different" records carrying the pinned divergence note.
 * tryValid probes compileDeploy and emit "deployed" markers.
 */
public final class EPLOtherInvalidScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "epl-other-invalid";
    private static final String DESCRIPTION =
            "EPLOtherInvalid (ords 0-3, all executions): invalid-func-params "
                    + "pins two tryInvalidCompile arity rejections — "
                    + "count(theString,theString,theString) expects at least 1 "
                    + "and up to 2 parameters, leaving(theString) expects no "
                    + "parameters — both unrepresentable (Go Count takes one "
                    + "argument, Leaving an optional bool predicate); "
                    + "invalid-syntax pins three parse/validation messages — "
                    + "`select * from *` (incorrect syntax near '*' at line 1 "
                    + "column 14), the reserved-keyword `r.start` where-clause "
                    + "error, and the `SupportBean(1=2=3)` three-way equals "
                    + "chain — all unrepresentable (no Go EPL parser or "
                    + "chained-equals AST); long-type-constant deploys `select "
                    + "2512570244 as value from SupportBean` and one "
                    + "SupportBean send delivers value=2512570244 as int64; "
                    + "different-joins replays the 50-probe compile-only "
                    + "validity matrix over SupportBean#length(3) sa/sb comma "
                    + "joins, the SupportBean+SupportMarketDataBean pair and "
                    + "the sa-left-outer-join-sb ON clause — 27 deployed, "
                    + "12 compile-error 'rejected' markers (unknown "
                    + "field/alias, mixed-type EqualOf pairs, non-ordered >= "
                    + "operand), 5 unrepresentable (unbalanced parens, "
                    + "ambiguous unqualified) and 6 intentionally-different "
                    + "(Go accepts non-equi outer ON, same-source ON keys, "
                    + "int>=string where and mixed-type ON keys).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/"
                    + "epl/other/EPLOtherInvalid.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-57d4ed46cb893b70792c",
            "java-runtime-1ca3b083af4e59da8024",
            "java-runtime-13ae251df45f9dc74efa",
            "java-runtime-7e8cd12d2a61f4041a0e"
    };
    private static final String[] EXECUTION_NAMES = {
            "EPLOtherInvalidFuncParams",
            "EPLOtherInvalidSyntax",
            "EPLOtherLongTypeConstant",
            "EPLOtherDifferentJoins"
    };
    private static final String[] STATIC_IDS = {
            "java-6cb657c46f10fbb54e9f",
            "java-1f8004583e889e7d519d",
            "java-5f23e933953bbae72045",
            "java-781d7db6d45ac322456b"
    };
    private static final String[] JAVA_FLAGS = {"INVALIDITY"};
    private static final String[] CASES = {
            "invalid-func-params",
            "invalid-syntax",
            "long-type-constant",
            "different-joins"
    };
    private static final int[] ORDINALS = {0, 1, 2, 3};

    // Verbatim transcriptions of EPLOtherInvalid.java lines 40-44, 54-61, 71
    // and the streamDef/streamDefTwo/outerJoinDef concatenations at lines
    // 83-91 and 148-151.
    private static final String EPL_COUNT_ARITY =
            "select count(theString, theString, theString) from SupportBean";
    private static final String EPL_LEAVING_ARITY =
            "select leaving(theString) from SupportBean";
    private static final String EPL_FROM_STAR = "select * from *";
    private static final String EPL_RESERVED =
            "select * from SupportBean a where a.intPrimitive between r.start and r.end";
    private static final String EPL_FILTER_CHAIN = "select * from SupportBean(1=2=3)";
    private static final String EPL_LONG_CONSTANT =
            "@name('s0') select 2512570244 as value from SupportBean";

    private static final String STREAM_DEF =
            "select * from SupportBean#length(3) as sa,SupportBean#length(3) as sb where ";
    private static final String STREAM_DEF_TWO =
            "select * from SupportBean#length(3),SupportMarketDataBean#length(3) where ";
    private static final String OUTER_JOIN_DEF =
            "select * from SupportBean#length(3) as sa left outer join "
                    + "SupportBean#length(3) as sb ";
    private static final String REJECTED = "rejected";

    // The pinned Java messages. Ords 0-1 assert the message text; ord 3
    // asserts rejection only, so its compile-error records pin 'rejected'
    // and its unrepresentable/intentionally-different records pin notes.
    private static final Map<String, String> PROBE_EPLS = new HashMap<>();
    private static final Map<String, String> PROBE_MESSAGES = new HashMap<>();
    static {
        PROBE_EPLS.put("count-three-args", EPL_COUNT_ARITY);
        PROBE_EPLS.put("leaving-with-param", EPL_LEAVING_ARITY);
        PROBE_EPLS.put("from-star", EPL_FROM_STAR);
        PROBE_EPLS.put("reserved-keyword", EPL_RESERVED);
        PROBE_EPLS.put("filter-equals-chain", EPL_FILTER_CHAIN);
        PROBE_MESSAGES.put("count-three-args",
                "Failed to validate select-clause expression "
                        + "'count(theString,theString,theString)': The 'count' "
                        + "function expects at least 1 and up to 2 parameters");
        PROBE_MESSAGES.put("leaving-with-param",
                "Failed to validate select-clause expression "
                        + "'leaving(theString)': The 'leaving' function expects "
                        + "no parameters");
        PROBE_MESSAGES.put("from-star",
                "Incorrect syntax near '*' at line 1 column 14, please check "
                        + "the from clause [select * from *]");
        PROBE_MESSAGES.put("reserved-keyword",
                "Incorrect syntax near 'start' (a reserved keyword) at line 1 "
                        + "column 59, please check the where clause [select * "
                        + "from SupportBean a where a.intPrimitive between "
                        + "r.start and r.end]");
        PROBE_MESSAGES.put("filter-equals-chain",
                "Failed to validate filter expression '1=2': Invalid use of "
                        + "equals, expecting left-hand side and right-hand "
                        + "side but received 3 expressions");
    }

    // The two syntax probes assert assertEquals (exact message); the other
    // ord 0-1 probes assert SupportMessageAssertUtil startsWith.
    private static final Set<String> EXACT_MESSAGE_PROBES = new HashSet<>(
            Arrays.asList("from-star", "reserved-keyword"));

    // The pinned notes the ord-3 unrepresentable records carry.
    private static final Map<String, String> UNREPRESENTABLE_NOTES = new HashMap<>();
    static {
        UNREPRESENTABLE_NOTES.put("where-arith-unbalanced",
                "sa.intPrimitive=2*(sa.intBoxed: the unbalanced-paren EPL text "
                        + "has no fluent form — the Go And/Or/arithmetic tree "
                        + "is structural; Java rejects the parse error at "
                        + "compile time");
        UNREPRESENTABLE_NOTES.put("where-or-unbalanced-open",
                "sa.intPrimitive=3 or (sa.intBoxed=2: the unbalanced-paren EPL "
                        + "text has no fluent form — the Go And/Or tree is "
                        + "structural; Java rejects the parse error at compile "
                        + "time");
        UNREPRESENTABLE_NOTES.put("where-or-unbalanced-close",
                "sa.intPrimitive=3 or sa.intBoxed=2): the unbalanced-paren EPL "
                        + "text has no fluent form — the Go And/Or tree is "
                        + "structural; Java rejects the parse error at compile "
                        + "time");
        UNREPRESENTABLE_NOTES.put("where-or-unbalanced-nested",
                "sa.intPrimitive=3 or ((sa.intBoxed=2): the unbalanced-paren "
                        + "EPL text has no fluent form — the Go And/Or tree is "
                        + "structural; Java rejects the parse error at compile "
                        + "time");
        UNREPRESENTABLE_NOTES.put("where-unqualified-int",
                "intPrimitive=3 over two SupportBean streams: the unqualified "
                        + "EPL column is ambiguous between sa and sb; Go "
                        + "JoinField requires an explicit source index, so the "
                        + "ambiguity has no fluent form; Java rejects the "
                        + "ambiguous property at compile time");
    }

    // The pinned divergence notes: Java rejects these clauses while the Go
    // build accepts them — the non-equi <= outer-join key, the two
    // same-source ON keys, the int-vs-string >= where comparison
    // (GreaterOrEqualOf takes untyped operands and its ordered-type check
    // accepts strings) and the two mixed-type ON keys (OnSourcesEqual never
    // type-checks join-field operands).
    private static final Map<String, String> DIVERGENT_NOTES = new HashMap<>();
    static {
        DIVERGENT_NOTES.put("where-int-ge-thestring",
                "sa.intPrimitive >= sa.theString: Java rejects the "
                        + "int-vs-string relational comparison at compile "
                        + "time; Go GreaterOrEqualOf takes untyped operands "
                        + "and its ordered-type check accepts strings, so the "
                        + "build succeeds");
        DIVERGENT_NOTES.put("on-boolboxed-eq-intboxed",
                "on sa.boolBoxed = sb.intBoxed: Java rejects the bool-vs-int "
                        + "ON key type mismatch at compile time; Go "
                        + "OnSourcesEqual takes untyped operands and never "
                        + "type-checks join-field operands, so the build "
                        + "succeeds");
        DIVERGENT_NOTES.put("on-bool-eq-thestring",
                "on sa.boolPrimitive = sb.theString: Java rejects the "
                        + "bool-vs-string ON key type mismatch at compile "
                        + "time; Go OnSourcesEqual takes untyped operands and "
                        + "never type-checks join-field operands, so the "
                        + "build succeeds");
        DIVERGENT_NOTES.put("on-int-le-intboxed",
                "on sa.intPrimitive <= sb.intBoxed: Java restricts outer-join "
                        + "ON to equality and rejects the <= comparison; Go "
                        + "accepts any JoinComparison on an outer join edge, "
                        + "so the build succeeds");
        DIVERGENT_NOTES.put("on-same-source-left",
                "on sa.intPrimitive = sa.intBoxed: Java requires outer-join "
                        + "ON keys to span both streams and rejects the "
                        + "same-source pair; Go validates field existence and "
                        + "source indexes only, so the build succeeds");
        DIVERGENT_NOTES.put("on-same-source-right",
                "on sb.intPrimitive = sb.intBoxed: Java requires outer-join "
                        + "ON keys to span both streams and rejects the "
                        + "same-source pair; Go validates field existence and "
                        + "source indexes only, so the build succeeds");
    }

    private static final String[] CASE_OBSERVATIONS = {
            "unrepresentable; two tryInvalidCompile probes pin the "
                    + "count/leaving arity messages verbatim — the "
                    + "three-argument count and the parameterized leaving have "
                    + "no typed-Go form (Count takes one argument, Leaving an "
                    + "optional bool predicate), so no Go rejection boundary "
                    + "is claimed",
            "unrepresentable; three probes pin the asserted Java messages "
                    + "verbatim — the `select * from *` and reserved-keyword "
                    + "`r.start` parse errors and the `SupportBean(1=2=3)` "
                    + "three-way equals chain are EPL-text surfaces with no "
                    + "Go AST",
            "deployed+listener; `select 2512570244 as value from SupportBean` "
                    + "deploys s0 and one SupportBean send delivers "
                    + "value=2512570244 as int64 (the literal exceeds int32, "
                    + "exercising Java's long literal typing)",
            "deployed+compile-error+unrepresentable+intentionally-different; "
                    + "the 50-probe validity matrix: 27 tryValid deploys, 12 "
                    + "compile-error 'rejected' markers (unknown field/alias "
                    + "sb.XX, sa.XX pairs, sX.theString, intPrimitive=x, "
                    + "boolBoxed=f plus the mixed-type EqualOf pairs and the "
                    + "bool >= operand Go rejects), 5 unrepresentable pins "
                    + "(unbalanced parens, ambiguous unqualified intPrimitive) "
                    + "and 6 intentionally-different pins (Go accepts the <= "
                    + "outer-join ON, the two same-source ON keys, the "
                    + "int>=string where and the two mixed-type ON keys Java "
                    + "rejects)",
    };
    private static final String[] CASE_EPLS = {
            EPL_COUNT_ARITY,
            EPL_FROM_STAR,
            EPL_LONG_CONSTANT,
            STREAM_DEF + "sa.intPrimitive = sb.theString"
    };

    private static final int EXPECTED_STEPS = 91;
    private static final int EXPECTED_RECORDS = 57;

    /**
     * Pinned per-case step keys rendered as
     * op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|at.
     * unrepresentable steps carry the pinned note or Java message in
     * expectError; build-error steps pin the 'rejected' marker; deploy steps
     * carry the byte-exact tryValid EPL.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("invalid-func-params", new String[]{
                "unrepresentable|count-three-args|||" + EPL_COUNT_ARITY + "||"
                        + PROBE_MESSAGES.get("count-three-args") + "|1|",
                "unrepresentable|leaving-with-param|||" + EPL_LEAVING_ARITY + "||"
                        + PROBE_MESSAGES.get("leaving-with-param") + "|1|",
        });
        CASE_STEPS.put("invalid-syntax", new String[]{
                "unrepresentable|from-star|||" + EPL_FROM_STAR + "||"
                        + PROBE_MESSAGES.get("from-star") + "|1|",
                "unrepresentable|reserved-keyword|||" + EPL_RESERVED + "||"
                        + PROBE_MESSAGES.get("reserved-keyword") + "|1|",
                "unrepresentable|filter-equals-chain|||" + EPL_FILTER_CHAIN + "||"
                        + PROBE_MESSAGES.get("filter-equals-chain") + "|1|",
        });
        CASE_STEPS.put("long-type-constant", new String[]{
                "deploy|s0|||" + EPL_LONG_CONSTANT + "||||",
                "deployed|s0|||||||",
                "send|||SupportBean||{}|||",
                "undeploy-all||||||||",
        });
        CASE_STEPS.put("different-joins", differentJoinsSteps());
    }

    /**
     * Renders the ord-3 step sequence in Java source order
     * (EPLOtherInvalid.java lines 93-164): each tryValid is a
     * deploy+deployed pair, each representable tryInvalid a build-error
     * step pinning 'rejected', each unrepresentable or divergent tryInvalid
     * an unrepresentable step pinning its note.
     */
    private static String[] differentJoinsSteps() {
        List<String> steps = new ArrayList<>();
        java.util.function.BiConsumer<String, String> deploy = (label, condition) -> {
            steps.add("deploy|" + label + "|||" + condition + "||||");
            steps.add("deployed|" + label + "|||||||");
        };
        java.util.function.BiConsumer<String, String> rejected = (label, condition) ->
                steps.add("build-error|" + label + "|||" + condition + "||" + REJECTED + "|1|");
        java.util.function.BiConsumer<String, String> unrep = (label, condition) ->
                steps.add("unrepresentable|" + label + "|||" + condition + "||"
                        + UNREPRESENTABLE_NOTES.get(label) + "|1|");
        java.util.function.BiConsumer<String, String> divergent = (label, condition) ->
                steps.add("unrepresentable|" + label + "|||" + condition + "||"
                        + DIVERGENT_NOTES.get(label) + "|1|");
        String sd = STREAM_DEF;
        String sd2 = STREAM_DEF_TWO;
        String oj = OUTER_JOIN_DEF;
        rejected.accept("where-int-eq-string", sd + "sa.intPrimitive = sb.theString");
        deploy.accept("where-int-eq-intboxed", sd + "sa.intPrimitive = sb.intBoxed");
        deploy.accept("where-int-eq-int", sd + "sa.intPrimitive = sb.intPrimitive");
        deploy.accept("where-int-eq-longboxed", sd + "sa.intPrimitive = sb.longBoxed");
        rejected.accept("where-and-intboxed-bool", sd + "sa.intPrimitive = sb.intPrimitive and sb.intBoxed = sa.boolPrimitive");
        deploy.accept("where-and-boolboxed-bool", sd + "sa.intPrimitive = sb.intPrimitive and sb.boolBoxed = sa.boolPrimitive");
        rejected.accept("where-and-thestring-sx", sd + "sa.intPrimitive = sb.intPrimitive and sb.intBoxed = sa.intPrimitive and sa.theString=sX.theString");
        deploy.accept("where-and-thestring-sb", sd + "sa.intPrimitive = sb.intPrimitive and sb.intBoxed = sa.intPrimitive and sa.theString=sb.theString");
        rejected.accept("where-or-thestring-sx", sd + "sa.intPrimitive = sb.intPrimitive or sa.theString=sX.theString");
        deploy.accept("where-or-intboxed-eq-int", sd + "sa.intPrimitive = sb.intPrimitive or sb.intBoxed = sa.intPrimitive");
        deploy.accept("where-const-int", sd + "sa.intPrimitive=5");
        deploy.accept("where-const-string-sq", sd + "sa.theString='4'");
        deploy.accept("where-const-string-dq", sd + "sa.theString=\"4\"");
        deploy.accept("where-const-bool", sd + "sa.boolPrimitive=false");
        deploy.accept("where-const-long", sd + "sa.longPrimitive=-5L");
        deploy.accept("where-const-double", sd + "sa.doubleBoxed=5.6d");
        deploy.accept("where-const-float", sd + "sa.floatPrimitive=-5.6f");
        rejected.accept("where-int-eq-str5", sd + "sa.intPrimitive='5'");
        rejected.accept("where-thestring-eq-5", sd + "sa.theString=5");
        rejected.accept("where-boolboxed-eq-f", sd + "sa.boolBoxed=f");
        rejected.accept("where-int-eq-x", sd + "sa.intPrimitive=x");
        deploy.accept("where-int-eq-double", sd + "sa.intPrimitive=5.5");
        deploy.accept("where-int-eq-arith-add", sd + "sa.intPrimitive=sa.intBoxed + 5");
        deploy.accept("where-int-eq-arith-mixed", sd + "sa.intPrimitive=2*sa.intBoxed - sa.intPrimitive/10 + 1");
        deploy.accept("where-int-eq-arith-parens", sd + "sa.intPrimitive=2*(sa.intBoxed - sa.intPrimitive)/(10 + 1)");
        unrep.accept("where-arith-unbalanced", sd + "sa.intPrimitive=2*(sa.intBoxed");
        deploy.accept("where-cmp-cross", sd + "sa.intPrimitive > sa.intBoxed and sb.doublePrimitive < sb.doubleBoxed");
        deploy.accept("where-cmp-same", sd + "sa.intPrimitive >= sa.intBoxed and sa.doublePrimitive <= sa.doubleBoxed");
        deploy.accept("where-cmp-arith", sd + "sa.intPrimitive > (sa.intBoxed + sb.doublePrimitive)");
        divergent.accept("where-int-ge-thestring", sd + "sa.intPrimitive >= sa.theString");
        rejected.accept("where-boolboxed-ge-bool", sd + "sa.boolBoxed >= sa.boolPrimitive");
        deploy.accept("where-nested-or", sd + "(sa.intPrimitive=3) or (sa.intBoxed=3 and sa.intPrimitive=1)");
        deploy.accept("where-nested-and", sd + "((sa.intPrimitive>3) or (sa.intBoxed<3)) and sa.boolBoxed=false");
        deploy.accept("where-nested-mixed", sd + "(sa.intPrimitive<=3 and sa.intPrimitive>=1) or (sa.boolBoxed=false and sa.boolPrimitive=true)");
        unrep.accept("where-or-unbalanced-open", sd + "sa.intPrimitive=3 or (sa.intBoxed=2");
        unrep.accept("where-or-unbalanced-close", sd + "sa.intPrimitive=3 or sa.intBoxed=2)");
        unrep.accept("where-or-unbalanced-nested", sd + "sa.intPrimitive=3 or ((sa.intBoxed=2)");
        unrep.accept("where-unqualified-int", sd + "intPrimitive=3");
        deploy.accept("where-two-unqualified-int", sd2 + "intPrimitive=3");
        deploy.accept("on-int-eq-intboxed", oj + "on sa.intPrimitive = sb.intBoxed");
        rejected.accept("on-int-eq-xx", oj + "on sa.intPrimitive = sb.XX");
        rejected.accept("on-xx-eq-xx", oj + "on sa.XX = sb.XX");
        rejected.accept("on-xx-eq-intboxed", oj + "on sa.XX = sb.intBoxed");
        divergent.accept("on-boolboxed-eq-intboxed", oj + "on sa.boolBoxed = sb.intBoxed");
        deploy.accept("on-bool-eq-boolboxed", oj + "on sa.boolPrimitive = sb.boolBoxed");
        divergent.accept("on-bool-eq-thestring", oj + "on sa.boolPrimitive = sb.theString");
        divergent.accept("on-int-le-intboxed", oj + "on sa.intPrimitive <= sb.intBoxed");
        divergent.accept("on-same-source-left", oj + "on sa.intPrimitive = sa.intBoxed");
        divergent.accept("on-same-source-right", oj + "on sb.intPrimitive = sb.intBoxed");
        deploy.accept("on-int-eq-intboxed-rev", oj + "on sb.intPrimitive = sa.intBoxed");
        steps.add("undeploy-all||||||||");
        return steps.toArray(new String[0]);
    }

    private EPLOtherInvalidScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: EPLOtherInvalidScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        JsonArray records = new JsonArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(caseIndex, allSteps, records);
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS + " records, got "
                    + records.size());
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", ID);
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        root.add("records", records);
        System.out.println(root.toString());
    }

    /**
     * Replays the case's steps on a fresh runtime. Every case registers
     * SupportBean; different-joins also registers SupportMarketDataBean for
     * the streamDefTwo probe. Deploy steps compile and deploy like
     * env.compileDeploy(epl); probes compile without a module path like
     * env.tryInvalidCompile(epl, ...)/compileWCheckedEx.
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        if ("different-joins".equals(caseName)) {
            configuration.getCommon().addEventType(SupportMarketDataBean.class);
        }
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-" + ID + "-" + RUNTIME_IDS[caseIndex], configuration);
        runtime.getEventService().advanceTime(0);
        try {
            Set<String> deployedLabels = new HashSet<>();
            Map<String, Integer> sequences = new HashMap<>();
            long[] listenerSequence = {0};
            boolean inCase = false;
            for (JsonValue stepValue : allSteps) {
                JsonObject step = stepValue.asObject();
                String operation = string(step, "op");
                if ("case".equals(operation)) {
                    inCase = caseName.equals(string(step, "case"));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                switch (operation) {
                    case "deploy":
                        deployStep(runtime, configuration, caseName, step,
                                deployedLabels, records, listenerSequence);
                        break;
                    case "deployed":
                        deployedStep(runtime, caseName, step, deployedLabels,
                                sequences, records);
                        break;
                    case "send":
                        sendStep(runtime, caseName, step);
                        break;
                    case "build-error":
                        rejectedStep(configuration, caseName, step, records);
                        break;
                    case "unrepresentable":
                        unrepresentableStep(configuration, caseName, step, records);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
                }
            }
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
        }
    }

    /**
     * Compiles and deploys the step's single-statement module, mirroring
     * env.compileDeploy(epl). The long-type-constant s0 deploy attaches the
     * trace listener like compileDeploy(epl).addListener("s0").
     */
    private static void deployStep(EPRuntime runtime, Configuration configuration,
                                   String caseName, JsonObject step, Set<String> deployedLabels,
                                   JsonArray records, long[] listenerSequence) throws Exception {
        String label = string(step, "statement");
        String epl = string(step, "epl");
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
        EPStatement[] statements = deployment.getStatements();
        if (statements.length != 1) {
            throw new IllegalStateException(caseName + " deploy " + label + " produced "
                    + statements.length + " statements, want 1");
        }
        if ("long-type-constant".equals(caseName) && "s0".equals(label)) {
            statements[0].addListener(new TraceWriter(records, caseName, runtime,
                    listenerSequence));
        }
        deployedLabels.add(label);
    }

    /**
     * Emits the deployed marker for a statement label the preceding deploy
     * registered, mirroring the per-statement deployed record.
     */
    private static void deployedStep(EPRuntime runtime, String caseName, JsonObject step,
                                     Set<String> deployedLabels, Map<String, Integer> sequences,
                                     JsonArray records) {
        String label = string(step, "statement");
        if (!deployedLabels.contains(label)) {
            throw new IllegalStateException("deployed marker for unknown statement " + label);
        }
        int sequence = sequences.merge(label + ":deployed", 1, Integer::sum);
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "deployed");
        record.add("statement", label);
        record.add("sequence", sequence);
        record.add("time", Instant.ofEpochMilli(
                runtime.getEventService().getCurrentTime()).toString());
        records.add(record);
    }

    /**
     * Mirrors env.sendEventBean(new SupportBean()): the pinned payload is
     * the empty object, so a default-constructed bean is sent.
     */
    private static void sendStep(EPRuntime runtime, String caseName, JsonObject step) {
        String eventType = string(step, "eventType");
        if (!"long-type-constant".equals(caseName) || !"SupportBean".equals(eventType)) {
            throw new IllegalStateException(caseName + " send " + eventType + " is not pinned");
        }
        JsonValue payload = step.get("payload");
        if (payload == null || !payload.isObject() || payload.asObject().size() != 0) {
            throw new IllegalStateException(caseName + " send payload is not the pinned empty object");
        }
        runtime.getEventService().sendEventBean(new SupportBean(), "SupportBean");
    }

    /**
     * Compiles an expected-invalid different-joins probe, mirroring
     * tryInvalid: the compile must throw EPCompileException (no message is
     * asserted), then the pinned 'rejected' marker is recorded as a
     * "compile-error" record.
     */
    private static void rejectedStep(Configuration configuration, String caseName,
                                     JsonObject step, JsonArray records) {
        String label = string(step, "statement");
        String epl = string(step, "epl");
        String expected = string(step, "expectError");
        if (!REJECTED.equals(expected)) {
            throw new IllegalStateException("build-error probe " + label
                    + " carries an unpinned marker " + expected);
        }
        String caught = compileError(configuration, epl);
        if (caught == null) {
            throw new IllegalStateException("build-error probe " + label
                    + " unexpectedly succeeded");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "compile-error");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", expected);
        records.add(record);
    }

    /**
     * Verifies an unrepresentable probe in-process before emitting the
     * pinned record: the compile must throw, and for ords 0-1 the caught
     * message must satisfy the pinned assertion (assertEquals for the two
     * syntax probes, startsWith for the tryInvalidCompile probes). Ord-3
     * unrepresentable probes assert rejection only and record the pinned
     * boundary note; the six divergent probes assert rejection and
     * record the pinned intentionally-different note.
     */
    private static void unrepresentableStep(Configuration configuration, String caseName,
                                            JsonObject step, JsonArray records) {
        String label = string(step, "statement");
        String epl = string(step, "epl");
        String expected = string(step, "expectError");
        String operation = "unrepresentable";
        if ("invalid-func-params".equals(caseName) || "invalid-syntax".equals(caseName)) {
            if (!expected.equals(PROBE_MESSAGES.get(label))) {
                throw new IllegalStateException("unrepresentable step " + label
                        + " carries an unpinned message");
            }
            String caught = compileError(configuration, epl);
            if (caught == null) {
                throw new IllegalStateException("unrepresentable probe " + label
                        + " unexpectedly succeeded");
            }
            if (EXACT_MESSAGE_PROBES.contains(label)) {
                if (!caught.equals(expected)) {
                    throw new IllegalStateException("unrepresentable message drift for "
                            + label + ": expected [" + expected + "] got [" + caught + "]");
                }
            } else if (!caught.startsWith(expected)) {
                throw new IllegalStateException("unrepresentable message drift for "
                        + label + ": expected prefix [" + expected + "] got [" + caught + "]");
            }
        } else if ("different-joins".equals(caseName)) {
            if (UNREPRESENTABLE_NOTES.containsKey(label)) {
                if (!expected.equals(UNREPRESENTABLE_NOTES.get(label))) {
                    throw new IllegalStateException("unrepresentable step " + label
                            + " carries an unpinned note");
                }
            } else if (DIVERGENT_NOTES.containsKey(label)) {
                if (!expected.equals(DIVERGENT_NOTES.get(label))) {
                    throw new IllegalStateException("intentionally-different step " + label
                            + " carries an unpinned note");
                }
                operation = "intentionally-different";
            } else {
                throw new IllegalStateException("unknown unrepresentable step " + label);
            }
            // Java rejects every pinned probe; the oracle verifies the
            // rejection in-process before emitting the marker.
            String caught = compileError(configuration, epl);
            if (caught == null) {
                throw new IllegalStateException("unrepresentable probe " + label
                        + " unexpectedly succeeded");
            }
        } else {
            throw new IllegalStateException("case " + caseName
                    + " has no unrepresentable steps");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", operation);
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", expected);
        records.add(record);
    }

    /**
     * Compiles the EPL without a module path and returns the caught
     * exception message, or null when the compile unexpectedly succeeds.
     */
    private static String compileError(Configuration configuration, String epl) {
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            return null;
        } catch (Exception ex) {
            return ex.getMessage() == null ? ex.getClass().getName() : ex.getMessage();
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
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), JAVA_FLAGS, "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + CASES.length + " cases");
        }
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !CASE_OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        for (String caseName : CASES) {
            validateCaseMarker(steps.get(offset++), caseName);
            String[] expected = CASE_STEPS.get(caseName);
            for (String key : expected) {
                JsonObject step = object(steps.get(offset++), "step");
                String actual = stepKey(step);
                if (!key.equals(actual)) {
                    throw new IllegalArgumentException("step is not pinned for " + caseName
                            + ": expected [" + key + "] got [" + actual + "]");
                }
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    /**
     * Renders one step as its pinned key:
     * op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|at
     * with the payload rendered as compacted JSON. Unknown fields are
     * rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
                "op", "case", "statement", "name", "eventType", "epl", "payload",
                "expectError", "compileWithoutPath", "at"));
        for (String field : step.names()) {
            if (!allowed.contains(field)) {
                throw new IllegalArgumentException("step has unexpected field " + field);
            }
        }
        JsonValue payload = step.get("payload");
        String payloadText = payload == null ? "" : payload.toString();
        String cwp = step.getBoolean("compileWithoutPath", false) ? "1" : "";
        return string(step, "op") + "|" + string(step, "statement") + "|" + string(step, "name")
                + "|" + string(step, "eventType") + "|" + string(step, "epl") + "|" + payloadText
                + "|" + string(step, "expectError") + "|" + cwp + "|" + string(step, "at");
    }

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!names.add(member.getName())) {
                    throw new IllegalArgumentException(
                            "duplicate JSON object key: " + member.getName());
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

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be a JSON object");
        }
        return value.asObject();
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be a JSON array");
        }
        return value.asArray();
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null) {
            return "";
        }
        if (!(value instanceof JsonString)) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be a JSON integer");
        }
        String text = value.toString();
        try {
            long parsed = Long.parseLong(text, 10);
            if (parsed < Integer.MIN_VALUE || parsed > Integer.MAX_VALUE) {
                throw new IllegalArgumentException(name + " is outside the Java int range");
            }
            return (int) parsed;
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(name + " is outside the Java long range", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String label) {
        JsonArray actual = array(value, label);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(label + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (!(item instanceof JsonString) || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(label + " mismatch at index " + index);
            }
        }
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short
                || value instanceof Byte) {
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

        private TraceWriter(JsonArray records, String caseName, EPRuntime runtime,
                            long[] sequence) {
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
                    .add("time", Instant.ofEpochMilli(
                            runtime.getEventService().getCurrentTime()).toString());
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
}
