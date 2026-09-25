package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// epl_other_invalid.go replays the four EPLOtherInvalid executions pinned by
// Draft 4.539 — the suite's compile-time invalidity surface plus one positive
// long-literal execution:
//
//   - invalid-func-params (ord 0, EPLOtherInvalidFuncParams, INVALIDITY):
//     two tryInvalidCompile probes pin the count/leaving arity rejections.
//     Both are unrepresentable in the typed Go API: Count takes exactly one
//     argument and Leaving takes an optional bool predicate, so the Java
//     arities are Go compile-time mismatches with no Build boundary. The
//     unrepresentable records pin the asserted Java messages verbatim.
//   - invalid-syntax (ord 1, EPLOtherInvalidSyntax, INVALIDITY): three
//     probes — two getSyntaxExceptionEPL assertEquals pins (the `from *`
//     parse error and the reserved-keyword `r.start` where-clause error)
//     and one tryInvalidCompile pin (the three-way `1=2=3` equals chain).
//     EPL-text parse errors and the chained-equals AST have no Go
//     counterpart, so all three pin as unrepresentable records carrying
//     the asserted Java messages.
//   - long-type-constant (ord 2, EPLOtherLongTypeConstant, no flags):
//     `select 2512570244 as value from SupportBean` deploys s0 and one
//     SupportBean send delivers value=2512570244 as a Java long. The Go
//     replay projects Literal(int64(2512570244)) — the `L` suffix spelling
//     is EPL-text-only; the int64 width is the parity assertion.
//   - different-joins (ord 3, EPLOtherDifferentJoins, INVALIDITY): the
//     compile-only validity matrix over SupportBean#length(3) sa/sb comma
//     joins, the SupportBean+SupportMarketDataBean pair, and the
//     sa-left-outer-join-sb ON clause. tryInvalid asserts only that
//     compilation fails (no message), so rejected probes pin the 'rejected'
//     marker; tryValid deploys and records a deployed marker.
//
// Approved differences (observably identical to the Java EPL):
//   - Where-clause probes map to JoinMany(...).Select(...).Where(...) with
//     JoinField(source, name) operands; EqualOf carries the numeric/string
//     coercion Java applies to `=` in filters.
//   - Outer-join ON probes map to Join(left, right, OnEqual(Field, Field))
//     .LeftOuter(): the two-stream Field form is the only Go boundary that
//     validates ON-operand field existence (JoinMany On conditions and
//     JoinChain edge conditions validate source indexes only), so the
//     unknown-field probes (sb.XX, sa.XX=sb.XX, sa.XX=sb.intBoxed) reject
//     there.
//   - Type-mismatch invalids (intPrimitive=theString, intBoxed=
//     boolPrimitive, theString=5, boolBoxed=f, intPrimitive='5',
//     boolPrimitive=theString, intPrimitive>=theString, boolBoxed>=
//     boolPrimitive) are unrepresentable: the typed Equal[T]/ordered
//     comparators reject mismatched operand types at Go compile time, so
//     no Build boundary is claimed and the pinned note is recorded.
//   - Unbalanced-paren probes and the ambiguous unqualified intPrimitive=3
//     over two same-type streams are EPL-text-only; the fluent And/Or tree
//     is structural and JoinField requires an explicit source index.
//   - Three Java rejections are intentionally-different: Go accepts
//     `on sa.intPrimitive <= sb.intBoxed` (any JoinComparison is allowed
//     on an outer ON), `on sa.intPrimitive = sa.intBoxed` and
//     `on sb.intPrimitive = sb.intBoxed` (no same-source ON ban). The Go
//     runner verifies the build succeeds; both sides record the pinned
//     intentionally-different note.

const (
	eplOtherInvalidID         = "epl-other-invalid"
	eplOtherInvalidJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	eplOtherInvalidSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherInvalid.java"
)

const eplOtherInvalidDescription = "EPLOtherInvalid (ords 0-3, all executions): invalid-func-params pins two tryInvalidCompile arity rejections — count(theString,theString,theString) expects at least 1 and up to 2 parameters, leaving(theString) expects no parameters — both unrepresentable (Go Count takes one argument, Leaving an optional bool predicate); invalid-syntax pins three parse/validation messages — `select * from *` (incorrect syntax near '*' at line 1 column 14), the reserved-keyword `r.start` where-clause error, and the `SupportBean(1=2=3)` three-way equals chain — all unrepresentable (no Go EPL parser or chained-equals AST); long-type-constant deploys `select 2512570244 as value from SupportBean` and one SupportBean send delivers value=2512570244 as int64; different-joins replays the 50-probe compile-only validity matrix over SupportBean#length(3) sa/sb comma joins, the SupportBean+SupportMarketDataBean pair and the sa-left-outer-join-sb ON clause — 27 deployed, 12 compile-error 'rejected' markers (unknown field/alias, mixed-type EqualOf pairs, non-ordered >= operand), 5 unrepresentable (unbalanced parens, ambiguous unqualified) and 6 intentionally-different (Go accepts non-equi outer ON, same-source ON keys, int>=string where and mixed-type ON keys)."

var (
	eplOtherInvalidJavaRuntimeIDs = []string{
		"java-runtime-57d4ed46cb893b70792c",
		"java-runtime-1ca3b083af4e59da8024",
		"java-runtime-13ae251df45f9dc74efa",
		"java-runtime-7e8cd12d2a61f4041a0e",
	}
	eplOtherInvalidJavaExecutions = []string{
		"EPLOtherInvalidFuncParams",
		"EPLOtherInvalidSyntax",
		"EPLOtherLongTypeConstant",
		"EPLOtherDifferentJoins",
	}
	eplOtherInvalidJavaStaticIDs = []string{
		"java-6cb657c46f10fbb54e9f",
		"java-1f8004583e889e7d519d",
		"java-5f23e933953bbae72045",
		"java-781d7db6d45ac322456b",
	}
	eplOtherInvalidJavaFlags = []string{"INVALIDITY"}
	eplOtherInvalidCases     = []string{
		"invalid-func-params",
		"invalid-syntax",
		"long-type-constant",
		"different-joins",
	}
	eplOtherInvalidOrdinals = []int{0, 1, 2, 3}
	eplOtherInvalidSources  = []string{
		eplOtherInvalidSource,
		eplOtherInvalidSource,
		eplOtherInvalidSource,
		eplOtherInvalidSource,
	}
)

var eplOtherInvalidCaseRuntimeIDs = map[string]string{
	"invalid-func-params": "java-runtime-57d4ed46cb893b70792c",
	"invalid-syntax":      "java-runtime-1ca3b083af4e59da8024",
	"long-type-constant":  "java-runtime-13ae251df45f9dc74efa",
	"different-joins":     "java-runtime-7e8cd12d2a61f4041a0e",
}

// The byte-exact EPLs the probes and deploys pin, transcribed verbatim from
// EPLOtherInvalid.java lines 40-44, 54-61, 71 and 83-162 (streamDef,
// streamDefTwo and outerJoinDef concatenations included).
const (
	eplOtherInvalidEPLCountArity   = "select count(theString, theString, theString) from SupportBean"
	eplOtherInvalidEPLLeavingArity = "select leaving(theString) from SupportBean"
	eplOtherInvalidEPLFromStar     = "select * from *"
	eplOtherInvalidEPLReserved     = "select * from SupportBean a where a.intPrimitive between r.start and r.end"
	eplOtherInvalidEPLFilterChain  = "select * from SupportBean(1=2=3)"
	eplOtherInvalidEPLLongConstant = "@name('s0') select 2512570244 as value from SupportBean"
)

const (
	eplOtherInvalidStreamDef     = "select * from SupportBean#length(3) as sa,SupportBean#length(3) as sb where "
	eplOtherInvalidStreamDefTwo  = "select * from SupportBean#length(3),SupportMarketDataBean#length(3) where "
	eplOtherInvalidOuterJoinDef  = "select * from SupportBean#length(3) as sa left outer join SupportBean#length(3) as sb "
	eplOtherInvalidRejectedValue = "rejected"
)

