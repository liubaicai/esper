package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextInitTermCorrelatedBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	IntBoxed     *int64 `esper:"intBoxed"`
	LongBoxed    *int64 `esper:"longBoxed"`
}

type contextInitTermCorrelatedS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
	P01 string  `esper:"p01"`
}

type contextInitTermCorrelatedS1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
	P11 string  `esper:"p11"`
}

const contextInitTermCorrelatedJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextInitTermCorrelatedJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTerm.java",
}

var contextInitTermCorrelatedJavaRuntimeIDs = []string{
	"java-runtime-9a99ccdac87623fb6c02",
	"java-runtime-4f7224de7f4a4a153c43",
	"java-runtime-d0776e587fee56bc29c3",
	"java-runtime-c950c1a80c0106288084",
}

var contextInitTermCorrelatedJavaExecutions = []string{
	"ContextStartEndPatternWithFilterCorrelatedWithAsName",
	"ContextStartEndPatternWithPatternCorrelatedWithAsName",
	"ContextStartEndFilterWithPatternCorrelatedWithAsName{soda=false}",
	"ContextStartEndFilterWithPatternCorrelatedWithAsName{soda=true}",
}

// runContextInitTermCorrelatedScenario replays the correlated as-name
// executions of ContextInitTerm: pattern-started contexts whose end
// condition (filter or pattern) references the initiating event through
// ContextInitiatingEvent, plus the filter-started context whose end is an
// OR of a correlated filter and a 30-second timer with output when
// terminated.
func runContextInitTermCorrelatedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range []string{
		"pattern-pattern-correlated",
		"pattern-filter-correlated",
		"filter-pattern-or-correlated",
	} {
		caseTrace, err := runContextInitTermCorrelatedCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-init-term-correlated case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextInitTermCorrelatedCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextInitTermCorrelatedBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermCorrelatedS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextInitTermCorrelatedS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}

	beanSource := esper.From[contextInitTermCorrelatedBean](env, "SupportBean")
	s0Base := esper.From[contextInitTermCorrelatedS0](env, "SupportBean_S0")
	s1Base := esper.From[contextInitTermCorrelatedS1](env, "SupportBean_S1")
	id := esper.Field[contextInitTermCorrelatedS1, int]("id")
	isS0 := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0"))
	isS1 := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S1"))

	var plan esper.Plan
	switch caseName {
	case "pattern-pattern-correlated":
		start := esper.PatternFrom(s0Base, "s0", esper.Literal(true))
		end := esper.PatternFrom(s1Base, "s1",
			esper.Equal[int](id, esper.Property[int](esper.ContextInitiatingEvent(), "id")))
		if _, err := esper.CreatePatternInitiatedTerminatedContext(env, "MyContext", start, end); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Aggregate(
			esper.Alias("c1", esper.ContextPatternField[int]("s0", "id")),
			esper.Alias("c2", esper.ContextPatternField[int]("s1", "id")),
		).Query(esper.StatementName("s0"), esper.WithContext("MyContext"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "pattern-filter-correlated":
		start := esper.PatternFrom(s0Base, "s0", esper.Literal(true))
		end := esper.And(isS1,
			esper.Equal[int](id, esper.Property[int](esper.ContextInitiatingEvent(), "id")))
		if _, err := esper.CreatePatternInitiatedTerminatedByFilterContext(env, "MyContext", start, end); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Query(esper.StatementName("s0"), esper.WithContext("MyContext"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "filter-pattern-or-correlated":
		end := esper.PatternFrom(s1Base, "s1",
			esper.Equal[int](id, esper.Property[int](esper.ContextInitiatingEvent(), "id")),
		).Or(esper.TimerInterval(s1Base, 30*time.Second))
		if _, err := esper.CreatePatternTerminatedContext(env, "MyContext", esper.Literal("global"), isS0, end); err != nil {
			return compat.Trace{}, err
		}
		query := s0Base.Aggregate(
			esper.Alias("c1", esper.Property[int](esper.ContextInitiatingEvent(), "id")),
			esper.Alias("c2", esper.ContextPatternField[int]("s1", "id")),
		).Query(esper.StatementName("s0"), esper.WithContext("MyContext"),
			esper.WithOutput(esper.OutputWhenTerminated()))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported context-init-term-correlated case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextInitTermCorrelatedPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-init-term-correlated statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextInitTermCorrelatedPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextInitTermCorrelatedBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextInitTermCorrelatedS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value contextInitTermCorrelatedS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported context-init-term-correlated event type %q", step.EventType)
	}
}
