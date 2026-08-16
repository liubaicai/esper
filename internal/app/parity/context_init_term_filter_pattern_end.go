package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextInitTermFilterPatternBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	IntBoxed     *int64 `esper:"intBoxed"`
	LongBoxed    *int64 `esper:"longBoxed"`
}

type contextInitTermFilterPatternS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
	P01 string  `esper:"p01"`
}

type contextInitTermFilterPatternS1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
	P11 string  `esper:"p11"`
}

type contextInitTermFilterPatternS2 struct {
	ID  int     `esper:"id"`
	P20 *string `esper:"p20"`
	P21 string  `esper:"p21"`
}

const contextInitTermFilterPatternJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextInitTermFilterPatternJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTerm.java",
}

var contextInitTermFilterPatternJavaRuntimeIDs = []string{
	"java-runtime-f9e6506389cf2186cfdc",
	"java-runtime-c8a41b14ac56c64e99b0",
}

var contextInitTermFilterPatternJavaExecutions = []string{
	"ContextInitTermFilterAndPattern",
	"ContextInitTermPatternAndAfter1Min",
}

// runContextInitTermFilterPatternEndScenario replays the correlated
// pattern-terminated executions of ContextInitTerm: an overlapping
// filter-initiated context whose sequence end pattern
// (S0(p00=theString) -> S1(p10=theString)) terminates the matching
// partition, and a pattern-initiated context
// (every S0 -> S1(id=s0.id)) with one-minute duration termination and
// start-tag projections.
func runContextInitTermFilterPatternEndScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range []string{
		"filter-and-pattern",
		"pattern-and-after-1-min",
	} {
		caseTrace, err := runContextInitTermFilterPatternEndCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-init-term-filter-pattern-end case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextInitTermFilterPatternEndCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextInitTermFilterPatternBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermFilterPatternS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermFilterPatternS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermFilterPatternS2](env, "SupportBean_S2"); err != nil {
		return compat.Trace{}, err
	}

	beanSource := esper.From[contextInitTermFilterPatternBean](env, "SupportBean")
	s0Base := esper.From[contextInitTermFilterPatternS0](env, "SupportBean_S0")
	s1Base := esper.From[contextInitTermFilterPatternS1](env, "SupportBean_S1")
	s2Base := esper.From[contextInitTermFilterPatternS2](env, "SupportBean_S2")
	isBean := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean"))

	var plan esper.Plan
	switch caseName {
	case "filter-and-pattern":
		initiatingTheString := esper.Property[*string](esper.ContextInitiatingEvent(), "theString")
		end := esper.PatternFrom(s0Base, "s0",
			esper.Equal[*string](esper.Field[contextInitTermFilterPatternS0, *string]("p00"), initiatingTheString),
		).Then(esper.PatternFrom(s1Base, "s1",
			esper.Equal[*string](esper.Field[contextInitTermFilterPatternS1, *string]("p10"), initiatingTheString)))
		if _, err := esper.CreateOverlappingPatternTerminatedContext(env, "CtxInitiated", esper.Literal("global"), isBean, end); err != nil {
			return compat.Trace{}, err
		}
		query := s2Base.Filter(esper.Equal[*string](
			esper.Field[contextInitTermFilterPatternS2, *string]("p20"),
			esper.Property[*string](esper.ContextInitiatingEvent(), "theString"))).
			Aggregate(
				esper.Alias("id", esper.Field[contextInitTermFilterPatternS2, int]("id")),
			).Query(esper.StatementName("S1"), esper.WithContext("CtxInitiated"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "pattern-and-after-1-min":
		start := esper.PatternFrom(s0Base, "s0", esper.Literal(true)).Then(esper.PatternFrom(s1Base, "s1",
			esper.Equal[int](
				esper.Field[contextInitTermFilterPatternS1, int]("id"),
				esper.TagField[int]("s0", "id"))),
		).Every()
		end := esper.TimerInterval(beanSource, time.Minute)
		if _, err := esper.CreateOverlappingPatternInitiatedTerminatedContext(env, "CtxInitiated", start, end); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Aggregate(
			esper.Alias("c1", esper.Field[contextInitTermFilterPatternBean, string]("theString")),
			esper.Alias("c2", esper.Sum[int](esper.Field[contextInitTermFilterPatternBean, int]("intPrimitive"))),
			esper.Alias("c3", esper.ContextPatternField[*string]("s0", "p00")),
			esper.Alias("c4", esper.ContextPatternField[*string]("s1", "p10")),
		).Query(esper.StatementName("S1"), esper.WithContext("CtxInitiated"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported context-init-term-filter-pattern-end case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextInitTermFilterPatternPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-init-term-filter-pattern-end statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextInitTermFilterPatternPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextInitTermFilterPatternBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextInitTermFilterPatternS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value contextInitTermFilterPatternS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	case "SupportBean_S2":
		var value contextInitTermFilterPatternS2
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S2: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported context-init-term-filter-pattern-end event type %q", step.EventType)
	}
}