// The pinned Java messages. Ords 0-1 assert the message text (assertEquals
// for the two syntax probes, SupportMessageAssertUtil startsWith for the
// tryInvalidCompile probes); ord 3 asserts rejection only, so its
// compile-error records pin the 'rejected' marker and its unrepresentable
// and intentionally-different records pin the boundary note.
var eplOtherInvalidProbeEPLs = map[string]string{
	"count-three-args":    eplOtherInvalidEPLCountArity,
	"leaving-with-param":  eplOtherInvalidEPLLeavingArity,
	"from-star":           eplOtherInvalidEPLFromStar,
	"reserved-keyword":    eplOtherInvalidEPLReserved,
	"filter-equals-chain": eplOtherInvalidEPLFilterChain,
}

var eplOtherInvalidProbeMessages = map[string]string{
	"count-three-args":    "Failed to validate select-clause expression 'count(theString,theString,theString)': The 'count' function expects at least 1 and up to 2 parameters",
	"leaving-with-param":  "Failed to validate select-clause expression 'leaving(theString)': The 'leaving' function expects no parameters",
	"from-star":           "Incorrect syntax near '*' at line 1 column 14, please check the from clause [select * from *]",
	"reserved-keyword":    "Incorrect syntax near 'start' (a reserved keyword) at line 1 column 59, please check the where clause [select * from SupportBean a where a.intPrimitive between r.start and r.end]",
	"filter-equals-chain": "Failed to validate filter expression '1=2': Invalid use of equals, expecting left-hand side and right-hand side but received 3 expressions",
}

// eplOtherInvalidUnrepresentableNotes pins the note each ord-3
// unrepresentable record carries: the Java rejection has no typed-Go
// boundary (EPL-text paren streams and unqualified-column ambiguity).
var eplOtherInvalidUnrepresentableNotes = map[string]string{
	"where-arith-unbalanced":     "sa.intPrimitive=2*(sa.intBoxed: the unbalanced-paren EPL text has no fluent form — the Go And/Or/arithmetic tree is structural; Java rejects the parse error at compile time",
	"where-or-unbalanced-open":   "sa.intPrimitive=3 or (sa.intBoxed=2: the unbalanced-paren EPL text has no fluent form — the Go And/Or tree is structural; Java rejects the parse error at compile time",
	"where-or-unbalanced-close":  "sa.intPrimitive=3 or sa.intBoxed=2): the unbalanced-paren EPL text has no fluent form — the Go And/Or tree is structural; Java rejects the parse error at compile time",
	"where-or-unbalanced-nested": "sa.intPrimitive=3 or ((sa.intBoxed=2): the unbalanced-paren EPL text has no fluent form — the Go And/Or tree is structural; Java rejects the parse error at compile time",
	"where-unqualified-int":      "intPrimitive=3 over two SupportBean streams: the unqualified EPL column is ambiguous between sa and sb; Go JoinField requires an explicit source index, so the ambiguity has no fluent form; Java rejects the ambiguous property at compile time",
}

// eplOtherInvalidIntentionallyDifferentNotes pins the divergence records:
// Java rejects these clauses while the Go build accepts them — the
// non-equi <= outer-join key, the two same-source ON keys, the
// int-vs-string >= where comparison (GreaterOrEqualOf takes untyped
// operands and its ordered-type check accepts strings) and the two
// mixed-type ON keys (OnSourcesEqual never type-checks join-field
// operands).
var eplOtherInvalidIntentionallyDifferentNotes = map[string]string{
	"where-int-ge-thestring":   "sa.intPrimitive >= sa.theString: Java rejects the int-vs-string relational comparison at compile time; Go GreaterOrEqualOf takes untyped operands and its ordered-type check accepts strings, so the build succeeds",
	"on-boolboxed-eq-intboxed": "on sa.boolBoxed = sb.intBoxed: Java rejects the bool-vs-int ON key type mismatch at compile time; Go OnSourcesEqual takes untyped operands and never type-checks join-field operands, so the build succeeds",
	"on-bool-eq-thestring":     "on sa.boolPrimitive = sb.theString: Java rejects the bool-vs-string ON key type mismatch at compile time; Go OnSourcesEqual takes untyped operands and never type-checks join-field operands, so the build succeeds",
	"on-int-le-intboxed":       "on sa.intPrimitive <= sb.intBoxed: Java restricts outer-join ON to equality and rejects the <= comparison; Go accepts any JoinComparison on an outer join edge, so the build succeeds",
	"on-same-source-left":      "on sa.intPrimitive = sa.intBoxed: Java requires outer-join ON keys to span both streams and rejects the same-source pair; Go validates field existence and source indexes only, so the build succeeds",
	"on-same-source-right":     "on sb.intPrimitive = sb.intBoxed: Java requires outer-join ON keys to span both streams and rejects the same-source pair; Go validates field existence and source indexes only, so the build succeeds",
}

var eplOtherInvalidCaseObservations = []string{
	"unrepresentable; two tryInvalidCompile probes pin the count/leaving arity messages verbatim — the three-argument count and the parameterized leaving have no typed-Go form (Count takes one argument, Leaving an optional bool predicate), so no Go rejection boundary is claimed",
	"unrepresentable; three probes pin the asserted Java messages verbatim — the `select * from *` and reserved-keyword `r.start` parse errors and the `SupportBean(1=2=3)` three-way equals chain are EPL-text surfaces with no Go AST",
	"deployed+listener; `select 2512570244 as value from SupportBean` deploys s0 and one SupportBean send delivers value=2512570244 as int64 (the literal exceeds int32, exercising Java's long literal typing)",
	"deployed+compile-error+unrepresentable+intentionally-different; the 50-probe validity matrix: 27 tryValid deploys, 12 compile-error 'rejected' markers (unknown field/alias sb.XX, sa.XX pairs, sX.theString, intPrimitive=x, boolBoxed=f plus the mixed-type EqualOf pairs and the bool >= operand Go rejects), 5 unrepresentable pins (unbalanced parens, ambiguous unqualified intPrimitive) and 6 intentionally-different pins (Go accepts the <= outer-join ON, the two same-source ON keys, the int>=string where and the two mixed-type ON keys Java rejects)",
}
var eplOtherInvalidCaseEPLs = []string{
	eplOtherInvalidEPLCountArity,
	eplOtherInvalidEPLFromStar,
	eplOtherInvalidEPLLongConstant,
	eplOtherInvalidStreamDef + "sa.intPrimitive = sb.theString",
}

// eplOtherInvalidCaseSteps pins the complete step sequence per case as
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|at
// keys. unrepresentable steps carry the pinned note or Java message in
// expectError; build-error steps pin the 'rejected' marker (Java asserts
// failure only); deploy steps carry the byte-exact tryValid EPL.
var eplOtherInvalidCaseSteps = map[string][]string{
	"invalid-func-params": {
		"unrepresentable|count-three-args|||" + eplOtherInvalidEPLCountArity + "||" + eplOtherInvalidProbeMessages["count-three-args"] + "|1|",
		"unrepresentable|leaving-with-param|||" + eplOtherInvalidEPLLeavingArity + "||" + eplOtherInvalidProbeMessages["leaving-with-param"] + "|1|",
	},
	"invalid-syntax": {
		"unrepresentable|from-star|||" + eplOtherInvalidEPLFromStar + "||" + eplOtherInvalidProbeMessages["from-star"] + "|1|",
		"unrepresentable|reserved-keyword|||" + eplOtherInvalidEPLReserved + "||" + eplOtherInvalidProbeMessages["reserved-keyword"] + "|1|",
		"unrepresentable|filter-equals-chain|||" + eplOtherInvalidEPLFilterChain + "||" + eplOtherInvalidProbeMessages["filter-equals-chain"] + "|1|",
	},
	"long-type-constant": {
		"deploy|s0|||" + eplOtherInvalidEPLLongConstant + "||||",
		"deployed|s0|||||||",
		"send|||SupportBean||{}|||",
		"undeploy-all||||||||",
	},
	"different-joins": eplOtherInvalidDifferentJoinsSteps(),
}

