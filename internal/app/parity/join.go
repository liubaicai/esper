package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// joinOrderEvent is the typed OrderEvent payload shared with the Java oracle.
type joinOrderEvent struct {
	OrderID string  `esper:"orderId"`
	Price   float64 `esper:"price"`
}

// joinPaymentEvent is the typed PaymentEvent payload shared with the Java oracle.
type joinPaymentEvent struct {
	OrderID string  `esper:"orderId"`
	Amount  float64 `esper:"amount"`
}

const (
	joinLengthWindowCasePlain  = "plain"
	joinLengthWindowCaseEvery2 = "every-2"
)

const joinLengthWindowJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var joinLengthWindowJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/join/EPLJoinNoWhereClause.java",
}

var (
	joinLengthWindowJavaRuntimeIDs = []string{
		"java-runtime-4aa869a9f1905a65ab3a",
		"java-runtime-7885da47c76db1ebb5f7",
	}
	joinLengthWindowJavaExecutions = []string{
		"EPLJoinJoinWInnerKeywordWOOnClause",
		"EPLJoinJoinNoWhereClause",
	}
)

// runJoinLengthWindowScenario replays each case in its own statement and
// concatenates the normalized traces, matching the Java oracle's per-case
// runtime isolation.
func runJoinLengthWindowScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{joinLengthWindowCasePlain, joinLengthWindowCaseEvery2}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runJoinLengthWindowCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("join length window case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("join length window scenario %q has no supported cases", scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runJoinLengthWindowCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[joinOrderEvent](env, "OrderEvent"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[joinPaymentEvent](env, "PaymentEvent"); err != nil {
		return compat.Trace{}, err
	}
	orders := esper.From[joinOrderEvent](env, "OrderEvent").Window(esper.LengthWindow(3))
	payments := esper.From[joinPaymentEvent](env, "PaymentEvent").Window(esper.LengthWindow(3))
	query := esper.Join(
		orders,
		payments,
		esper.OnEqual(
			esper.Field[joinOrderEvent, string]("orderId"),
			esper.Field[joinPaymentEvent, string]("orderId"),
		),
	).Select(
		esper.SelectLeft("orderId", esper.Field[joinOrderEvent, string]("orderId")),
		esper.SelectRight("amount", esper.Field[joinPaymentEvent, float64]("amount")),
	)
	options := []esper.QueryOption{
		esper.StatementName("s0"),
		esper.OrderBy(
			esper.Ascending(esper.JoinField[string](0, "orderId")),
			esper.Ascending(esper.JoinField[float64](1, "amount")),
		),
	}
	if caseName == joinLengthWindowCaseEvery2 {
		options = append(options, esper.WithOutput(esper.OutputEvery(2)))
	}
	plan, err := env.Build(query.Query(options...))
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeJoinLengthWindowPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown join length window statement %q", name)
		}
		return statement, nil
	})
}

func decodeJoinLengthWindowPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "OrderEvent":
		var value joinOrderEvent
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode OrderEvent: %w", err)
		}
		return value, nil
	case "PaymentEvent":
		var value joinPaymentEvent
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode PaymentEvent: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported join length window event type %q", step.EventType)
	}
}
