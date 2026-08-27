package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const resultSetQueryTypeWTimeBatchJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultSetQueryTypeWTimeBatchJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeWTimeBatch.java",
}

var (
	resultSetQueryTypeWTimeBatchJavaRuntimeIDs = []string{
		"java-runtime-2f347eeeb6e7595bf504",
		"java-runtime-50b076da85020a750f08",
		"java-runtime-77a1932ea0676781652b",
		"java-runtime-d38820596d857deae67c",
		"java-runtime-a68578cc7531f737bd0d",
		"java-runtime-3c33a86f202c6bf3471b",
		"java-runtime-d14f54fbebaf213da348",
		"java-runtime-c76ba92d72beb61d0a66",
	}
	resultSetQueryTypeWTimeBatchJavaExecutions = []string{
		"ResultSetQueryTypeTimeBatchRowForAllNoJoin",
		"ResultSetQueryTypeTimeBatchRowForAllJoin",
		"ResultSetQueryTypeTimeBatchRowPerEventNoJoin",
		"ResultSetQueryTypeTimeBatchRowPerEventJoin",
		"ResultSetQueryTypeTimeBatchRowPerGroupNoJoin",
		"ResultSetQueryTypeTimeBatchRowPerGroupJoin",
		"ResultSetQueryTypeTimeBatchAggrGroupedNoJoin",
		"ResultSetQueryTypeTimeBatchAggrGroupedJoin",
	}
)

var resultSetQueryTypeWTimeBatchCases = []string{
	"time-batch-row-for-all-nojoin",
	"time-batch-row-for-all-join",
	"time-batch-row-per-event-nojoin",
	"time-batch-row-per-event-join",
	"time-batch-row-per-group-nojoin",
	"time-batch-row-per-group-join",
	"time-batch-aggr-grouped-nojoin",
	"time-batch-aggr-grouped-join",
}

type wtbMarketData struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume *int64  `esper:"volume"`
}

type wtbBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func runResultSetQueryTypeWTimeBatchScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range resultSetQueryTypeWTimeBatchCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultSetQueryTypeWTimeBatchCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset querytype w-time-batch case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("resultset querytype w-time-batch scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runResultSetQueryTypeWTimeBatchCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(ctx) }()
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}

	if _, err := esper.RegisterStruct[wtbMarketData](env, "SupportMarketDataBean"); err != nil {
		return trace, err
	}
	if _, err := esper.RegisterStruct[wtbBean](env, "SupportBean"); err != nil {
		return trace, err
	}

	symbol := esper.Field[wtbMarketData, string]("symbol")
	volume := esper.Field[wtbMarketData, *int64]("volume")
	price := esper.Field[wtbMarketData, float64]("price")
	md := esper.From[wtbMarketData](env, "SupportMarketDataBean").Window(esper.TimeBatch(1 * time.Second))
	var plan esper.Plan
	var planErr error
	switch caseName {
	case "time-batch-row-for-all-nojoin":
		plan, planErr = env.Build(md.Aggregate(
			esper.Alias("sumPrice", esper.Sum[float64](price)),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "time-batch-row-for-all-join":
		joined := esper.Join(
			md,
			esper.From[wtbBean](env, "SupportBean").Window(esper.KeepAll()),
			espreOnEqualSymbol(),
		)
		joinedPrice := esper.JoinField[float64](0, "price")
		plan, planErr = env.Build(joined.Aggregate(
			esper.Alias("sumPrice", esper.Sum[float64](joinedPrice)),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "time-batch-row-per-event-nojoin":
		plan, planErr = env.Build(md.Aggregate(
			esper.Alias("symbol", symbol),
			esper.Alias("sumPrice", esper.Sum[float64](price)),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "time-batch-row-per-event-join":
		joined := esper.Join(
			md,
			esper.From[wtbBean](env, "SupportBean").Window(esper.KeepAll()),
			espreOnEqualSymbol(),
		)
		// Row-per-event over a join: aggregate with an ungrouped running sum
		// keyed to the batch; plain symbol column binds the driving event.
		plan, planErr = env.Build(joined.Aggregate(
			esper.Alias("symbol", esper.JoinField[string](0, "symbol")),
			esper.Alias("sumPrice", esper.Sum[float64](esper.JoinField[float64](0, "price"))),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "time-batch-row-per-group-nojoin":
		plan, planErr = env.Build(md.GroupBy(symbol).Select(
			esper.Alias("symbol", symbol),
			esper.Alias("sumPrice", esper.Sum[float64](price)),
		).Query(
			esper.StatementName("s0"),
			esper.WithOldStream(),
			esper.OrderBy(esper.Ascending(esper.ResultField[string]("symbol"))),
		))
	case "time-batch-row-per-group-join":
		gSymbol := esper.JoinField[string](0, "symbol")
		gPrice := esper.JoinField[float64](0, "price")
		plan, planErr = env.Build(esper.Join(
			md,
			esper.From[wtbBean](env, "SupportBean").Window(esper.KeepAll()),
			espreOnEqualSymbol(),
		).GroupBy(gSymbol).Select(
			esper.Alias("symbol", gSymbol),
			esper.Alias("sumPrice", esper.Sum[float64](gPrice)),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "time-batch-aggr-grouped-nojoin":
		plan, planErr = env.Build(md.GroupBy(symbol).Select(
			esper.Alias("symbol", symbol),
			esper.Alias("sumPrice", esper.Sum[float64](price)),
			esper.Alias("volume", volume),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
	case "time-batch-aggr-grouped-join":
		gSymbol := esper.JoinField[string](0, "symbol")
		gPrice := esper.JoinField[float64](0, "price")
		gVolume := esper.JoinField[*int64](0, "volume")
		plan, planErr = env.Build(esper.Join(
			md,
			esper.From[wtbBean](env, "SupportBean").Window(esper.KeepAll()),
			espreOnEqualSymbol(),
		).GroupBy(gSymbol).Select(
			esper.Alias("symbol", gSymbol),
			esper.Alias("sumPrice", esper.Sum[float64](gPrice)),
			esper.Alias("volume", gVolume),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
	default:
		return trace, fmt.Errorf("unsupported case %q", caseName)
	}
	if planErr != nil {
		return trace, planErr
	}

	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return trace, err
	}
	statement := deployment.Statements()[0]
	seq := uint64(0)
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		newRows := compat.NormalizeResults(batch.New)
		oldRows := compat.NormalizeResults(batch.Old)
		if len(newRows) == 0 && len(oldRows) == 0 {
			return nil
		}
		seq++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: "s0",
			Sequence:  seq,
			Time:      batch.Time.UTC().Format(time.RFC3339),
			New:       newRows,
			Old:       oldRows,
		})
		return nil
	}); err != nil {
		return trace, err
	}

	for _, step := range scenario.Steps {
		switch step.Op {
		case "send":
			if err := wtbSendEvent(ctx, engine, step); err != nil {
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
		case "case":
			continue
		default:
			return trace, fmt.Errorf("unsupported step %q", step.Op)
		}
	}
	return trace, nil
}

func espreOnEqualSymbol() esper.JoinCondition {
	return esper.OnEqual(
		esper.JoinField[string](0, "symbol"),
		esper.JoinField[string](1, "theString"))
}

func wtbSendEvent(ctx context.Context, engine *esper.Engine, step compat.Step) error {
	switch step.EventType {
	case "SupportMarketDataBean":
		var payload struct {
			Symbol string  `json:"symbol"`
			Price  float64 `json:"price"`
			Volume *int64  `json:"volume"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		return engine.SendEvent(ctx, wtbMarketData{Symbol: payload.Symbol, Price: payload.Price, Volume: payload.Volume})
	case "SupportBean":
		var payload wtbBean
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		return engine.SendEvent(ctx, payload)
	}
	return fmt.Errorf("unknown event type %q", step.EventType)
}
