package parity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the EPLOtherCreateExpression slice (ords 0-4; ord 4
// runs once per infra kind).
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 0 EPLOtherInvalid
//     java-runtime-e3078c54652f7f72cd73
//   - ord 1 EPLOtherParseSpecialAndMixedExprAndScript
//     java-runtime-2e66cbcd61d52f258cb4
//   - ord 2 EPLOtherExprAndScriptLifecycleAndFilter (OBSERVEROPS)
//     java-runtime-c0c5df502bf2651e6acf
//   - ord 3 EPLOtherScriptUse (intentionally-different)
//     java-runtime-c2595ccced4ddaea854d
//   - ord 4 EPLOtherExpressionUse{namedWindow=true|false}
//     java-runtime-61a0f3f3916ac25443d7
//
// The declared-expression halves replay through DefineExpression,
// ExpressionRef, statement-local WithExpression shadowing, Filter and
// SubqueryValue over the named window/table. Java's create-expression
// statements deploy placeholder selects so the deployed markers pin the
// module fan-out; the expression bodies register env-level (or
// statement-local for the s2 rebind and the TwoPi shadow). Script surfaces
// (js: overloads, callIt, SODA round-trips) and the stateless-statement
// flags carry pinned unrepresentable records the Java oracle verifies
// in-process.
const eplOtherCreateExpressionID = "epl-other-create-expression"

const eplOtherCreateExpressionDescription = "EPLOtherCreateExpression slice (ords 0-4): invalid replays the duplicate declared-expression compile-error (script duplicate-by-arity unrepresentable); parse-mixed-expr replays the declared-expression halves (myexpr concat select, scalarfilter enum chain with the stateless flag pinned as unrepresentable, js:callIt unrepresentable); lifecycle-filter replays the declared-expression filter pass (deploy MyFilter=1, filtered select, undeploy-all, redeploy =2, rebind proof; the js:MyFilter pass unrepresentable); script-use is intentionally-different (script overloading by arity plus SODA round-trips); expression-use replays parts A-D over a keepall named window and a no-key table (TwoPi/factorPi selects, statement-local TwoPi shadow, JoinMultiplication deploy, deferred subquery myexpr over MyInfra with the stateful flag pinned as unrepresentable). Deployed markers pin the module fan-out, listener records carry the new-data rows, compile-error records carry the pinned Java message, and unrepresentable records pin the Java surfaces with no Go boundary (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherCreateExpression.java)."

const eplOtherCreateExpressionJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
const eplOtherCreateExpressionSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherCreateExpression.java"

// Byte-exact EPL pins (EPLOtherCreateExpression.java lines 47-58, 68-95,
// 106-128, 138-193, 207-214). The s0 select of parse-mixed-expr is narrowed
// to the representable c1 column; the Java original also projects
// myscript('x') as c0.
const (
	eplOtherCreateExpressionE1            = "@name('s0') @public create expression E1 {''}"
	eplOtherCreateExpressionE1Dup         = "create expression E1 {''}"
	eplOtherCreateExpressionAbcTwo        = "@public create expression int js:abc(p1, p2) [p1*p2]"
	eplOtherCreateExpressionAbcDup        = "create expression int js:abc(a, a) [p1*p2]"
	eplOtherCreateExpressionMyscript      = "@public create expression string js:myscript(p1) [\"--\"+p1+\"--\"]"
	eplOtherCreateExpressionMyexpr        = "@public create expression myexpr {sb => '--'||theString||'--'}"
	eplOtherCreateExpressionSelectMyexpr  = "@name('s0') select myexpr(sb) as c1 from SupportBean as sb"
	eplOtherCreateExpressionScalarfilter  = "@public create expression scalarfilter {s =>    strvals.where(y => y != 'E1') }"
	eplOtherCreateExpressionSelectScalar  = "@name('s0') select scalarfilter(t).where(x => x != 'E2') as val1 from SupportCollection as t"
	eplOtherCreateExpressionCallit        = "@public create expression com.espertech.esper.common.internal.support.SupportBean js:callIt() [ new com.espertech.esper.common.internal.support.SupportBean('E1', 10); ]"
	eplOtherCreateExpressionSelectCallit  = "@name('s0') select callIt() as val0, callIt().getTheString() as val1 from SupportBean as sb"
	eplOtherCreateExpressionExprOne       = "@name('expr-one') @public create expression MyFilter {sb => intPrimitive = 1}"
	eplOtherCreateExpressionExprTwo       = "@name('expr-two') @public create expression MyFilter {sb => intPrimitive = 2}"
	eplOtherCreateExpressionS1            = "@name('s1') select * from SupportBean(MyFilter(sb)) as sb"
	eplOtherCreateExpressionS2            = "@name('s2') select * from SupportBean(MyFilter(sb)) as sb"
	eplOtherCreateExpressionScriptOne     = "@name('expr-one') @public create expression boolean js:MyFilter(intPrimitive) [intPrimitive==1]"
	eplOtherCreateExpressionScriptTwo     = "@name('expr-two') @public create expression boolean js:MyFilter(intPrimitive) [intPrimitive==2]"
	eplOtherCreateExpressionScriptS1      = "@name('s1') select * from SupportBean(MyFilter(intPrimitive)) as sb"
	eplOtherCreateExpressionScriptS2      = "@name('s2') select * from SupportBean(MyFilter(intPrimitive)) as sb"
	eplOtherCreateExpressionAbcTwoTen     = "@public create expression int js:abc(p1, p2) [p1*p2*10]"
	eplOtherCreateExpressionAbcOneTen     = "@public create expression int js:abc(p1) [p1*10]"
	eplOtherCreateExpressionSelectAbc     = "@name('s0') select abc(intPrimitive, doublePrimitive) as c0, abc(intPrimitive) as c1 from SupportBean"
	eplOtherCreateExpressionSomescript    = "@name('expr') @public create expression somescript(i1) ['a']"
	eplOtherCreateExpressionSelectSomescr = "@name('select') select somescript(1) from SupportBean"
	eplOtherCreateExpressionTwoPi         = "@public create expression TwoPi {Math.PI * 2}"
	eplOtherCreateExpressionFactorPi      = "@public create expression factorPi {sb => Math.PI * intPrimitive}"
	eplOtherCreateExpressionSelectTwoPi   = "@name('s0') select TwoPi() as c0,(select TwoPi() from SupportBean_S0#lastevent) as c1,factorPi(sb) as c2 from SupportBean sb"
	eplOtherCreateExpressionSelectLocal   = "@name('s0') expression TwoPi {Math.PI * 10} select TwoPi() as c0 from SupportBean"
	eplOtherCreateExpressionJoinExpr      = "@name('expr') @public create expression JoinMultiplication {(s1,s2) => s1.intPrimitive*s2.id}"
	eplOtherCreateExpressionJoinSelect    = "@name('join') select JoinMultiplication(sb,s0) from SupportBean#lastevent as sb, SupportBean_S0#lastevent as s0"
	eplOtherCreateExpressionMyexprInfra   = "@public create expression myexpr {(select intPrimitive from MyInfra)}"
	eplOtherCreateExpressionCreateNW      = "@public create window MyInfra#keepall as SupportBean"
	eplOtherCreateExpressionCreateTbl     = "@public create table MyInfra(theString string, intPrimitive int)"
	eplOtherCreateExpressionInsertInfra   = "insert into MyInfra select theString, intPrimitive from SupportBean"
	eplOtherCreateExpressionSelectInfra   = "@name('s0') select myexpr() as c0 from SupportBean_S0"
)

// Pinned notes the unrepresentable records carry; the Java oracle verifies
// each surface in-process before emitting the same record.
const (
	eplOtherCreateExpressionNoteScriptDup = "Java deploys @public create expression int js:abc(p1, p2) [p1*p2] then rejects the same-arity redeclare with 'Script 'abc' that takes the same number of parameters has already been declared'; no Go script boundary"
	eplOtherCreateExpressionNoteStateless = "Java asserts s0 isStatelessSelect=true for the chained scalarfilter select; the Go engine exposes no stateless-statement introspection"
	eplOtherCreateExpressionNoteCallit    = "Java deploys js:callIt() returning a new SupportBean('E1',10) and selects callIt()/callIt().getTheString() emitting {val0.theString='E1',val0.intPrimitive=10,val1='E1'}; no Go script boundary"
	eplOtherCreateExpressionNoteScriptFlt = "Java repeats the lifecycle for boolean js:MyFilter(intPrimitive) scripts (deploy =1, filtered select, undeploy-all, redeploy =2, rebind proof); no Go script boundary"
	eplOtherCreateExpressionNoteScriptUse = "Java overloads js:abc by arity (p1*p2*10 vs p1*10) emitting {c0=350,c1=100} on makeBean(10,3.5), then SODA round-trips @name('expr') create expression somescript(i1) ['a'] and @name('select') select somescript(1) from SupportBean; no Go script or SODA boundary"
	eplOtherCreateExpressionNoteStatefulN = "Java asserts s0 isStatelessSelect=false for the myexpr subquery select over named window MyInfra; the Go engine exposes no stateless-statement introspection"
	eplOtherCreateExpressionNoteStatefulT = "Java asserts s0 isStatelessSelect=false for the myexpr subquery select over table MyInfra; the Go engine exposes no stateless-statement introspection"
)

var (
	eplOtherCreateExpressionJavaSources = []string{
		eplOtherCreateExpressionSource,
	}
	eplOtherCreateExpressionJavaRuntimeIDs = []string{
		"java-runtime-e3078c54652f7f72cd73",
		"java-runtime-2e66cbcd61d52f258cb4",
		"java-runtime-c0c5df502bf2651e6acf",
		"java-runtime-c2595ccced4ddaea854d",
		"java-runtime-61a0f3f3916ac25443d7",
		"java-runtime-61a0f3f3916ac25443d7",
	}
	eplOtherCreateExpressionJavaExecutions = []string{
		"EPLOtherInvalid",
		"EPLOtherParseSpecialAndMixedExprAndScript",
		"EPLOtherExprAndScriptLifecycleAndFilter",
		"EPLOtherScriptUse",
		"EPLOtherExpressionUse{namedWindow=true}",
		"EPLOtherExpressionUse{namedWindow=false}",
	}
	eplOtherCreateExpressionJavaStaticIDs = []string{
		"java-683caba87edd0bf7ecea",
		"java-dd3e693c0a24401dc305",
		"java-e292f8a1c482bb84b1e4",
		"java-ae1621ba24392298ec2f",
		"java-089123ca8c5a8e4a85b6",
		"java-089123ca8c5a8e4a85b6",
	}
	eplOtherCreateExpressionJavaFlags = []string{"OBSERVEROPS"}
	eplOtherCreateExpressionCases     = []string{
		"invalid",
		"parse-mixed-expr",
		"lifecycle-filter",
		"script-use",
		"expression-use-nw",
		"expression-use-table",
	}
	eplOtherCreateExpressionOrdinals = []int{0, 1, 2, 3, 4, 4}
)

// eplOtherCreateExpressionCaseSpec pins one case: identity, the case-level
// observation/EPL the scenario repeats, and the store variant for part D.
type eplOtherCreateExpressionCaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	observation string
	epl         string
	table       bool
}

