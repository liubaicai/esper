package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextInitTermKeyedSummedEvent struct {
	Grp   string `esper:"grp"`
	Key   string `esper:"key"`
	Value int    `esper:"value"`
}

type contextInitTermKeyedInitEvent struct {
	Grp string `esper:"grp"`
}

type contextInitTermKeyedTermEvent struct {
	Grp string `esper:"grp"`
}

const contextInitTermKeyedJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextInitTermKeyedJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTerm.java",
}

var contextInitTermKeyedJavaRuntimeIDs = []string{
	"java-runtime-e1f8f03e39a2fb3b6bcb",
}

var contextInitTermKeyedJavaExecutions = []string{
	"ContextInitTermAggregationGrouped",
}

// runContextInitTermKeyedAggregationScenario replays the keyed
// initiated-terminated aggregation execution of ContextInitTerm: a context
// keyed by the initiating event's grp aggregates SummedEvent rows grouped
// by key inside each partition, with TermEvent terminating the matching
// partition and a later InitEvent starting a fresh partition.
func runContextInitTermKeyedAggregationScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	caseTrace, err := runContextInitTermKeyedAggregationCase(ctx, scenario, "keyed-aggregation")
	if err != nil {
		return compat.Trace{}, fmt.Errorf("context-init-term-keyed-aggregation: %w", err)
	}
	trace.Records = append(trace.Records, caseTrace.Records...)
	return trace, nil
}

func runContextInitTermKeyedAggregationCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextInitTermKeyedSummedEvent](env, "SummedEvent"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermKeyedInitEvent](env, "InitEvent"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermKeyedTermEvent](env, "TermEvent"); err != nil {
		return compat.Trace{}, err
	}

	summedBase := esper.From[contextInitTermKeyedSummedEvent](env, "SummedEvent")
	grp := esper.Field[contextInitTermKeyedSummedEvent, string]("grp")
	key := esper.Field[contextInitTermKeyedSummedEvent, string]("key")
	isInit := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("InitEvent"))
	isTerm := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("TermEvent"))
	startKey := esper.Field[contextInitTermKeyedInitEvent, string]("grp")
	end := esper.And(isTerm, esper.Equal[string](
		esper.Field[contextInitTermKeyedTermEvent, string]("grp"),
		esper.ContextKeyValue[string](0)))
	if _, err := esper.CreateInitiatedTerminatedContext(env, "MyContext", startKey, isInit, end); err != nil {
		return compat.Trace{}, err
	}
	query := summedBase.Filter(esper.Equal[string](grp, esper.Property[string](esper.ContextInitiatingEvent(), "grp"))).
		GroupBy(key).Select(
		esper.Alias("c0", key),
		esper.Alias("c1", esper.Sum[int](esper.Field[contextInitTermKeyedSummedEvent, int]("value"))),
	).Query(esper.StatementName("s0"), esper.WithContext("MyContext"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env)
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		_ = engine.Close(context.Background())
		return compat.Trace{}, err
	}
	if len(deployment.Statements()) != 1 {
		_ = engine.Close(context.Background())
		return compat.Trace{}, fmt.Errorf("expected one statement, got %d", len(deployment.Statements()))
	}
	statement := deployment.Statements()[0]
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextInitTermKeyedPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-init-term-keyed-aggregation statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextInitTermKeyedPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SummedEvent":
		var value contextInitTermKeyedSummedEvent
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SummedEvent: %w", err)
		}
		return value, nil
	case "InitEvent":
		var value contextInitTermKeyedInitEvent
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode InitEvent: %w", err)
		}
		return value, nil
	case "TermEvent":
		var value contextInitTermKeyedTermEvent
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode TermEvent: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported context-init-term-keyed-aggregation event type %q", step.EventType)
	}
}
