package esper

import (
	"context"
	"errors"
	"testing"
	"time"
)

type variableChangeTestListener struct {
	engine *Engine
	events []VariableChangeEvent
}

func (l *variableChangeTestListener) OnVariableChanged(event VariableChangeEvent) {
	l.events = append(l.events, event)
	if l.engine != nil {
		_, _ = l.engine.GetVariable(event.Name)
	}
}

func TestVariableChangeListenerAndContextPartitionIDUpdate(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("global", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "variable-context", Field[runtimeTestTrade, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterContextVariable("variable-context", "scoped", 5); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		StatementName("variable-listener-context"),
		WithContext("variable-context"),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	listener := &variableChangeTestListener{engine: engine}
	if err := engine.AddVariableChangeListener("", listener); err != nil {
		t.Fatal(err)
	}
	if err := engine.AddVariableChangeListener("global", listener); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SetVariable(context.Background(), "global", 2); err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	descriptors := statement.ContextPartitions()
	if len(descriptors) != 1 {
		t.Fatalf("context partitions = %#v", descriptors)
	}
	if err := engine.SetContextVariableByID(context.Background(), "variable-context", descriptors[0].ID, "scoped", 7); err != nil {
		t.Fatal(err)
	}
	if len(listener.events) != 2 {
		t.Fatalf("variable change events = %#v", listener.events)
	}
	global := listener.events[0]
	if global.Name != "global" || global.PartitionID != -1 || global.ContextName != "" || !global.Old.Equal(Present(1)) || !global.New.Equal(Present(2)) {
		t.Fatalf("global variable event = %#v", global)
	}
	scoped := listener.events[1]
	if scoped.Name != "scoped" || scoped.ContextName != "variable-context" || scoped.PartitionID != descriptors[0].ID || scoped.PartitionKey != descriptors[0].Key || !scoped.Old.Equal(Present(5)) || !scoped.New.Equal(Present(7)) {
		t.Fatalf("context variable event = %#v", scoped)
	}
	if err := engine.SetContextVariableByID(context.Background(), "variable-context", 999, "scoped", 8); err == nil || !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("unknown context partition update error = %v", err)
	}
	engine.RemoveVariableChangeListener("", listener)
	engine.RemoveVariableChangeListener("global", listener)
}

func TestVariableReferenceAndAtomicUpdate(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("threshold", 10.0); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	stream := From[runtimeTestTrade](env, "Trade").Filter(
		Greater[float64](
			Field[runtimeTestTrade, float64]("price"),
			VariableRef[float64]("threshold"),
		),
	)
	plan, err := env.Build(stream.Query(StatementName("variable-filter")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var seen []float64
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				return errors.New("variable-filter result is not an event")
			}
			seen = append(seen, event.Underlying().(runtimeTestTrade).Price)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 11}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SetVariable(context.Background(), "threshold", 12.0); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 13}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 12}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[0] != 11 || seen[1] != 13 {
		t.Fatalf("variable-filter results = %#v", seen)
	}
	value, ok := engine.GetVariable("threshold")
	if !ok || !value.Equal(Present(12.0)) {
		t.Fatalf("threshold snapshot = %#v, %v", value, ok)
	}
	snapshot := engine.Variables()
	snapshot["threshold"] = Present(999.0)
	current, _ := engine.GetVariable("threshold")
	if !current.Equal(Present(12.0)) {
		t.Fatalf("mutating returned variable map changed engine state: %v", current)
	}
}

func TestVariableBuildValidationAndConstant(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if err := env.RegisterVariable("limit", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Filter(
		Greater[float64](Field[runtimeTestTrade, float64]("price"), VariableRef[float64]("unknown")),
	).Query()); err == nil || !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("unknown variable error = %v", err)
	}
	if _, err := env.Build(From[runtimeTestTrade](env, "Trade").Filter(
		Equal[string](Field[runtimeTestTrade, string]("symbol"), VariableRef[string]("limit")),
	).Query()); err == nil || !errors.Is(err, ErrorTypeMismatch) {
		t.Fatalf("variable type mismatch error = %v", err)
	}
	if err := env.RegisterVariable("constantLimit", 10.0, ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SetVariable(context.Background(), "constantLimit", 11.0); err == nil || !errors.Is(err, ErrorState) {
		t.Fatalf("constant variable update error = %v", err)
	}
	if err := engine.SetVariable(context.Background(), "limit", "wrong"); err == nil || !errors.Is(err, ErrorTypeMismatch) {
		t.Fatalf("runtime variable type mismatch error = %v", err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(0, 0).UTC()); err != nil {
		t.Fatal(err)
	}
}

func TestVariableBatchUpdateIsAtomic(t *testing.T) {
	env := NewEnvironment()
	if err := env.RegisterVariable("a", 1); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("b", "initial"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if err := engine.SetVariables(context.Background(), VariableAssignment{Name: "a", Value: 2}, VariableAssignment{Name: "b", Value: 3}); err == nil || !errors.Is(err, ErrorTypeMismatch) {
		t.Fatalf("batch type error = %v", err)
	}
	a, _ := engine.GetVariable("a")
	b, _ := engine.GetVariable("b")
	if !a.Equal(Present(1)) || !b.Equal(Present("initial")) {
		t.Fatalf("failed batch partially changed state: a=%v b=%v", a, b)
	}
}

func TestSourceLessSelectOnceUsesVariables(t *testing.T) {
	env := NewEnvironment()
	if err := env.RegisterVariable("threshold", 7); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(SelectOnce(env, Alias("value", VariableRef[int]("threshold"))))
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewEngine(env).ExecuteFireAndForget(context.Background(), plan)
	if err != nil || len(result.Results()) != 1 {
		t.Fatalf("source-less result = %#v, err=%v", result, err)
	}
	row, ok := result.Results()[0].Row()
	if !ok || row.Get("value").Any() != 7 {
		t.Fatalf("source-less row = %#v", result.Results()[0])
	}
}