var eplOtherCreateExpressionCaseSpecs = []eplOtherCreateExpressionCaseSpec{
	{
		name:      "invalid",
		ordinal:   0,
		runtimeID: "java-runtime-e3078c54652f7f72cd73",
		execution: "EPLOtherInvalid",
		observation: "deployed+compile-error+unrepresentable; @public create expression E1 {''}" +
			" deploys, then tryInvalidCompile rejects the redeclared E1 with" +
			" 'Expression 'E1' has already been declared'; the js:abc script deploy and its" +
			" same-arity duplicate rejection (name+parameter-count identity) have no Go" +
			" script boundary",
		epl: eplOtherCreateExpressionE1 + ";\n" +
			eplOtherCreateExpressionE1Dup + ";\n" +
			eplOtherCreateExpressionAbcTwo + ";\n" +
			eplOtherCreateExpressionAbcDup + ";\n",
	},
	{
		name:      "parse-mixed-expr",
		ordinal:   1,
		runtimeID: "java-runtime-2e66cbcd61d52f258cb4",
		execution: "EPLOtherParseSpecialAndMixedExprAndScript",
		observation: "deployed+listener+unrepresentable; myexpr {sb => '--'||theString||'--'} feeds" +
			" select myexpr(sb) as c1 emitting {c1='--E1--'} on SupportBean(E1,1) (the Java" +
			" select also projects myscript('x') as c0, unrepresentable); scalarfilter" +
			" {s => strvals.where(y => y != 'E1')} chained .where(x => x != 'E2') over" +
			" SupportCollection('E1,E2,E3,E4') emits val1=[E3,E4] and Java asserts s0" +
			" stateless; the js:callIt bean-factory select is unrepresentable",
		epl: eplOtherCreateExpressionMyscript + ";\n" +
			eplOtherCreateExpressionMyexpr + ";\n" +
			eplOtherCreateExpressionSelectMyexpr + ";\n" +
			eplOtherCreateExpressionScalarfilter + ";\n" +
			eplOtherCreateExpressionSelectScalar + ";\n" +
			eplOtherCreateExpressionCallit + ";\n" +
			eplOtherCreateExpressionSelectCallit + ";\n",
	},
	{
		name:      "lifecycle-filter",
		ordinal:   2,
		runtimeID: "java-runtime-c0c5df502bf2651e6acf",
		execution: "EPLOtherExprAndScriptLifecycleAndFilter",
		observation: "deployed+listener+unrepresentable; MyFilter {sb => intPrimitive = 1} filters" +
			" select * from SupportBean(MyFilter(sb)): E1/0 silent, E2/1 fires the full bean" +
			" row; undeploy-all then expr-two redeploys MyFilter = 2 and s2 rebinds: E3/0" +
			" and E4/1 silent, E4/2 fires; the js:MyFilter script pass is unrepresentable",
		epl: eplOtherCreateExpressionExprOne + ";\n" +
			eplOtherCreateExpressionS1 + ";\n" +
			eplOtherCreateExpressionExprTwo + ";\n" +
			eplOtherCreateExpressionS2 + ";\n" +
			eplOtherCreateExpressionScriptOne + ";\n" +
			eplOtherCreateExpressionScriptS1 + ";\n" +
			eplOtherCreateExpressionScriptTwo + ";\n" +
			eplOtherCreateExpressionScriptS2 + ";\n",
	},
	{
		name:      "script-use",
		ordinal:   3,
		runtimeID: "java-runtime-c2595ccced4ddaea854d",
		execution: "EPLOtherScriptUse",
		observation: "unrepresentable; Java overloads js:abc by arity (p1*p2*10 vs p1*10), coerces" +
			" the intPrimitive/doublePrimitive arguments, and SODA round-trips the" +
			" somescript create-expression and select statements; no Go script or SODA" +
			" boundary exists",
		epl: eplOtherCreateExpressionAbcTwoTen + ";\n" +
			eplOtherCreateExpressionAbcOneTen + ";\n" +
			eplOtherCreateExpressionSelectAbc + ";\n" +
			eplOtherCreateExpressionSomescript + ";\n" +
			eplOtherCreateExpressionSelectSomescr + ";\n",
	},
	{
		name:      "expression-use-nw",
		ordinal:   4,
		runtimeID: "java-runtime-61a0f3f3916ac25443d7",
		execution: "EPLOtherExpressionUse{namedWindow=true}",
		observation: "deployed+listener+unrepresentable; TwoPi/factorPi feed select TwoPi()," +
			" (select TwoPi() from SupportBean_S0#lastevent), factorPi(sb) emitting" +
			" [2pi,2pi,3pi] on SupportBean(E1,3) after S0(10); statement-local TwoPi" +
			" {Math.PI * 10} shadows the env registration emitting c0=10pi;" +
			" JoinMultiplication deploys over two #lastevent streams; myexpr" +
			" {(select intPrimitive from MyInfra)} binds the keepall window deferred and" +
			" emits c0=100 on S0(1,'E1') with s0 asserted stateful",
		epl: eplOtherCreateExpressionTwoPi + ";\n" +
			eplOtherCreateExpressionFactorPi + ";\n" +
			eplOtherCreateExpressionSelectTwoPi + ";\n" +
			eplOtherCreateExpressionSelectLocal + ";\n" +
			eplOtherCreateExpressionJoinExpr + ";\n" +
			eplOtherCreateExpressionJoinSelect + ";\n" +
			eplOtherCreateExpressionMyexprInfra + ";\n" +
			eplOtherCreateExpressionCreateNW + ";\n" +
			eplOtherCreateExpressionInsertInfra + ";\n" +
			eplOtherCreateExpressionSelectInfra + ";\n",
	},
	{
		name:      "expression-use-table",
		ordinal:   4,
		runtimeID: "java-runtime-61a0f3f3916ac25443d7",
		execution: "EPLOtherExpressionUse{namedWindow=false}",
		observation: "deployed+listener+unrepresentable; TwoPi/factorPi feed select TwoPi()," +
			" (select TwoPi() from SupportBean_S0#lastevent), factorPi(sb) emitting" +
			" [2pi,2pi,3pi] on SupportBean(E1,3) after S0(10); statement-local TwoPi" +
			" {Math.PI * 10} shadows the env registration emitting c0=10pi;" +
			" JoinMultiplication deploys over two #lastevent streams; myexpr" +
			" {(select intPrimitive from MyInfra)} binds the no-key table deferred and" +
			" emits c0=100 on S0(1,'E1') with s0 asserted stateful",
		epl: eplOtherCreateExpressionTwoPi + ";\n" +
			eplOtherCreateExpressionFactorPi + ";\n" +
			eplOtherCreateExpressionSelectTwoPi + ";\n" +
			eplOtherCreateExpressionSelectLocal + ";\n" +
			eplOtherCreateExpressionJoinExpr + ";\n" +
			eplOtherCreateExpressionJoinSelect + ";\n" +
			eplOtherCreateExpressionMyexprInfra + ";\n" +
			eplOtherCreateExpressionCreateTbl + ";\n" +
			eplOtherCreateExpressionInsertInfra + ";\n" +
			eplOtherCreateExpressionSelectInfra + ";\n",
		table: true,
	},
}

