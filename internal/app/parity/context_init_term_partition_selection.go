package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextInitTermPartitionSelectionBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type contextInitTermPartitionSelectionS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
	P01 string  `esper:"p01"`
}

type contextInitTermPartitionSelectionS1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
	P11 string  `esper:"p11"`
}

const contextInitTermPartitionSelectionJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextInitTermPartitionSelectionJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTerm.java",
}

var contextInitTermPartitionSelectionJavaRuntimeIDs = []string{
	"java-runtime-128472d2f525e2c4a199", // ContextInitTermContextPartitionSelection
}

var contextInitTermPartitionSelectionJavaExecutions = []string{
	"ContextInitTermContextPartitionSelection",
}

// runContextInitTermPartitionSelectionScenario replays the iterator-selector
// execution of ContextInitTerm: an overlapping filter-initiated context
// terminated by a correlated id filter, a grouped keepall aggregate reading
// the context id and initiating event property, iterator snapshots by
// partition ID and by an initiating-event filtered selector, an always-false
// filtered selector, and an invalid segmented selector rejected with the
// InvalidContextPartitionSelector category.
func runContextInitTermPartitionSelectionScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextInitTermPartitionSelectionCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-init-term-partition-selection case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextInitTermPartitionSelectionCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextInitTermPartitionSelectionBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermPartitionSelectionS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermPartitionSelectionS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextInitTermPartitionSelectionBean](env, "SupportBean")
	isS0 := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0"))
	end := esper.And(
		esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S1")),
		esper.Equal[int](
			esper.Field[contextInitTermPartitionSelectionS1, int]("id"),
			esper.Property[int](esper.ContextInitiatingEvent(), "id")))
	if _, err := esper.CreateOverlappingInitiatedTerminatedContext(env, "MyCtx", esper.Literal("global"), isS0, end); err != nil {
		return compat.Trace{}, err
	}
	query := beanSource.Window(esper.KeepAll()).GroupBy(
		esper.Field[contextInitTermPartitionSelectionBean, string]("theString"),
	).Select(
		esper.Alias("c0", esper.ContextField[int]("id")),
		esper.Alias("c1", esper.Property[string](esper.ContextInitiatingEvent(), "p00")),
		esper.Alias("c2", esper.Field[contextInitTermPartitionSelectionBean, string]("theString")),
		esper.Alias("c3", esper.Sum[int](esper.Field[contextInitTermPartitionSelectionBean, int]("intPrimitive"))),
	).Query(esper.StatementName("s0"), esper.WithContext("MyCtx"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env)
	initial, ok := initialAdvanceTime(scenario, caseName)
	if ok {
		initialTime, err := time.Parse(time.RFC3339Nano, initial)
		if err != nil {
			_ = engine.Close(context.Background())
			return compat.Trace{}, err
		}
		if err := engine.AdvanceTime(ctx, initialTime); err != nil {
			_ = engine.Close(context.Background())
			return compat.Trace{}, err
		}
	}
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextInitTermPartitionSelectionPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-init-term-partition-selection statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextInitTermPartitionSelectionPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextInitTermPartitionSelectionBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextInitTermPartitionSelectionS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value contextInitTermPartitionSelectionS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported context-init-term-partition-selection event type %q", step.EventType)
	}
}
