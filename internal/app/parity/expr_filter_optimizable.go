package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

var efoJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/filter/ExprFilterOptimizable.java",
}

type efoBean struct {
	TheString     string   `esper:"theString"`
	IntPrimitive  int32    `esper:"intPrimitive"`
	LongPrimitive int64    `esper:"longPrimitive"`
	BigDecimal    *big.Rat `esper:"bigDecimal"`
}

type efoInKeywordBean struct {
	Ints        []int          `esper:"ints"`
	Longs       []int64        `esper:"longs"`
	MapOfIntKey map[int]string `esper:"mapOfIntKey"`
	CollOfInt   []int          `esper:"collOfInt"`
}

type efoOverrideBase struct {
	Val string `esper:"val"`
}

type efoOverrideOne struct {
	Val    string `esper:"val"`
	ValOne string `esper:"valOne"`
}

var efoJavaRuntimeIDs = []string{
	"java-runtime-5b7c33d842b03005e93c",
	"java-runtime-b74571a9e545ee3e6f10",
	"java-runtime-82215500a28aa2dd7719",
	"java-runtime-515496ef6ad24a7c21fc",
	"java-runtime-56bb65b18fd2618f280e",
	"java-runtime-41e7953d04564de1dee3",
	"java-runtime-d8bdabd44164244862f9",
	"java-runtime-9ffc389f600462a2ad5d",
	"java-runtime-67affae1accaef51cda4",
}

var efoJavaExecutions = []string{
	"ExprFilterInAndNotInKeywordMultivalue",
	"ExprFilterOptimizableMethodInvocationContext",
	"ExprFilterOptimizableTypeOf",
	"ExprFilterOptimizableVariableAndSeparateThread",
	"ExprFilterOrToInRewrite",
	"ExprFilterOrContext",
	"ExprFilterPatternUDFFilterOptimizable",
	"ExprFilterDeployTimeConstant",
	"ExprFilterRegExManyOr",
}

func runEfoScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, cn := range []string{
		"in-and-not-in-multivalue", "method-invocation-context", "typeof",
		"variable-and-separate-thread", "or-to-in-rewrite", "or-context",
		"pattern-udf", "deploy-time-constant", "regex-many-or",
	} {
		if !scenarioHasCase(scenario, cn) {
			continue
		}
		ct, err := runEfoCase(ctx, scenario, cn)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("case %q: %w", cn, err)
		}
		trace.Records = append(trace.Records, ct.Records...)
	}
	if len(trace.Records) == 0 {
		return compat.Trace{}, fmt.Errorf("no supported cases")
	}
	return trace, nil
}

// efoPlan is one deploy module: an ordered step within a case, carrying one
// or more statements to deploy when its named step is reached.
type efoPlan struct {
	name    string
	queries []esper.Query
	params  esper.ParameterValues
}

type efoCaseRuntime struct {
	env           *esper.Environment
	engine        *esper.Engine
	caseName      string
	trace         *compat.Trace
	seqByStmt     map[string]uint64
	statements    map[string]*esper.Statement
	deploymentIDs []string
	plans         []efoPlan
	pendingObs    []compat.TraceRecord
}

func runEfoCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[efoBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[efoInKeywordBean](env, "SupportInKeywordBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[efoOverrideBase](env, "SupportOverrideBase"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[efoOverrideOne](env, "SupportOverrideOne"); err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env, esper.WithRuntimeURI("parity-efo-"+caseName))
	defer func() { _ = engine.Close(context.Background()) }()

	rt := &efoCaseRuntime{
		env: env, engine: engine, caseName: caseName,
		trace:      &compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID},
		seqByStmt:  make(map[string]uint64),
		statements: make(map[string]*esper.Statement),
	}

	add := func(name string, params esper.ParameterValues, queries ...esper.Query) error {
		for _, q := range queries {
			if _, bErr := env.Build(q); bErr != nil {
				return bErr
			}
		}
		rt.plans = append(rt.plans, efoPlan{name: name, queries: queries, params: params})
		return nil
	}

	bean := func() esper.Stream[efoBean] { return esper.From[efoBean](env, "SupportBean") }
	ts := func() esper.Expression[string] { return esper.Field[efoBean, string]("theString") }
	ip := func() esper.Expression[int32] { return esper.Field[efoBean, int32]("intPrimitive") }
	lp := func() esper.Expression[int64] { return esper.Field[efoBean, int64]("longPrimitive") }

	switch caseName {
	case "in-and-not-in-multivalue":
		kwStream := func() esper.Stream[efoInKeywordBean] {
			return esper.From[efoInKeywordBean](env, "SupportInKeywordBean").Window(esper.LengthWindow(2))
		}
		// 1. plain in (ints)
		if err := add("s0", nil, esper.Select(kwStream().Filter(
			esper.InOf(esper.Literal(1), esper.Field[efoInKeywordBean, []int]("ints")),
		)).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}
		// 2. pattern every a -> b(intPrimitive in (a.ints))
		patIn := esper.PatternFrom(esper.From[efoInKeywordBean](env, "SupportInKeywordBean"), "a", esper.Literal(true)).Every().
			Then(esper.PatternFrom(bean(), "b", esper.InOf(
				esper.TagField[int32]("b", "intPrimitive"), esper.TagField[[]int]("a", "ints"))))
		if err := add("s0", nil, patIn.Select(
			esper.Alias("a", efoTagMap(esper.PatternEvent("a"))),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}
		// 3. plain not in (ints)
		if err := add("s0", nil, esper.Select(kwStream().Filter(
			esper.NotInOf(esper.Literal(1), esper.Field[efoInKeywordBean, []int]("ints")),
		)).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}
		// 4. pattern every a -> b(intPrimitive not in (a.ints))
		patNotIn := esper.PatternFrom(esper.From[efoInKeywordBean](env, "SupportInKeywordBean"), "a", esper.Literal(true)).Every().
			Then(esper.PatternFrom(bean(), "b", esper.NotInOf(
				esper.TagField[int32]("b", "intPrimitive"), esper.TagField[[]int]("a", "ints"))))
		if err := add("s0", nil, patNotIn.Select(
			esper.Alias("a", efoTagMap(esper.PatternEvent("a"))),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}
		// 5. context module: s1 keepall + s2 stream filter, both in-context.
		ctxStart := esper.PatternFrom(esper.From[efoInKeywordBean](env, "SupportInKeywordBean"), "mie", esper.Literal(true)).Every()
		if _, err := esper.CreatePatternInitiatedTerminatedContext(env, "MyContext",
			ctxStart, neverPattern(bean())); err != nil {
			return compat.Trace{}, err
		}
		ctxIn := esper.InOf(ip(), esper.ContextPatternField[[]int]("mie", "ints"))
		s1 := esper.Select(esper.From[efoBean](env, "SupportBean").
			Window(esper.KeepAll()).Filter(ctxIn)).
			Query(esper.StatementName("s1"), esper.WithContext("MyContext"))
		s2 := esper.Select(esper.From[efoBean](env, "SupportBean").Filter(ctxIn)).
			Query(esper.StatementName("s2"), esper.WithContext("MyContext"))
		if err := add("s1", nil, s2, s1); err != nil {
			return compat.Trace{}, err
		}

	case "method-invocation-context":
		udf := esper.Func1Ctx[esper.Event, string]("myCustomOkFunction",
			func(e esper.Event, ctx esper.EvalContext) string {
				meta := ctx.Metadata
				cp := meta.ContextPartitionID
				if cp == 0 {
					cp = -1 // non-context statements evaluate with partition -1
				}
				runtimeURI := meta.RuntimeURI
				if runtimeURI == "" && ctx.Engine != nil {
					runtimeURI = ctx.Engine.RuntimeURI()
				}
				rt.pendingObs = []compat.TraceRecord{
					{Name: "runtimeURI", Value: runtimeURI},
					{Name: "functionName", Value: "myCustomOkFunction"},
					{Name: "statementUserObject", Value: efoNullState(meta.StatementUserObject)},
					{Name: "contextPartitionId", Value: cp},
				}
				return "OK"
			}, esper.EventValue[esper.Event]())
		if err := add("s0", nil, esper.Select(bean().Filter(
			esper.Equal[string](udf, esper.Literal("OK"))),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}

	case "typeof":
		base := esper.From[efoOverrideBase](env, "SupportOverrideBase")
		if err := add("s0", nil, esper.Select(base.Filter(
			esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportOverrideBase"))),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}

	case "variable-and-separate-thread":
		if err := env.RegisterVariable("myCheckServiceProvider", &efoCheckService{}); err != nil {
			return compat.Trace{}, err
		}
		if err := add("s0", nil, esper.Select(bean().Filter(
			esper.Func1Ctx[esper.Event, bool]("myCheckServiceProviderCheck",
				func(e esper.Event, ctx esper.EvalContext) bool { return true },
				esper.EventValue[esper.Event]()),
		)).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}

	case "or-to-in-rewrite":
		forms := []func() esper.Expression[bool]{
			func() esper.Expression[bool] {
				return esper.Or(esper.Equal[string](ts(), esper.Literal("a")), esper.Equal[string](ts(), esper.Literal("b")))
			},
			func() esper.Expression[bool] {
				return esper.Or(esper.Equal[string](ts(), esper.Literal("a")), esper.Equal[string](esper.Literal("b"), ts()))
			},
			func() esper.Expression[bool] {
				return esper.Or(esper.Equal[string](esper.Literal("a"), ts()), esper.Equal[string](ts(), esper.Literal("b")))
			},
			func() esper.Expression[bool] {
				return esper.Or(esper.Equal[string](esper.Literal("a"), ts()), esper.Equal[string](esper.Literal("b"), ts()))
			},
		}
		for _, form := range forms {
			if err := add("s0", nil, esper.Select(bean().Filter(form())).Query(esper.StatementName("s0"))); err != nil {
				return compat.Trace{}, err
			}
		}

	case "or-context":
		if _, err := esper.CreateInitiatedContext(env, "MyContext",
			esper.Field[efoBean, string]("theString"), esper.Literal(true)); err != nil {
			return compat.Trace{}, err
		}
		if err := add("select", nil, esper.Select(bean().Filter(esper.Or(
			esper.Equal[string](ts(), esper.Literal("A")),
			esper.Equal[int32](ip(), esper.Literal(int32(1))),
		))).Query(esper.StatementName("select"), esper.WithContext("MyContext"))); err != nil {
			return compat.Trace{}, err
		}

	case "pattern-udf":
		predicate := esper.Func2[*big.Rat, *big.Rat, bool](
			"myCustomBigDecimalEquals",
			func(first, second *big.Rat) bool {
				if first == nil || second == nil {
					return false
				}
				return first.Cmp(second) == 0
			},
			esper.TagField[*big.Rat]("a", "bigDecimal"),
			esper.TagField[*big.Rat]("b", "bigDecimal"),
		)
		pattern := esper.PatternFrom(bean(), "a", esper.Literal(true)).Then(
			esper.PatternFrom(bean(), "b", predicate))
		if err := add("s0", nil, pattern.Select(
			esper.Alias("a", efoTagMap(esper.PatternEvent("a"))),
			esper.Alias("b", efoTagMap(esper.PatternEvent("b"))),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}

	case "deploy-time-constant":
		if err := registerEfoVariables(env); err != nil {
			return compat.Trace{}, err
		}
		// 1-2: equals subs both directions (p0 = 'abc')
		s1q := esper.Select(bean().Filter(esper.Equal[string](ts(), esper.Parameter[string]("p0")))).Query(esper.StatementName("s0"))
		s2q := esper.Select(bean().Filter(esper.Equal[string](esper.Parameter[string]("p0"), ts()))).Query(esper.StatementName("s0"))
		if err := add("s0", esper.ParameterValues{"p0": "abc"}, s1q); err != nil {
			return compat.Trace{}, err
		}
		if err := add("s0", esper.ParameterValues{"p0": "abc"}, s2q); err != nil {
			return compat.Trace{}, err
		}
		// 3-4: equals variable both directions
		v1q := esper.Select(bean().Filter(esper.Equal[string](ts(), esper.VariableRef[string]("var_optimizable_equals")))).Query(esper.StatementName("s0"))
		v2q := esper.Select(bean().Filter(esper.Equal[string](esper.VariableRef[string]("var_optimizable_equals"), ts()))).Query(esper.StatementName("s0"))
		if err := add("s0", nil, v1q); err != nil {
			return compat.Trace{}, err
		}
		if err := add("s0", nil, v2q); err != nil {
			return compat.Trace{}, err
		}
		// 5-6: long coercion subs both directions (longPrimitive = p0:int, p0=100)
		c1q := esper.Select(bean().Filter(esper.Equal[int64](lp(), esper.Parameter[int64]("p0")))).Query(esper.StatementName("s0"))
		c2q := esper.Select(bean().Filter(esper.Equal[int64](esper.Parameter[int64]("p0"), lp()))).Query(esper.StatementName("s0"))
		if err := add("s0", esper.ParameterValues{"p0": int64(100)}, c1q); err != nil {
			return compat.Trace{}, err
		}
		if err := add("s0", esper.ParameterValues{"p0": int64(100)}, c2q); err != nil {
			return compat.Trace{}, err
		}
		// 7-8: relop subs both directions (intPrimitive > p0, p0=10)
		r1q := esper.Select(bean().Filter(esper.Greater[int32](ip(), esper.Parameter[int32]("p0")))).Query(esper.StatementName("s0"))
		r2q := esper.Select(bean().Filter(esper.Less[int32](esper.Parameter[int32]("p0"), ip()))).Query(esper.StatementName("s0"))
		if err := add("s0", esper.ParameterValues{"p0": int32(10)}, r1q); err != nil {
			return compat.Trace{}, err
		}
		if err := add("s0", esper.ParameterValues{"p0": int32(10)}, r2q); err != nil {
			return compat.Trace{}, err
		}
		// 9-10: relop variable both directions
		rv1q := esper.Select(bean().Filter(esper.Greater[int32](ip(), esper.VariableRef[int32]("var_optimizable_relop")))).Query(esper.StatementName("s0"))
		rv2q := esper.Select(bean().Filter(esper.Less[int32](esper.VariableRef[int32]("var_optimizable_relop"), ip()))).Query(esper.StatementName("s0"))
		if err := add("s0", nil, rv1q); err != nil {
			return compat.Trace{}, err
		}
		if err := add("s0", nil, rv2q); err != nil {
			return compat.Trace{}, err
		}
		// 11-12: in subs / variable (p0=10, p1=11)
		i1q := esper.Select(bean().Filter(esper.InOf(ip(), esper.Parameter[int32]("p0"), esper.Parameter[int32]("p1")))).Query(esper.StatementName("s0"))
		if err := add("s0", esper.ParameterValues{"p0": int32(10), "p1": int32(11)}, i1q); err != nil {
			return compat.Trace{}, err
		}
		i2q := esper.Select(bean().Filter(esper.InOf(ip(),
			esper.VariableRef[int32]("var_optimizable_start"), esper.VariableRef[int32]("var_optimizable_end")))).Query(esper.StatementName("s0"))
		if err := add("s0", nil, i2q); err != nil {
			return compat.Trace{}, err
		}
		// 13-14: in array subs / variable (p0=[10,11] primitive array)
		ia1q := esper.Select(bean().Filter(esper.InOf(ip(), esper.Parameter[[]int32]("p0")))).Query(esper.StatementName("s0"))
		if err := add("s0", esper.ParameterValues{"p0": []int32{10, 11}}, ia1q); err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterVariable("var_optimizable_array", []int32{10, 11}); err != nil {
			return compat.Trace{}, err
		}
		ia2q := esper.Select(bean().Filter(esper.InOf(ip(), esper.VariableRef[[]int32]("var_optimizable_array")))).Query(esper.StatementName("s0"))
		if err := add("s0", nil, ia2q); err != nil {
			return compat.Trace{}, err
		}
		// 15-16: between numeric subs / variable
		b1q := esper.Select(bean().Filter(esper.BetweenOf(ip(), esper.Parameter[int32]("p0"), esper.Parameter[int32]("p1")))).Query(esper.StatementName("s0"))
		if err := add("s0", esper.ParameterValues{"p0": int32(10), "p1": int32(11)}, b1q); err != nil {
			return compat.Trace{}, err
		}
		b2q := esper.Select(bean().Filter(esper.BetweenOf(ip(),
			esper.VariableRef[int32]("var_optimizable_start"), esper.VariableRef[int32]("var_optimizable_end")))).Query(esper.StatementName("s0"))
		if err := add("s0", nil, b2q); err != nil {
			return compat.Trace{}, err
		}
		// 17-18: between string subs / variable
		bs1q := esper.Select(bean().Filter(esper.BetweenOf(ts(), esper.Parameter[string]("p0"), esper.Parameter[string]("p1")))).Query(esper.StatementName("s0"))
		if err := add("s0", esper.ParameterValues{"p0": "c", "p1": "d"}, bs1q); err != nil {
			return compat.Trace{}, err
		}
		bs2q := esper.Select(bean().Filter(esper.BetweenOf(ts(),
			esper.VariableRef[string]("var_optimizable_start_string"), esper.VariableRef[string]("var_optimizable_end_string")))).Query(esper.StatementName("s0"))
		if err := add("s0", nil, bs2q); err != nil {
			return compat.Trace{}, err
		}

	case "regex-many-or":
		re := func() esper.Expression[string] { return esper.RegexpMatch(ts(), esper.Literal(".*test.*")) }
		orExpr := re()
		for i := 1; i < 17; i++ {
			orExpr = esper.Or(orExpr, re())
		}
		if err := add("s0", nil, esper.Select(bean().Filter(orExpr)).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}

	default:
		return compat.Trace{}, fmt.Errorf("unsupported expr-filter-optimizable case %q", caseName)
	}

	deploy := func(name string) error {
		for i := range rt.plans {
			if rt.plans[i].name != name {
				continue
			}
			item := rt.plans[i]
			rt.plans = append(rt.plans[:i], rt.plans[i+1:]...)
			for _, q := range item.queries {
				plan, err := env.Build(q)
				if err != nil {
					return err
				}
				var deployment *esper.Deployment
				if item.params != nil {
					deployment, err = engine.DeployWithParameters(ctx, plan, item.params)
				} else {
					deployment, err = engine.Deploy(ctx, plan)
				}
				if err != nil {
					return err
				}
				rt.deploymentIDs = append(rt.deploymentIDs, deployment.ID())
				for _, st := range deployment.Statements() {
					stmt := st
					rt.statements[stmt.Name()] = stmt
					if stmt.Name() == "case-setup" {
						continue
					}
					if _, subErr := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
						hasNew := len(batch.New) > 0
						hasOld := len(batch.Old) > 0
						if !hasNew && !hasOld {
							return nil
						}
						record := rt.makeRecord(stmt.Name(), batch)
						for _, obs := range rt.pendingObs {
							obs.Case = rt.caseName
							obs.Operation = "observation"
							obs.Statement = stmt.Name()
							obs.Sequence = record.Sequence
							obs.Time = record.Time
							rt.trace.Records = append(rt.trace.Records, obs)
						}
						rt.pendingObs = nil
						rt.trace.Records = append(rt.trace.Records, record)
						return nil
					}); subErr != nil {
						return subErr
					}
				}
			}
			return nil
		}
		return fmt.Errorf("no plan named %q", name)
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			switch step.EventType {
			case "SupportBean":
				var payload map[string]any
				if err := json.Unmarshal(step.Payload, &payload); err != nil {
					return *rt.trace, err
				}
				v := efoBean{
					TheString:     jsonString(payload["theString"]),
					IntPrimitive:  jsonInt32(payload["intPrimitive"]),
					LongPrimitive: jsonInt64(payload["longPrimitive"]),
				}
				if bd, ok := payload["bigDecimal"]; ok && bd != nil {
					v.BigDecimal = new(big.Rat).SetInt64(int64(jsonFloat(bd)))
				}
				if err := engine.SendEvent(ctx, v); err != nil {
					return *rt.trace, err
				}
			case "SupportInKeywordBean":
				var v efoInKeywordBean
				if err := json.Unmarshal(step.Payload, &v); err != nil {
					return *rt.trace, err
				}
				if err := engine.SendEvent(ctx, v); err != nil {
					return *rt.trace, err
				}
			case "SupportOverrideBase":
				var v efoOverrideBase
				if err := json.Unmarshal(step.Payload, &v); err != nil {
					return *rt.trace, err
				}
				if err := engine.SendEvent(ctx, v); err != nil {
					return *rt.trace, err
				}
			case "SupportOverrideOne":
				var v efoOverrideOne
				if err := json.Unmarshal(step.Payload, &v); err != nil {
					return *rt.trace, err
				}
				if err := engine.SendEvent(ctx, v); err != nil {
					return *rt.trace, err
				}
			default:
				return *rt.trace, fmt.Errorf("unsupported event type %q", step.EventType)
			}
		case "deploy":
			if err := rt.undeployAll(ctx); err != nil {
				return *rt.trace, err
			}
			if err := deploy(step.Statement); err != nil {
				return *rt.trace, err
			}
		default:
			return *rt.trace, fmt.Errorf("unsupported expr-filter-optimizable op %q (case %q)", step.Op, caseName)
		}
	}
	return *rt.trace, nil
}

func (rt *efoCaseRuntime) makeRecord(stmt string, batch esper.ResultBatch) compat.TraceRecord {
	rt.seqByStmt[stmt]++
	record := compat.TraceRecord{
		Case:      rt.caseName,
		Operation: "listener",
		Statement: stmt,
		Time:      batch.Time.UTC().Format(time.RFC3339),
		Sequence:  rt.seqByStmt[stmt],
	}
	record.New = compat.NormalizeResults(batch.New)
	record.Old = compat.NormalizeResults(batch.Old)
	if record.New == nil {
		record.New = []compat.ResultRecord{}
	}
	if record.Old == nil {
		record.Old = []compat.ResultRecord{}
	}
	return record
}

// efoNullState mirrors the Java oracle's scalar normalization for a null
// object value: {state:null}.
func efoNullState(v any) any {
	if v == nil {
		return map[string]any{"state": "null"}
	}
	return v
}

func registerEfoVariables(env *esper.Environment) error {
	if err := env.RegisterVariable("var_optimizable_equals", "abc"); err != nil {
		return err
	}
	if err := env.RegisterVariable("var_optimizable_relop", int32(10)); err != nil {
		return err
	}
	if err := env.RegisterVariable("var_optimizable_start", int32(10)); err != nil {
		return err
	}
	if err := env.RegisterVariable("var_optimizable_end", int32(11)); err != nil {
		return err
	}
	if err := env.RegisterVariable("var_optimizable_start_string", "c"); err != nil {
		return err
	}
	if err := env.RegisterVariable("var_optimizable_end_string", "d"); err != nil {
		return err
	}
	return nil
}

// neverPattern produces a pattern that never matches: it waits for a pattern
// b=SupportBean(false) — used as the (unused within the scenario) end of an
// initiated-terminated context, mirroring Java's 'terminated after 24 hours'
// which never fires during the test.
func neverPattern(stream esper.Stream[efoBean]) esper.PatternStream {
	return esper.PatternFrom(stream, "end", esper.Literal(false))
}

// efoCheckService mirrors Java's MyCheckServiceProvider whose check() returns
// true; the variable value is registered before the filter is deployed.
type efoCheckService struct{}

func jsonString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func jsonInt32(v any) int32 {
	if v == nil {
		return 0
	}
	if n, ok := v.(float64); ok {
		return int32(n)
	}
	return 0
}

func jsonInt64(v any) int64 {
	if v == nil {
		return 0
	}
	if n, ok := v.(float64); ok {
		return int64(n)
	}
	return 0
}

func jsonFloat(v any) float64 {
	if n, ok := v.(float64); ok {
		return n
	}
	return 0
}

func (rt *efoCaseRuntime) undeployAll(ctx context.Context) error {
	ids := rt.deploymentIDs
	rt.deploymentIDs = nil
	for _, id := range ids {
		if err := rt.engine.Undeploy(ctx, id); err != nil {
			return err
		}
	}
	rt.statements = make(map[string]*esper.Statement)
	rt.seqByStmt = make(map[string]uint64)
	return nil
}

// efoTagMap renders a pattern tag Event as the same plain-map shape the Java
// oracle emits for pattern tags (raw property map, absent values as null).
func efoTagMap(tag esper.Expression[esper.Event]) esper.Expression[map[string]any] {
	return esper.Func1Ctx[esper.Event, map[string]any]("efoTagMap",
		func(e esper.Event, _ esper.EvalContext) map[string]any {
			out := make(map[string]any, len(e.Schema().Fields()))
			for _, field := range e.Schema().Fields() {
				out[field.Name] = efoTagValue(e.Get(field.Name))
			}
			return out
		}, tag)
}

// efoTagValue maps engine value renderings to the Java oracle's scalar
// normalization: null/missing become nil (rendered {state:null}), and
// big.Rat (Go's BigDecimal counterpart) becomes a float64.
func efoTagValue(v esper.Value) any {
	if v.State() == esper.ValueNull || v.State() == esper.ValueMissing {
		return nil
	}
	if r, ok := v.Any().(*big.Rat); ok {
		if r == nil {
			return nil
		}
		f, _ := r.Float64()
		return f
	}
	return v.Any()
}