// eplOtherCreateExpressionBean mirrors the full pinned SupportBean property
// surface: the lifecycle selects project select *, so the trace carries
// every property — primitives default, boxed pointers stay nil, and
// charPrimitive mirrors the Java char default "\u0000".
type eplOtherCreateExpressionBean struct {
	TheString       string   `esper:"theString"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntPrimitive    int32    `esper:"intPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	CharPrimitive   string   `esper:"charPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	IntBoxed        *int     `esper:"intBoxed"`
	LongBoxed       *int64   `esper:"longBoxed"`
	CharBoxed       *string  `esper:"charBoxed"`
	ShortBoxed      *int16   `esper:"shortBoxed"`
	ByteBoxed       *int8    `esper:"byteBoxed"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BigDecimal      *float64 `esper:"bigDecimal"`
	BigInteger      *int64   `esper:"bigInteger"`
	EnumValue       *string  `esper:"enumValue"`
}

// eplOtherCreateExpressionS0 mirrors SupportBean_S0: p00-p03 are Java
// Strings that stay null unless the constructor sets them.
type eplOtherCreateExpressionS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
	P01 *string `esper:"p01"`
	P02 *string `esper:"p02"`
	P03 *string `esper:"p03"`
}

// eplOtherCreateExpressionCollection mirrors SupportCollection: strvals is
// the string list the scalarfilter enum chain reads.
type eplOtherCreateExpressionCollection struct {
	Strvals []string `esper:"strvals"`
}

func eplOtherCreateExpressionCaseSpecFor(name string) (eplOtherCreateExpressionCaseSpec, bool) {
	for _, spec := range eplOtherCreateExpressionCaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return eplOtherCreateExpressionCaseSpec{}, false
}

// runEplOtherCreateExpressionScenario replays the six cases, one fresh
// environment and engine per case (each Java execution gets its own
// runtime and ends with undeployAll).
func runEplOtherCreateExpressionScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", eplOtherCreateExpressionID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range eplOtherCreateExpressionCaseSpecs {
		caseTrace, err := runEplOtherCreateExpressionCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", eplOtherCreateExpressionID, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

// eplOtherCreateExpressionCaseState carries one case's replay state: the
// environment-level expression registrations, the live deployments keyed
// by scenario label (a label may deploy twice, e.g. s0), the listener
// sequence counters and the emitted records.
type eplOtherCreateExpressionCaseState struct {
	env          *esper.Environment
	engine       *esper.Engine
	caseName     string
	spec         eplOtherCreateExpressionCaseSpec
	deployments  map[string][]*esper.Deployment
	deployOrder  []string
	deployCounts map[string]int
	deployed     map[string]bool
	sequences    map[string]uint64
	records      []compat.TraceRecord
}

func runEplOtherCreateExpressionCase(ctx context.Context, scenario compat.Scenario, spec eplOtherCreateExpressionCaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[eplOtherCreateExpressionBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[eplOtherCreateExpressionS0](env, "SupportBean_S0"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[eplOtherCreateExpressionCollection](env, "SupportCollection"); err != nil {
		return nil, err
	}
	state := &eplOtherCreateExpressionCaseState{
		env:          env,
		caseName:     spec.name,
		spec:         spec,
		deployments:  map[string][]*esper.Deployment{},
		deployCounts: map[string]int{},
		deployed:     map[string]bool{},
		sequences:    map[string]uint64{},
	}
	state.engine = esper.NewEngine(env,
		esper.WithRuntimeURI(spec.runtimeID),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = state.engine.Close(context.Background()) }()
	defer func() { _ = state.undeployAll(context.Background()) }()

	inCase := false
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			inCase = step.Case == spec.name
			continue
		}
		if !inCase {
			continue
		}
		var err error
		switch step.Op {
		case "deploy":
			err = state.deploy(ctx, step)
		case "deployed":
			err = state.deployedMarker(step)
		case "send":
			err = state.send(ctx, step)
		case "undeploy":
			err = state.undeploy(ctx, step.Statement)
		case "undeploy-all":
			err = state.undeployAll(ctx)
		case "build-error":
			err = state.buildError(step)
		case "unrepresentable":
			err = state.unrepresentable(step)
		default:
			err = fmt.Errorf("unsupported step op %q", step.Op)
		}
		if err != nil {
			return nil, err
		}
	}
	if len(state.records) == 0 {
		return nil, fmt.Errorf("case %q produced no records", spec.name)
	}
	return state.records, nil
}

// deploy maps one scenario deploy statement onto the typed chain. Java's
// create-expression statements register the expression env-level and deploy
// a placeholder select so the deployed marker pins the module fan-out; the
// consumer statements build the real select/filter/join.
func (s *eplOtherCreateExpressionCaseState) deploy(ctx context.Context, step compat.Step) error {
	ordinal := s.deployCounts[step.Statement]
	s.deployCounts[step.Statement] = ordinal + 1
	plan, err := s.buildPlan(step.Statement, ordinal)
	if err != nil {
		return fmt.Errorf("%s: build %q: %w", eplOtherCreateExpressionID, step.Statement, err)
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: deploy %q: %w", eplOtherCreateExpressionID, step.Statement, err)
	}
	s.deployments[step.Statement] = append(s.deployments[step.Statement], deployment)
	s.deployOrder = append(s.deployOrder, step.Statement)
	if eplOtherCreateExpressionListened(s.caseName, step.Statement) {
		for _, statement := range deployment.Statements() {
			if statement.Name() != step.Statement {
				continue
			}
			name := step.Statement
			if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				s.record(name, batch)
				return nil
			}); err != nil {
				return err
			}
		}
	}
	s.deployed[step.Statement] = true
	return nil
}

func eplOtherCreateExpressionListened(caseName, statement string) bool {
	switch caseName {
	case "parse-mixed-expr":
		return statement == "s0"
	case "lifecycle-filter":
		return statement == "s1" || statement == "s2"
	case "expression-use-nw", "expression-use-table":
		return statement == "s0"
	default:
		return false
	}
}

func (s *eplOtherCreateExpressionCaseState) record(statement string, batch esper.ResultBatch) {
	s.sequences[statement]++
	rec := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "listener",
		Statement: statement,
		Sequence:  s.sequences[statement],
		Time:      compat.FormatTraceTime(batch.Time),
		New:       compat.NormalizeResults(batch.New),
		Old:       compat.NormalizeResults(batch.Old),
	}
	if len(rec.Old) == 0 {
		rec.Old = nil
	}
	s.records = append(s.records, rec)
}

func (s *eplOtherCreateExpressionCaseState) deployedMarker(step compat.Step) error {
	if !s.deployed[step.Statement] {
		return fmt.Errorf("%s: deployed marker for unknown statement %q", eplOtherCreateExpressionID, step.Statement)
	}
	s.sequences[step.Statement+":deployed"]++
	s.records = append(s.records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "deployed",
		Statement: step.Statement,
		Sequence:  s.sequences[step.Statement+":deployed"],
		Time:      compat.FormatTraceTime(s.engine.Now()),
	})
	return nil
}

func (s *eplOtherCreateExpressionCaseState) send(ctx context.Context, step compat.Step) error {
	payload, err := eplOtherCreateExpressionDecodePayload(step)
	if err != nil {
		return err
	}
	return s.engine.Send(ctx, step.EventType, payload)
}

// undeploy removes the most recent deployment registered under the label,
// mirroring undeployModuleContaining: the s0 label deploys twice in the
// parse-mixed and expression-use cases and the undeploy targets the live
// one.
func (s *eplOtherCreateExpressionCaseState) undeploy(ctx context.Context, label string) error {
	stack := s.deployments[label]
	if len(stack) == 0 {
		return fmt.Errorf("%s: unknown undeploy label %q", eplOtherCreateExpressionID, label)
	}
	deployment := stack[len(stack)-1]
	if err := deployment.Undeploy(ctx); err != nil {
		return fmt.Errorf("%s: undeploy %q: %w", eplOtherCreateExpressionID, label, err)
	}
	s.deployments[label] = stack[:len(stack)-1]
	for index := len(s.deployOrder) - 1; index >= 0; index-- {
		if s.deployOrder[index] == label {
			s.deployOrder = append(s.deployOrder[:index], s.deployOrder[index+1:]...)
			break
		}
	}
	return nil
}

// undeployAll removes deployments in reverse deploy order so dependents
// undeploy before the objects they reference, mirroring undeployAll. The
// environment-level expression registrations survive (Java's path.clear()
// drops them, but no case redeclares an env expression afterward: s2 binds
// the =2 body through the statement-local WithExpression shadow).
func (s *eplOtherCreateExpressionCaseState) undeployAll(ctx context.Context) error {
	for index := len(s.deployOrder) - 1; index >= 0; index-- {
		label := s.deployOrder[index]
		stack := s.deployments[label]
		if len(stack) == 0 {
			continue
		}
		deployment := stack[len(stack)-1]
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", eplOtherCreateExpressionID, label, err)
		}
		s.deployments[label] = stack[:len(stack)-1]
	}
	s.deployOrder = nil
	return nil
}

// buildError runs the expected-invalid compile probe: redeclaring E1
// env-level fails with the duplicate declared-expression error, and the
// record carries the pinned Java message once the Go rejection verifies
// (convention: infra_nwtable_event_type.go buildError).
func (s *eplOtherCreateExpressionCaseState) buildError(step compat.Step) error {
	record := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
	}
	if step.Statement != "duplicate-expression" {
		return fmt.Errorf("%s: unknown build-error probe %q", eplOtherCreateExpressionID, step.Statement)
	}
	buildErr := esper.DefineExpression[string](s.env, "E1", esper.Literal(""))
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", eplOtherCreateExpressionID, step.Statement)
	}
	var duplicate *esper.DuplicateModuleObjectError
	if !errors.As(buildErr, &duplicate) || duplicate.Kind != esper.DeploymentResourceExpression || duplicate.Name != "E1" {
		return fmt.Errorf("%s: build-error probe %q drift: got %v", eplOtherCreateExpressionID, step.Statement, buildErr)
	}
	if step.ExpectError != "" {
		record.Value = step.ExpectError
	}
	s.records = append(s.records, record)
	return nil
}

// unrepresentable emits the pinned record for a Java surface with no Go
// boundary. The oracle verifies the surface in-process before emitting the
// same record; the Go side only pins the note.
func (s *eplOtherCreateExpressionCaseState) unrepresentable(step compat.Step) error {
	want, ok := eplOtherCreateExpressionNotes[step.Statement]
	if step.Statement == "stateful-s0" && s.caseName == "expression-use-table" {
		want = eplOtherCreateExpressionNoteStatefulT
	}
	if !ok || want != step.ExpectError {
		return fmt.Errorf("%s: unrepresentable step %q carries an unpinned note %q", eplOtherCreateExpressionID, step.Statement, step.ExpectError)
	}
	s.records = append(s.records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "unrepresentable",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

var eplOtherCreateExpressionNotes = map[string]string{
	"script-duplicate":     eplOtherCreateExpressionNoteScriptDup,
	"stateless-s0":         eplOtherCreateExpressionNoteStateless,
	"script-callit":        eplOtherCreateExpressionNoteCallit,
	"script-filter":        eplOtherCreateExpressionNoteScriptFlt,
	"script-overload-soda": eplOtherCreateExpressionNoteScriptUse,
	"stateful-s0":          eplOtherCreateExpressionNoteStatefulN,
}

// buildPlan maps one scenario deploy statement onto the typed chain. The
// ordinal disambiguates labels that deploy more than once per case (s0 in
// parse-mixed-expr and expression-use-*).
func (s *eplOtherCreateExpressionCaseState) buildPlan(statement string, ordinal int) (esper.Plan, error) {
	env := s.env
	beanSource := esper.From[eplOtherCreateExpressionBean](env, "SupportBean")
	placeholder := func() (esper.Plan, error) {
		// Java's create-expression statement occupies the statement slot
		// without a select; the placeholder deploys a silent select * so
		// the deployed marker pins the module fan-out.
		return env.Build(esper.Select(beanSource).Query(esper.StatementName(statement)))
	}
	switch s.caseName {
	case "invalid":
		if statement == "s0" {
			if err := esper.DefineExpression[string](env, "E1", esper.Literal("")); err != nil {
				return esper.Plan{}, err
			}
			return placeholder()
		}
	case "parse-mixed-expr":
		switch statement {
		case "myexpr":
			// @public create expression myexpr {sb => '--'||theString||'--'}
			if err := esper.DefineExpression[string](env, "myexpr", esper.Concat(
				esper.Literal("--"),
				esper.NestedField[string](esper.ExpressionParam[esper.Event]("sb"), "theString"),
				esper.Literal("--"),
			)); err != nil {
				return esper.Plan{}, err
			}
			return placeholder()
		case "scalarfilter":
			// @public create expression scalarfilter {s => strvals.where(y => y != 'E1')}
			if err := esper.DefineExpression[[]string](env, "scalarfilter", esper.EnumWhere[string](
				esper.NestedField[[]string](esper.ExpressionParam[esper.Event]("s"), "strvals"),
				esper.NotEqual[string](esper.EnumElement[string](), esper.Literal("E1")),
			)); err != nil {
				return esper.Plan{}, err
			}
			return placeholder()
		case "s0":
			if ordinal == 0 {
				// @name('s0') select myexpr(sb) as c1 from SupportBean as sb
				return env.Build(esper.Select(beanSource,
					esper.Alias("c1", esper.ExpressionRef[string](env, "myexpr",
						esper.EventValue[esper.Event]())),
				).Query(esper.StatementName("s0")))
			}
			// @name('s0') select scalarfilter(t).where(x => x != 'E2') as
			// val1 from SupportCollection as t
			return env.Build(esper.Select(
				esper.From[eplOtherCreateExpressionCollection](env, "SupportCollection"),
				esper.Alias("val1", esper.EnumWhere[string](
					esper.ExpressionRef[[]string](env, "scalarfilter",
						esper.EventValue[esper.Event]()),
					esper.NotEqual[string](esper.EnumElement[string](), esper.Literal("E2")),
				)),
			).Query(esper.StatementName("s0")))
		}
	case "lifecycle-filter":
		switch statement {
		case "expr-one":
			// @name('expr-one') @public create expression MyFilter
			// {sb => intPrimitive = 1}
			if err := esper.DefineExpression[bool](env, "MyFilter", esper.Equal[int32](
				esper.NestedField[int32](esper.ExpressionParam[esper.Event]("sb"), "intPrimitive"),
				esper.Literal(int32(1)),
			)); err != nil {
				return esper.Plan{}, err
			}
			return placeholder()
		case "expr-two":
			// Java redeploys MyFilter {sb => intPrimitive = 2}; the env
			// registration stays =1 and s2 binds the new body through the
			// statement-local WithExpression shadow, matching Java's
			// rebind semantics.
			return placeholder()
		case "s1":
			// @name('s1') select * from SupportBean(MyFilter(sb)) as sb
			return env.Build(beanSource.
				Filter(esper.ExpressionRef[bool](env, "MyFilter",
					esper.EventValue[esper.Event]())).
				Query(esper.StatementName("s1")))
		case "s2":
			// @name('s2') select * from SupportBean(MyFilter(sb)) as sb —
			// the statement-local MyFilter = 2 shadows the env =1 body.
			return env.Build(beanSource.
				Filter(esper.ExpressionRef[bool](env, "MyFilter",
					esper.EventValue[esper.Event]())).
				Query(esper.StatementName("s2")).
				WithExpression("MyFilter", esper.Equal[int32](
					esper.NestedField[int32](esper.ExpressionParam[esper.Event]("sb"), "intPrimitive"),
					esper.Literal(int32(2)),
				)))
		}
	case "expression-use-nw", "expression-use-table":
		switch statement {
		case "TwoPi":
			// @public create expression TwoPi {Math.PI * 2}
			if err := esper.DefineExpression[float64](env, "TwoPi",
				esper.Multiply[float64](esper.Literal(math.Pi), esper.Literal(2.0))); err != nil {
				return esper.Plan{}, err
			}
			return placeholder()
		case "factorPi":
			// @public create expression factorPi {sb => Math.PI * intPrimitive}
			if err := esper.DefineExpression[float64](env, "factorPi", esper.Multiply[float64](
				esper.Literal(math.Pi),
				esper.Cast[int32, float64](esper.NestedField[int32](
					esper.ExpressionParam[esper.Event]("sb"), "intPrimitive")),
			)); err != nil {
				return esper.Plan{}, err
			}
			return placeholder()
		case "expr":
			// @name('expr') @public create expression JoinMultiplication
			// {(s1,s2) => s1.intPrimitive*s2.id}
			if err := esper.DefineExpression[int32](env, "JoinMultiplication", esper.Multiply[int32](
				esper.NestedField[int32](esper.ExpressionParam[esper.Event]("s1"), "intPrimitive"),
				esper.Cast[int, int32](esper.NestedField[int](
					esper.ExpressionParam[esper.Event]("s2"), "id")),
			)); err != nil {
				return esper.Plan{}, err
			}
			return placeholder()
		case "join":
			// @name('join') select JoinMultiplication(sb,s0) from
			// SupportBean#lastevent as sb, SupportBean_S0#lastevent as s0 —
			// deploy-only in Java (no listener, no sends).
			return env.Build(esper.Join(
				beanSource.Window(esper.LastEvent()),
				esper.From[eplOtherCreateExpressionS0](env, "SupportBean_S0").Window(esper.LastEvent()),
			).Select(esper.SelectFrom(0, "JoinMultiplication(sb,s0)", esper.ExpressionRef[int32](env,
				"JoinMultiplication",
				esper.JoinEventValue[esper.Event](0),
				esper.JoinEventValue[esper.Event](1),
			))).Query(esper.StatementName("join")))
		case "s0":
			switch ordinal {
			case 0:
				// @name('s0') select TwoPi() as c0,(select TwoPi() from
				// SupportBean_S0#lastevent) as c1,factorPi(sb) as c2 from
				// SupportBean sb
				return env.Build(esper.Select(beanSource,
					esper.Alias("c0", esper.ExpressionRef[float64](env, "TwoPi")),
					esper.Alias("c1", esper.SubqueryValue[float64](
						esper.FromAny(env, "SupportBean_S0").Window(esper.LastEvent()),
						esper.ExpressionRef[float64](env, "TwoPi"),
					)),
					esper.Alias("c2", esper.ExpressionRef[float64](env, "factorPi",
						esper.EventValue[esper.Event]())),
				).Query(esper.StatementName("s0")))
			case 1:
				// @name('s0') expression TwoPi {Math.PI * 10} select
				// TwoPi() as c0 from SupportBean — the statement-local
				// declaration shadows the env registration.
				return env.Build(esper.Select(beanSource,
					esper.Alias("c0", esper.ExpressionRef[float64](env, "TwoPi")),
				).Query(esper.StatementName("s0")).
					WithExpression("TwoPi",
						esper.Multiply[float64](esper.Literal(math.Pi), esper.Literal(10.0))))
			default:
				// @name('s0') select myexpr() as c0 from SupportBean_S0
				return env.Build(esper.Select(
					esper.From[eplOtherCreateExpressionS0](env, "SupportBean_S0"),
					esper.Alias("c0", esper.ExpressionRef[int32](env, "myexpr")),
				).Query(esper.StatementName("s0")))
			}
		case "myexpr":
			// @public create expression myexpr {(select intPrimitive from
			// MyInfra)} — defined before MyInfra exists; the subquery
			// source binds when the consumer statement builds.
			var infra esper.RecordStream
			if s.spec.table {
				infra = esper.FromTable(env, "MyInfra")
			} else {
				infra = esper.FromNamedWindow(env, "MyInfra")
			}
			if err := esper.DefineExpression[int32](env, "myexpr",
				esper.SubqueryValue[int32](infra, esper.Field[any, int32]("intPrimitive"))); err != nil {
				return esper.Plan{}, err
			}
			return placeholder()
		case "create":
			if s.spec.table {
				// @public create table MyInfra(theString string, intPrimitive int)
				if _, err := esper.CreateTable(env, "MyInfra", []esper.TableColumn{
					{Name: "theString", Type: reflect.TypeOf("")},
					{Name: "intPrimitive", Type: reflect.TypeOf(int32(0))},
				}); err != nil {
					return esper.Plan{}, err
				}
				return env.Build(esper.FromTable(env, "MyInfra").Query(esper.StatementName("create")))
			}
			// @public create window MyInfra#keepall as SupportBean
			schema, ok := env.Schema("SupportBean")
			if !ok {
				return esper.Plan{}, fmt.Errorf("%s: SupportBean schema is not registered", eplOtherCreateExpressionID)
			}
			if _, err := esper.CreateNamedWindow(env, "MyInfra", schema,
				esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return esper.Plan{}, err
			}
			return env.Build(esper.FromNamedWindow(env, "MyInfra").
				CreateNamedWindowQuery(esper.StatementName("create")))
		case "insert":
			// insert into MyInfra select theString, intPrimitive from SupportBean
			if s.spec.table {
				return env.Build(esper.OnRecord(beanSource.AsRecord()).InsertIntoTable("MyInfra",
					esper.SetColumn("theString", esper.Field[any, string]("theString")),
					esper.SetColumn("intPrimitive", esper.Field[any, int32]("intPrimitive")),
				).Query(esper.StatementName("insert")))
			}
			return env.Build(esper.Select(beanSource,
				esper.Alias("theString", esper.Field[eplOtherCreateExpressionBean, string]("theString")),
				esper.Alias("intPrimitive", esper.Field[eplOtherCreateExpressionBean, int32]("intPrimitive")),
			).InsertInto("MyInfra", esper.StatementName("insert")))
		}
	}
	return esper.Plan{}, fmt.Errorf("%s: case %q has no deploy fixture for %q (ordinal %d)",
		eplOtherCreateExpressionID, s.caseName, statement, ordinal)
}

// eplOtherCreateExpressionDecodePayload converts a send payload into the
// typed host event. SupportBean carries the full pinned surface with the
// Java defaults (charPrimitive "\u0000"); SupportBean_S0 keeps p00-p03
// null unless the payload sets p00; SupportCollection splits the pinned
// CSV into the strvals list.
func eplOtherCreateExpressionDecodePayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var payload struct {
			TheString       *string  `json:"theString"`
			IntPrimitive    *int32   `json:"intPrimitive"`
			DoublePrimitive *float64 `json:"doublePrimitive"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		bean := eplOtherCreateExpressionBean{CharPrimitive: "\u0000"}
		if payload.TheString != nil {
			bean.TheString = *payload.TheString
		}
		if payload.IntPrimitive != nil {
			bean.IntPrimitive = *payload.IntPrimitive
		}
		if payload.DoublePrimitive != nil {
			bean.DoublePrimitive = *payload.DoublePrimitive
		}
		return bean, nil
	case "SupportBean_S0":
		var payload struct {
			ID  *int    `json:"id"`
			P00 *string `json:"p00"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		if payload.ID == nil {
			return nil, fmt.Errorf("SupportBean_S0 payload requires id")
		}
		return eplOtherCreateExpressionS0{ID: *payload.ID, P00: payload.P00}, nil
	case "SupportCollection":
		var payload struct {
			Strvals string `json:"strvals"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportCollection: %w", err)
		}
		if payload.Strvals == "" {
			return nil, fmt.Errorf("SupportCollection payload requires strvals")
		}
		return eplOtherCreateExpressionCollection{Strvals: strings.Split(payload.Strvals, ",")}, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", eplOtherCreateExpressionID, step.EventType)
	}
}

