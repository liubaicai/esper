package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the remaining ExprFilterOptimizableLookupableLimitedExpr
// executions (ords 1, 2, 6, 7 and 8 — the five executions not covered by
// case.expr-filter-optimizable-lookupable-limited).
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 1 ExprFilterOptLkupEqualsOneStmtWPatternSharingIndex
//     java-runtime-bfeb8b79de810cba8674
//   - ord 2 ExprFilterOptLkupEqualsMultiStmtSharingIndex
//     java-runtime-2d0d6df65e1884579f65
//   - ord 6 ExprFilterOptLkupDisqualify (STATICHOOK, unrepresentable)
//     java-runtime-530153bd9731b1f9e439
//   - ord 7 ExprFilterOptLkupCurrentTimestampWEquals
//     java-runtime-20e9b043737a1b6b2c0d
//   - ord 8 ExprFilterOptLkupCurrentTimestampCompare
//     java-runtime-0cd68bf813f2dd238a93
//
// equals-pattern-sharing maps the every s0 -> every S1 pattern onto
// PatternFrom().Every().Then(PatternFrom().Every()) with the 'ax' = p10||p11
// filter as Equal(Literal, Concat) and the s0.id asc ordering as
// OrderBy(Ascending(Property(ResultField("s0"), "id"))). equals-multi-stmt-
// sharing deploys the five pinned statements as separate Go plans behind one
// module label: s0/s1 use Filter+Concat against the literal, s2 reads the
// env-registered constant variable through VariableRef, s3 runs under the
// pattern-initiated MyContextOne reading the start tag through
// ContextPatternField, and s4 runs under the overlapping pattern-initiated
// MyContextTwo (the overlapping controller routes the initiating S1 event
// into the partition's pattern, matching Esper's delivery of the start
// event). disqualify is compile-only in Java (STATICHOOK): the oracle
// verifies each probe's BOOLEAN_EXPRESSION filter plan in-process and the Go
// side emits the pinned unrepresentable markers. current-timestamp-equals
// maps a.longPrimitive = current_timestamp()+longPrimitive onto
// Equal(TagField, Add(CurrentTimestamp, Field)) and current-timestamp-compare
// maps getSecondOfMinute()%2=0 onto Equal(ModuloOf(DateTimeGet(
// CurrentTimestamp, "second"), 2), 0) with advance-time steps driving the
// engine clock.
const efolrID = "expr-filter-opt-lkup-limited-remaining"
const efolrJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
const efolrSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/filter/ExprFilterOptimizableLookupableLimitedExpr.java"
const efolrDescription = "Remaining five executions of ExprFilterOptimizableLookupableLimitedExpr (pinned 9e1b9f1cc9117fea4bf33ab043762c045d73839c) replayed with one fresh runtime per case: equals-pattern-sharing (ord 1, every s0=SupportBean_S0 -> every SupportBean_S1('ax' = p10 || p11) order by s0.id asc — two S0 events arm two subexpressions and one S1(10,'a','x') fires both rows ordered {1},{2}), equals-multi-stmt-sharing (ord 2, five statements sharing the p00||p01='ax' lookupable — s0/s1 literal, s2 constant variable VAR, s3 context MyContextOne reading context.s1.p10, s4 context MyContextTwo pattern a=SupportBean_S1 -> SupportBean_S0(a.p10 = p00||p01) — S1(0,'ax') initiates both context partitions and S0(10,'a','x') fires all five), disqualify (ord 6, STATICHOOK: seven compile-time probes whose filter plans are BOOLEAN_EXPRESSION — non-constant variable, table column, subquery, lambda, script expression, current_timestamp and a statement-local inlined class; unrepresentable because Go has no filter-plan hook), current-timestamp-equals (ord 7, pattern a=SupportBean -> SupportBean(a.longPrimitive = current_timestamp()+longPrimitive) — at t=1000 the 1123 send arms and the 123 send fires), current-timestamp-compare (ord 8, SupportBean(current_timestamp().getSecondOfMinute()%2=0) — fires at second 0 and 2, silent at second 1 and 3). Deployed markers pin the module fan-out, listener records carry the new-data rows, and unrepresentable records pin the verified Java surfaces with no Go boundary (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/filter/ExprFilterOptimizableLookupableLimitedExpr.java)."

