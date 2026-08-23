package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the avg-HAVING family of ResultSetQueryTypeHaving:
// non-grouped HAVING comparing a property against an aggregate over a length
// window (irstream old-stream re-evaluation included), its two-stream join
// twin, and the compile-only mixed aggregated/non-aggregated HAVING.
var resultSetQueryTypeHavingJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeHaving.java",
}

type havingMarketData struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
	Feed   *string `esper:"feed"`
}

type havingBeanString struct {
	TheString string `esper:"theString"`
}

var (
	resultsetQueryTypeHavingJavaRuntimeIDs = []string{
		"java-runtime-8a05b3435dcc1ad1f414", // StatementOM (object-model twin)
		"java-runtime-c33f49e879157f89fb62", // Statement (text model)
		"java-runtime-c5e8204a0ec635e5ded3", // StatementJoin
		"java-runtime-2aa18c37135a5c99514b", // SumHavingNoAggregatedProp
	}
	resultsetQueryTypeHavingJavaExecutions = []string{
		"ResultSetQueryTypeStatementOM",
		"ResultSetQueryTypeStatement",
		"ResultSetQueryTypeStatementJoin",
		"ResultSetQueryTypeSumHavingNoAggregatedProp",
	}
)

func runResultSetQueryTypeHavingScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	// having-statement replays twice: the scenario declares one entry per
	// runtime ID (text model + object model twin); the oracle emits two
	// identical replay groups and so does this runner.
	for _, caseName := range []string{
		"having-statement", "having-statement",
		"having-sum-noagg-prop",
	} {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		caseTrace, err := runResultSetQueryTypeHavingCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-query-type-having case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if len(trace.Records) == 0 {
		return compat.Trace{}, fmt.Errorf("resultset-query-type-having scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runResultSetQueryTypeHavingCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	str := reflect.TypeOf("")
	f64 := reflect.TypeOf(float64(0))
	i64 := reflect.TypeOf(int64(0))
	if _, err := esper.RegisterMap(env, "SupportMarketDataBean", []esper.FieldSpec{
		esper.FieldDef("symbol", str),
		esper.FieldDef("price", f64),
		esper.FieldDef("volume", i64),
		esper.FieldDef("feed", str),
	}); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[havingBeanString](env, "SupportBeanString"); err != nil {
		return compat.Trace{}, err
	}

	symbol := func() esper.Expression[string] { return esper.Field[havingMarketData, string]("symbol") }
	price := func() esper.Expression[float64] { return esper.Field[havingMarketData, float64]("price") }
	volume := func() esper.Expression[int64] { return esper.Field[havingMarketData, int64]("volume") }

	var plans []struct {
		name     string
		plan     esper.Plan
		listener bool
	}
	addPlan := func(name string, listener bool, query esper.Query) error {
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		plans = append(plans, struct {
			name     string
			plan     esper.Plan
			listener bool
		}{name, plan, listener})
		return nil
	}

	switch caseName {
	case "having-statement":
		err = addPlan("s0", true, esper.From[havingMarketData](env, "SupportMarketDataBean").
			Window(esper.LengthWindow(5)).
			Aggregate(
				esper.Alias("symbol", symbol()),
				esper.Alias("price", price()),
				esper.Alias("avgPrice", esper.Avg[float64](price())),
			).
			Having(esper.Less[float64](price(), esper.Avg[float64](price()))).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
		if err != nil {
			return compat.Trace{}, err
		}
	case "having-statement-join":
		// Java: where one.theString = two.symbol (inner-join predicate).
		err = addPlan("s0", true, esper.JoinMany(
			esper.JoinSource(esper.From[havingBeanString](env, "SupportBeanString").Window(esper.LengthWindow(100))),
			esper.JoinSource(esper.From[havingMarketData](env, "SupportMarketDataBean").Window(esper.LengthWindow(5))),
		).On(
			esper.OnSourcesEqual(0,
				esper.JoinField[string](0, "theString"),
				1, esper.JoinField[string](1, "symbol")),
		).Aggregate(
			esper.Alias("symbol", esper.JoinField[string](1, "symbol")),
			esper.Alias("price", esper.JoinField[float64](1, "price")),
			esper.Alias("avgPrice", esper.Avg[float64](esper.JoinField[float64](1, "price"))),
		).
			Having(esper.Less[float64](
				esper.JoinField[float64](1, "price"),
				esper.Avg[float64](esper.JoinField[float64](1, "price")),
			)).
			Query(esper.StatementName("s0"), esper.WithOldStream()))
		if err != nil {
			return compat.Trace{}, err
		}
	case "having-sum-noagg-prop":
		err = addPlan("s0", false, esper.From[havingMarketData](env, "SupportMarketDataBean").
			Window(esper.LengthWindow(5)).
			Aggregate(
				esper.Alias("symbol", symbol()),
				esper.Alias("price", price()),
				esper.Alias("avgPrice", esper.Avg[float64](price())),
			).
			Having(esper.Less[int64](volume(), esper.Avg[float64](price()))).
			Query(esper.StatementName("s0")))
		if err != nil {
			return compat.Trace{}, err
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported resultset-query-type-having case %q", caseName)
	}

	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	seq := uint64(0)
	for _, item := range plans {
		deployment, err := engine.Deploy(ctx, item.plan)
		if err != nil {
			return trace, err
		}
		if !item.listener {
			continue
		}
		statement := deployment.Statements()[0]
		if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			seq++
			record := compat.TraceRecord{
				Case:      caseName,
				Operation: "listener",
				Statement: statement.Name(),
				Time:      batch.Time.UTC().Format(time.RFC3339),
				Sequence:  seq,
			}
			record.New = compat.NormalizeResults(batch.New)
			record.Old = compat.NormalizeResults(batch.Old)
			if len(record.New) == 0 && len(record.Old) == 0 {
				seq--
				return nil
			}
			trace.Records = append(trace.Records, record)
			return nil
		}); err != nil {
			return trace, err
		}
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			var payload struct {
				Symbol    string  `json:"symbol"`
				Price     float64 `json:"price"`
				TheString string  `json:"theString"`
			}
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return trace, fmt.Errorf("resultset-query-type-having decode %s: %w", step.EventType, err)
			}
			switch step.EventType {
			case "SupportMarketDataBean":
				event := map[string]any{
					"symbol": payload.Symbol,
					"price":  payload.Price,
					"volume": int64(0),
					"feed":   nil,
				}
				if err := engine.Send(ctx, step.EventType, event); err != nil {
					return trace, err
				}
			case "SupportBeanString":
				if err := engine.SendEvent(ctx, havingBeanString{TheString: payload.TheString}); err != nil {
					return trace, err
				}
			default:
				return trace, fmt.Errorf("unsupported event type %q", step.EventType)
			}
		case "deployed":
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
			})
		default:
			return trace, fmt.Errorf("unsupported resultset-query-type-having step op %q", step.Op)
		}
	}
	return trace, nil
}
