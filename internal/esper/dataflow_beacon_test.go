package esper

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestDataflowBeaconOptionsIterationsAndFinalMarkerMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	definition, err := DefineDataflow(env, "beacon-options-flow").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{
			Iterations:   3,
			InitialDelay: time.Millisecond,
			Interval:     time.Millisecond,
		}, "A", "B").
		Emitter("sink").
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if instance.State() != DataflowComplete {
		t.Fatalf("beacon finite state = %v, want complete", instance.State())
	}
	if got, want := instance.Outputs(), []any{"A", "B", "A"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("beacon finite outputs = %#v, want %#v", got, want)
	}
	if stats := instance.Stats(); stats.Processed != 3 || stats.Emitted != 3 {
		t.Fatalf("beacon finite stats = %#v", stats)
	}
	if signals := instance.Signals(); len(signals) != 1 || !isDataflowFinalMarker(signals[0]) {
		t.Fatalf("beacon final marker signals = %#v", signals)
	}
}

func TestDataflowBeaconFactoryReceivesStableIterationContext(t *testing.T) {
	env := NewEnvironment()
	contexts := make([]DataflowBeaconContext, 0, 3)
	definition, err := DefineDataflow(env, "beacon-factory-flow").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{
			Iterations: 3,
			Factory: func(_ context.Context, beacon DataflowBeaconContext) (any, error) {
				contexts = append(contexts, beacon)
				return fmt.Sprintf("%s-%d", beacon.OperatorName, beacon.Iteration), nil
			},
		}).
		Emitter("sink").
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflowWithOptions(context.Background(), definition, DataflowOptions{InstanceID: "beacon-instance"})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(contexts) != 3 {
		t.Fatalf("beacon factory contexts = %#v, want three calls", contexts)
	}
	for index, beacon := range contexts {
		if beacon.DataflowName != "beacon-factory-flow" || beacon.InstanceID != "beacon-instance" || beacon.OperatorName != "source" || beacon.Iteration != index || beacon.Now.IsZero() {
			t.Fatalf("beacon factory context[%d] = %#v", index, beacon)
		}
	}
	if got, want := instance.Outputs(), []any{"source-0", "source-1", "source-2"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("beacon factory outputs = %#v, want %#v", got, want)
	}
}

func TestDataflowBeaconWithoutIterationLimitRunsUntilCanceled(t *testing.T) {
	env := NewEnvironment()
	var calls atomic.Int32
	ready := make(chan struct{})
	var readyOnce atomic.Bool
	definition, err := DefineDataflow(env, "beacon-unlimited-flow").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{
			Interval: time.Millisecond,
			Factory: func(_ context.Context, beacon DataflowBeaconContext) (any, error) {
				count := calls.Add(1)
				if count >= 3 && readyOnce.CompareAndSwap(false, true) {
					close(ready)
				}
				return beacon.Iteration, nil
			},
		}).
		Emitter("sink").
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("unlimited beacon did not emit three values")
	}
	if instance.State() != DataflowRunning {
		t.Fatalf("unlimited beacon state = %v, want running", instance.State())
	}
	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := instance.Join(context.Background()); err == nil || !errors.Is(err, ErrorCanceled) {
		t.Fatalf("unlimited beacon join error = %v, want cancellation", err)
	}
	if instance.State() != DataflowCanceled {
		t.Fatalf("unlimited beacon canceled state = %v", instance.State())
	}
}

func TestDataflowBeaconOptionsRejectNegativeTiming(t *testing.T) {
	env := NewEnvironment()
	if _, err := DefineDataflow(env, "invalid-beacon-options").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{Iterations: -1}, "value").
		Build(); err == nil {
		t.Fatal("negative beacon iterations were accepted")
	}
	if _, err := DefineDataflow(env, "invalid-beacon-delay").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{InitialDelay: -time.Second}, "value").
		Build(); err == nil {
		t.Fatal("negative beacon initial delay was accepted")
	}
}