// Byte-exact EPL pins (ExprFilterOptimizableLookupableLimitedExpr.java lines
// 63, 85-94, 99-101, 107-115, 118-124, 129-145).
const (
	efolrEPLPatternSharing = "@name('s0') select * from pattern[every s0=SupportBean_S0 -> every SupportBean_S1('ax' = p10 || p11)] order by s0.id asc;\n"
	efolrEPLObjects        = "@public create variable string MYVARIABLE_NONCONSTANT = 'abc';\n" +
		"@public create table MyTable(tablecol string);\n" +
		"@public create window MyWindow#keepall as SupportBean;\n" +
		"@public create inlined_class \"\"\"\n" +
		"  public class Helper {\n" +
		"    public static String doit(Object param) { return null;}\n" +
		"    public static String doit(Object one, Object two) { return null;}\n" +
		"  }\n" +
		"\"\"\";\n" +
		"@public create expression MyDeclaredExpr { (select theString from MyWindow) };\n" +
		"@public create expression MyHandThrough {v => v};\n" +
		"@public create expression string js:MyJavaScript(param) [\"a\"];\n"
	efolrEPLCurrentTimestampEquals = "@name('s0') select * from pattern[a=SupportBean -> SupportBean(a.longPrimitive = current_timestamp() + longPrimitive)];\n"
	efolrEPLMultiStmt              = "@name('s0') select * from SupportBean_S0(p00 || p01 = 'ax');\n" +
		"@name('s1') select * from SupportBean_S0(p00 || p01 = 'ax');\n" +
		"create constant variable string VAR = 'ax';\n" +
		"@name('s2') select * from SupportBean_S0(p00 || p01 = VAR);\n" +
		"create context MyContextOne start SupportBean_S1 as s1;\n" +
		"@name('s3') context MyContextOne select * from SupportBean_S0(p00 || p01 = context.s1.p10);\n" +
		"create context MyContextTwo start SupportBean_S1 as s1;\n" +
		"@name('s4') context MyContextTwo select * from pattern[a=SupportBean_S1 -> SupportBean_S0(a.p10 = p00     ||     p01)];\n"
	efolrEPLCurrentTimestampCompare = "@name('s0') select * from SupportBean(current_timestamp().getSecondOfMinute()%2=0);\n"
)

// efolrHook is the INTERNAL_FILTERSPEC hook prefix the disqualify probes
// carry verbatim (the hook class name is pinned inside the EPL text).
const efolrHook = "@Hook(type=HookType.INTERNAL_FILTERSPEC, hook='com.espertech.esper.regressionlib.support.filter.SupportFilterPlanHook')"

// efolrDisqualifyProbe pins one compile-time probe: the label, the byte-exact
// EPL (hook prefix included) and the note the unrepresentable record carries.
type efolrDisqualifyProbe struct {
	label string
	epl   string
	note  string
}

var efolrDisqualifyProbes = []efolrDisqualifyProbe{
	{
		label: "variable-nonconstant",
		epl:   efolrHook + "select * from SupportBean(theString||MYVARIABLE_NONCONSTANT='ax')",
		note:  "SupportBean(theString||MYVARIABLE_NONCONSTANT='ax') compiles with filter plan BOOLEAN_EXPRESSION (non-constant variable); no Go filter-plan hook",
	},
	{
		label: "table-column",
		epl:   efolrHook + "select * from SupportBean(theString||MyTable.tablecol='ax')",
		note:  "SupportBean(theString||MyTable.tablecol='ax') compiles with filter plan BOOLEAN_EXPRESSION (table column); no Go filter-plan hook",
	},
	{
		label: "subquery",
		epl:   efolrHook + "select * from SupportBean(theString||(select theString from MyWindow)='ax')",
		note:  "SupportBean(theString||(select theString from MyWindow)='ax') compiles with filter plan BOOLEAN_EXPRESSION (subquery); no Go filter-plan hook",
	},
	{
		label: "lambda",
		epl:   efolrHook + "select * from SupportBeanArrayCollMap(id || setOfString.where(v => v=id).firstOf() = 'ax')",
		note:  "SupportBeanArrayCollMap(id || setOfString.where(v => v=id).firstOf() = 'ax') compiles with filter plan BOOLEAN_EXPRESSION (lambda); no Go filter-plan hook",
	},
	{
		label: "script",
		epl:   efolrHook + "select * from pattern[s0=SupportBean_S0 -> SupportBean(MyJavaScript(theString)='x')]",
		note:  "pattern[s0=SupportBean_S0 -> SupportBean(MyJavaScript(theString)='x')] compiles with filter plan BOOLEAN_EXPRESSION (script expression); no Go filter-plan hook",
	},
	{
		label: "current-timestamp",
		epl:   efolrHook + "select * from SupportBean(current_timestamp()=1)",
		note:  "SupportBean(current_timestamp()=1) compiles with filter plan BOOLEAN_EXPRESSION (current_timestamp); no Go filter-plan hook",
	},
	{
		label: "inlined-class",
		epl: efolrHook + "inlined_class \"\"\"\n" +
			"  public class LocalHelper {\n" +
			"    public static String doit(Object param) {\n" +
			"      return null;\n" +
			"    }\n" +
			"  }\n" +
			"\"\"\"\n" +
			"select * from SupportBean(LocalHelper.doit(theString) = 'abc')",
		note: "SupportBean(LocalHelper.doit(theString) = 'abc') compiles with filter plan BOOLEAN_EXPRESSION (statement-local inlined class); no Go filter-plan hook",
	},
}