// eplOtherInvalidDifferentJoinsSteps renders the ord-3 step sequence in Java
// source order (EPLOtherInvalid.java lines 93-164): each tryValid is a
// deploy+deployed pair, each representable tryInvalid a build-error step
// pinning 'rejected', each unrepresentable or divergent tryInvalid an
// unrepresentable step pinning its note.
func eplOtherInvalidDifferentJoinsSteps() []string {
	deploy := func(label, condition string) []string {
		return []string{
			"deploy|" + label + "|||" + condition + "||||",
			"deployed|" + label + "|||||||",
		}
	}
	rejected := func(label, condition string) []string {
		return []string{"build-error|" + label + "|||" + condition + "||" + eplOtherInvalidRejectedValue + "|1|"}
	}
	unrep := func(label, condition string) []string {
		return []string{"unrepresentable|" + label + "|||" + condition + "||" + eplOtherInvalidUnrepresentableNotes[label] + "|1|"}
	}
	divergent := func(label, condition string) []string {
		return []string{"unrepresentable|" + label + "|||" + condition + "||" + eplOtherInvalidIntentionallyDifferentNotes[label] + "|1|"}
	}
	sd := eplOtherInvalidStreamDef
	sd2 := eplOtherInvalidStreamDefTwo
	oj := eplOtherInvalidOuterJoinDef
	steps := []string{}
	appendSteps := func(entries ...[]string) {
		for _, entry := range entries {
			steps = append(steps, entry...)
		}
	}
	appendSteps(
		rejected("where-int-eq-string", sd+"sa.intPrimitive = sb.theString"),
		deploy("where-int-eq-intboxed", sd+"sa.intPrimitive = sb.intBoxed"),
		deploy("where-int-eq-int", sd+"sa.intPrimitive = sb.intPrimitive"),
		deploy("where-int-eq-longboxed", sd+"sa.intPrimitive = sb.longBoxed"),
		rejected("where-and-intboxed-bool", sd+"sa.intPrimitive = sb.intPrimitive and sb.intBoxed = sa.boolPrimitive"),
		deploy("where-and-boolboxed-bool", sd+"sa.intPrimitive = sb.intPrimitive and sb.boolBoxed = sa.boolPrimitive"),
		rejected("where-and-thestring-sx", sd+"sa.intPrimitive = sb.intPrimitive and sb.intBoxed = sa.intPrimitive and sa.theString=sX.theString"),
		deploy("where-and-thestring-sb", sd+"sa.intPrimitive = sb.intPrimitive and sb.intBoxed = sa.intPrimitive and sa.theString=sb.theString"),
		rejected("where-or-thestring-sx", sd+"sa.intPrimitive = sb.intPrimitive or sa.theString=sX.theString"),
		deploy("where-or-intboxed-eq-int", sd+"sa.intPrimitive = sb.intPrimitive or sb.intBoxed = sa.intPrimitive"),
		deploy("where-const-int", sd+"sa.intPrimitive=5"),
		deploy("where-const-string-sq", sd+"sa.theString='4'"),
		deploy("where-const-string-dq", sd+"sa.theString=\"4\""),
		deploy("where-const-bool", sd+"sa.boolPrimitive=false"),
		deploy("where-const-long", sd+"sa.longPrimitive=-5L"),
		deploy("where-const-double", sd+"sa.doubleBoxed=5.6d"),
		deploy("where-const-float", sd+"sa.floatPrimitive=-5.6f"),
		rejected("where-int-eq-str5", sd+"sa.intPrimitive='5'"),
		rejected("where-thestring-eq-5", sd+"sa.theString=5"),
		rejected("where-boolboxed-eq-f", sd+"sa.boolBoxed=f"),
		rejected("where-int-eq-x", sd+"sa.intPrimitive=x"),
		deploy("where-int-eq-double", sd+"sa.intPrimitive=5.5"),
		deploy("where-int-eq-arith-add", sd+"sa.intPrimitive=sa.intBoxed + 5"),
		deploy("where-int-eq-arith-mixed", sd+"sa.intPrimitive=2*sa.intBoxed - sa.intPrimitive/10 + 1"),
		deploy("where-int-eq-arith-parens", sd+"sa.intPrimitive=2*(sa.intBoxed - sa.intPrimitive)/(10 + 1)"),
		unrep("where-arith-unbalanced", sd+"sa.intPrimitive=2*(sa.intBoxed"),
		deploy("where-cmp-cross", sd+"sa.intPrimitive > sa.intBoxed and sb.doublePrimitive < sb.doubleBoxed"),
		deploy("where-cmp-same", sd+"sa.intPrimitive >= sa.intBoxed and sa.doublePrimitive <= sa.doubleBoxed"),
		deploy("where-cmp-arith", sd+"sa.intPrimitive > (sa.intBoxed + sb.doublePrimitive)"),
		divergent("where-int-ge-thestring", sd+"sa.intPrimitive >= sa.theString"),
		rejected("where-boolboxed-ge-bool", sd+"sa.boolBoxed >= sa.boolPrimitive"),
		deploy("where-nested-or", sd+"(sa.intPrimitive=3) or (sa.intBoxed=3 and sa.intPrimitive=1)"),
		deploy("where-nested-and", sd+"((sa.intPrimitive>3) or (sa.intBoxed<3)) and sa.boolBoxed=false"),
		deploy("where-nested-mixed", sd+"(sa.intPrimitive<=3 and sa.intPrimitive>=1) or (sa.boolBoxed=false and sa.boolPrimitive=true)"),
		unrep("where-or-unbalanced-open", sd+"sa.intPrimitive=3 or (sa.intBoxed=2"),
		unrep("where-or-unbalanced-close", sd+"sa.intPrimitive=3 or sa.intBoxed=2)"),
		unrep("where-or-unbalanced-nested", sd+"sa.intPrimitive=3 or ((sa.intBoxed=2)"),
		unrep("where-unqualified-int", sd+"intPrimitive=3"),
		deploy("where-two-unqualified-int", sd2+"intPrimitive=3"),
		deploy("on-int-eq-intboxed", oj+"on sa.intPrimitive = sb.intBoxed"),
		rejected("on-int-eq-xx", oj+"on sa.intPrimitive = sb.XX"),
		rejected("on-xx-eq-xx", oj+"on sa.XX = sb.XX"),
		rejected("on-xx-eq-intboxed", oj+"on sa.XX = sb.intBoxed"),
		divergent("on-boolboxed-eq-intboxed", oj+"on sa.boolBoxed = sb.intBoxed"),
		deploy("on-bool-eq-boolboxed", oj+"on sa.boolPrimitive = sb.boolBoxed"),
		divergent("on-bool-eq-thestring", oj+"on sa.boolPrimitive = sb.theString"),
		divergent("on-int-le-intboxed", oj+"on sa.intPrimitive <= sb.intBoxed"),
		divergent("on-same-source-left", oj+"on sa.intPrimitive = sa.intBoxed"),
		divergent("on-same-source-right", oj+"on sb.intPrimitive = sb.intBoxed"),
		deploy("on-int-eq-intboxed-rev", oj+"on sb.intPrimitive = sa.intBoxed"),
	)
	steps = append(steps, "undeploy-all||||||||")
	return steps
}

// eplOtherInvalidBean mirrors the SupportBean fields the join matrix
// exercises.
type eplOtherInvalidBean struct {
	TheString       string   `esper:"theString"`
	IntPrimitive    int      `esper:"intPrimitive"`
	IntBoxed        *int     `esper:"intBoxed"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	LongBoxed       *int64   `esper:"longBoxed"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
}

// eplOtherInvalidMarket mirrors SupportMarketDataBean for the two-type
// streamDefTwo probe.
type eplOtherInvalidMarket struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
	Feed   *string `esper:"feed"`
}

