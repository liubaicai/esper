package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextInitTermFilterOperatorsBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	IntBoxed     *int64 `esper:"intBoxed"`
	ShortBoxed   *int64 `esper:"shortBoxed"`
	LongBoxed    *int64 `esper:"longBoxed"`
}

type contextInitTermFilterOperatorsS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
	P01 string  `esper:"p01"`
}

type contextInitTermFilterOperatorsS1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
	P11 string  `esper:"p11"`
}

const contextInitTermFilterOperatorsJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextInitTermFilterOperatorsJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTerm.java",
}

var contextInitTermFilterOperatorsJavaRuntimeIDs = []string{
	"java-runtime-5c073601befa8b082832",
	"java-runtime-f0010ebfc433a45f3619",
	"java-runtime-621848b54b379207392b",
}

var contextInitTermFilterOperatorsJavaExecutions = []string{
	"ContextInitTermFilterAllOperators",
	"ContextInitTermFilterBooleanOperator",
	"ContextInitTermPatternInitiatedStraightSelect",
}

// runContextInitTermFilterOperatorsScenario replays the correlated-filter
// executions of ContextInitTerm: the 25-operator filter matrix
// (ContextInitTermFilterAllOperators, initiator id=10 compared against
// intBoxed/shortBoxed with every relational, in/between, is/is-not and
// null semantics), the arithmetic correlated filter
// (intPrimitive + context.sb.id = 5, ContextInitTermFilterBooleanOperator)
// and the OR-pattern start with unbound-tag projections
// (every (a=S0 or b=S1), ContextInitTermPatternInitiatedStraightSelect).
func runContextInitTermFilterOperatorsScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	var order []string
	for _, caseName := range scenarioCaseOrder(scenario) {
		order = append(order, caseName)
	}
	for _, caseName := range order {
		caseTrace, err := runContextInitTermFilterOperatorsCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-init-term-filter-operators case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func scenarioCaseOrder(scenario compat.Scenario) []string {
	var order []string
	for _, step := range scenario.Steps {
		if step.Op == "case" && step.Case != "" {
			order = append(order, step.Case)
		}
	}
	return order
}

func runContextInitTermFilterOperatorsCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextInitTermFilterOperatorsBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermFilterOperatorsS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermFilterOperatorsS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextInitTermFilterOperatorsBean](env, "SupportBean")
	s0Base := esper.From[contextInitTermFilterOperatorsS0](env, "SupportBean_S0")
	s1Base := esper.From[contextInitTermFilterOperatorsS1](env, "SupportBean_S1")
	isS0 := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0"))

	ctxID := func() esper.Expr {
		return esper.Property[*int64](esper.ContextInitiatingEvent(), "id")
	}
	duration := 10*24*time.Hour + 5*time.Hour + 2*time.Minute + time.Second + 11*time.Millisecond
	intBoxed := func() esper.Expr {
		return esper.Field[contextInitTermFilterOperatorsBean, *int64]("intBoxed")
	}
	shortBoxed := func() esper.Expr {
		return esper.Field[contextInitTermFilterOperatorsBean, *int64]("shortBoxed")
	}

	filterByOperator := func(name string) (esper.Expression[bool], error) {
		switch name {
		case "op-eq-intboxed-lhs":
			return esper.EqualOf(ctxID(), intBoxed()), nil
		case "op-eq-intboxed-rhs":
			return esper.EqualOf(intBoxed(), ctxID()), nil
		case "op-gt-intboxed":
			return esper.GreaterOf(ctxID(), intBoxed()), nil
		case "op-ge-intboxed":
			return esper.GreaterOrEqualOf(ctxID(), intBoxed()), nil
		case "op-lt-intboxed":
			return esper.LessOf(ctxID(), intBoxed()), nil
		case "op-le-intboxed":
			return esper.LessOrEqualOf(ctxID(), intBoxed()), nil
		case "op-lt-intboxed-rev":
			return esper.LessOf(intBoxed(), ctxID()), nil
		case "op-le-intboxed-rev":
			return esper.LessOrEqualOf(intBoxed(), ctxID()), nil
		case "op-gt-intboxed-rev":
			return esper.GreaterOf(intBoxed(), ctxID()), nil
		case "op-ge-intboxed-rev":
			return esper.GreaterOrEqualOf(intBoxed(), ctxID()), nil
		case "op-in-intboxed":
			return esper.InOf(intBoxed(), ctxID()), nil
		case "op-between-intboxed":
			return esper.BetweenOf(intBoxed(), ctxID(), ctxID()), nil
		case "op-ne-intboxed-lhs":
			return esper.NotEqualOf(ctxID(), intBoxed()), nil
		case "op-ne-intboxed-rhs":
			return esper.NotEqualOf(intBoxed(), ctxID()), nil
		case "op-notin-intboxed":
			return esper.NotInOf(intBoxed(), ctxID()), nil
		case "op-notbetween-intboxed":
			return esper.NotBetweenOf(intBoxed(), ctxID(), ctxID()), nil
		case "op-is-intboxed-lhs":
			return esper.Is(ctxID(), intBoxed()), nil
		case "op-is-intboxed-rhs":
			return esper.Is(intBoxed(), ctxID()), nil
		case "op-isnot-intboxed-lhs":
			return esper.IsNot(ctxID(), intBoxed()), nil
		case "op-isnot-intboxed-rhs":
			return esper.IsNot(intBoxed(), ctxID()), nil
		case "op-eq-short-lhs":
			return esper.EqualOf(ctxID(), shortBoxed()), nil
		case "op-eq-short-rhs":
			return esper.EqualOf(shortBoxed(), ctxID()), nil
		case "op-gt-short":
			return esper.GreaterOf(ctxID(), shortBoxed()), nil
		case "op-lt-short-rev":
			return esper.LessOf(shortBoxed(), ctxID()), nil
		case "op-in-short":
			return esper.InOf(shortBoxed(), ctxID()), nil
		default:
			return nil, fmt.Errorf("unsupported operator case %q", name)
		}
	}

	var plan esper.Plan
	switch caseName {
	case "op-boolean-arithmetic":
		if _, err := esper.CreateOverlappingPatternTerminatedContext(env, "EverySupportBean", esper.Literal("global"), isS0, esper.TimerInterval(beanSource, duration)); err != nil {
			return compat.Trace{}, err
		}
		sum := esper.Add[int](esper.Field[contextInitTermFilterOperatorsBean, int]("intPrimitive"), esper.Property[int](esper.ContextInitiatingEvent(), "id"))
		query := beanSource.Filter(esper.EqualOf(sum, esper.Literal(5))).Aggregate(
			esper.Alias("c0", esper.Field[contextInitTermFilterOperatorsBean, string]("theString")),
			esper.Alias("c1", esper.Field[contextInitTermFilterOperatorsBean, int]("intPrimitive")),
			esper.Alias("c2", esper.Property[*string](esper.ContextInitiatingEvent(), "p00")),
		).Query(esper.StatementName("s0"), esper.WithContext("EverySupportBean"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "op-pattern-straight-select":
		start := esper.PatternFrom(s0Base, "a", esper.Literal(true)).Or(esper.PatternFrom(s1Base, "b", esper.Literal(true))).Every()
		if _, err := esper.CreateOverlappingPatternInitiatedTerminatedContext(env, "EverySupportBean", start, esper.TimerInterval(beanSource, time.Minute)); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Aggregate(
			esper.Alias("c1", esper.ContextPatternField[*int64]("a", "id")),
			esper.Alias("c2", esper.ContextPatternField[*int64]("b", "id")),
			esper.Alias("c3", esper.Field[contextInitTermFilterOperatorsBean, string]("theString")),
		).Query(esper.StatementName("s0"), esper.WithContext("EverySupportBean"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	default:
		filter, err := filterByOperator(caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateOverlappingPatternTerminatedContext(env, "EverySupportBean", esper.Literal("global"), isS0, esper.TimerInterval(beanSource, duration)); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Filter(filter).Aggregate(
			esper.Alias("c0", esper.Field[contextInitTermFilterOperatorsBean, string]("theString")),
			esper.Alias("c1", esper.Field[contextInitTermFilterOperatorsBean, int]("intPrimitive")),
			esper.Alias("c2", esper.Property[*string](esper.ContextInitiatingEvent(), "p00")),
		).Query(esper.StatementName("s0"), esper.WithContext("EverySupportBean"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextInitTermFilterOperatorsPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-init-term-filter-operators statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextInitTermFilterOperatorsPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextInitTermFilterOperatorsBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextInitTermFilterOperatorsS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value contextInitTermFilterOperatorsS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported context-init-term-filter-operators event type %q", step.EventType)
	}
}