var (
	efolrJavaSources = []string{
		efolrSource,
	}
	efolrJavaRuntimeIDs = []string{
		"java-runtime-bfeb8b79de810cba8674",
		"java-runtime-2d0d6df65e1884579f65",
		"java-runtime-530153bd9731b1f9e439",
		"java-runtime-20e9b043737a1b6b2c0d",
		"java-runtime-0cd68bf813f2dd238a93",
	}
	efolrJavaExecutions = []string{
		"ExprFilterOptLkupEqualsOneStmtWPatternSharingIndex",
		"ExprFilterOptLkupEqualsMultiStmtSharingIndex",
		"ExprFilterOptLkupDisqualify",
		"ExprFilterOptLkupCurrentTimestampWEquals",
		"ExprFilterOptLkupCurrentTimestampCompare",
	}
	efolrJavaStaticIDs = []string{
		"java-d829f38c7827ed840c38",
		"java-f492b3ef1c2cf8f82889",
		"java-2bdf30f91311c50a7b13",
		"java-ce489fee11685f77be34",
		"java-7a65d419ecb8b96a4c38",
	}
	efolrJavaFlags = []string{"STATICHOOK"}
	efolrCases     = []string{
		"equals-pattern-sharing",
		"equals-multi-stmt-sharing",
		"disqualify",
		"current-timestamp-equals",
		"current-timestamp-compare",
	}
)

// efolrCaseSpec pins one case: identity and the case-level observation/EPL
// the scenario repeats.
type efolrCaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	observation string
	epl         string
}

var efolrCaseSpecs = []efolrCaseSpec{
	{
		name:      "equals-pattern-sharing",
		ordinal:   1,
		runtimeID: "java-runtime-bfeb8b79de810cba8674",
		execution: "ExprFilterOptLkupEqualsOneStmtWPatternSharingIndex",
		observation: "deployed+listener; every s0=SupportBean_S0 -> every" +
			" SupportBean_S1('ax' = p10 || p11) order by s0.id asc: S0(1) and" +
			" S0(2) arm two subexpressions sharing one S1 index entry, then" +
			" S1(10,'a','x') completes both and the ordered delivery carries" +
			" s0={id:1} then s0={id:2}",
		epl: efolrEPLPatternSharing,
	},
	{
		name:      "equals-multi-stmt-sharing",
		ordinal:   2,
		runtimeID: "java-runtime-2d0d6df65e1884579f65",
		execution: "ExprFilterOptLkupEqualsMultiStmtSharingIndex",
		observation: "deployed+listener; one module deploys s0/s1" +
			" (p00||p01='ax'), s2 (p00||p01=VAR constant variable), s3 (context" +
			" MyContextOne, p00||p01=context.s1.p10) and s4 (context" +
			" MyContextTwo, pattern a=SupportBean_S1 ->" +
			" SupportBean_S0(a.p10 = p00||p01)); S1(0,'ax') initiates both" +
			" context partitions silently and S0(10,'a','x') fires all five" +
			" statements once",
		epl: efolrEPLMultiStmt,
	},
	{
		name:      "disqualify",
		ordinal:   6,
		runtimeID: "java-runtime-530153bd9731b1f9e439",
		execution: "ExprFilterOptLkupDisqualify",
		observation: "unrepresentable; STATICHOOK compile-only: the objects" +
			" preamble (MYVARIABLE_NONCONSTANT, MyTable, MyWindow, Helper" +
			" inlined class, MyDeclaredExpr, MyHandThrough, js:MyJavaScript)" +
			" compiles onto the path and each of the seven probes compiles" +
			" with the INTERNAL_FILTERSPEC hook asserting filter operator" +
			" BOOLEAN_EXPRESSION — non-constant variable, table column," +
			" subquery, lambda, script expression, current_timestamp and a" +
			" statement-local inlined class; no Go filter-plan hook exists",
		epl: efolrEPLObjects,
	},
	{
		name:      "current-timestamp-equals",
		ordinal:   7,
		runtimeID: "java-runtime-20e9b043737a1b6b2c0d",
		execution: "ExprFilterOptLkupCurrentTimestampWEquals",
		observation: "deployed+listener; pattern a=SupportBean ->" +
			" SupportBean(a.longPrimitive = current_timestamp()+longPrimitive):" +
			" at t=1000 the SupportBean(longPrimitive=1123) send arms the" +
			" first atom and the SupportBean(longPrimitive=123) send matches" +
			" 1000+123=1123, firing one row carrying tag a",
		epl: efolrEPLCurrentTimestampEquals,
	},
	{
		name:      "current-timestamp-compare",
		ordinal:   8,
		runtimeID: "java-runtime-0cd68bf813f2dd238a93",
		execution: "ExprFilterOptLkupCurrentTimestampCompare",
		observation: "deployed+listener;" +
			" SupportBean(current_timestamp().getSecondOfMinute()%2=0): sends" +
			" at t=0 and t=999 (second 0) fire, t=1000 and t=1999 (second 1)" +
			" stay silent, t=2000 (second 2) fires and t=3000 (second 3)" +
			" stays silent",
		epl: efolrEPLCurrentTimestampCompare,
	},
}