// eplOtherInvalidCaseState carries the per-case replay state: the
// environment/engine pair, deployed-statement bookkeeping and the per-label
// sequence counters.
type eplOtherInvalidCaseState struct {
	caseName       string
	env            *esper.Environment
	engine         *esper.Engine
	trace          *compat.Trace
	deployedLabels map[string]bool
	deployments    map[string]*esper.Deployment
	deployOrder    []string
	statements     map[string]*esper.Statement
	sequences      map[string]uint64
	listenerSeq    map[string]uint64
}

func runEplOtherInvalidScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateEplOtherInvalidScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	return executeEplOtherInvalid(ctx, scenario, &trace)
}

// executeEplOtherInvalid replays the scenario: each case runs on a fresh
// environment/engine pair (one runtime per Java execution) and every step
// dispatches to the matching runtime action.
func executeEplOtherInvalid(ctx context.Context, scenario compat.Scenario, trace *compat.Trace) (compat.Trace, error) {
	var state *eplOtherInvalidCaseState
	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return *trace, err
		}
		if step.Op != "case" && state == nil {
			return *trace, fmt.Errorf("%s: step %q arrives before any case marker", eplOtherInvalidID, step.Op)
		}
		switch step.Op {
		case "case":
			if state != nil && state.engine != nil {
				_ = state.engine.Close(context.Background())
			}
			var err error
			state, err = startEplOtherInvalidCase(step.Case, trace)
			if err != nil {
				return *trace, err
			}
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return *trace, err
			}
		case "deployed":
			if err := state.deployed(step); err != nil {
				return *trace, err
			}
		case "send":
			if err := state.send(ctx, step); err != nil {
				return *trace, err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
				return *trace, err
			}
		case "unrepresentable":
			if err := state.unrepresentable(step); err != nil {
				return *trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *trace, err
			}
		default:
			return *trace, fmt.Errorf("%s: unsupported step op %q", eplOtherInvalidID, step.Op)
		}
	}
	if state != nil && state.engine != nil {
		_ = state.engine.Close(context.Background())
	}
	return *trace, nil
}

