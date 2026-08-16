package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextInitTermInclusiveBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive *int64 `esper:"longPrimitive"`
}

type contextInitTermInclusiveS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
	P01 *string `esper:"p01"`
}

type contextInitTermInclusiveS1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
	P11 *string `esper:"p11"`
}

const contextInitTermInclusiveJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextInitTermInclusiveJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTerm.java",
}

var contextInitTermInclusiveJavaRuntimeIDs = []string{
	"java-runtime-b00a0be1e0e85ad8cb4e", // ContextInitTermPatternInclusion
	"java-runtime-adb023e3a25302b8dafb", // ContextInitTermFilterInitiatedStraightEquals
}

var contextInitTermInclusiveJavaExecutions = []string{
	"ContextInitTermPatternInclusion",
	"ContextInitTermFilterInitiatedStraightEquals",
}

// runContextInitTermInclusiveEqualsScenario replays the @Inclusive pattern
// start and straight-equality filter start executions of ContextInitTerm:
// an every-distinct 10-second pattern start whose match events are analyzed
// by the partition (with output last when terminated), the multi-event
// pattern start whose routed S0/S1 events complete an inner pattern, and an
// overlapping filter start with a correlated equality filter and one-minute
// duration end.
func runContextInitTermInclusiveEqualsScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextInitTermInclusiveEqualsCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-init-term-inclusive-equals case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextInitTermInclusiveEqualsCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextInitTermInclusiveBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermInclusiveS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermInclusiveS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextInitTermInclusiveBean](env, "SupportBean")
	s0Base := esper.From[contextInitTermInclusiveS0](env, "SupportBean_S0")
	s1Base := esper.From[contextInitTermInclusiveS1](env, "SupportBean_S1")
	theString := esper.Field[contextInitTermInclusiveBean, string]("theString")
	intPrimitive := esper.Field[contextInitTermInclusiveBean, int]("intPrimitive")

	var plan esper.Plan
	switch caseName {
	case "pattern-inclusion":
		start := esper.PatternFrom(beanSource, "a", esper.Literal(true)).
			EveryDistinctFor(10*time.Second, esper.Field[contextInitTermInclusiveBean, string]("theString"))
		end := esper.TimerInterval(beanSource, 10*time.Second)
		if _, err := esper.CreateOverlappingPatternInitiatedTerminatedContextInclusive(env, "CtxPerId", start, end); err != nil {
			return compat.Trace{}, err
		}
		query := esper.Select(beanSource.Filter(esper.Equal[string](theString, esper.ContextPatternField[string]("a", "theString"))),
			esper.Alias("theString", theString),
			esper.Alias("intPrimitive", intPrimitive),
			esper.Alias("longPrimitive", esper.Field[contextInitTermInclusiveBean, *int64]("longPrimitive")),
		).Query(esper.StatementName("s0"), esper.WithContext("CtxPerId"),
			esper.WithOutput(esper.OutputWhenTerminated(esper.OutputLast())))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "pattern-inclusion-multi":
		start := esper.PatternFrom(s0Base, "a", esper.Literal(true)).Then(esper.PatternFrom(s1Base, "b", esper.Literal(true))).Every()
		end := esper.TimerInterval(s0Base, 10*time.Second)
		if _, err := esper.CreateOverlappingPatternInitiatedTerminatedContextInclusive(env, "CtxPerId", start, end); err != nil {
			return compat.Trace{}, err
		}
		statementPattern := esper.PatternFrom(s0Base, "a", esper.Literal(true)).Then(esper.PatternFrom(s1Base, "b", esper.Literal(true))).Every()
		query := statementPattern.Select(
			esper.Alias("a_id", esper.TagField[int]("a", "id")),
			esper.Alias("a_p00", esper.TagField[*string]("a", "p00")),
			esper.Alias("a_p01", esper.TagField[*string]("a", "p01")),
			esper.Alias("b_id", esper.TagField[int]("b", "id")),
			esper.Alias("b_p10", esper.TagField[*string]("b", "p10")),
			esper.Alias("b_p11", esper.TagField[*string]("b", "p11")),
		).Query(esper.StatementName("s0"), esper.WithContext("CtxPerId"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "filter-straight-equals":
		startFilter := esper.And(
			esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean")),
			esper.Like(theString, esper.Literal("I%")),
		)
		if _, err := esper.CreateOverlappingPatternTerminatedContext(env, "EverySupportBean", esper.Literal("global"), startFilter,
			esper.TimerInterval(beanSource, time.Minute)); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Filter(esper.Equal[int](
			intPrimitive,
			esper.Property[int](esper.ContextInitiatingEvent(), "intPrimitive"),
		)).Aggregate(
			esper.Alias("c1", esper.Sum[int64](esper.Field[contextInitTermInclusiveBean, *int64]("longPrimitive"))),
		).Query(esper.StatementName("s0"), esper.WithContext("EverySupportBean"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported context-init-term-inclusive-equals case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextInitTermInclusivePayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-init-term-inclusive-equals statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextInitTermInclusivePayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextInitTermInclusiveBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextInitTermInclusiveS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value contextInitTermInclusiveS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported context-init-term-inclusive-equals event type %q", step.EventType)
	}
}
