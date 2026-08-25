package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type obaggMarketData struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
}

type obaggBeanString struct {
	TheString string `esper:"theString"`
}

type rfaSupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type rfaSupportA struct {
	ID string `esper:"id"`
}

var resultSetOrderByRowForAllJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderByRowForAll.java",
}

var resultSetOrderByRowForAllJavaRuntimeIDs = []string{
	"java-runtime-e6f5c075be4979efc531", // ResultSetNoOutputRateJoin
	"java-runtime-7642ad83057714f39501", // ResultSetOutputDefault{join=false}
	"java-runtime-046ed3b9a90cf000c5c7", // ResultSetOutputDefault{join=true}
}

var resultSetOrderByRowForAllJavaExecutions = []string{
	"ResultSetNoOutputRateJoin",
	"ResultSetOutputDefault{join=false}",
	"ResultSetOutputDefault{join=true}",
}

func runResultSetOrderByRowForAllScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range []string{"no-output-rate-join", "output-default-no-join", "output-default-join"} {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultSetOrderByRowForAllCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("scenario has no supported cases")
	}
	return trace, nil
}

func runResultSetOrderByRowForAllCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	for _, r := range []struct {
		name string
		fn   func() error
	}{
		{"MD", func() error { _, e := esper.RegisterStruct[obaggMarketData](env, "SupportMarketDataBean"); return e }},
		{"SBS", func() error { _, e := esper.RegisterStruct[obaggBeanString](env, "SupportBeanString"); return e }},
		{"SB", func() error { _, e := esper.RegisterStruct[rfaSupportBean](env, "SupportBean"); return e }},
		{"SA", func() error { _, e := esper.RegisterStruct[rfaSupportA](env, "SupportBean_A"); return e }},
	} {
		if err := r.fn(); err != nil {
			return compat.Trace{}, fmt.Errorf("register %s: %w", r.name, err)
		}
	}

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
	case "no-output-rate-join":
		mdStream := esper.From[obaggMarketData](env, "SupportMarketDataBean").Window(esper.LengthWindow(10))
		strStream := esper.From[obaggBeanString](env, "SupportBeanString").Window(esper.LengthWindow(100))
		joined := esper.Join(mdStream, strStream,
			esper.OnEqual(
				esper.Field[obaggMarketData, string]("symbol"),
				esper.Field[obaggBeanString, string]("theString")))
		jSum := esper.Sum[float64](esper.JoinField[float64](0, "price"))
		q := joined.Aggregate(esper.Alias("sumPrice", jSum)).
			Query(esper.StatementName("s0"), esper.OrderBy(esper.Ascending(esper.JoinField[float64](0, "price"))))
		if err := addPlan("s0", false, q); err != nil {
			return compat.Trace{}, err
		}

	case "output-default-no-join":
		intPrim := esper.Field[rfaSupportBean, int]("intPrimitive")
		theStr := esper.Field[rfaSupportBean, string]("theString")
		sumP := esper.Sum[int](intPrim)
		lastS := esper.Last[string](theStr)
		q := esper.From[rfaSupportBean](env, "SupportBean").Window(esper.LengthWindow(2)).
			Aggregate(esper.Alias("c0", sumP), esper.Alias("c1", lastS)).
			Query(append([]esper.QueryOption{esper.StatementName("s0"), esper.WithOldStream()},
				esper.WithOutput(esper.OutputEvery(3)),
				esper.OrderBy(esper.Descending(sumP)))...)
		if err := addPlan("s0", true, q); err != nil {
			return compat.Trace{}, err
		}

	case "output-default-join":
		intPrim := esper.JoinField[int](0, "intPrimitive")
		theStr := esper.JoinField[string](0, "theString")
		sumP := esper.Sum[int](intPrim)
		lastS := esper.Last[string](theStr)
		sbStream := esper.From[rfaSupportBean](env, "SupportBean").Window(esper.LengthWindow(2))
		aStream := esper.From[rfaSupportA](env, "SupportBean_A").Window(esper.KeepAll())
		cross := esper.Join(sbStream, aStream)
		q := cross.Aggregate(esper.Alias("c0", sumP), esper.Alias("c1", lastS)).
			Query(append([]esper.QueryOption{esper.StatementName("s0"), esper.WithOldStream()},
				esper.WithOutput(esper.OutputEvery(3)),
				esper.OrderBy(esper.Descending(sumP)))...)
		if err := addPlan("s0", true, q); err != nil {
			return compat.Trace{}, err
		}
	}

	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	statements := map[string]*esper.Statement{}
	seq := uint64(0)

	for _, item := range plans {
		deployment, deployErr := engine.Deploy(ctx, item.plan)
		if deployErr != nil {
			return trace, deployErr
		}
		for _, statement := range deployment.Statements() {
			statements[statement.Name()] = statement
		}
		if !item.listener {
			continue
		}
		for _, statement := range deployment.Statements() {
			captured := statement
			if _, subErr := captured.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				newRows := compat.NormalizeResults(batch.New)
				oldRows := compat.NormalizeResults(batch.Old)
				if len(newRows) == 0 && len(oldRows) == 0 {
					return nil
				}
				seq++
				trace.Records = append(trace.Records, compat.TraceRecord{
					Case: caseName, Operation: "listener", Statement: captured.Name(),
					Sequence: seq, Time: batch.Time.UTC().Format(time.RFC3339),
					New: newRows, Old: oldRows,
				})
				return nil
			}); subErr != nil {
				return trace, subErr
			}
		}
	}

	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			switch step.EventType {
			case "SupportMarketDataBean":
				var pl struct {
					Symbol string  `json:"symbol"`
					Price  float64 `json:"price"`
				}
				if err := json.Unmarshal(step.Payload, &pl); err != nil {
					return trace, err
				}
				ev := obaggMarketData{Symbol: pl.Symbol, Price: pl.Price}
				if sendErr := engine.Send(ctx, step.EventType, ev); sendErr != nil {
					return trace, sendErr
				}
			case "SupportBean":
				var pl struct {
					TheString    *string `json:"theString"`
					IntPrimitive int     `json:"intPrimitive"`
				}
				if err := json.Unmarshal(step.Payload, &pl); err != nil {
					return trace, err
				}
				ev := rfaSupportBean{IntPrimitive: pl.IntPrimitive}
				if pl.TheString != nil {
					ev.TheString = *pl.TheString
				}
				if sendErr := engine.Send(ctx, step.EventType, ev); sendErr != nil {
					return trace, sendErr
				}
			case "SupportBeanString":
				var pl struct {
					TheString string `json:"theString"`
				}
				if err := json.Unmarshal(step.Payload, &pl); err != nil {
					return trace, err
				}
				if sendErr := engine.Send(ctx, step.EventType, obaggBeanString{TheString: pl.TheString}); sendErr != nil {
					return trace, sendErr
				}
			case "SupportBean_A":
				var pl struct {
					ID string `json:"id"`
				}
				if err := json.Unmarshal(step.Payload, &pl); err != nil {
					return trace, err
				}
				if sendErr := engine.Send(ctx, step.EventType, rfaSupportA{ID: pl.ID}); sendErr != nil {
					return trace, sendErr
				}
			default:
				return trace, fmt.Errorf("unsupported event type %q", step.EventType)
			}
		case "snapshot":
			statement, ok := statements[step.Statement]
			if !ok {
				return trace, fmt.Errorf("unknown snapshot %q", step.Statement)
			}
			snapshot, snapErr := statement.Snapshot(ctx)
			if snapErr != nil {
				return trace, snapErr
			}
			rows := compat.NormalizeResults(snapshot.Results())
			sort.SliceStable(rows, func(left, right int) bool {
				leftJSON, _ := json.Marshal(rows[left].Fields)
				rightJSON, _ := json.Marshal(rows[right].Fields)
				return string(leftJSON) < string(rightJSON)
			})
			seq++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case: caseName, Operation: "snapshot", Statement: step.Statement,
				Time: "1970-01-01T00:00:00Z", Sequence: seq, New: rows,
			})
		default:
			return trace, fmt.Errorf("unsupported op %q", step.Op)
		}
	}
	return trace, nil
}