// eplOtherCreateExpressionCaseSteps pins the complete step sequence per
// case: case marker, deploy/deployed pairs in Java compileDeploy order,
// sends, the undeploy/undeploy-all lifecycle steps, the build-error probe
// and the unrepresentable markers.
var eplOtherCreateExpressionCaseSteps = map[string][]string{
	"invalid": {
		"deploy:s0:" + eplOtherCreateExpressionE1,
		"deployed:s0",
		"build-error:duplicate-expression:" + eplOtherCreateExpressionE1Dup + ":" +
			"Expression 'E1' has already been declared",
		"unrepresentable:script-duplicate:" + eplOtherCreateExpressionNoteScriptDup,
		"undeploy-all",
	},
	"parse-mixed-expr": {
		"deploy:myexpr:" + eplOtherCreateExpressionMyexpr,
		"deployed:myexpr",
		"deploy:s0:" + eplOtherCreateExpressionSelectMyexpr,
		"deployed:s0",
		`send:SupportBean:{"intPrimitive":1,"theString":"E1"}`,
		"undeploy:s0",
		"deploy:scalarfilter:" + eplOtherCreateExpressionScalarfilter,
		"deployed:scalarfilter",
		"deploy:s0:" + eplOtherCreateExpressionSelectScalar,
		"deployed:s0",
		"unrepresentable:stateless-s0:" + eplOtherCreateExpressionNoteStateless,
		`send:SupportCollection:{"strvals":"E1,E2,E3,E4"}`,
		"undeploy-all",
		"unrepresentable:script-callit:" + eplOtherCreateExpressionNoteCallit,
		"undeploy-all",
	},
	"lifecycle-filter": {
		"deploy:expr-one:" + eplOtherCreateExpressionExprOne,
		"deployed:expr-one",
		"deploy:s1:" + eplOtherCreateExpressionS1,
		"deployed:s1",
		`send:SupportBean:{"intPrimitive":0,"theString":"E1"}`,
		`send:SupportBean:{"intPrimitive":1,"theString":"E2"}`,
		"undeploy-all",
		"deploy:expr-two:" + eplOtherCreateExpressionExprTwo,
		"deployed:expr-two",
		"deploy:s2:" + eplOtherCreateExpressionS2,
		"deployed:s2",
		`send:SupportBean:{"intPrimitive":0,"theString":"E3"}`,
		`send:SupportBean:{"intPrimitive":1,"theString":"E4"}`,
		`send:SupportBean:{"intPrimitive":2,"theString":"E4"}`,
		"undeploy-all",
		"unrepresentable:script-filter:" + eplOtherCreateExpressionNoteScriptFlt,
	},
	"script-use": {
		"unrepresentable:script-overload-soda:" + eplOtherCreateExpressionNoteScriptUse,
	},
	"expression-use-nw":    eplOtherCreateExpressionUseSteps(eplOtherCreateExpressionCreateNW, eplOtherCreateExpressionNoteStatefulN),
	"expression-use-table": eplOtherCreateExpressionUseSteps(eplOtherCreateExpressionCreateTbl, eplOtherCreateExpressionNoteStatefulT),
}