// efolrBean mirrors the map-typed SupportBean surface the remaining
// executions read: theString feeds the disqualify probes and longPrimitive
// feeds the current_timestamp filters. Declaration order is the sorted
// property order the Java oracle renders (longPrimitive before theString).
type efolrBean struct {
	LongPrimitive int64   `esper:"longPrimitive"`
	TheString     *string `esper:"theString"`
}

// efolrS0 mirrors the map-typed SupportBean_S0: id drives the order-by and
// p00/p01 feed the shared p00||p01 lookupable.
type efolrS0 struct {
	ID    int     `esper:"id"`
	P00   *string `esper:"p00"`
	P01   *string `esper:"p01"`
	P02   *string `esper:"p02"`
	P03   *string `esper:"p03"`
	Value int     `esper:"value"`
}

// efolrS1 mirrors the map-typed SupportBean_S1: p10/p11 feed the 'ax' concat
// filter and p10 is the context start tag's correlated field.
type efolrS1 struct {
	ID  int     `esper:"id"`
	P10 string  `esper:"p10"`
	P11 *string `esper:"p11"`
	P12 *string `esper:"p12"`
	P13 *string `esper:"p13"`
}


// runEfolrScenario replays the five cases, one fresh environment and engine
// per case (each Java execution gets its own runtime and ends with
// undeployAll).
func runEfolrScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", efolrID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range efolrCaseSpecs {
		caseTrace, err := runEfolrCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", efolrID, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

// efolrCaseState carries one case's replay state: the live deployments keyed
// by scenario label (the module label holds one deployment per contained
// statement), the listener sequence counters, the in-flight send buffer and
// the emitted records.
type efolrCaseState struct {
	env         *esper.Environment
	engine      *esper.Engine
	caseName    string
	spec        efolrCaseSpec
	deployments map[string][]*esper.Deployment
	deployOrder []string
	deployed    map[string]bool
	sequences   map[string]uint64
	pending     []compat.TraceRecord
	records     []compat.TraceRecord
}