func TestDataflowBeaconTimingExpressionsUseInstantiationSnapshot(t *testing.T) {
	env := NewEnvironment()
	if err := env.RegisterVariable("beacon_initial", time.Duration(0)); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("beacon_interval", time.Duration(0)); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "beacon-timing-expressions").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{
			Iterations:             2,
			InitialDelayExpression: VariableRef[time.Duration]("beacon_initial"),
			IntervalExpression:     VariableRef[time.Duration]("beacon_interval"),
		}, "value").
		Emitter("sink").
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SetVariable(context.Background(), "beacon_initial", time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := engine.SetVariable(context.Background(), "beacon_interval", time.Hour); err != nil {
		t.Fatal(err)
	}
	resolved := instance.operators["source"].BeaconOptions
	if resolved.InitialDelay != 0 || resolved.Interval != 0 {
		t.Fatalf("resolved beacon timing = initial %s interval %s", resolved.InitialDelay, resolved.Interval)
	}
	runContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := instance.Run(runContext); err != nil {
		t.Fatal(err)
	}
	if outputs := instance.Outputs(); len(outputs) != 2 {
		t.Fatalf("timing expression outputs = %#v", outputs)
	}
}

func TestDataflowBeaconTimingParameterProviderOverridesDefaults(t *testing.T) {
	env := NewEnvironment()
	definition, err := DefineDataflow(env, "beacon-timing-provider").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{
			Iterations:   5,
			InitialDelay: time.Hour,
			Interval:     time.Hour,
		}, "value").
		Emitter("sink").
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	var contexts []DataflowParameterContext
	instance, err := NewEngine(env).InstantiateDataflowWithOptions(context.Background(), definition, DataflowOptions{
		ParameterProvider: func(parameter DataflowParameterContext) (any, bool) {
			contexts = append(contexts, parameter)
			switch parameter.ParameterName {
			case DataflowBeaconIterationsParameter:
				return int64(2), true
			case DataflowBeaconInitialDelayParameter, DataflowBeaconIntervalParameter:
				return time.Duration(0), true
			default:
				return nil, false
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if outputs := instance.Outputs(); len(outputs) != 2 {
		t.Fatalf("timing provider outputs = %#v", outputs)
	}
	if len(contexts) != 3 {
		t.Fatalf("timing provider contexts = %#v", contexts)
	}
	wantNames := []string{"initialDelay", "interval", "iterations"}
	for index, parameter := range contexts {
		if parameter.ParameterName != wantNames[index] || parameter.Factory.Kind != BeaconSourceKind || !parameter.Factory.IsBuiltin() {
			t.Fatalf("timing provider context[%d] = %#v", index, parameter)
		}
	}
	if contexts[0].DefaultValue != time.Hour || contexts[1].DefaultValue != time.Hour || contexts[2].DefaultValue != 5 {
		t.Fatalf("timing provider defaults = %#v", contexts)
	}
}

func TestDataflowBeaconTimingParameterProviderPrecedesExpressions(t *testing.T) {
	env := NewEnvironment()
	if err := env.RegisterVariable("beacon_iterations", -1); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("beacon_delay", -time.Second); err != nil {
		t.Fatal(err)
	}
	definition, err := DefineDataflow(env, "beacon-timing-provider-precedence").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{
			IterationsExpression:   VariableRef[int]("beacon_iterations"),
			InitialDelayExpression: VariableRef[time.Duration]("beacon_delay"),
			IntervalExpression:     VariableRef[time.Duration]("beacon_delay"),
		}, "value").
		Emitter("sink").
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflowWithOptions(context.Background(), definition, DataflowOptions{
		ParameterProvider: func(parameter DataflowParameterContext) (any, bool) {
			switch parameter.ParameterName {
			case DataflowBeaconIterationsParameter:
				return 1, true
			case DataflowBeaconInitialDelayParameter, DataflowBeaconIntervalParameter:
				return time.Duration(0), true
			default:
				return nil, false
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if outputs := instance.Outputs(); len(outputs) != 1 || outputs[0] != "value" {
		t.Fatalf("provider-precedence outputs = %#v", outputs)
	}
}

func TestDataflowBeaconTimingExpressionsHaveStablePlanIdentity(t *testing.T) {
	build := func(initialDelay, interval time.Duration) Plan {
		t.Helper()
		env := NewEnvironment()
		if _, err := RegisterStruct[dataflowBeaconFieldEvent](env, "BeaconTimingPlanEvent"); err != nil {
			t.Fatal(err)
		}
		if _, err := DefineDataflow(env, "beacon-timing-plan").
			BeaconSourceWithOptions("source", DataflowBeaconOptions{
				Iterations:             1,
				InitialDelayExpression: Literal(initialDelay),
				IntervalExpression:     Literal(interval),
			}).
			Build(); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(From[dataflowBeaconFieldEvent](env, "BeaconTimingPlanEvent").Query())
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}
	first := build(time.Second, 2*time.Second)
	same := build(time.Second, 2*time.Second)
	differentInitial := build(3*time.Second, 2*time.Second)
	differentInterval := build(time.Second, 4*time.Second)
	if first.Hash() != same.Hash() {
		t.Fatalf("equivalent beacon timing expressions have different hashes: %s != %s", first.Hash(), same.Hash())
	}
	if first.Hash() == differentInitial.Hash() || first.Hash() == differentInterval.Hash() {
		t.Fatalf("different beacon timing expressions share plan hash %s", first.Hash())
	}
}

func TestDataflowBeaconTimingExpressionsAndProviderRejectInvalidValues(t *testing.T) {
	env := NewEnvironment()
	if _, err := DefineDataflow(env, "beacon-conflicting-initial").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{InitialDelay: time.Second, InitialDelayExpression: Literal(time.Second)}).
		Build(); err == nil {
		t.Fatal("beacon accepted fixed and expression initial delay")
	}
	if _, err := DefineDataflow(env, "beacon-conflicting-interval").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{Interval: time.Second, IntervalExpression: Literal(time.Second)}).
		Build(); err == nil {
		t.Fatal("beacon accepted fixed and expression interval")
	}
	if _, err := DefineDataflow(env, "beacon-wrong-duration-expression").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{InitialDelayExpression: Literal(int64(1))}).
		Build(); err == nil {
		t.Fatal("beacon accepted a non-duration timing expression")
	}
	if _, err := DefineDataflow(env, "beacon-field-duration-expression").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{InitialDelayExpression: Field[any, time.Duration]("delay")}).
		Build(); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("beacon field-dependent duration expression error = %v", err)
	}
	if _, err := DefineDataflow(env, "beacon-parameter-duration-expression").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{IntervalExpression: Parameter[time.Duration]("delay")}).
		Build(); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("beacon parameter-dependent duration expression error = %v", err)
	}
	negativeDefinition, err := DefineDataflow(env, "beacon-negative-duration-expression").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{InitialDelayExpression: Literal(-time.Second)}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewEngine(env).InstantiateDataflow(context.Background(), negativeDefinition); err == nil {
		t.Fatal("beacon accepted a negative evaluated duration")
	}
	providerDefinition, err := DefineDataflow(env, "beacon-invalid-timing-provider").
		BeaconSourceWithOptions("source", DataflowBeaconOptions{Iterations: 1}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewEngine(env).InstantiateDataflowWithOptions(context.Background(), providerDefinition, DataflowOptions{
		ParameterProvider: func(parameter DataflowParameterContext) (any, bool) {
			if parameter.ParameterName == DataflowBeaconIntervalParameter {
				return "wrong", true
			}
			return nil, false
		},
	}); err == nil || !errors.Is(err, ErrorTypeMismatch) {
		t.Fatalf("beacon non-duration interval provider error = %v", err)
	}
	for _, test := range []struct {
		name      string
		parameter string
		value     any
	}{
		{name: "iterations", parameter: DataflowBeaconIterationsParameter, value: -1},
		{name: "initial-delay", parameter: DataflowBeaconInitialDelayParameter, value: -time.Second},
		{name: "interval", parameter: DataflowBeaconIntervalParameter, value: -time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewEngine(env).InstantiateDataflowWithOptions(context.Background(), providerDefinition, DataflowOptions{
				ParameterProvider: func(parameter DataflowParameterContext) (any, bool) {
					if parameter.ParameterName == test.parameter {
						return test.value, true
					}
					return nil, false
				},
			})
			if err == nil || !errors.Is(err, ErrorInvalidRule) {
				t.Fatalf("negative %s provider error = %v", test.parameter, err)
			}
		})
	}
}