func eplOtherCreateExpressionUseSteps(createEpl, statefulNote string) []string {
	return []string{
		"deploy:TwoPi:" + eplOtherCreateExpressionTwoPi,
		"deployed:TwoPi",
		"deploy:factorPi:" + eplOtherCreateExpressionFactorPi,
		"deployed:factorPi",
		"deploy:s0:" + eplOtherCreateExpressionSelectTwoPi,
		"deployed:s0",
		`send:SupportBean_S0:{"id":10}`,
		`send:SupportBean:{"intPrimitive":3,"theString":"E1"}`,
		"undeploy:s0",
		"deploy:s0:" + eplOtherCreateExpressionSelectLocal,
		"deployed:s0",
		`send:SupportBean:{"intPrimitive":0,"theString":"E1"}`,
		"deploy:expr:" + eplOtherCreateExpressionJoinExpr,
		"deployed:expr",
		"deploy:join:" + eplOtherCreateExpressionJoinSelect,
		"deployed:join",
		"undeploy-all",
		"deploy:myexpr:" + eplOtherCreateExpressionMyexprInfra,
		"deployed:myexpr",
		"deploy:create:" + createEpl,
		"deployed:create",
		"deploy:insert:" + eplOtherCreateExpressionInsertInfra,
		"deployed:insert",
		"deploy:s0:" + eplOtherCreateExpressionSelectInfra,
		"deployed:s0",
		"unrepresentable:stateful-s0:" + statefulNote,
		`send:SupportBean:{"intPrimitive":100,"theString":"E1"}`,
		`send:SupportBean_S0:{"id":1,"p00":"E1"}`,
		"undeploy-all",
	}
}

