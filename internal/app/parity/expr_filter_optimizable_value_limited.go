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

var efovJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/filter/ExprFilterOptimizableValueLimitedExpr.java",
}

type efovBean struct {
	TheString       string   `esper:"theString"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntPrimitive    int32    `esper:"intPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	CharPrimitive   int32    `esper:"charPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float64  `esper:"floatPrimitive"`
	IntBoxed        *int32   `esper:"intBoxed"`
	LongBoxed       *int64   `esper:"longBoxed"`
	ShortBoxed      *int16   `esper:"shortBoxed"`
	ByteBoxed       *int8    `esper:"byteBoxed"`
	CharBoxed       *int32   `esper:"charBoxed"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	EnumValue       *string  `esper:"enumValue"`
	BigDecimal      *big.Rat `esper:"bigDecimal"`
	BigInteger      *big.Int `esper:"bigInteger"`
}

type efovS0 struct {
	ID    int    `esper:"id"`
	P00   string `esper:"p00"`
	P01   string `esper:"p01"`
	P02   string `esper:"p02"`
	P03   string `esper:"p03"`
	Value int    `esper:"value"`
}

type efovS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
	P11 string `esper:"p11"`
	P12 string `esper:"p12"`
	P13 string `esper:"p13"`
}

type efovS2 struct {
	ID  int    `esper:"id"`
	P20 string `esper:"p20"`
	P21 string `esper:"p21"`
	P22 string `esper:"p22"`
	P23 string `esper:"p23"`
}

var efovJavaRuntimeIDs = []string{
	"java-runtime-a2154e07f7abfac66d64",
	"java-runtime-2db81262da4d3b1c6251",
	"java-runtime-ea00ef620a7902f2bab4",
	"java-runtime-15c00c4c0b398fd5e023",
	"java-runtime-869f600f41f8c10e72e0",
	"java-runtime-ee6996bf56d450131a4a",
	"java-runtime-88ebfce1e79071f90f69",
	"java-runtime-9e6dafb20b6ecb46df9a",
	"java-runtime-6b9f4a2bdb855e3f172b",
}

var efovJavaExecutions = []string{
	"ExprFilterOptValEqualsFromPatternSingle",
	"ExprFilterOptValEqualsFromPatternMulti",
	"ExprFilterOptValEqualsFromPatternConstant",
	"ExprFilterOptValEqualsFromPatternHalfConstant",
	"ExprFilterOptValEqualsFromPatternWithDotMethod",
	"ExprFilterOptValEqualsContextWithStart",
	"ExprFilterOptValInSetOfValueWPatternWCoercion",
	"ExprFilterOptValInRangeWCoercion",
	"ExprFilterOptValOrRewrite",
}

func runEfovScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, cn := range []string{
		"from-pattern-single", "from-pattern-multi", "from-pattern-constant",
		"from-pattern-half-constant", "from-pattern-with-dot-method",
		"context-with-start", "in-set-of-value-wcoercion",
		"in-range-wcoercion", "or-rewrite",
	} {
		if !scenarioHasCase(scenario, cn) {
			continue
		}
		ct, err := runEfovCase(ctx, scenario, cn)
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

type efovPlan struct {
	name    string
	queries []esper.Query
}

type efovRuntime struct {
	env           *esper.Environment
	engine        *esper.Engine
	caseName      string
	trace         *compat.Trace
	seqByStmt     map[string]uint64
	statements    map[string]*esper.Statement
	deploymentIDs []string
	plans         []efovPlan
}

func runEfovCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	for _, reg := range []func() error{
		func() error { _, e := esper.RegisterStruct[efovBean](env, "SupportBean"); return e },
		func() error { _, e := esper.RegisterStruct[efovS0](env, "SupportBean_S0"); return e },
		func() error { _, e := esper.RegisterStruct[efovS1](env, "SupportBean_S1"); return e },
		func() error { _, e := esper.RegisterStruct[efovS2](env, "SupportBean_S2"); return e },
	} {
		if err := reg(); err != nil {
			return compat.Trace{}, err
		}
	}

	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	rt := &efovRuntime{
		env: env, engine: engine, caseName: caseName,
		trace:      &compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID},
		seqByStmt:  make(map[string]uint64),
		statements: make(map[string]*esper.Statement),
	}

	add := func(name string, queries ...esper.Query) error {
		for _, q := range queries {
			if _, bErr := env.Build(q); bErr != nil {
				return bErr
			}
		}
		rt.plans = append(rt.plans, efovPlan{name: name, queries: queries})
		return nil
	}

	bean := func() esper.Stream[efovBean] { return esper.From[efovBean](env, "SupportBean") }
	s0 := func() esper.Stream[efovS0] { return esper.From[efovS0](env, "SupportBean_S0") }
	s1 := func() esper.Stream[efovS1] { return esper.From[efovS1](env, "SupportBean_S1") }
	s2 := func() esper.Stream[efovS2] { return esper.From[efovS2](env, "SupportBean_S2") }
	ts := func() esper.Expression[string] { return esper.Field[efovBean, string]("theString") }
	lp := func() esper.Expression[int64] { return esper.Field[efovBean, int64]("longPrimitive") }

	switch caseName {
	case "from-pattern-single":
		pat := esper.PatternFrom(s0(), "a", esper.Literal(true)).Every().
			Then(esper.PatternFrom(bean(), "b", esper.Equal[string](
				esper.Concat(esper.TagField[string]("a", "p00"), esper.TagField[string]("a", "p01")),
				ts())))
		if err := add("s0", pat.Select(
			esper.Alias("a", efovTagMap(esper.PatternEvent("a"))),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}

	case "from-pattern-multi":
		pat := esper.PatternFrom(s0(), "a", esper.Literal(true)).MatchUntil(2, 2).Every().
			Then(esper.PatternFrom(s1(), "b", esper.Literal(true))).
			Then(esper.PatternFrom(bean(), "c", esper.Equal[string](
				ts(),
				esper.Concat(esper.TagFieldAt[string]("a", 0, "p00"), esper.TagField[string]("b", "p10")))))
		if err := add("s0", pat.Select(
			esper.Alias("a", efovTagArray(esper.TagEvents("a"))),
			esper.Alias("b", efovTagMap(esper.PatternEvent("b"))),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}

	case "from-pattern-constant":
		pat := esper.PatternFrom(s0(), "s0", esper.Literal(true)).Every().
			Then(esper.PatternFrom(s1(), "s1", esper.Literal(true))).
			Then(esper.PatternFrom(bean(), "b", esper.Equal[string](esper.Literal("ax"), ts())))
		if err := add("s0", pat.Select(
			esper.Alias("_phantom", esper.Literal("")),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}

	case "from-pattern-half-constant":
		pat := esper.PatternFrom(s0(), "s0", esper.Literal(true)).Every().
			Then(esper.PatternFrom(s1(), "s1", esper.Literal(true))).
			Then(esper.PatternFrom(bean(), "b", esper.Equal[string](
				esper.Concat(esper.Literal("a"), esper.TagField[string]("s1", "p10")),
				ts())))
		if err := add("s0", pat.Select(
			esper.Alias("s0", efovTagMap(esper.PatternEvent("s0"))),
			esper.Alias("s1", efovTagMap(esper.PatternEvent("s1"))),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}

	case "from-pattern-with-dot-method":
		pat := esper.PatternFrom(bean(), "a", esper.Literal(true)).
			Then(esper.PatternFrom(bean(), "b", esper.Equal[string](
				ts(), esper.TagField[string]("a", "theString"))))
		if err := add("s0", pat.Select(
			esper.Alias("a", efovBeanString(esper.PatternEvent("a"))),
			esper.Alias("b", efovBeanString(esper.PatternEvent("b"))),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}

	case "context-with-start":
		start := esper.Equal[string](
			esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0"))
		if _, err := esper.CreateInitiatedContext(env, "MyContext", esper.Literal("global"), start); err != nil {
			return compat.Trace{}, err
		}
		pred := esper.Equal[string](
			esper.Concat(
				esper.Property[string](esper.ContextInitiatingEvent(), "p00"),
				esper.Property[string](esper.ContextInitiatingEvent(), "p01")),
			ts())
		if err := add("s0", esper.Select(bean().Filter(pred)).
			Query(esper.StatementName("s0"), esper.WithContext("MyContext"))); err != nil {
			return compat.Trace{}, err
		}

	case "in-set-of-value-wcoercion":
		pat := esper.PatternFrom(s0(), "a", esper.Literal(true)).
			Then(esper.PatternFrom(s1(), "b", esper.Literal(true))).
			Then(esper.PatternFrom(s2(), "c", esper.Literal(true))).
			Then(esper.PatternFrom(bean(), "s", esper.InOf(
				lp(),
				esper.TagField[int64]("a", "id"),
				esper.TagField[int64]("b", "id"),
				esper.TagField[int64]("c", "id"))).Every())
		if err := add("s0", pat.Select(
			esper.Alias("a", efovTagMap(esper.PatternEvent("a"))),
			esper.Alias("b", efovTagMap(esper.PatternEvent("b"))),
			esper.Alias("c", efovTagMap(esper.PatternEvent("c"))),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}

	case "in-range-wcoercion":
		lower := esper.Subtract[int64](esper.TagField[int64]("a", "id"), esper.Literal(int64(2)))
		upper := esper.Add[int64](esper.TagField[int64]("b", "id"), esper.Literal(int64(2)))
		mk := func(inclusive bool) esper.Expression[bool] {
			if inclusive {
				return esper.BetweenRangeOf(lp(), lower, upper, true, true)
			}
			return esper.NotBetweenRangeOf(lp(), lower, upper, true, true)
		}
		patIn := esper.PatternFrom(s0(), "a", esper.Literal(true)).
			Then(esper.PatternFrom(s1(), "b", esper.Literal(true))).
			Then(esper.PatternFrom(bean(), "s", mk(true)).Every())
		if err := add("s0", patIn.Select(
			esper.Alias("a", efovTagMap(esper.PatternEvent("a"))),
			esper.Alias("b", efovTagMap(esper.PatternEvent("b"))),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}
		patNotIn := esper.PatternFrom(s0(), "a", esper.Literal(true)).
			Then(esper.PatternFrom(s1(), "b", esper.Literal(true))).
			Then(esper.PatternFrom(bean(), "s", mk(false)).Every())
		if err := add("s0", patNotIn.Select(
			esper.Alias("a", efovTagMap(esper.PatternEvent("a"))),
			esper.Alias("b", efovTagMap(esper.PatternEvent("b"))),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}

	case "or-rewrite":
		start := esper.Equal[string](
			esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0"))
		if _, err := esper.CreateInitiatedContext(env, "MyContext", esper.Literal("global"), start); err != nil {
			return compat.Trace{}, err
		}
		p00 := esper.Property[string](esper.ContextInitiatingEvent(), "p00")
		p01 := esper.Property[string](esper.ContextInitiatingEvent(), "p01")
		pred := esper.Or(
			esper.Equal[string](esper.Concat(p00, p01), ts()),
			esper.Equal[string](esper.Concat(p01, p00), ts()))
		if err := add("s0", esper.Select(bean().Filter(pred)).
			Query(esper.StatementName("s0"), esper.WithContext("MyContext"))); err != nil {
			return compat.Trace{}, err
		}

	default:
		return compat.Trace{}, fmt.Errorf("unsupported expr-filter-optimizable-value-limited case %q", caseName)
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
				deployment, err := engine.Deploy(ctx, plan)
				if err != nil {
					return err
				}
				rt.deploymentIDs = append(rt.deploymentIDs, deployment.ID())
				for _, st := range deployment.Statements() {
					stmt := st
					rt.statements[stmt.Name()] = stmt
					if _, subErr := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
						hasNew := len(batch.New) > 0
						hasOld := len(batch.Old) > 0
						if !hasNew && !hasOld {
							return nil
						}
						record := rt.makeRecord(stmt.Name(), batch)
						if rt.caseName == "from-pattern-constant" {
							// The pinned pattern select * has no tags; the Java oracle
							// emits an empty property row. Drop the phantom projection.
							record.New = efovEmptyRows(record.New)
						}
						if rt.caseName == "context-with-start" || rt.caseName == "or-rewrite" {
							// Java's bean normalize renders the unset charPrimitive as the
							// string of the NUL character, not the numeric 0.
							efovCharPrimitiveString(record.New)
						}
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
			if err := rt.send(ctx, step.EventType, step.Payload); err != nil {
				return *rt.trace, err
			}
		case "deploy":
			if err := rt.undeployAll(ctx); err != nil {
				return *rt.trace, err
			}
			if err := deploy(step.Statement); err != nil {
				return *rt.trace, err
			}
		default:
			return *rt.trace, fmt.Errorf("unsupported expr-filter-optimizable-value-limited op %q (case %q)", step.Op, caseName)
		}
	}
	return *rt.trace, nil
}

func (rt *efovRuntime) makeRecord(stmt string, batch esper.ResultBatch) compat.TraceRecord {
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

func (rt *efovRuntime) undeployAll(ctx context.Context) error {
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

func (rt *efovRuntime) send(ctx context.Context, eventType string, payload json.RawMessage) error {
	switch eventType {
	case "SupportBean":
		var p map[string]any
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		v := efovBean{
			TheString:       jsonString(p["theString"]),
			BoolPrimitive:   jsonBool(p["boolPrimitive"]),
			IntPrimitive:    jsonInt32(p["intPrimitive"]),
			LongPrimitive:   jsonInt64(p["longPrimitive"]),
			DoublePrimitive: jsonFloat(p["doublePrimitive"]),
		}
		return rt.engine.SendEvent(ctx, v)
	case "SupportBean_S0":
		var p map[string]any
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		v := efovS0{ID: int(jsonInt64(p["id"])), P00: jsonString(p["p00"]), P01: jsonString(p["p01"]),
			P02: jsonString(p["p02"]), P03: jsonString(p["p03"]), Value: int(jsonInt64(p["value"]))}
		return rt.engine.SendEvent(ctx, v)
	case "SupportBean_S1":
		var p map[string]any
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		v := efovS1{ID: int(jsonInt64(p["id"])), P10: jsonString(p["p10"]),
			P11: jsonString(p["p11"]), P12: jsonString(p["p12"]), P13: jsonString(p["p13"])}
		return rt.engine.SendEvent(ctx, v)
	case "SupportBean_S2":
		var p map[string]any
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		v := efovS2{ID: int(jsonInt64(p["id"])),
			P20: jsonString(p["p20"]), P21: jsonString(p["p21"]), P22: jsonString(p["p22"]), P23: jsonString(p["p23"])}
		return rt.engine.SendEvent(ctx, v)
	default:
		return fmt.Errorf("unsupported event type %q", eventType)
	}
}

func jsonBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// efovBeanString renders a pattern-tag SupportBean the way the Java oracle
// renders bean tags: SupportBean(<theString>, <intPrimitive>).
func efovBeanString(tag esper.Expression[esper.Event]) esper.Expression[string] {
	return esper.Func1Ctx[esper.Event, string]("efovBeanString",
		func(e esper.Event, _ esper.EvalContext) string {
			ts, _ := e.Get("theString").Any().(string)
			ip, _ := e.Get("intPrimitive").Any().(int32)
			return fmt.Sprintf("SupportBean(%s, %d)", ts, ip)
		}, tag)
}

// efovTagArray renders a repeated pattern tag as an array of plain maps.
func efovTagArray(events esper.Expression[[]esper.Event]) esper.Expression[[]map[string]any] {
	return esper.Func1Ctx[[]esper.Event, []map[string]any]("efovTagArray",
		func(events []esper.Event, _ esper.EvalContext) []map[string]any {
			rows := make([]map[string]any, 0, len(events))
			for _, e := range events {
				row := make(map[string]any, len(e.Schema().Fields()))
				for _, field := range e.Schema().Fields() {
					row[field.Name] = e.Get(field.Name).Any()
				}
				rows = append(rows, row)
			}
			return rows
		}, events)
}

// efovTagMap renders a pattern tag Event as a plain map (mirrors the Java
// oracle's pattern-tag rendering).
func efovTagMap(tag esper.Expression[esper.Event]) esper.Expression[map[string]any] {
	return esper.Func1Ctx[esper.Event, map[string]any]("efovTagMap",
		func(e esper.Event, _ esper.EvalContext) map[string]any {
			out := make(map[string]any, len(e.Schema().Fields()))
			for _, field := range e.Schema().Fields() {
				out[field.Name] = e.Get(field.Name).Any()
			}
			return out
		}, tag)
}

// efovEmptyRows rewrites rows to the empty-field shape the Java oracle emits
// for a pattern select * with no matched tags.
func efovEmptyRows(rows []compat.ResultRecord) []compat.ResultRecord {
	out := make([]compat.ResultRecord, 0, len(rows))
	for range rows {
		out = append(out, compat.ResultRecord{Kind: "row", Fields: map[string]any{}})
	}
	return out
}

// efovCharPrimitiveString renders the unset charPrimitive the way the Java
// oracle does: a single NUL character string instead of the numeric 0.
func efovCharPrimitiveString(rows []compat.ResultRecord) {
	for _, row := range rows {
		if v, ok := row.Fields["charPrimitive"]; ok {
			if n, ok := v.(int32); ok && n == 0 {
				row.Fields["charPrimitive"] = "\u0000"
			}
		}
	}
}
