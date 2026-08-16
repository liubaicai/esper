package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextStartEndBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	IntBoxed     *int64 `esper:"intBoxed"`
	ShortBoxed   *int64 `esper:"shortBoxed"`
	LongBoxed    *int64 `esper:"longBoxed"`
}

type contextStartEndS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
	P01 *string `esper:"p01"`
}

type contextStartEndS1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
	P11 string  `esper:"p11"`
}

type contextStartEndS2 struct {
	ID  int     `esper:"id"`
	P20 *string `esper:"p20"`
	P21 string  `esper:"p21"`
}

type contextStartEndS3 struct {
	ID  int     `esper:"id"`
	P30 *string `esper:"p30"`
	P31 string  `esper:"p31"`
}

const contextStartEndJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextStartEndJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTerm.java",
}

var contextStartEndJavaRuntimeIDs = []string{
	"java-runtime-9a99ccdac87623fb6c02",
	"java-runtime-0d946f8f7b3a0c9afdcd",
	"java-runtime-1da794026834900ba946",
}

var contextStartEndJavaExecutions = []string{
	"ContextStartEndPatternWithFilterCorrelatedWithAsName",
	"ContextStartEndPatternCorrelated",
	"ContextInitTermPatternCorrelated",
}

// runContextStartEndCorrelatedScenario replays the correlated
// start/end-boundary executions of ContextInitTerm: a pattern-start context
// with a correlated filter end (starter.s0.id), an OR-alternative start
// whose correlated OR end references the bound or unbound start tag, and an
// overlapping pattern-initiated context whose same-bean termination pattern
// correlates on the start tag's theString.
func runContextStartEndCorrelatedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextStartEndCorrelatedCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-start-end-correlated case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextStartEndCorrelatedCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextStartEndBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextStartEndS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextStartEndS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextStartEndS2](env, "SupportBean_S2"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextStartEndS3](env, "SupportBean_S3"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextStartEndBean](env, "SupportBean")
	s0Base := esper.From[contextStartEndS0](env, "SupportBean_S0")
	s1Base := esper.From[contextStartEndS1](env, "SupportBean_S1")
	s2Base := esper.From[contextStartEndS2](env, "SupportBean_S2")
	s3Base := esper.From[contextStartEndS3](env, "SupportBean_S3")

	var plan esper.Plan
	switch caseName {
	case "start-end-pattern-filter":
		start := esper.PatternFrom(s0Base, "s0", esper.Literal(true))
		end := esper.And(
			esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S1")),
			esper.Equal[int](
				esper.Field[contextStartEndS1, int]("id"),
				esper.TagField[int]("s0", "id")))
		if _, err := esper.CreatePatternInitiatedTerminatedByFilterContext(env, "MyContext", start, end); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Query(esper.StatementName("s0"), esper.WithContext("MyContext"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "start-end-or-correlated":
		start := esper.PatternFrom(s0Base, "a", esper.Literal(true)).Or(esper.PatternFrom(s1Base, "b", esper.Literal(true)))
		end := esper.PatternFrom(s2Base, "s2", esper.Equal[int](
			esper.Field[contextStartEndS2, int]("id"),
			esper.TagField[int]("a", "id"))).Or(esper.PatternFrom(s3Base, "s3", esper.Equal[int](
			esper.Field[contextStartEndS3, int]("id"),
			esper.TagField[int]("b", "id"))))
		if _, err := esper.CreatePatternInitiatedTerminatedContext(env, "MyContext", start, end); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Query(esper.StatementName("s0"), esper.WithContext("MyContext"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "init-term-pattern-correlated":
		intPrimitive := esper.Field[contextStartEndBean, int]("intPrimitive")
		theString := esper.Field[contextStartEndBean, string]("theString")
		start := esper.PatternFrom(beanSource, "a", esper.Equal[int](intPrimitive, esper.Literal(0))).Every()
		end := esper.PatternFrom(beanSource, "end", esper.And(
			esper.Equal[string](theString, esper.TagField[string]("a", "theString")),
			esper.Equal[int](intPrimitive, esper.Literal(1)),
		))
		if _, err := esper.CreateOverlappingPatternInitiatedTerminatedContext(env, "ACtx", start, end); err != nil {
			return compat.Trace{}, err
		}
		query := s0Base.Filter(esper.Equal[*string](
			esper.Field[contextStartEndS0, *string]("p00"),
			esper.ContextPatternField[*string]("a", "theString"),
		)).Query(esper.StatementName("s0"), esper.WithContext("ACtx"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported context-start-end-correlated case %q", caseName)
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
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextStartEndPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-start-end-correlated statement %q", name)
		}
		return statement, nil
	})
}

func decodeContextStartEndPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextStartEndBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextStartEndS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value contextStartEndS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	case "SupportBean_S2":
		var value contextStartEndS2
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S2: %w", err)
		}
		return value, nil
	case "SupportBean_S3":
		var value contextStartEndS3
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S3: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported context-start-end-correlated event type %q", step.EventType)
	}
}
