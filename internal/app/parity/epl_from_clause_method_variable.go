package parity

import (
	"context"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for EPLFromClauseMethodVariable: from-clause method
// sources parameterized by variables — a constant-shaped service, a
// non-constant service re-sampled per trigger event with mid-stream on-set
// writes (both soda compile variants replay the same builder), per-partition
// context variables under an overlapping initiated context, and triggerless
// Map/ObjectArray handlers observed through the iterator. The suite's sixth
// execution (EPLFromClauseMethodVariableInvalid) is compile-only and covered
// by Go Build-rejection tests per the invalidity policy, not by this chain.
type fcmVariableBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type fcmVariableS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

type fcmVariableS1 struct {
	ID int `esper:"id"`
}

type fcmVariableS2 struct {
	ID  int    `esper:"id"`
	P20 string `esper:"p20"`
}

const eplFromClauseMethodVariableJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eplFromClauseMethodVariableJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/fromclausemethod/EPLFromClauseMethodVariable.java",
}

var eplFromClauseMethodVariableJavaRuntimeIDs = []string{
	"java-runtime-b6134319ddad78ee7888",
	"java-runtime-7fa38fe9688391d3f9da",
	"java-runtime-1c125ff69ef065d76608",
	"java-runtime-26c863d3cc9d5aca7cc7",
	"java-runtime-2989d0cd737e30ae5633",
}

var eplFromClauseMethodVariableJavaExecutions = []string{
	"EPLFromClauseMethodConstantVariable",
	"EPLFromClauseMethodNonConstantVariable{soda=true}",
	"EPLFromClauseMethodNonConstantVariable{soda=false}",
	"EPLFromClauseMethodContextVariable",
	"EPLFromClauseMethodVariableMapAndOA",
}

var eplFromClauseMethodVariableCases = []string{
	"constant-variable",
	"nonconstant-soda-true",
	"nonconstant-soda-false",
	"context-variable",
	"map-and-oa",
}

func runEplFromClauseMethodVariableScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseName := range eplFromClauseMethodVariableCases {
		caseTrace, err := runEplFromClauseMethodVariableCase(ctx, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("epl-from-clause-method-variable case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func eplFromClauseMethodVariableEngine(ctx context.Context, caseIndex int) (*esper.Environment, *esper.Engine, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[fcmVariableBean](env, "SupportBean"); err != nil {
		return nil, nil, err
	}
	if _, err := esper.RegisterStruct[fcmVariableS0](env, "SupportBean_S0"); err != nil {
		return nil, nil, err
	}
	if _, err := esper.RegisterStruct[fcmVariableS1](env, "SupportBean_S1"); err != nil {
		return nil, nil, err
	}
	if _, err := esper.RegisterStruct[fcmVariableS2](env, "SupportBean_S2"); err != nil {
		return nil, nil, err
	}
	return env, esper.NewEngine(env,
		esper.WithRuntimeURI(eplFromClauseMethodVariableJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	), nil
}

func eplFromClauseMethodVariableFetchSchema(env *esper.Environment) (esper.Schema, error) {
	return esper.NewMapSchema("FCMFetchABeanRow", []esper.FieldSpec{
		esper.FieldDef("id", reflect.TypeOf("")),
	})
}

// eplFromClauseMethodVariableProvider mirrors fetchABean(intPrimitive): the
// rendered id is "_" + intPrimitive + "_" + postfix. The constant service
// passes an empty postfix variable name; the non-constant and context
// services read the named variable's current value from the per-invocation
// snapshot.
func eplFromClauseMethodVariableProvider(schema esper.Schema, postfixVar string) esper.MethodProvider {
	return esper.MethodProviderFunc(func(_ context.Context, request esper.MethodRequest) ([]esper.Event, error) {
		number, _ := request.Trigger.Get("intPrimitive").Any().(int)
		postfix := ""
		if postfixVar != "" {
			if value, ok := request.Variables[postfixVar]; ok && value.Any() != nil {
				postfix, _ = value.Any().(string)
			}
		}
		event, err := esper.NewEvent(schema, map[string]any{
			"id": fmt.Sprintf("_%d_%s", number, postfix),
		}, request.Now)
		if err != nil {
			return nil, err
		}
		return []esper.Event{event}, nil
	})
}

func eplFromClauseMethodVariableJoinQuery(env *esper.Environment, schema esper.Schema, provider esper.MethodProvider, options ...esper.QueryOption) esper.Query {
	stream := esper.From[fcmVariableBean](env, "SupportBean")
	method := esper.FromMethod[map[string]any](env, "h0", schema, provider)
	options = append([]esper.QueryOption{esper.StatementName("s0")}, options...)
	return esper.Join(stream, method).
		Select(esper.SelectRight("c0", esper.Field[map[string]any, string]("id"))).
		Query(options...)
}

func runEplFromClauseMethodVariableCase(ctx context.Context, caseName string) ([]compat.TraceRecord, error) {
	caseIndex := -1
	for index, name := range eplFromClauseMethodVariableCases {
		if name == caseName {
			caseIndex = index
			break
		}
	}
	env, engine, err := eplFromClauseMethodVariableEngine(ctx, caseIndex)
	if err != nil {
		return nil, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := uint64(0)
	var records []compat.TraceRecord
	record := func(operation string, batch esper.ResultBatch) {
		sequence++
		records = append(records, compat.TraceRecord{
			Case:      caseName,
			Operation: operation,
			Statement: "s0",
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
		})
	}
	subscribe := func(deployment *esper.Deployment) error {
		_, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			record("listener", batch)
			return nil
		})
		return err
	}

	switch caseIndex {
	case 0: // constant-variable
		schema, err := eplFromClauseMethodVariableFetchSchema(env)
		if err != nil {
			return nil, err
		}
		plan, err := env.Build(eplFromClauseMethodVariableJoinQuery(env, schema, eplFromClauseMethodVariableProvider(schema, "")))
		if err != nil {
			return nil, err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return nil, err
		}
		if err := subscribe(deployment); err != nil {
			return nil, err
		}
		for _, primitive := range []int{10, 20} {
			if err := engine.Send(ctx, "SupportBean", fcmVariableBean{TheString: fmt.Sprintf("E%d", primitive/10), IntPrimitive: primitive}); err != nil {
				return nil, err
			}
		}
	case 1, 2: // nonconstant-soda-true / nonconstant-soda-false
		if err := env.RegisterVariable("postfix", "postfix"); err != nil {
			return nil, err
		}
		setter, err := env.Build(esper.OnEvent(esper.From[fcmVariableS0](env, "SupportBean_S0")).
			SetVariable("postfix", esper.Field[fcmVariableS0, string]("p00")).
			Query(esper.StatementName("postfix-set")))
		if err != nil {
			return nil, err
		}
		if _, err := engine.Deploy(ctx, setter); err != nil {
			return nil, err
		}
		schema, err := eplFromClauseMethodVariableFetchSchema(env)
		if err != nil {
			return nil, err
		}
		plan, err := env.Build(eplFromClauseMethodVariableJoinQuery(env, schema, eplFromClauseMethodVariableProvider(schema, "postfix")))
		if err != nil {
			return nil, err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return nil, err
		}
		if err := subscribe(deployment); err != nil {
			return nil, err
		}
		sends := []func() error{
			func() error {
				return engine.Send(ctx, "SupportBean", fcmVariableBean{TheString: "E1", IntPrimitive: 10})
			},
			func() error { return engine.Send(ctx, "SupportBean_S0", fcmVariableS0{ID: 1, P00: "newpostfix"}) },
			func() error {
				return engine.Send(ctx, "SupportBean", fcmVariableBean{TheString: "E1", IntPrimitive: 20})
			},
			func() error { return engine.Send(ctx, "SupportBean_S0", fcmVariableS0{ID: 2, P00: "postfix"}) },
			func() error {
				return engine.Send(ctx, "SupportBean", fcmVariableBean{TheString: "E1", IntPrimitive: 30})
			},
		}
		for _, send := range sends {
			if err := send(); err != nil {
				return nil, err
			}
		}
	case 3: // context-variable
		start := esper.PatternFrom(esper.From[fcmVariableS0](env, "SupportBean_S0"), "c_s0", esper.Literal(true))
		end := esper.PatternFrom(esper.From[fcmVariableS1](env, "SupportBean_S1"), "c_s1",
			esper.Equal[int](esper.Field[fcmVariableS1, int]("id"), esper.TagField[int]("c_s0", "id")))
		if _, err := esper.CreateOverlappingPatternInitiatedTerminatedContext(env, "MyContext", start, end); err != nil {
			return nil, err
		}
		if err := env.RegisterContextVariable("MyContext", "postfix", "context_postfix"); err != nil {
			return nil, err
		}
		schema, err := eplFromClauseMethodVariableFetchSchema(env)
		if err != nil {
			return nil, err
		}
		sb := esper.From[fcmVariableBean](env, "SupportBean").
			Filter(esper.Equal[int](esper.Field[fcmVariableBean, int]("intPrimitive"), esper.ContextPatternField[int]("c_s0", "id")))
		method := esper.FromMethod[map[string]any](env, "h0", schema, eplFromClauseMethodVariableProvider(schema, "postfix"))
		joinPlan, err := env.Build(esper.Join(sb, method).
			Select(esper.SelectRight("c0", esper.Field[map[string]any, string]("id"))).
			Query(esper.StatementName("s0"), esper.WithContext("MyContext")))
		if err != nil {
			return nil, err
		}
		setterPlan, err := env.Build(esper.OnEvent(esper.From[fcmVariableS2](env, "SupportBean_S2").
			Filter(esper.Equal[int](esper.Field[fcmVariableS2, int]("id"), esper.ContextPatternField[int]("c_s0", "id")))).
			SetVariable("postfix", esper.Field[fcmVariableS2, string]("p20")).
			Query(esper.StatementName("s2-set"), esper.WithContext("MyContext")))
		if err != nil {
			return nil, err
		}
		if _, err := engine.Deploy(ctx, setterPlan); err != nil {
			return nil, err
		}
		deployment, err := engine.Deploy(ctx, joinPlan)
		if err != nil {
			return nil, err
		}
		if err := subscribe(deployment); err != nil {
			return nil, err
		}
		sends := []func() error{
			func() error { return engine.Send(ctx, "SupportBean_S0", fcmVariableS0{ID: 1}) },
			func() error { return engine.Send(ctx, "SupportBean_S0", fcmVariableS0{ID: 2}) },
			func() error {
				return engine.Send(ctx, "SupportBean", fcmVariableBean{TheString: "E1", IntPrimitive: 1})
			},
			func() error {
				return engine.Send(ctx, "SupportBean", fcmVariableBean{TheString: "E2", IntPrimitive: 2})
			},
			func() error { return engine.Send(ctx, "SupportBean_S2", fcmVariableS2{ID: 1, P20: "a"}) },
			func() error { return engine.Send(ctx, "SupportBean_S2", fcmVariableS2{ID: 2, P20: "b"}) },
			func() error {
				return engine.Send(ctx, "SupportBean", fcmVariableBean{TheString: "E1", IntPrimitive: 1})
			},
			func() error {
				return engine.Send(ctx, "SupportBean", fcmVariableBean{TheString: "E2", IntPrimitive: 2})
			},
		}
		for _, send := range sends {
			if err := send(); err != nil {
				return nil, err
			}
		}
	case 4: // map-and-oa — triggerless handler sources, iterator observation
		for _, variant := range []string{"map", "oa"} {
			schema, err := esper.NewMapSchema("FCMHandlerRow-"+variant, []esper.FieldSpec{
				esper.FieldDef("field1", reflect.TypeOf("")),
				esper.FieldDef("field2", reflect.TypeOf("")),
			})
			if err != nil {
				return nil, err
			}
			provider := esper.MethodProviderFunc(func(_ context.Context, request esper.MethodRequest) ([]esper.Event, error) {
				event, err := esper.NewEvent(schema, map[string]any{"field1": "a", "field2": "b"}, request.Now)
				if err != nil {
					return nil, err
				}
				return []esper.Event{event}, nil
			})
			plan, err := env.Build(esper.Select(
				esper.FromMethod[map[string]any](env, "h0", schema, provider),
				esper.Alias("field1", esper.Field[map[string]any, string]("field1")),
				esper.Alias("field2", esper.Field[map[string]any, string]("field2")),
			).Query(esper.StatementName("s0")))
			if err != nil {
				return nil, err
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return nil, err
			}
			snapshot, err := deployment.Statements()[0].Snapshot(ctx)
			if err != nil {
				return nil, err
			}
			rows := snapshot.Results()
			sequence++
			records = append(records, compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: "s0",
				Sequence:  sequence,
				Time:      compat.FormatTraceTime(time.Unix(0, 0).UTC()),
				New:       compat.NormalizeResults(rows),
			})
			if err := deployment.Undeploy(ctx); err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("unsupported epl-from-clause-method-variable case %q", caseName)
	}
	return records, nil
}
