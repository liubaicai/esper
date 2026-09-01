package parity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// resultsetQueryTypeRowPerGroupHavingSupportBean mirrors the full pinned
// com.espertech.esper.common.internal.support.SupportBean property surface.
// The having-count execution selects wildcard rows, so the trace carries
// every property: primitives default, boxed pointers stay nil for null, and
// charPrimitive mirrors the Java char default "\u0000".
type resultsetQueryTypeRowPerGroupHavingSupportBean struct {
	TheString       string   `esper:"theString"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntPrimitive    int      `esper:"intPrimitive"`
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

type resultsetQueryTypeRowPerGroupHavingMarketData struct {
	Symbol string  `esper:"symbol"`
	ID     *string `esper:"id"`
	Price  float64 `esper:"price"`
	Volume *int64  `esper:"volume"`
	Feed   *string `esper:"feed"`
}

type resultsetQueryTypeRowPerGroupHavingBeanString struct {
	TheString string `esper:"theString"`
}

const resultsetQueryTypeRowPerGroupHavingID = "resultset-querytype-row-per-group-having"

const resultsetQueryTypeRowPerGroupHavingDescription = "Row-per-group having scenarios replaying ResultSetQueryTypeRowPerGroupHaving executions over SupportBean/SupportBeanString/SupportMarketDataBean: wildcard row-per-group count gate, irstream sum(price) per symbol with having both joined and single-view (pre-subtraction remove-stream sums), time_batch count flushes and the declared-expression having compile boundary"

const resultsetQueryTypeRowPerGroupHavingJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetQueryTypeRowPerGroupHavingJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRowPerGroupHaving.java",
}

// Case order fixes the runtime-ID index mapping below. The five executions
// share one Java class, one scenario, one oracle and one runner surface, so
// the unit ports them together (the ECSM multi-runtime precedent).
var resultsetQueryTypeRowPerGroupHavingJavaRuntimeIDs = []string{
	"java-runtime-fabf6dfeea92bd82d953",
	"java-runtime-b77f112e44eb71ef5269",
	"java-runtime-8e64b633898a3cf68ed8",
	"java-runtime-3673c61f7b1a281d9970",
	"java-runtime-cd60cf2c28460a91c7d1",
}

var resultsetQueryTypeRowPerGroupHavingJavaExecutions = []string{
	"ResultSetQueryTypeHavingCount",
	"ResultSetQueryTypeSumJoin",
	"ResultSetQueryTypeSumOneView",
	"ResultSetQueryTypeRowPerGroupBatch",
	"ResultSetQueryTypeRowPerGroupDefinedExpr",
}

var resultsetQueryTypeRowPerGroupHavingCases = []string{
	"having-count",
	"sum-join",
	"sum-one-view",
	"batch",
	"defined-expr",
}

// havingCountInvalidEPL pins the compile-invalid declared-expression having
// of ordinal 4: F(longPrimitive) references a non-aggregated property outside
// the group-by, which Java rejects at compile time. The Go Build gate must
// reject the equivalent having-containment violation with the pinned sentence.
const havingCountInvalidEPL = "expression F {v -> v} select sum(intPrimitive) from SupportBean group by theString having count(*) > F(longPrimitive)"

// runResultsetQueryTypeRowPerGroupHavingScenario replays the five grouped
// row-per-group having executions. Java semantics pinned by the oracle:
// remove-stream rows carry pre-subtraction aggregate values (the leaving
// event's contribution is still included), the having gate applies per group
// to both streams, and istream-only statements drop the old side even when
// the window posts removals. The defined-expr execution is compile-only: it
// verifies a valid declared-expression having builds and that a having
// referencing a non-aggregated property outside the group-by is rejected with
// Java's exact diagnostic.
func runResultsetQueryTypeRowPerGroupHavingScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range resultsetQueryTypeRowPerGroupHavingCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultsetQueryTypeRowPerGroupHavingCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset querytype row-per-group having case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("resultset querytype row-per-group having scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runResultsetQueryTypeRowPerGroupHavingCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetQueryTypeRowPerGroupHavingSupportBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetQueryTypeRowPerGroupHavingMarketData](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	join := caseName == "sum-join"
	if join {
		if _, err := esper.RegisterStruct[resultsetQueryTypeRowPerGroupHavingBeanString](env, "SupportBeanString"); err != nil {
			return compat.Trace{}, err
		}
	}

	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(ctx) }()

	var plan esper.Plan
	var planErr error
	switch caseName {
	case "having-count":
		// Java: select * from SupportBean(intPrimitive = 3)#length(10) as e1
		// group by theString having count(*) > 2. The wildcard row renders
		// every SupportBean property; the Go typed surface spells the full
		// property set explicitly (Java property names, identical observable
		// field map) because an aggregate requires at least one projection.
		theString := esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, string]("theString")
		countRef := esper.Greater[int64](esper.CountAll(), esper.Literal(int64(2)))
		plan, planErr = env.Build(esper.From[resultsetQueryTypeRowPerGroupHavingSupportBean](env, "SupportBean").
			Filter(esper.Equal[int](esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, int]("intPrimitive"), esper.Literal(3))).
			Window(esper.LengthWindow(10)).
			GroupBy(theString).
			Select(resultsetQueryTypeRowPerGroupHavingWildcardSelections()...).
			Having(countRef).
			Query(esper.StatementName("s0")))
	case "sum-join", "sum-one-view":
		// Java: select irstream symbol, sum(price) as mySum ... group by
		// symbol having sum(price) >= 100 over SupportMarketDataBean#length(3)
		// [joined with SupportBeanString#length(100) on theString = symbol].
		plan, planErr = buildResultsetQueryTypeRowPerGroupHavingSum(env, join)
	case "batch":
		// Java: select count(*) as y from SupportBean#time_batch(1 seconds)
		// group by theString having count(*) > 0. Istream-only: the window's
		// old side never reaches the listener.
		theString := esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, string]("theString")
		plan, planErr = env.Build(esper.From[resultsetQueryTypeRowPerGroupHavingSupportBean](env, "SupportBean").
			Window(esper.TimeBatch(1 * time.Second)).
			GroupBy(theString).
			Select(esper.Alias("y", esper.CountAll())).
			Having(esper.Greater[int64](esper.CountAll(), esper.Literal(int64(0)))).
			Query(esper.StatementName("s0")))
	case "defined-expr":
		// Java: expression F {v -> v} select sum(intPrimitive) from
		// SupportBean group by theString having count(*) > F(1) compiles, the
		// F(longPrimitive) variant is a compile error. The valid declared
		// expression builds; the invalid variant must fail with Java's exact
		// sentence.
		if err := esper.DefineExpression[int64](env, "F", esper.ExpressionParam[int64]("v")); err != nil {
			return trace, err
		}
		// The valid form count(*) > F(1) must build cleanly: Java compiles
		// it as a separate statement before probing the invalid variant.
		if _, err := havingCountValidPlan(env); err != nil {
			return trace, err
		}
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "deployed",
			Statement: "s0-valid",
			Time:      "1970-01-01T00:00:00Z",
		})
		for _, step := range scenario.Steps {
			switch step.Op {
			case "case":
				continue
			case "build-error":
				if step.Statement != "s0-invalid" {
					return trace, fmt.Errorf("unknown defined-expr build probe %q", step.Statement)
				}
				_, buildErr := env.Build(havingCountInvalidPlan(env))
				message := "<no-error>"
				if buildErr != nil {
					var espErr *esper.Error
					if errors.As(buildErr, &espErr) && espErr.Message != "" {
						message = espErr.Message
					} else {
						message = buildErr.Error()
					}
					message = message + " [" + step.Epl + "]"
				}
				trace.Records = append(trace.Records, compat.TraceRecord{
					Case:      caseName,
					Operation: "build-error",
					Statement: step.Statement,
					Time:      "1970-01-01T00:00:00Z",
					Value:     message,
				})
			default:
				return trace, fmt.Errorf("unsupported defined-expr step %q", step.Op)
			}
		}
		return trace, nil
	default:
		return compat.Trace{}, fmt.Errorf("unsupported resultset querytype row-per-group having case %q", caseName)
	}
	if planErr != nil {
		return compat.Trace{}, planErr
	}

	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one statement, got %d", len(statements))
	}
	statement := statements[0]
	seq := uint64(0)
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		newRows := compat.NormalizeResults(batch.New)
		oldRows := compat.NormalizeResults(batch.Old)
		if len(newRows) == 0 && len(oldRows) == 0 {
			return nil
		}
		seq++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "listener", Statement: statement.Name(),
			Sequence: seq, Time: batch.Time.UTC().Format(time.RFC3339), New: newRows, Old: oldRows,
		})
		return nil
	}); err != nil {
		return compat.Trace{}, err
	}

	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			if err := sendResultsetQueryTypeRowPerGroupHavingEvent(ctx, engine, step); err != nil {
				return trace, err
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return trace, fmt.Errorf("decode advance-time: %w", err)
			}
			if err := engine.AdvanceTime(ctx, at.UTC()); err != nil {
				return trace, err
			}
		default:
			return trace, fmt.Errorf("unsupported step %q", step.Op)
		}
	}
	return trace, nil
}

// havingCountValidPlan mirrors the valid declared-expression having:
// expression F {v -> v} select sum(intPrimitive) from SupportBean group by
// theString having count(*) > F(1). The declared-expression argument is a
// literal, so the having carries no non-aggregated property reference.
func havingCountValidPlan(env *esper.Environment) (esper.Plan, error) {
	theString := esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, string]("theString")
	countRef := esper.Greater[int64](esper.CountAll(), esper.ExpressionRef[int64](env, "F", esper.Literal(int64(1))))
	return env.Build(esper.From[resultsetQueryTypeRowPerGroupHavingSupportBean](env, "SupportBean").
		Window(esper.LengthWindow(10)).
		GroupBy(theString).
		Select(esper.Alias("sum(intPrimitive)", esper.Sum[int](esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, int]("intPrimitive")))).
		Having(countRef).
		Query(esper.StatementName("s0-valid")))
}

// havingCountInvalidPlan mirrors the invalid declared-expression having: the
// group-by is theString while the having compares count(*) against a
// declared-expression call whose argument reads longPrimitive, a
// non-aggregated property outside the group-by.
func havingCountInvalidPlan(env *esper.Environment) esper.Query {
	theString := esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, string]("theString")
	havingArg := esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, int64]("longPrimitive")
	countRef := esper.Greater[int64](esper.CountAll(), esper.ExpressionRef[int64](env, "F", havingArg))
	return esper.From[resultsetQueryTypeRowPerGroupHavingSupportBean](env, "SupportBean").
		Window(esper.LengthWindow(10)).
		GroupBy(theString).
		Select(esper.Alias("sum(intPrimitive)", esper.Sum[int](esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, int]("intPrimitive")))).
		Having(countRef).
		Query(esper.StatementName("s0-invalid"))
}

// resultsetQueryTypeRowPerGroupHavingWildcardSelections spells the full
// SupportBean property surface with Java property names (Java's wildcard
// projection renders every event property flat).
func resultsetQueryTypeRowPerGroupHavingWildcardSelections() []esper.Selection {
	bean := func(name string) esper.Expr {
		switch name {
		case "theString":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, string]("theString")
		case "boolPrimitive":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, bool]("boolPrimitive")
		case "intPrimitive":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, int]("intPrimitive")
		case "longPrimitive":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, int64]("longPrimitive")
		case "charPrimitive":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, string]("charPrimitive")
		case "shortPrimitive":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, int16]("shortPrimitive")
		case "bytePrimitive":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, int8]("bytePrimitive")
		case "floatPrimitive":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, float32]("floatPrimitive")
		case "doublePrimitive":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, float64]("doublePrimitive")
		case "boolBoxed":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, *bool]("boolBoxed")
		case "intBoxed":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, *int]("intBoxed")
		case "longBoxed":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, *int64]("longBoxed")
		case "charBoxed":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, *string]("charBoxed")
		case "shortBoxed":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, *int16]("shortBoxed")
		case "byteBoxed":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, *int8]("byteBoxed")
		case "floatBoxed":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, *float32]("floatBoxed")
		case "doubleBoxed":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, *float64]("doubleBoxed")
		case "bigDecimal":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, *float64]("bigDecimal")
		case "bigInteger":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, *int64]("bigInteger")
		case "enumValue":
			return esper.Field[resultsetQueryTypeRowPerGroupHavingSupportBean, *string]("enumValue")
		}
		panic("unknown SupportBean property " + name)
	}
	names := []string{
		"theString", "boolPrimitive", "intPrimitive", "longPrimitive",
		"charPrimitive", "shortPrimitive", "bytePrimitive", "floatPrimitive",
		"doublePrimitive", "boolBoxed", "intBoxed", "longBoxed", "charBoxed",
		"shortBoxed", "byteBoxed", "floatBoxed", "doubleBoxed", "bigDecimal",
		"bigInteger", "enumValue",
	}
	selections := make([]esper.Selection, 0, len(names))
	for _, name := range names {
		selections = append(selections, esper.Alias(name, bean(name)))
	}
	return selections
}

// buildResultsetQueryTypeRowPerGroupHavingSum constructs the irstream
// symbol/sum(price) having statement shared by ordinals 1 and 2. The join
// twin reads market fields through JoinField(0, ...) so the group key and
// aggregate bind against the driving market side of the tuple.
func buildResultsetQueryTypeRowPerGroupHavingSum(env *esper.Environment, join bool) (esper.Plan, error) {
	market := esper.From[resultsetQueryTypeRowPerGroupHavingMarketData](env, "SupportMarketDataBean").
		Window(esper.LengthWindow(3))
	if !join {
		symbol := esper.Field[resultsetQueryTypeRowPerGroupHavingMarketData, string]("symbol")
		price := esper.Field[resultsetQueryTypeRowPerGroupHavingMarketData, float64]("price")
		sumPrice := esper.Sum[float64](price)
		return env.Build(market.GroupBy(symbol).Select(
			esper.Alias("symbol", symbol),
			esper.Alias("mySum", sumPrice),
		).Having(esper.GreaterOrEqual[float64](sumPrice, esper.Literal(100.0))).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
	}
	seed := esper.From[resultsetQueryTypeRowPerGroupHavingBeanString](env, "SupportBeanString").Window(esper.LengthWindow(100))
	symbol := esper.JoinField[string](0, "symbol")
	price := esper.JoinField[float64](0, "price")
	sumPrice := esper.Sum[float64](price)
	joined := esper.Join(market, seed, esper.OnEqual(
		esper.JoinField[string](1, "theString"),
		symbol,
	))
	return env.Build(joined.GroupBy(symbol).Select(
		esper.Alias("symbol", symbol),
		esper.Alias("mySum", sumPrice),
	).Having(esper.GreaterOrEqual[float64](sumPrice, esper.Literal(100.0))).
		Query(esper.StatementName("s0"), esper.WithOldStream()))
}

func sendResultsetQueryTypeRowPerGroupHavingEvent(ctx context.Context, engine *esper.Engine, step compat.Step) error {
	switch step.EventType {
	case "SupportBean":
		var payload struct {
			TheString    string `json:"theString"`
			IntPrimitive int    `json:"intPrimitive"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		// The Java char default renders as "\u0000" in the trace; the Go
		// string zero would render as "".
		return engine.Send(ctx, "SupportBean", resultsetQueryTypeRowPerGroupHavingSupportBean{
			TheString: payload.TheString, IntPrimitive: payload.IntPrimitive,
			CharPrimitive: "\u0000",
		})
	case "SupportBeanString":
		var payload struct {
			TheString string `json:"theString"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		return engine.Send(ctx, "SupportBeanString", resultsetQueryTypeRowPerGroupHavingBeanString{TheString: payload.TheString})
	case "SupportMarketDataBean":
		var payload struct {
			Symbol string  `json:"symbol"`
			Price  float64 `json:"price"`
			Volume *int64  `json:"volume"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		return engine.Send(ctx, "SupportMarketDataBean", resultsetQueryTypeRowPerGroupHavingMarketData{
			Symbol: payload.Symbol, Price: payload.Price, Volume: payload.Volume,
		})
	}
	return fmt.Errorf("unknown event type %q", step.EventType)
}

func resultsetQueryTypeRowPerGroupHavingRuntimeID(caseName string) string {
	for index, name := range resultsetQueryTypeRowPerGroupHavingCases {
		if name == caseName {
			return resultsetQueryTypeRowPerGroupHavingJavaRuntimeIDs[index]
		}
	}
	return "resultset-querytype-row-per-group-having-unknown"
}