// startEplOtherInvalidCase builds the fresh per-case environment: the
// SupportBean event type every case declares plus the SupportMarketDataBean
// type the different-joins suite registers, and the engine pinned to the
// case's Java runtime id at the epoch start time.
func startEplOtherInvalidCase(caseName string, trace *compat.Trace) (*eplOtherInvalidCaseState, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[eplOtherInvalidBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if caseName == "different-joins" {
		if _, err := esper.RegisterStruct[eplOtherInvalidMarket](env, "SupportMarketDataBean"); err != nil {
			return nil, err
		}
	}
	state := &eplOtherInvalidCaseState{
		caseName:       caseName,
		env:            env,
		deployedLabels: map[string]bool{},
		deployments:    map[string]*esper.Deployment{},
		statements:     map[string]*esper.Statement{},
		sequences:      map[string]uint64{},
		listenerSeq:    map[string]uint64{},
		trace:          trace,
	}
	state.engine = esper.NewEngine(env,
		esper.WithRuntimeURI(eplOtherInvalidCaseRuntimeIDs[caseName]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	return state, nil
}

// deploy executes one deploy step: the long-type-constant s0 select or one
// different-joins tryValid fluent equivalent of the pinned EPL. The s0
// statement subscribes the trace listener like env.compileDeploy(epl)
// .addListener("s0").
func (s *eplOtherInvalidCaseState) deploy(ctx context.Context, step compat.Step) error {
	label := step.Statement
	var query esper.Query
	switch s.caseName {
	case "long-type-constant":
		if label != "s0" || step.Epl != eplOtherInvalidEPLLongConstant {
			return fmt.Errorf("%s: case %q deploy %q carries an unpinned EPL %q", eplOtherInvalidID, s.caseName, label, step.Epl)
		}
		supportBean := esper.From[eplOtherInvalidBean](s.env, "SupportBean")
		query = esper.Select(supportBean,
			esper.Alias("value", esper.Literal(int64(2512570244)))).
			Query(esper.StatementName("s0"))
	case "different-joins":
		built, err := s.differentJoinsQuery(label, step.Epl)
		if err != nil {
			return err
		}
		query = built
	default:
		return fmt.Errorf("%s: case %q has no deploy steps", eplOtherInvalidID, s.caseName)
	}
	plan, err := s.env.Build(query)
	if err != nil {
		return fmt.Errorf("%s: deploy %q: %w", eplOtherInvalidID, label, err)
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: deploy %q: %w", eplOtherInvalidID, label, err)
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return fmt.Errorf("%s: deploy %q produced %d statements, want 1", eplOtherInvalidID, label, len(statements))
	}
	s.deployedLabels[label] = true
	s.deployments[label] = deployment
	s.deployOrder = append(s.deployOrder, label)
	s.statements[label] = statements[0]
	if s.caseName == "long-type-constant" && label == "s0" {
		statement := statements[0]
		if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			if len(batch.New) == 0 && len(batch.Old) == 0 {
				return nil
			}
			s.listenerSeq[statement.Name()]++
			record := compat.TraceRecord{
				Case:      s.caseName,
				Operation: "listener",
				Statement: statement.Name(),
				Sequence:  s.listenerSeq[statement.Name()],
				Time:      compat.FormatTraceTime(s.engine.Now()),
				New:       compat.NormalizeResults(batch.New),
				Old:       compat.NormalizeResults(batch.Old),
			}
			s.trace.Records = append(s.trace.Records, record)
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// deployed emits the deployed marker for a statement label the preceding
// deploy step registered, mirroring the oracle's per-statement marker.
func (s *eplOtherInvalidCaseState) deployed(step compat.Step) error {
	if !s.deployedLabels[step.Statement] {
		return fmt.Errorf("%s: deployed marker for unknown statement %q", eplOtherInvalidID, step.Statement)
	}
	s.sequences[step.Statement+":deployed"]++
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "deployed",
		Statement: step.Statement,
		Sequence:  s.sequences[step.Statement+":deployed"],
		Time:      compat.FormatTraceTime(s.engine.Now()),
	})
	return nil
}

// send mirrors env.sendEventBean(new SupportBean()): the pinned payload
// decodes to the zero bean and is sent to the runtime.
func (s *eplOtherInvalidCaseState) send(ctx context.Context, step compat.Step) error {
	if s.caseName != "long-type-constant" || step.EventType != "SupportBean" {
		return fmt.Errorf("%s: case %q send %q is not pinned", eplOtherInvalidID, s.caseName, step.EventType)
	}
	var bean eplOtherInvalidBean
	if err := json.Unmarshal(step.Payload, &bean); err != nil {
		return fmt.Errorf("%s: decode SupportBean payload: %w", eplOtherInvalidID, err)
	}
	if err := s.engine.Send(ctx, step.EventType, bean); err != nil {
		return fmt.Errorf("%s: send: %w", eplOtherInvalidID, err)
	}
	return nil
}

// buildError runs one expected-invalid different-joins probe against the
// fluent equivalent of the pinned EPL. Each probe verifies Go rejects the
// nearest expressible boundary — the unknown-field/alias checks in the join
// where scope or the two-stream ON field validation — before recording the
// pinned 'rejected' marker (Java asserts failure only, no message).
func (s *eplOtherInvalidCaseState) buildError(step compat.Step) error {
	if s.caseName != "different-joins" {
		return fmt.Errorf("%s: case %q has no build-error steps", eplOtherInvalidID, s.caseName)
	}
	if step.ExpectError != eplOtherInvalidRejectedValue {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned marker %q", eplOtherInvalidID, step.Statement, step.ExpectError)
	}
	if err := eplOtherInvalidStepEPLFor(step); err != nil {
		return err
	}
	var buildErr error
	switch step.Statement {
	case "where-and-thestring-sx":
		// `... and sa.theString=sX.theString` — the unknown alias sX maps to
		// a source index outside the two-source join.
		buildErr = s.buildJoinWhere(step, esper.And(
			esper.EqualOf(esper.JoinField[int](0, "intPrimitive"), esper.JoinField[int](1, "intPrimitive")),
			esper.And(
				esper.EqualOf(esper.JoinField[*int](1, "intBoxed"), esper.JoinField[int](0, "intPrimitive")),
				esper.EqualOf(esper.JoinField[string](0, "theString"), esper.JoinField[string](2, "theString")))))
	case "where-or-thestring-sx":
		// `... or sa.theString=sX.theString` — same unknown-alias boundary.
		buildErr = s.buildJoinWhere(step, esper.Or(
			esper.EqualOf(esper.JoinField[int](0, "intPrimitive"), esper.JoinField[int](1, "intPrimitive")),
			esper.EqualOf(esper.JoinField[string](0, "theString"), esper.JoinField[string](2, "theString"))))
	case "where-int-eq-x":
		// `sa.intPrimitive=x` — the bare identifier x resolves against the
		// stream schema and fails the unknown-field check.
		buildErr = s.buildJoinWhere(step,
			esper.EqualOf(esper.JoinField[int](0, "intPrimitive"), esper.JoinField[int](0, "x")))
	case "where-int-eq-string":
		// `sa.intPrimitive = sb.theString` — EqualOf takes untyped operands;
		// the mixed int-vs-string pair fails the equality-type check.
		buildErr = s.buildJoinWhere(step,
			esper.EqualOf(esper.JoinField[int](0, "intPrimitive"), esper.JoinField[string](1, "theString")))
	case "where-and-intboxed-bool":
		// `... and sb.intBoxed = sa.boolPrimitive` — the mixed *int-vs-bool
		// EqualOf pair fails the same equality-type check.
		buildErr = s.buildJoinWhere(step, esper.And(
			esper.EqualOf(esper.JoinField[int](0, "intPrimitive"), esper.JoinField[int](1, "intPrimitive")),
			esper.EqualOf(esper.JoinField[*int](1, "intBoxed"), esper.JoinField[bool](0, "boolPrimitive"))))
	case "where-int-eq-str5":
		// `sa.intPrimitive='5'` — the int-vs-string-literal EqualOf pair
		// fails the equality-type check.
		buildErr = s.buildJoinWhere(step,
			esper.EqualOf(esper.JoinField[int](0, "intPrimitive"), esper.Literal("5")))
	case "where-thestring-eq-5":
		// `sa.theString=5` — the string-vs-int-literal EqualOf pair fails
		// the equality-type check.
		buildErr = s.buildJoinWhere(step,
			esper.EqualOf(esper.JoinField[string](0, "theString"), esper.Literal(5)))
	case "where-boolboxed-eq-f":
		// `sa.boolBoxed=f` — the bare identifier f resolves against the
		// stream schema and fails the unknown-field check, the same
		// boundary as where-int-eq-x.
		buildErr = s.buildJoinWhere(step,
			esper.EqualOf(esper.JoinField[*bool](0, "boolBoxed"), esper.JoinField[*bool](0, "f")))
	case "where-boolboxed-ge-bool":
		// `sa.boolBoxed >= sa.boolPrimitive` — GreaterOrEqualOf unwraps the
		// *bool operand to bool, which is not an ordered type.
		buildErr = s.buildJoinWhere(step,
			esper.GreaterOrEqualOf(esper.JoinField[*bool](0, "boolBoxed"), esper.JoinField[bool](0, "boolPrimitive")))
	case "on-int-eq-xx", "on-xx-eq-xx", "on-xx-eq-intboxed":
		// The two-stream Field form is the only Go boundary that validates
		// ON-operand field existence.
		buildErr = s.buildOuterJoinOn(step)
	default:
		return fmt.Errorf("%s: unknown build-error probe %q", eplOtherInvalidID, step.Statement)
	}
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", eplOtherInvalidID, step.Statement)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// unrepresentable emits the pinned record for a Java surface with no Go
// boundary. Ords 0-1 record the pinned Java message; ord-3 unrepresentable
// probes record the pinned boundary note; the six divergent clauses
// verify the Go build accepts the fluent equivalent (Java rejects) before
// recording the pinned intentionally-different note.
func (s *eplOtherInvalidCaseState) unrepresentable(step compat.Step) error {
	operation := "unrepresentable"
	var want string
	switch s.caseName {
	case "invalid-func-params", "invalid-syntax":
		pinned, ok := eplOtherInvalidProbeEPLs[step.Statement]
		if !ok || step.Epl != pinned {
			return fmt.Errorf("%s: unrepresentable step %q carries an unpinned EPL %q", eplOtherInvalidID, step.Statement, step.Epl)
		}
		want = eplOtherInvalidProbeMessages[step.Statement]
	case "different-joins":
		if err := eplOtherInvalidStepEPLFor(step); err != nil {
			return err
		}
		if note, ok := eplOtherInvalidUnrepresentableNotes[step.Statement]; ok {
			want = note
		} else if note, ok := eplOtherInvalidIntentionallyDifferentNotes[step.Statement]; ok {
			// Go accepts the fluent equivalent Java rejects; verify the
			// build succeeds before pinning the divergence.
			if buildErr := s.buildDivergentJoin(step); buildErr != nil {
				return fmt.Errorf("%s: intentionally-different probe %q build drift: %w", eplOtherInvalidID, step.Statement, buildErr)
			}
			want = note
			operation = "intentionally-different"
		} else {
			return fmt.Errorf("%s: unknown unrepresentable step %q", eplOtherInvalidID, step.Statement)
		}
	default:
		return fmt.Errorf("%s: case %q has no unrepresentable steps", eplOtherInvalidID, s.caseName)
	}
	if step.ExpectError != want {
		return fmt.Errorf("%s: unrepresentable step %q carries an unpinned value %q", eplOtherInvalidID, step.Statement, step.ExpectError)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: operation,
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// undeployAll mirrors env.undeployAll(): every deployment registered by the
// case's deploy steps is undeployed in deploy order.
func (s *eplOtherInvalidCaseState) undeployAll(ctx context.Context) error {
	for _, label := range s.deployOrder {
		deployment, ok := s.deployments[label]
		if !ok {
			continue
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", eplOtherInvalidID, label, err)
		}
		delete(s.deployments, label)
	}
	s.deployOrder = nil
	return nil
}

// joinSources returns the sa/sb length(3) window pair the different-joins
// probes share.
func (s *eplOtherInvalidCaseState) joinSources() (esper.JoinInput, esper.JoinInput) {
	sa := esper.From[eplOtherInvalidBean](s.env, "SupportBean").Window(esper.LengthWindow(3))
	sb := esper.From[eplOtherInvalidBean](s.env, "SupportBean").Window(esper.LengthWindow(3))
	return esper.JoinSource(sa), esper.JoinSource(sb)
}

// joinSelect pins the wildcard projection's fluent form: both source events
// under their stream aliases.
func eplOtherInvalidJoinSelect() []esper.JoinSelection {
	return []esper.JoinSelection{
		esper.SelectSourceEvent(0, "sa"),
		esper.SelectSourceEvent(1, "sb"),
	}
}

// buildJoinWhere builds the comma-join fluent equivalent of
// `select * from sa,sb where <predicate>` without deploying.
func (s *eplOtherInvalidCaseState) buildJoinWhere(step compat.Step, predicate esper.Expression[bool]) error {
	sa, sb := s.joinSources()
	_, err := s.env.Build(esper.JoinMany(sa, sb).Select(eplOtherInvalidJoinSelect()...).Where(predicate).Query())
	return err
}

// buildOuterJoinOn builds the left-outer-join fluent equivalent of
// `select * from sa left outer join sb on <condition>` using the two-stream
// Field form — the only Go boundary that validates ON-operand field
// existence — without deploying.
func (s *eplOtherInvalidCaseState) buildOuterJoinOn(step compat.Step) error {
	sa := esper.From[eplOtherInvalidBean](s.env, "SupportBean").Window(esper.LengthWindow(3))
	sb := esper.From[eplOtherInvalidBean](s.env, "SupportBean").Window(esper.LengthWindow(3))
	var condition esper.JoinCondition
	switch step.Statement {
	case "on-int-eq-xx":
		condition = esper.OnEqual(
			esper.Field[eplOtherInvalidBean, int]("intPrimitive"),
			esper.Field[eplOtherInvalidBean, int]("XX"))
	case "on-xx-eq-xx":
		condition = esper.OnEqual(
			esper.Field[eplOtherInvalidBean, int]("XX"),
			esper.Field[eplOtherInvalidBean, int]("XX"))
	case "on-xx-eq-intboxed":
		condition = esper.OnEqual(
			esper.Field[eplOtherInvalidBean, int]("XX"),
			esper.Field[eplOtherInvalidBean, *int]("intBoxed"))
	default:
		return fmt.Errorf("%s: probe %q is not an ON build-error", eplOtherInvalidID, step.Statement)
	}
	_, err := s.env.Build(esper.Join(sa, sb, condition).LeftOuter().
		Select(eplOtherInvalidJoinSelect()...).Query())
	return err
}

// buildDivergentJoin builds the fluent equivalent of a clause Java rejects
// but Go accepts: the non-equi <= outer-join key, the two same-source ON
// keys, the two mixed-type ON keys (OnSourcesEqual never type-checks
// join-field operands) and the int-vs-string >= where comparison
// (GreaterOrEqualOf's ordered-type check accepts strings). A nil return
// proves the divergence.
func (s *eplOtherInvalidCaseState) buildDivergentJoin(step compat.Step) error {
	if step.Statement == "where-int-ge-thestring" {
		return s.buildJoinWhere(step,
			esper.GreaterOrEqualOf(esper.JoinField[int](0, "intPrimitive"), esper.JoinField[string](0, "theString")))
	}
	sa := esper.From[eplOtherInvalidBean](s.env, "SupportBean").Window(esper.LengthWindow(3))
	sb := esper.From[eplOtherInvalidBean](s.env, "SupportBean").Window(esper.LengthWindow(3))
	var condition esper.JoinCondition
	switch step.Statement {
	case "on-int-le-intboxed":
		condition = esper.OnSourcesCompare(0,
			esper.JoinField[int](0, "intPrimitive"),
			1, esper.JoinField[*int](1, "intBoxed"), esper.JoinLessOrEqual)
	case "on-same-source-left":
		condition = esper.OnSourcesEqual(0,
			esper.JoinField[int](0, "intPrimitive"),
			0, esper.JoinField[*int](0, "intBoxed"))
	case "on-same-source-right":
		condition = esper.OnSourcesEqual(1,
			esper.JoinField[int](1, "intPrimitive"),
			1, esper.JoinField[*int](1, "intBoxed"))
	case "on-boolboxed-eq-intboxed":
		condition = esper.OnSourcesEqual(0,
			esper.JoinField[*bool](0, "boolBoxed"),
			1, esper.JoinField[*int](1, "intBoxed"))
	case "on-bool-eq-thestring":
		condition = esper.OnSourcesEqual(0,
			esper.JoinField[bool](0, "boolPrimitive"),
			1, esper.JoinField[string](1, "theString"))
	default:
		return fmt.Errorf("%s: probe %q is not a divergent clause", eplOtherInvalidID, step.Statement)
	}
	_, err := s.env.Build(esper.JoinMany(esper.JoinSource(sa), esper.JoinSource(sb)).LeftOuter().On(condition).
		Select(eplOtherInvalidJoinSelect()...).Query())
	return err
}

// differentJoinsQuery renders the fluent equivalent of one pinned tryValid
// EPL. The comma joins map to JoinMany(...).Select(...).Where(...) with
// EqualOf/ordered-comparator operands; the outer joins map to
// JoinMany(...).LeftOuter().On(OnSourcesEqual/OnSourcesCompare); the
// two-type probe joins SupportBean with SupportMarketDataBean.
func (s *eplOtherInvalidCaseState) differentJoinsQuery(label, epl string) (esper.Query, error) {
	sa, sb := s.joinSources()
	where := func(predicate esper.Expression[bool]) esper.Query {
		return esper.JoinMany(sa, sb).Select(eplOtherInvalidJoinSelect()...).Where(predicate).Query()
	}
	on := func(conditions ...esper.JoinCondition) esper.Query {
		return esper.JoinMany(sa, sb).LeftOuter().On(conditions...).Select(eplOtherInvalidJoinSelect()...).Query()
	}
	intP := func(source int) esper.Expression[int] { return esper.JoinField[int](source, "intPrimitive") }
	intB := func(source int) esper.Expression[*int] { return esper.JoinField[*int](source, "intBoxed") }
	longB := func(source int) esper.Expression[*int64] { return esper.JoinField[*int64](source, "longBoxed") }
	str := func(source int) esper.Expression[string] { return esper.JoinField[string](source, "theString") }
	boolP := func(source int) esper.Expression[bool] { return esper.JoinField[bool](source, "boolPrimitive") }
	boolB := func(source int) esper.Expression[*bool] { return esper.JoinField[*bool](source, "boolBoxed") }
	dblP := func(source int) esper.Expression[float64] { return esper.JoinField[float64](source, "doublePrimitive") }
	dblB := func(source int) esper.Expression[*float64] { return esper.JoinField[*float64](source, "doubleBoxed") }

	var query esper.Query
	switch label {
	case "where-int-eq-intboxed":
		query = where(esper.EqualOf(intP(0), intB(1)))
	case "where-int-eq-int":
		query = where(esper.EqualOf(intP(0), intP(1)))
	case "where-int-eq-longboxed":
		query = where(esper.EqualOf(intP(0), longB(1)))
	case "where-and-boolboxed-bool":
		query = where(esper.And(
			esper.EqualOf(intP(0), intP(1)),
			esper.EqualOf(boolB(1), boolP(0))))
	case "where-and-thestring-sb":
		query = where(esper.And(
			esper.EqualOf(intP(0), intP(1)),
			esper.And(
				esper.EqualOf(intB(1), intP(0)),
				esper.EqualOf(str(0), str(1)))))
	case "where-or-intboxed-eq-int":
		query = where(esper.Or(
			esper.EqualOf(intP(0), intP(1)),
			esper.EqualOf(intB(1), intP(0))))
	case "where-const-int":
		query = where(esper.EqualOf(intP(0), esper.Literal(5)))
	case "where-const-string-sq", "where-const-string-dq":
		// '4' and "4" are the same EPL string literal.
		query = where(esper.EqualOf(str(0), esper.Literal("4")))
	case "where-const-bool":
		query = where(esper.EqualOf(boolP(0), esper.Literal(false)))
	case "where-const-long":
		query = where(esper.EqualOf(esper.JoinField[int64](0, "longPrimitive"), esper.Literal(int64(-5))))
	case "where-const-double":
		query = where(esper.EqualOf(dblB(0), esper.Literal(5.6)))
	case "where-const-float":
		query = where(esper.EqualOf(esper.JoinField[float32](0, "floatPrimitive"), esper.Literal(float32(-5.6))))
	case "where-int-eq-double":
		query = where(esper.EqualOf(intP(0), esper.Literal(5.5)))
	case "where-int-eq-arith-add":
		query = where(esper.EqualOf(intP(0),
			esper.AddOf[int](intB(0), esper.Literal(5))))
	case "where-int-eq-arith-mixed":
		query = where(esper.EqualOf(intP(0),
			esper.AddOf[int](esper.SubtractOf[int](
				esper.MultiplyOf[int](esper.Literal(2), intB(0)),
				esper.Divide[int](intP(0), esper.Literal(10))),
				esper.Literal(1))))
	case "where-int-eq-arith-parens":
		query = where(esper.EqualOf(intP(0),
			esper.Divide[int](
				esper.MultiplyOf[int](esper.Literal(2), esper.SubtractOf[int](intB(0), intP(0))),
				esper.AddOf[int](esper.Literal(10), esper.Literal(1)))))
	case "where-cmp-cross":
		query = where(esper.And(
			esper.GreaterOf(intP(0), intB(0)),
			esper.LessOf(dblP(1), dblB(1))))
	case "where-cmp-same":
		query = where(esper.And(
			esper.GreaterOrEqualOf(intP(0), intB(0)),
			esper.LessOrEqualOf(dblP(0), dblB(0))))
	case "where-cmp-arith":
		query = where(esper.GreaterOf(intP(0),
			esper.AddOf[float64](intB(0), dblP(1))))
	case "where-nested-or":
		query = where(esper.Or(
			esper.EqualOf(intP(0), esper.Literal(3)),
			esper.And(
				esper.EqualOf(intB(0), esper.Literal(3)),
				esper.EqualOf(intP(0), esper.Literal(1)))))
	case "where-nested-and":
		query = where(esper.And(
			esper.Or(
				esper.GreaterOf(intP(0), esper.Literal(3)),
				esper.LessOf(intB(0), esper.Literal(3))),
			esper.EqualOf(boolB(0), esper.Literal(false))))
	case "where-nested-mixed":
		query = where(esper.Or(
			esper.And(
				esper.LessOrEqualOf(intP(0), esper.Literal(3)),
				esper.GreaterOrEqualOf(intP(0), esper.Literal(1))),
			esper.And(
				esper.EqualOf(boolB(0), esper.Literal(false)),
				esper.EqualOf(boolP(0), esper.Literal(true)))))
	case "where-two-unqualified-int":
		// `intPrimitive=3` over SupportBean+SupportMarketDataBean resolves
		// to the only stream declaring the property — source 0.
		market := esper.From[eplOtherInvalidMarket](s.env, "SupportMarketDataBean").Window(esper.LengthWindow(3))
		query = esper.JoinMany(sa, esper.JoinSource(market)).Select(eplOtherInvalidJoinSelect()...).
			Where(esper.EqualOf(intP(0), esper.Literal(3))).Query()
	case "on-int-eq-intboxed":
		query = on(esper.OnSourcesEqual(0, intP(0), 1, intB(1)))
	case "on-bool-eq-boolboxed":
		query = on(esper.OnSourcesEqual(0, boolP(0), 1, boolB(1)))
	case "on-int-eq-intboxed-rev":
		query = on(esper.OnSourcesEqual(1, intP(1), 0, intB(0)))
	default:
		return esper.Query{}, fmt.Errorf("%s: no fluent query for deploy %q epl %q", eplOtherInvalidID, label, epl)
	}
	if step := eplOtherInvalidDeployEPLs[label]; step != epl {
		return esper.Query{}, fmt.Errorf("%s: deploy %q carries an unpinned EPL %q", eplOtherInvalidID, label, epl)
	}
	return query, nil
}

// eplOtherInvalidDeployEPLs pins the byte-exact tryValid EPL per deploy
// label so the fluent equivalent cannot drift from the Java text.
var eplOtherInvalidDeployEPLs = map[string]string{
	"where-int-eq-intboxed":     eplOtherInvalidStreamDef + "sa.intPrimitive = sb.intBoxed",
	"where-int-eq-int":          eplOtherInvalidStreamDef + "sa.intPrimitive = sb.intPrimitive",
	"where-int-eq-longboxed":    eplOtherInvalidStreamDef + "sa.intPrimitive = sb.longBoxed",
	"where-and-boolboxed-bool":  eplOtherInvalidStreamDef + "sa.intPrimitive = sb.intPrimitive and sb.boolBoxed = sa.boolPrimitive",
	"where-and-thestring-sb":    eplOtherInvalidStreamDef + "sa.intPrimitive = sb.intPrimitive and sb.intBoxed = sa.intPrimitive and sa.theString=sb.theString",
	"where-or-intboxed-eq-int":  eplOtherInvalidStreamDef + "sa.intPrimitive = sb.intPrimitive or sb.intBoxed = sa.intPrimitive",
	"where-const-int":           eplOtherInvalidStreamDef + "sa.intPrimitive=5",
	"where-const-string-sq":     eplOtherInvalidStreamDef + "sa.theString='4'",
	"where-const-string-dq":     eplOtherInvalidStreamDef + "sa.theString=\"4\"",
	"where-const-bool":          eplOtherInvalidStreamDef + "sa.boolPrimitive=false",
	"where-const-long":          eplOtherInvalidStreamDef + "sa.longPrimitive=-5L",
	"where-const-double":        eplOtherInvalidStreamDef + "sa.doubleBoxed=5.6d",
	"where-const-float":         eplOtherInvalidStreamDef + "sa.floatPrimitive=-5.6f",
	"where-int-eq-double":       eplOtherInvalidStreamDef + "sa.intPrimitive=5.5",
	"where-int-eq-arith-add":    eplOtherInvalidStreamDef + "sa.intPrimitive=sa.intBoxed + 5",
	"where-int-eq-arith-mixed":  eplOtherInvalidStreamDef + "sa.intPrimitive=2*sa.intBoxed - sa.intPrimitive/10 + 1",
	"where-int-eq-arith-parens": eplOtherInvalidStreamDef + "sa.intPrimitive=2*(sa.intBoxed - sa.intPrimitive)/(10 + 1)",
	"where-cmp-cross":           eplOtherInvalidStreamDef + "sa.intPrimitive > sa.intBoxed and sb.doublePrimitive < sb.doubleBoxed",
	"where-cmp-same":            eplOtherInvalidStreamDef + "sa.intPrimitive >= sa.intBoxed and sa.doublePrimitive <= sa.doubleBoxed",
	"where-cmp-arith":           eplOtherInvalidStreamDef + "sa.intPrimitive > (sa.intBoxed + sb.doublePrimitive)",
	"where-nested-or":           eplOtherInvalidStreamDef + "(sa.intPrimitive=3) or (sa.intBoxed=3 and sa.intPrimitive=1)",
	"where-nested-and":          eplOtherInvalidStreamDef + "((sa.intPrimitive>3) or (sa.intBoxed<3)) and sa.boolBoxed=false",
	"where-nested-mixed":        eplOtherInvalidStreamDef + "(sa.intPrimitive<=3 and sa.intPrimitive>=1) or (sa.boolBoxed=false and sa.boolPrimitive=true)",
	"where-two-unqualified-int": eplOtherInvalidStreamDefTwo + "intPrimitive=3",
	"on-int-eq-intboxed":        eplOtherInvalidOuterJoinDef + "on sa.intPrimitive = sb.intBoxed",
	"on-bool-eq-boolboxed":      eplOtherInvalidOuterJoinDef + "on sa.boolPrimitive = sb.boolBoxed",
	"on-int-eq-intboxed-rev":    eplOtherInvalidOuterJoinDef + "on sb.intPrimitive = sa.intBoxed",
}

// eplOtherInvalidStepEPLs pins the byte-exact tryInvalid EPL per probe label.
var eplOtherInvalidStepEPLs = map[string]string{
	"where-int-eq-string":        eplOtherInvalidStreamDef + "sa.intPrimitive = sb.theString",
	"where-and-intboxed-bool":    eplOtherInvalidStreamDef + "sa.intPrimitive = sb.intPrimitive and sb.intBoxed = sa.boolPrimitive",
	"where-and-thestring-sx":     eplOtherInvalidStreamDef + "sa.intPrimitive = sb.intPrimitive and sb.intBoxed = sa.intPrimitive and sa.theString=sX.theString",
	"where-or-thestring-sx":      eplOtherInvalidStreamDef + "sa.intPrimitive = sb.intPrimitive or sa.theString=sX.theString",
	"where-int-eq-str5":          eplOtherInvalidStreamDef + "sa.intPrimitive='5'",
	"where-thestring-eq-5":       eplOtherInvalidStreamDef + "sa.theString=5",
	"where-boolboxed-eq-f":       eplOtherInvalidStreamDef + "sa.boolBoxed=f",
	"where-int-eq-x":             eplOtherInvalidStreamDef + "sa.intPrimitive=x",
	"where-arith-unbalanced":     eplOtherInvalidStreamDef + "sa.intPrimitive=2*(sa.intBoxed",
	"where-int-ge-thestring":     eplOtherInvalidStreamDef + "sa.intPrimitive >= sa.theString",
	"where-boolboxed-ge-bool":    eplOtherInvalidStreamDef + "sa.boolBoxed >= sa.boolPrimitive",
	"where-or-unbalanced-open":   eplOtherInvalidStreamDef + "sa.intPrimitive=3 or (sa.intBoxed=2",
	"where-or-unbalanced-close":  eplOtherInvalidStreamDef + "sa.intPrimitive=3 or sa.intBoxed=2)",
	"where-or-unbalanced-nested": eplOtherInvalidStreamDef + "sa.intPrimitive=3 or ((sa.intBoxed=2)",
	"where-unqualified-int":      eplOtherInvalidStreamDef + "intPrimitive=3",
	"on-int-eq-xx":               eplOtherInvalidOuterJoinDef + "on sa.intPrimitive = sb.XX",
	"on-xx-eq-xx":                eplOtherInvalidOuterJoinDef + "on sa.XX = sb.XX",
	"on-xx-eq-intboxed":          eplOtherInvalidOuterJoinDef + "on sa.XX = sb.intBoxed",
	"on-boolboxed-eq-intboxed":   eplOtherInvalidOuterJoinDef + "on sa.boolBoxed = sb.intBoxed",
	"on-bool-eq-thestring":       eplOtherInvalidOuterJoinDef + "on sa.boolPrimitive = sb.theString",
	"on-int-le-intboxed":         eplOtherInvalidOuterJoinDef + "on sa.intPrimitive <= sb.intBoxed",
	"on-same-source-left":        eplOtherInvalidOuterJoinDef + "on sa.intPrimitive = sa.intBoxed",
	"on-same-source-right":       eplOtherInvalidOuterJoinDef + "on sb.intPrimitive = sb.intBoxed",
}

// loadEplOtherInvalidScenario decodes the scenario with the strict-shape
// checks the raw-mutation tests pin: no duplicate JSON keys, the exact
// top-level field set, pinned metadata and case metadata, and the pinned
// per-case step keys so unknown or mutated steps fail the replay.
func loadEplOtherInvalidScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", eplOtherInvalidID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", eplOtherInvalidID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplOtherInvalidID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplOtherInvalidID, err)
	}
	if err := requireEplOtherInvalidFields(root,
		"version", "id", "description", "javaCommit", "javaSource",
		"javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version     string `json:"version"`
		ID          string `json:"id"`
		Description string `json:"description"`
		JavaCommit  string `json:"javaCommit"`
		JavaSource  string `json:"javaSource"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eplOtherInvalidID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eplOtherInvalidID ||
		metadata.Description != eplOtherInvalidDescription ||
		metadata.JavaCommit != eplOtherInvalidJavaCommit ||
		metadata.JavaSource != eplOtherInvalidSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", eplOtherInvalidID)
	}
	if err := validateEplOtherInvalidStringArray(root["javaRuntimes"], eplOtherInvalidJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplOtherInvalidStringArray(root["javaNames"], eplOtherInvalidJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplOtherInvalidStringArray(root["javaStaticIds"], eplOtherInvalidJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplOtherInvalidStringArray(root["javaFlags"], eplOtherInvalidJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(eplOtherInvalidCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", eplOtherInvalidID, len(eplOtherInvalidCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEplOtherInvalidFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case          string `json:"case"`
			Ordinal       int    `json:"ordinal"`
			RuntimeID     string `json:"runtimeId"`
			ExecutionName string `json:"executionName"`
			Observation   string `json:"observation"`
			EPL           string `json:"epl"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if definition.Case != eplOtherInvalidCases[index] ||
			definition.Ordinal != eplOtherInvalidOrdinals[index] ||
			definition.RuntimeID != eplOtherInvalidJavaRuntimeIDs[index] ||
			definition.ExecutionName != eplOtherInvalidJavaExecutions[index] ||
			definition.Observation != eplOtherInvalidCaseObservations[index] ||
			definition.EPL != eplOtherInvalidCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", eplOtherInvalidID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", eplOtherInvalidID, err)
	}
	offset := 0
	for _, caseName := range eplOtherInvalidCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", eplOtherInvalidID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eplOtherInvalidID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", eplOtherInvalidID, offset, caseName)
		}
		offset++
		want, ok := eplOtherInvalidCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", eplOtherInvalidID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", eplOtherInvalidID, caseName)
		}
		for _, pinned := range want {
			key, err := eplOtherInvalidStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eplOtherInvalidID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", eplOtherInvalidID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", eplOtherInvalidID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplOtherInvalidID, err)
	}
	if len(scenario.Steps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", eplOtherInvalidID)
	}
	return scenario, nil
}

