package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultSetQueryTypeRowForAllJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultSetQueryTypeRowForAllJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRowForAll.java",
}

var resultSetQueryTypeRowForAllJavaRuntimeIDs = []string{
	"java-runtime-1d0f29a53fbb381563ec",
	"java-runtime-061f312bd81e3cb1c340",
	"java-runtime-422d65bd284a78278d56e",
	"java-runtime-9a7f951d363bb58d506e",
	"java-runtime-780e3dede1e586fde738",
}

var resultSetQueryTypeRowForAllJavaExecutions = []string{
	"ResultSetQueryTypeRowForAllSumOneView",
	"ResultSetQueryTypeRowForAllSumJoin",
	"ResultSetQueryTypeRowForAllAvgPerSym",
	"ResultSetQueryTypeRowForAllSelectStarStdGroupBy",
	"ResultSetQueryTypeRowForAllSelectExprGroupWin",
}

var resultSetQueryTypeRowForAllCases = []string{
	"sum-one-view",
	"sum-join",
	"avg-per-sym",
	"select-star-std-group-by",
	"select-expr-group-win",
}

type resultSetQueryTypeRowForAllBean struct {
	TheString string `esper:"theString"`
	LongBoxed *int64 `esper:"longBoxed"`
}

type resultSetQueryTypeRowForAllBeanString struct {
	TheString string `esper:"theString"`
}

type resultSetQueryTypeRowForAllPriceEvent struct {
	Price int    `esper:"price"`
	Sym   string `esper:"sym"`
}

type resultSetQueryTypeRowForAllMarketData struct {
	Symbol string  `esper:"symbol"`
	ID     *string `esper:"id"`
	Price  float64 `esper:"price"`
	Volume *int64  `esper:"volume"`
	Feed   *string `esper:"feed"`
}

func runResultSetQueryTypeRowForAllScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range resultSetQueryTypeRowForAllCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultSetQueryTypeRowForAllCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset querytype row-for-all case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("resultset querytype row-for-all scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runResultSetQueryTypeRowForAllCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultSetQueryTypeRowForAllBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultSetQueryTypeRowForAllBeanString](env, "SupportBeanString"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultSetQueryTypeRowForAllPriceEvent](env, "SupportPriceEvent"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultSetQueryTypeRowForAllMarketData](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}

	var query esper.Query
	switch caseName {
	case "sum-one-view":
		longBoxed := esper.Field[resultSetQueryTypeRowForAllBean, *int64]("longBoxed")
		longValue := esper.Cast[*int64, int64](longBoxed)
		query = esper.From[resultSetQueryTypeRowForAllBean](env, "SupportBean").
			Window(esper.TimeWindow(10*time.Second)).
			Aggregate(esper.Alias("mySum", esper.Sum[int64](longValue))).
			Query(esper.StatementName("s0"), esper.WithOldStream())
	case "sum-join":
		left := esper.From[resultSetQueryTypeRowForAllBeanString](env, "SupportBeanString").Window(esper.KeepAll())
		right := esper.From[resultSetQueryTypeRowForAllBean](env, "SupportBean").Window(esper.TimeWindow(10 * time.Second))
		joined := esper.Join(left, right, esper.OnEqual(
			esper.Field[resultSetQueryTypeRowForAllBeanString, string]("theString"),
			esper.Field[resultSetQueryTypeRowForAllBean, string]("theString"),
		))
		longBoxed := esper.JoinField[*int64](1, "longBoxed")
		longValue := esper.Cast[*int64, int64](longBoxed)
		query = joined.Aggregate(esper.Alias("mySum", esper.Sum[int64](longValue))).
			Query(esper.StatementName("s0"), esper.WithOldStream())
	case "avg-per-sym":
		sym := esper.Field[resultSetQueryTypeRowForAllPriceEvent, string]("sym")
		price := esper.Field[resultSetQueryTypeRowForAllPriceEvent, int]("price")
		query = esper.From[resultSetQueryTypeRowForAllPriceEvent](env, "SupportPriceEvent").
			Window(esper.GroupWindow(sym, esper.LengthWindow(2))).
			GroupBy(esper.Literal("row-for-all")).
			Select(esper.Alias("avgp", esper.Avg[int](price)), esper.Alias("sym", sym)).
			Query(esper.StatementName("s0"), esper.WithOldStream())
	case "select-star-std-group-by":
		symbol := esper.Field[resultSetQueryTypeRowForAllMarketData, string]("symbol")
		query = esper.From[resultSetQueryTypeRowForAllMarketData](env, "SupportMarketDataBean").
			Window(esper.GroupWindow(symbol, esper.LengthWindow(2))).
			Query(esper.StatementName("s0"))
	case "select-expr-group-win":
		symbol := esper.Field[resultSetQueryTypeRowForAllMarketData, string]("symbol")
		price := esper.Field[resultSetQueryTypeRowForAllMarketData, float64]("price")
		query = esper.Select(
			esper.From[resultSetQueryTypeRowForAllMarketData](env, "SupportMarketDataBean").
				Window(esper.GroupWindow(symbol, esper.LengthWindow(2))),
			esper.Alias("price", price),
		).Query(esper.StatementName("s0"))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported case %q", caseName)
	}

	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one statement, got %d", len(statements))
	}
	statement := statements[0]
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	seq := uint64(0)
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		newRows := compat.NormalizeResults(batch.New)
		oldRows := compat.NormalizeResults(batch.Old)
		if caseName == "select-star-std-group-by" {
			for i := range newRows {
				newRows[i].Fields["__type"] = "SupportMarketDataBean"
			}
			for i := range oldRows {
				oldRows[i].Fields["__type"] = "SupportMarketDataBean"
			}
		}
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
			if err := sendResultSetQueryTypeRowForAllEvent(ctx, engine, step); err != nil {
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
		case "snapshot":
			if step.Statement != statement.Name() {
				return trace, fmt.Errorf("unknown snapshot statement %q", step.Statement)
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return trace, err
			}
			rows := compat.NormalizeResults(result.Results())
			seq++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case: caseName, Operation: "snapshot", Statement: statement.Name(),
				Sequence: seq, Time: engine.Now().UTC().Format(time.RFC3339), New: rows,
			})
		default:
			return trace, fmt.Errorf("unsupported step %q", step.Op)
		}
	}
	return trace, nil
}

func sendResultSetQueryTypeRowForAllEvent(ctx context.Context, engine *esper.Engine, step compat.Step) error {
	switch step.EventType {
	case "SupportBean":
		var payload struct {
			TheString *string `json:"theString"`
			LongBoxed *int64  `json:"longBoxed"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		event := resultSetQueryTypeRowForAllBean{}
		if payload.TheString != nil {
			event.TheString = *payload.TheString
		}
		event.LongBoxed = payload.LongBoxed
		return engine.Send(ctx, step.EventType, event)
	case "SupportBeanString":
		var payload struct {
			TheString string `json:"theString"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		return engine.Send(ctx, step.EventType, resultSetQueryTypeRowForAllBeanString{TheString: payload.TheString})
	case "SupportPriceEvent":
		var payload struct {
			Price int    `json:"price"`
			Sym   string `json:"sym"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		return engine.Send(ctx, step.EventType, resultSetQueryTypeRowForAllPriceEvent{Price: payload.Price, Sym: payload.Sym})
	case "SupportMarketDataBean":
		var payload struct {
			Symbol string  `json:"symbol"`
			ID     *string `json:"id"`
			Price  float64 `json:"price"`
			Volume *int64  `json:"volume"`
			Feed   *string `json:"feed"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		return engine.Send(ctx, step.EventType, resultSetQueryTypeRowForAllMarketData{
			Symbol: payload.Symbol, ID: payload.ID, Price: payload.Price, Volume: payload.Volume, Feed: payload.Feed,
		})
	default:
		return fmt.Errorf("unknown event type %q", step.EventType)
	}
}