func runEfolrCase(ctx context.Context, scenario compat.Scenario, spec efolrCaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[efolrBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[efolrS0](env, "SupportBean_S0"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[efolrS1](env, "SupportBean_S1"); err != nil {
		return nil, err
	}
	state := &efolrCaseState{
		env:         env,
		caseName:    spec.name,
		spec:        spec,
		deployments: map[string][]*esper.Deployment{},
		deployed:    map[string]bool{},
		sequences:   map[string]uint64{},
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
		case "advance-time":
			var at time.Time
			at, err = time.Parse(time.RFC3339Nano, step.At)
			if err == nil {
				err = state.engine.AdvanceTime(ctx, at)
			}
		case "deploy":
			err = state.deploy(ctx, step)
		case "deployed":
			err = state.deployedMarker(step)
		case "send":
			err = state.send(ctx, step)
		case "unrepresentable":
			err = state.unrepresentable(step)
		case "undeploy-all":
			err = state.undeployAll(ctx)
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

// deploy maps one scenario deploy step onto the typed chain. The module
// label of equals-multi-stmt-sharing registers the constant variable and the
// two contexts env-level (Java's create statements are module-level objects)
// and deploys each contained statement as its own Go plan while the marker
// stays one record per Java deploy call.
func (s *efolrCaseState) deploy(ctx context.Context, step compat.Step) error {
	plans, err := s.buildPlans(step.Statement)
	if err != nil {
		return fmt.Errorf("%s: build %q: %w", efolrID, step.Statement, err)
	}
	for _, plan := range plans {
		deployment, err := s.engine.Deploy(ctx, plan)
		if err != nil {
			return fmt.Errorf("%s: deploy %q: %w", efolrID, step.Statement, err)
		}
		s.deployments[step.Statement] = append(s.deployments[step.Statement], deployment)
		for _, statement := range deployment.Statements() {
			name := statement.Name()
			if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				s.record(name, batch)
				return nil
			}); err != nil {
				return err
			}
		}
	}
	s.deployOrder = append(s.deployOrder, step.Statement)
	s.deployed[step.Statement] = true
	return nil
}

// buildPlans maps one scenario deploy label onto the typed plan chain. The
// module label returns one plan per contained statement in module order.
func (s *efolrCaseState) buildPlans(label string) ([]esper.Plan, error) {
	env := s.env
	beanSource := esper.From[efolrBean](env, "SupportBean")
	s0Source := esper.From[efolrS0](env, "SupportBean_S0")
	s1Source := esper.From[efolrS1](env, "SupportBean_S1")

	switch s.caseName {
	case "equals-pattern-sharing":
		if label != "s0" {
			return nil, fmt.Errorf("unknown statement %q", label)
		}
		pat := esper.PatternFrom(s0Source, "s0", esper.Literal(true)).Every().
			Then(esper.PatternFrom(s1Source, "s1", esper.Equal[string](
				esper.Literal("ax"),
				esper.Concat(
					esper.Field[efolrS1, string]("p10"),
					esper.Field[efolrS1, string]("p11")))).Every())
		// Java's `order by s0.id asc` sorts the delivered match batch; the
		// Go pattern order-by surface only accepts ResultField keys naming
		// projected selections, so the runner sorts the batch by the
		// projected s0.id in record() instead (identical observable order).
		plan, err := env.Build(pat.Select(
			esper.Alias("s0", efolrTagMap(esper.PatternEvent("s0"))),
		).Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil

	case "equals-multi-stmt-sharing":
		if label != "module" {
			return nil, fmt.Errorf("unknown statement %q", label)
		}
		if err := env.RegisterVariable("VAR", "ax", esper.ConstantVariable()); err != nil {
			return nil, err
		}
		if _, err := esper.CreatePatternInitiatedContext(env, "MyContextOne",
			esper.PatternFrom(s1Source, "s1", esper.Literal(true))); err != nil {
			return nil, err
		}
		// Esper's pattern-initiated context delivers the initiating event to
		// the partition's statements; the overlapping controller is the Go
		// form that routes the start match (a=SupportBean_S1) into s4's
		// pattern. One S1 send still allocates one partition.
		if _, err := esper.CreateOverlappingPatternInitiatedContext(env, "MyContextTwo",
			esper.PatternFrom(s1Source, "s1", esper.Literal(true))); err != nil {
			return nil, err
		}
		concat := func() esper.Expression[string] {
			return esper.Concat(
				esper.Field[efolrS0, string]("p00"),
				esper.Field[efolrS0, string]("p01"))
		}
		plans := make([]esper.Plan, 0, 5)
		plain := func(name string) error {
			plan, err := env.Build(esper.Select(s0Source.Filter(
				esper.Equal[string](concat(), esper.Literal("ax")))).
				Query(esper.StatementName(name)))
			if err != nil {
				return err
			}
			plans = append(plans, plan)
			return nil
		}
		if err := plain("s0"); err != nil {
			return nil, err
		}
		if err := plain("s1"); err != nil {
			return nil, err
		}
		plan, err := env.Build(esper.Select(s0Source.Filter(
			esper.Equal[string](concat(), esper.VariableRef[string]("VAR")))).
			Query(esper.StatementName("s2")))
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
		plan, err = env.Build(esper.Select(s0Source.Filter(
			esper.Equal[string](concat(),
				esper.ContextPatternField[string]("s1", "p10")))).
			Query(esper.StatementName("s3"), esper.WithContext("MyContextOne")))
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
		pat := esper.PatternFrom(s1Source, "a", esper.Literal(true)).
			Then(esper.PatternFrom(s0Source, "s0end", esper.Equal[string](
				esper.TagField[string]("a", "p10"),
				esper.Concat(
					esper.Field[efolrS0, string]("p00"),
					esper.Field[efolrS0, string]("p01")))))
		plan, err = env.Build(pat.Select(
			esper.Alias("a", efolrTagMap(esper.PatternEvent("a"))),
		).Query(esper.StatementName("s4"), esper.WithContext("MyContextTwo")))
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
		return plans, nil

	case "current-timestamp-equals":
		if label != "s0" {
			return nil, fmt.Errorf("unknown statement %q", label)
		}
		pat := esper.PatternFrom(beanSource, "a", esper.Literal(true)).
			Then(esper.PatternFrom(beanSource, "b", esper.Equal[int64](
				esper.TagField[int64]("a", "longPrimitive"),
				esper.Add[int64](
					esper.CurrentTimestamp(),
					esper.Field[efolrBean, int64]("longPrimitive")))))
		plan, err := env.Build(pat.Select(
			esper.Alias("a", efolrTagMap(esper.PatternEvent("a"))),
		).Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil

	case "current-timestamp-compare":
		if label != "s0" {
			return nil, fmt.Errorf("unknown statement %q", label)
		}
		plan, err := env.Build(esper.Select(beanSource.Filter(
			esper.Equal[int64](
				esper.ModuloOf[int64](
					esper.DateTimeGet[int64](esper.CurrentTimestamp(), "second"),
					esper.Literal(int64(2))),
				esper.Literal(int64(0))))).
			Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{plan}, nil
	}
	return nil, fmt.Errorf("no plan named %q for case %q", label, s.caseName)
}

// record buffers one listener record for the in-flight send. Java dispatches
// the S0 event to the five statements in module order, so send flushes the
// buffer sorted by statement name.
func (s *efolrCaseState) record(statement string, batch esper.ResultBatch) {
	s.sequences[statement]++
	newRows := batch.New
	if s.caseName == "equals-pattern-sharing" && len(newRows) > 1 {
		// `order by s0.id asc`: sort the delivered match rows by the
		// projected s0 tag's id (see buildPlans for why the key cannot be
		// expressed as a pattern ResultField).
		newRows = append([]esper.Result(nil), newRows...)
		sort.SliceStable(newRows, func(i, j int) bool {
			return efolrTagID(newRows[i]) < efolrTagID(newRows[j])
		})
	}
	rec := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "listener",
		Statement: statement,
		Sequence:  s.sequences[statement],
		Time:      compat.FormatTraceTime(batch.Time),
		New:       compat.NormalizeResults(newRows),
		Old:       compat.NormalizeResults(batch.Old),
	}
	if len(rec.Old) == 0 {
		rec.Old = nil
	}
	s.pending = append(s.pending, rec)
}

// efolrTagID reads the projected s0 tag's id field for the
// equals-pattern-sharing order-by sort.
func efolrTagID(result esper.Result) int64 {
	row, ok := result.Row()
	if !ok {
		return 0
	}
	tag, ok := row.Get("s0").Any().(map[string]any)
	if !ok {
		return 0
	}
	return efolrInt64(tag["id"])
}
func (s *efolrCaseState) deployedMarker(step compat.Step) error {
	if !s.deployed[step.Statement] {
		return fmt.Errorf("%s: deployed marker for unknown statement %q", efolrID, step.Statement)
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

// unrepresentable emits the pinned marker for one disqualify probe. The Java
// oracle compiles the probe in-process and asserts the BOOLEAN_EXPRESSION
// filter plan before emitting the same record; the Go side has no
// filter-plan hook, so the pinned note is the observable contract.
func (s *efolrCaseState) unrepresentable(step compat.Step) error {
	if s.caseName != "disqualify" {
		return fmt.Errorf("%s: unrepresentable step outside the disqualify case", efolrID)
	}
	for _, probe := range efolrDisqualifyProbes {
		if probe.label != step.Statement {
			continue
		}
		if step.Epl != probe.epl || step.ExpectError != probe.note {
			return fmt.Errorf("%s: unrepresentable step %q is not pinned", efolrID, step.Statement)
		}
		s.records = append(s.records, compat.TraceRecord{
			Case:      s.caseName,
			Operation: "unrepresentable",
			Statement: step.Statement,
			Sequence:  0,
			Value:     step.ExpectError,
		})
		return nil
	}
	return fmt.Errorf("%s: unknown unrepresentable step %q", efolrID, step.Statement)
}

func (s *efolrCaseState) send(ctx context.Context, step compat.Step) error {
	payload, err := efolrDecodePayload(step)
	if err != nil {
		return err
	}
	if err := s.engine.Send(ctx, step.EventType, payload); err != nil {
		return err
	}
	// Java dispatches the shared-index event to the five statements in
	// module order; the buffered listener records emit sorted by name.
	sort.SliceStable(s.pending, func(i, j int) bool {
		return s.pending[i].Statement < s.pending[j].Statement
	})
	s.records = append(s.records, s.pending...)
	s.pending = nil
	return nil
}

// undeployAll removes deployments in reverse deploy order, draining every
// statement a module label deployed, mirroring undeployAll.
func (s *efolrCaseState) undeployAll(ctx context.Context) error {
	for index := len(s.deployOrder) - 1; index >= 0; index-- {
		label := s.deployOrder[index]
		for _, deployment := range s.deployments[label] {
			if err := deployment.Undeploy(ctx); err != nil {
				return fmt.Errorf("%s: undeploy-all %q: %w", efolrID, label, err)
			}
		}
		delete(s.deployments, label)
	}
	s.deployOrder = nil
	return nil
}

// efolrDecodePayload converts a scenario send payload into the typed host
// event for the pinned event types.
func efolrDecodePayload(step compat.Step) (any, error) {
	var payload map[string]any
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return nil, fmt.Errorf("%s: decode %s payload: %w", efolrID, step.EventType, err)
	}
	switch step.EventType {
	case "SupportBean":
		return efolrBean{
			LongPrimitive: efolrInt64(payload["longPrimitive"]),
			TheString:     efolrStringPtr(payload["theString"]),
		}, nil
	case "SupportBean_S0":
		return efolrS0{
			ID:    int(efolrInt64(payload["id"])),
			P00:   efolrStringPtr(payload["p00"]),
			P01:   efolrStringPtr(payload["p01"]),
			P02:   efolrStringPtr(payload["p02"]),
			P03:   efolrStringPtr(payload["p03"]),
			Value: int(efolrInt64(payload["value"])),
		}, nil
	case "SupportBean_S1":
		return efolrS1{
			ID:  int(efolrInt64(payload["id"])),
			P10: efolrString(payload["p10"]),
			P11: efolrStringPtr(payload["p11"]),
			P12: efolrStringPtr(payload["p12"]),
			P13: efolrStringPtr(payload["p13"]),
		}, nil
	default:
		return nil, fmt.Errorf("%s: unsupported event type %q", efolrID, step.EventType)
	}
}

func efolrInt64(value any) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case json.Number:
		parsed, _ := v.Int64()
		return parsed
	default:
		return 0
	}
}

func efolrStringPtr(value any) *string {
	if value == nil {
		return nil
	}
	if text, ok := value.(string); ok {
		return &text
	}
	return nil
}
func efolrString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

// efolrTagMap renders a pattern tag Event as a plain map (mirrors the Java
// oracle's pattern-tag rendering of map event types).
func efolrTagMap(tag esper.Expression[esper.Event]) esper.Expression[map[string]any] {
	return esper.Func1Ctx[esper.Event, map[string]any]("efolrTagMap",
		func(e esper.Event, _ esper.EvalContext) map[string]any {
			out := make(map[string]any, len(e.Schema().Fields()))
			for _, field := range e.Schema().Fields() {
				out[field.Name] = e.Get(field.Name).Any()
			}
			return out
		}, tag)
}

// loadEfolrScenario decodes the scenario JSON with strict field checking and
// pins the metadata, case identities and step sequence.
func loadEfolrScenario(reader io.Reader) (compat.Scenario, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, err
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario must be a JSON object: %w", efolrID, err)
	}
	if err := requireEfolrFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", efolrID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != efolrID ||
		metadata.Description != efolrDescription ||
		metadata.JavaCommit != efolrJavaCommit || metadata.JavaSource != efolrSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", efolrID)
	}
	if err := efolrRequireEqual(metadata.JavaFlags, efolrJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}
	if err := efolrRequireEqual(metadata.JavaRuntimes, efolrJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := efolrRequireEqual(metadata.JavaNames, efolrJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := efolrRequireEqual(metadata.JavaStaticID, efolrJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", efolrID, err)
	}
	if len(rawCases) != len(efolrCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", efolrID, len(rawCases), len(efolrCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEfolrFields(object,
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
		spec := efolrCaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != spec.runtimeID || definition.ExecutionName != spec.execution {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d identity = %+v, want case %q ordinal %d runtime %s execution %s",
				efolrID, index, definition, spec.name, spec.ordinal, spec.runtimeID, spec.execution)
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
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", efolrID, err)
	}
	if len(rawSteps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", efolrID)
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
			if err := requireEfolrFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "advance-time":
			if err := requireEfolrFields(object, "op", "case", "at"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireEfolrFields(object, "op", "case", "statement", "epl"); err != nil {
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
			if !efolrDeployPinned(step.Case, step.Statement, step.EPL) {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploy %q/%q EPL is not pinned", index, step.Case, step.Statement)
			}
		case "deployed":
			if err := requireEfolrFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireEfolrFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := efolrDecodePayload(compat.Step{EventType: step.EventType, Payload: step.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "unrepresentable":
			if err := requireEfolrFields(object, "op", "case", "statement", "epl", "expectError"); err != nil {
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
			if step.Case != "disqualify" || !efolrUnrepresentablePinned(step.Statement, step.EPL, step.ExpectError) {
				return compat.Scenario{}, fmt.Errorf("scenario step %d unrepresentable is not pinned", index)
			}
		case "undeploy-all":
			if err := requireEfolrFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
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
	if err := validateEfolrRawSteps(rawSteps, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// efolrDeployPinned reports whether the deploy step's EPL belongs to the
// pinned set for the case/statement pair; the ordered case-steps pin fixes
// each position.
func efolrDeployPinned(caseName, statement, epl string) bool {
	pinned, ok := map[string]map[string]string{
		"equals-pattern-sharing": {
			"s0": efolrEPLPatternSharing,
		},
		"equals-multi-stmt-sharing": {
			"module": efolrEPLMultiStmt,
		},
		"current-timestamp-equals": {
			"s0": efolrEPLCurrentTimestampEquals,
		},
		"current-timestamp-compare": {
			"s0": efolrEPLCurrentTimestampCompare,
		},
	}[caseName]
	if !ok {
		return false
	}
	want, ok := pinned[statement]
	return ok && epl == want
}

// efolrUnrepresentablePinned reports whether the unrepresentable step's
// label, EPL and note match one pinned disqualify probe.
func efolrUnrepresentablePinned(statement, epl, note string) bool {
	for _, probe := range efolrDisqualifyProbes {
		if probe.label == statement && probe.epl == epl && probe.note == note {
			return true
		}
	}
	return false
}

func efolrRequireEqual(got, want []string, label string) error {
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

func requireEfolrFields(object map[string]json.RawMessage, names ...string) error {
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

// validateEfolrRawSteps pins the complete step sequence per case against the
// raw JSON objects.
func validateEfolrRawSteps(rawSteps []json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range efolrCases {
		want, ok := efolrCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", efolrID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", efolrID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", efolrID, offset, caseName)
		}
		var marker struct {
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil || marker.Case != caseName {
			return fmt.Errorf("%s step %d must open case %q", efolrID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := efolrStepKey(rawSteps[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", efolrID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned: got %q want %q", efolrID, caseName, index+1, actual, pinned)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario steps contain an unexpected suffix", efolrID)
	}
	return nil
}

// efolrStepKey renders a raw step object into its pinned string form.
func efolrStepKey(raw json.RawMessage, operation string) (string, error) {
	var step struct {
		Statement   string          `json:"statement"`
		EPL         string          `json:"epl"`
		EventType   string          `json:"eventType"`
		At          string          `json:"at"`
		ExpectError string          `json:"expectError"`
		Payload     json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", fmt.Errorf("decode step: %w", err)
	}
	switch operation {
	case "advance-time":
		return "advance-time:" + step.At, nil
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
	case "unrepresentable":
		return "unrepresentable:" + step.Statement + ":" + step.EPL + ":" + step.ExpectError, nil
	case "undeploy-all":
		return "undeploy-all", nil
	}
	return "", fmt.Errorf("unsupported op %q", operation)
}

// efolrCaseSteps pins the ordered step sequence per case (the case marker
// itself is validated by position).
var efolrCaseSteps = map[string][]string{
	"equals-pattern-sharing": {
		"advance-time:1970-01-01T00:00:00Z",
		"deploy:s0:" + efolrEPLPatternSharing,
		"deployed:s0",
		`send:SupportBean_S0:{"id":1,"p00":null,"p01":null,"p02":null,"p03":null,"value":0}`,
		`send:SupportBean_S0:{"id":2,"p00":null,"p01":null,"p02":null,"p03":null,"value":0}`,
		`send:SupportBean_S1:{"id":10,"p10":"a","p11":"x","p12":null,"p13":null}`,
		"undeploy-all",
	},
	"equals-multi-stmt-sharing": {
		"advance-time:1970-01-01T00:00:00Z",
		"deploy:module:" + efolrEPLMultiStmt,
		"deployed:module",
		`send:SupportBean_S1:{"id":0,"p10":"ax","p11":null,"p12":null,"p13":null}`,
		`send:SupportBean_S0:{"id":10,"p00":"a","p01":"x","p02":null,"p03":null,"value":0}`,
		"undeploy-all",
	},
	"disqualify": {
		"unrepresentable:variable-nonconstant:" + efolrDisqualifyProbes[0].epl + ":" + efolrDisqualifyProbes[0].note,
		"unrepresentable:table-column:" + efolrDisqualifyProbes[1].epl + ":" + efolrDisqualifyProbes[1].note,
		"unrepresentable:subquery:" + efolrDisqualifyProbes[2].epl + ":" + efolrDisqualifyProbes[2].note,
		"unrepresentable:lambda:" + efolrDisqualifyProbes[3].epl + ":" + efolrDisqualifyProbes[3].note,
		"unrepresentable:script:" + efolrDisqualifyProbes[4].epl + ":" + efolrDisqualifyProbes[4].note,
		"unrepresentable:current-timestamp:" + efolrDisqualifyProbes[5].epl + ":" + efolrDisqualifyProbes[5].note,
		"unrepresentable:inlined-class:" + efolrDisqualifyProbes[6].epl + ":" + efolrDisqualifyProbes[6].note,
	},
	"current-timestamp-equals": {
		"advance-time:1970-01-01T00:00:00Z",
		"deploy:s0:" + efolrEPLCurrentTimestampEquals,
		"deployed:s0",
		"advance-time:1970-01-01T00:00:01Z",
		`send:SupportBean:{"longPrimitive":1123,"theString":null}`,
		`send:SupportBean:{"longPrimitive":123,"theString":null}`,
		"undeploy-all",
	},
	"current-timestamp-compare": {
		"advance-time:1970-01-01T00:00:00Z",
		"deploy:s0:" + efolrEPLCurrentTimestampCompare,
		"deployed:s0",
		`send:SupportBean:{"longPrimitive":0,"theString":null}`,
		"advance-time:1970-01-01T00:00:00.999Z",
		`send:SupportBean:{"longPrimitive":0,"theString":null}`,
		"advance-time:1970-01-01T00:00:01Z",
		`send:SupportBean:{"longPrimitive":0,"theString":null}`,
		"advance-time:1970-01-01T00:00:01.999Z",
		`send:SupportBean:{"longPrimitive":0,"theString":null}`,
		"advance-time:1970-01-01T00:00:02Z",
		`send:SupportBean:{"longPrimitive":0,"theString":null}`,
		"advance-time:1970-01-01T00:00:03Z",
		`send:SupportBean:{"longPrimitive":0,"theString":null}`,
		"undeploy-all",
	},
}