// eplOtherInvalidStepKey renders one raw step as its pinned key:
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|at
// with the payload compacted. Unknown fields on the step object are
// rejected.
func eplOtherInvalidStepKey(raw json.RawMessage) (string, error) {
	var object map[string]json.RawMessage
	if err := strictObject(raw, &object); err != nil {
		return "", err
	}
	var step struct {
		Op                 string          `json:"op"`
		Case               string          `json:"case"`
		Statement          string          `json:"statement"`
		Name               string          `json:"name"`
		EventType          string          `json:"eventType"`
		Epl                string          `json:"epl"`
		Payload            json.RawMessage `json:"payload"`
		ExpectError        string          `json:"expectError"`
		CompileWithoutPath bool            `json:"compileWithoutPath"`
		At                 string          `json:"at"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string]bool{
		"op": true, "case": true, "statement": true, "name": true,
		"eventType": true, "epl": true, "payload": true, "expectError": true,
		"compileWithoutPath": true, "at": true,
	}
	for field := range object {
		if !allowed[field] {
			return "", fmt.Errorf("step has unexpected field %q", field)
		}
	}
	payload := ""
	if len(step.Payload) != 0 {
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, step.Payload); err != nil {
			return "", fmt.Errorf("payload: %w", err)
		}
		payload = compacted.String()
	}
	cwp := ""
	if step.CompileWithoutPath {
		cwp = "1"
	}
	return step.Op + "|" + step.Statement + "|" + step.Name + "|" + step.EventType +
		"|" + step.Epl + "|" + payload + "|" + step.ExpectError + "|" + cwp +
		"|" + step.At, nil
}

// validateEplOtherInvalidScenario re-checks a decoded scenario (used when
// the runner receives a scenario decoded by the generic loader path). The
// generic compat.Step op whitelist accepts the unrepresentable op this
// scenario pins, so validation is the id and step-count invariants.
func validateEplOtherInvalidScenario(scenario compat.Scenario) error {
	if scenario.ID != eplOtherInvalidID {
		return fmt.Errorf("%s scenario id %q is not pinned", eplOtherInvalidID, scenario.ID)
	}
	if len(scenario.Steps) == 0 {
		return fmt.Errorf("%s scenario has no steps", eplOtherInvalidID)
	}
	return nil
}

func requireEplOtherInvalidFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", eplOtherInvalidID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", eplOtherInvalidID, name)
		}
	}
	return nil
}

func validateEplOtherInvalidStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}

// eplOtherInvalidStepEPLFor returns the pinned EPL for a different-joins
// probe label so build-error and unrepresentable dispatch can verify the
// step's byte-exact text.
func eplOtherInvalidStepEPLFor(step compat.Step) error {
	pinned, ok := eplOtherInvalidStepEPLs[step.Statement]
	if !ok || step.Epl != pinned {
		return fmt.Errorf("%s: step %q carries an unpinned EPL %q", eplOtherInvalidID, step.Statement, step.Epl)
	}
	return nil
}