// loadEplOtherCreateExpressionScenario decodes the scenario with the strict
// contract shared by the differential runners: no duplicate or unknown
// JSON fields, pinned metadata, pinned per-case runtime/execution/EPL, and
// a per-op step field whitelist.
func loadEplOtherCreateExpressionScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", eplOtherCreateExpressionID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", eplOtherCreateExpressionID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplOtherCreateExpressionID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplOtherCreateExpressionID, err)
	}
	if err := requireEplOtherCreateExpressionFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version      string   `json:"version"`
		ID           string   `json:"id"`
		Description  string   `json:"description"`
		JavaCommit   string   `json:"javaCommit"`
		JavaSource   string   `json:"javaSource"`
		JavaRuntimes []string `json:"javaRuntimes"`
		JavaNames    []string `json:"javaNames"`
		JavaStaticID []string `json:"javaStaticIds"`
		JavaFlags    []string `json:"javaFlags"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eplOtherCreateExpressionID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eplOtherCreateExpressionID ||
		metadata.Description != eplOtherCreateExpressionDescription ||
		metadata.JavaCommit != eplOtherCreateExpressionJavaCommit || metadata.JavaSource != eplOtherCreateExpressionSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", eplOtherCreateExpressionID)
	}
	if err := eplOtherCreateExpressionRequireEqual(metadata.JavaFlags, eplOtherCreateExpressionJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}
	if err := eplOtherCreateExpressionRequireEqual(metadata.JavaRuntimes, eplOtherCreateExpressionJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := eplOtherCreateExpressionRequireEqual(metadata.JavaNames, eplOtherCreateExpressionJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := eplOtherCreateExpressionRequireEqual(metadata.JavaStaticID, eplOtherCreateExpressionJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", eplOtherCreateExpressionID, err)
	}
	if len(rawCases) != len(eplOtherCreateExpressionCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", eplOtherCreateExpressionID, len(rawCases), len(eplOtherCreateExpressionCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEplOtherCreateExpressionFields(object,
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
		spec := eplOtherCreateExpressionCaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != spec.runtimeID || definition.ExecutionName != spec.execution {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d identity = %+v, want case %q ordinal %d runtime %s execution %s",
				eplOtherCreateExpressionID, index, definition, spec.name, spec.ordinal, spec.runtimeID, spec.execution)
		}
		if definition.Observation != spec.observation {
			return compat.Scenario{}, fmt.Errorf("scenario case %q observation does not match the pinned slice description", spec.name)
		}
		if definition.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("scenario case %q EPL does not match the Java source", spec.name)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", eplOtherCreateExpressionID, err)
	}
	if len(rawSteps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", eplOtherCreateExpressionID)
	}
	steps := make([]compat.Step, len(rawSteps))
	operations := make([]string, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		operations[index] = operation
		switch operation {
		case "case":
			if err := requireEplOtherCreateExpressionFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireEplOtherCreateExpressionFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string `json:"case"`
				Statement string `json:"statement"`
				EPL       string `json:"epl"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if !eplOtherCreateExpressionDeployPinned(step.Case, step.Statement, step.EPL) {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploy %q/%q EPL is not pinned", index, step.Case, step.Statement)
			}
		case "deployed":
			if err := requireEplOtherCreateExpressionFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireEplOtherCreateExpressionFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := eplOtherCreateExpressionDecodePayload(compat.Step{EventType: step.EventType, Payload: step.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy":
			if err := requireEplOtherCreateExpressionFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireEplOtherCreateExpressionFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "build-error":
			if err := requireEplOtherCreateExpressionFields(object, "op", "case", "statement", "epl", "expectError"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case        string `json:"case"`
				Statement   string `json:"statement"`
				EPL         string `json:"epl"`
				ExpectError string `json:"expectError"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Case != "invalid" || step.Statement != "duplicate-expression" ||
				step.EPL != eplOtherCreateExpressionE1Dup ||
				step.ExpectError != "Expression 'E1' has already been declared" {
				return compat.Scenario{}, fmt.Errorf("scenario step %d build-error is not pinned", index)
			}
		case "unrepresentable":
			if err := requireEplOtherCreateExpressionFields(object, "op", "case", "statement", "expectError"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case        string `json:"case"`
				Statement   string `json:"statement"`
				ExpectError string `json:"expectError"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			want, ok := eplOtherCreateExpressionNotes[step.Statement]
			if step.Statement == "stateful-s0" && step.Case == "expression-use-table" {
				want = eplOtherCreateExpressionNoteStatefulT
			}
			if !ok || want != step.ExpectError {
				return compat.Scenario{}, fmt.Errorf("scenario step %d unrepresentable note is not pinned", index)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplOtherCreateExpressionRawSteps(rawSteps, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// eplOtherCreateExpressionDeployPinned reports whether the deploy step's
// EPL belongs to the pinned set for the case/statement pair; the ordered
// case-steps pin fixes each position.
func eplOtherCreateExpressionDeployPinned(caseName, statement, epl string) bool {
	pinned, ok := map[string]map[string][]string{
		"invalid": {
			"s0": {eplOtherCreateExpressionE1},
		},
		"parse-mixed-expr": {
			"myexpr":       {eplOtherCreateExpressionMyexpr},
			"scalarfilter": {eplOtherCreateExpressionScalarfilter},
			"s0":           {eplOtherCreateExpressionSelectMyexpr, eplOtherCreateExpressionSelectScalar},
		},
		"lifecycle-filter": {
			"expr-one": {eplOtherCreateExpressionExprOne},
			"expr-two": {eplOtherCreateExpressionExprTwo},
			"s1":       {eplOtherCreateExpressionS1},
			"s2":       {eplOtherCreateExpressionS2},
		},
		"expression-use-nw": {
			"TwoPi":    {eplOtherCreateExpressionTwoPi},
			"factorPi": {eplOtherCreateExpressionFactorPi},
			"s0":       {eplOtherCreateExpressionSelectTwoPi, eplOtherCreateExpressionSelectLocal, eplOtherCreateExpressionSelectInfra},
			"expr":     {eplOtherCreateExpressionJoinExpr},
			"join":     {eplOtherCreateExpressionJoinSelect},
			"myexpr":   {eplOtherCreateExpressionMyexprInfra},
			"create":   {eplOtherCreateExpressionCreateNW},
			"insert":   {eplOtherCreateExpressionInsertInfra},
		},
		"expression-use-table": {
			"TwoPi":    {eplOtherCreateExpressionTwoPi},
			"factorPi": {eplOtherCreateExpressionFactorPi},
			"s0":       {eplOtherCreateExpressionSelectTwoPi, eplOtherCreateExpressionSelectLocal, eplOtherCreateExpressionSelectInfra},
			"expr":     {eplOtherCreateExpressionJoinExpr},
			"join":     {eplOtherCreateExpressionJoinSelect},
			"myexpr":   {eplOtherCreateExpressionMyexprInfra},
			"create":   {eplOtherCreateExpressionCreateTbl},
			"insert":   {eplOtherCreateExpressionInsertInfra},
		},
	}[caseName]
	if !ok {
		return false
	}
	for _, want := range pinned[statement] {
		if epl == want {
			return true
		}
	}
	return false
}

func eplOtherCreateExpressionRequireEqual(got, want []string, label string) error {
	if len(got) != len(want) {
		return fmt.Errorf("%s = %v, want %v", label, got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			return fmt.Errorf("%s = %v, want %v", label, got, want)
		}
	}
	return nil
}

func requireEplOtherCreateExpressionFields(object map[string]json.RawMessage, names ...string) error {
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	for name := range object {
		if !allowed[name] {
			return fmt.Errorf("unexpected field %q", name)
		}
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("missing field %q", name)
		}
	}
	return nil
}

// validateEplOtherCreateExpressionRawSteps pins the complete step sequence
// per case against the raw JSON objects.
func validateEplOtherCreateExpressionRawSteps(rawSteps []json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range eplOtherCreateExpressionCases {
		want, ok := eplOtherCreateExpressionCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", eplOtherCreateExpressionID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", eplOtherCreateExpressionID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", eplOtherCreateExpressionID, offset, caseName)
		}
		var marker struct {
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil || marker.Case != caseName {
			return fmt.Errorf("%s step %d must open case %q", eplOtherCreateExpressionID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := eplOtherCreateExpressionStepKey(rawSteps[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", eplOtherCreateExpressionID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned: got %q want %q", eplOtherCreateExpressionID, caseName, index+1, actual, pinned)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario steps contain an unexpected suffix", eplOtherCreateExpressionID)
	}
	return nil
}

// eplOtherCreateExpressionStepKey renders a raw step object into its pinned
// string form.
func eplOtherCreateExpressionStepKey(raw json.RawMessage, operation string) (string, error) {
	var step struct {
		Statement   string          `json:"statement"`
		EPL         string          `json:"epl"`
		ExpectError string          `json:"expectError"`
		EventType   string          `json:"eventType"`
		Payload     json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", fmt.Errorf("decode step: %w", err)
	}
	switch operation {
	case "deploy":
		return "deploy:" + step.Statement + ":" + step.EPL, nil
	case "deployed":
		return "deployed:" + step.Statement, nil
	case "send":
		var payload map[string]any
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return "", fmt.Errorf("send payload: %w", err)
		}
		canonical, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("send payload: %w", err)
		}
		return "send:" + step.EventType + ":" + string(canonical), nil
	case "undeploy":
		return "undeploy:" + step.Statement, nil
	case "undeploy-all":
		return "undeploy-all", nil
	case "build-error":
		return "build-error:" + step.Statement + ":" + step.EPL + ":" + step.ExpectError, nil
	case "unrepresentable":
		return "unrepresentable:" + step.Statement + ":" + step.ExpectError, nil
	}
	return "", fmt.Errorf("unsupported op %q", operation)
}
