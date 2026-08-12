package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

type clientBasicBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func newClientBasicEnvironment(t *testing.T) (*Environment, Stream[clientBasicBean]) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientBasicBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env, From[clientBasicBean](env, "SupportBean")
}

func TestClientBasicAggregationParity(t *testing.T) {
	env, source := newClientBasicEnvironment(t)
	plan, err := env.Build(source.Aggregate(Alias("cnt", CountAll())).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var counts []int64
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "client basic aggregate result is not a row")
			}
			counts = append(counts, row.Get("cnt").Any().(int64))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := engine.SendEvent(context.Background(), clientBasicBean{TheString: "E1"}); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(counts, []int64{1, 2, 3}) {
		t.Fatalf("aggregate counts = %#v, want [1 2 3]", counts)
	}
}

func TestClientBasicAnnotationParity(t *testing.T) {
	env, source := newClientBasicEnvironment(t)
	plan, err := env.Build(source.Query(StatementName("abc")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statements := deployment.Statements()
	if len(statements) != 1 || statements[0].Name() != "abc" {
		t.Fatalf("statement name metadata = %#v", statements)
	}
}

func TestClientBasicFilterParity(t *testing.T) {
	env, source := newClientBasicEnvironment(t)
	plan, err := env.Build(source.Filter(
		Equal[int](Field[clientBasicBean, int]("intPrimitive"), Literal(1)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	invoked := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		invoked += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, value := range []int{1, 0, 1, 0} {
		before := invoked
		if err := engine.SendEvent(context.Background(), clientBasicBean{TheString: "E", IntPrimitive: value}); err != nil {
			t.Fatal(err)
		}
		wantDelta := 0
		if value == 1 {
			wantDelta = 1
		}
		if invoked-before != wantDelta {
			t.Fatalf("filter value %d invocation delta = %d, want %d", value, invoked-before, wantDelta)
		}
	}
}

func TestClientBasicLengthWindowParity(t *testing.T) {
	env, source := newClientBasicEnvironment(t)
	plan, err := env.Build(source.Window(LengthWindow(2)).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	inputs := []*clientBasicBean{{TheString: "E1"}, {TheString: "E2"}, {TheString: "E3"}, {TheString: "E4"}}
	for index, input := range inputs {
		if err := engine.SendEvent(context.Background(), input); err != nil {
			t.Fatal(err)
		}
		if len(batches) != index+1 || len(batches[index].New) != 1 {
			t.Fatalf("length-window batch %d = %#v", index, batches)
		}
		newEvent, ok := batches[index].New[0].Event()
		if !ok || newEvent.Underlying() != input {
			t.Fatalf("length-window new identity %d = %#v, want %p", index, batches[index].New, input)
		}
		if index < 2 {
			if len(batches[index].Old) != 0 {
				t.Fatalf("length-window unexpected old at %d = %#v", index, batches[index].Old)
			}
			continue
		}
		if len(batches[index].Old) != 1 {
			t.Fatalf("length-window missing old at %d = %#v", index, batches[index].Old)
		}
		oldEvent, ok := batches[index].Old[0].Event()
		if !ok || oldEvent.Underlying() != inputs[index-2] {
			t.Fatalf("length-window old identity %d = %#v, want %p", index, batches[index].Old, inputs[index-2])
		}
	}
}

func TestClientBasicPatternParity(t *testing.T) {
	env, source := newClientBasicEnvironment(t)
	origin := time.Unix(0, 0).UTC()
	plan, err := env.Build(TimerAt(source, origin.Add(10*time.Second)).Select(
		Alias("now", CurrentTime()),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(origin))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	invoked := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		invoked += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(9999*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if invoked != 0 {
		t.Fatalf("timer invoked before deadline = %d", invoked)
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
	if invoked != 1 {
		t.Fatalf("timer invocation at deadline = %d, want 1", invoked)
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(9999999*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if invoked != 1 {
		t.Fatalf("one-shot timer invoked again = %d", invoked)
	}
}

func TestClientBasicSelectParity(t *testing.T) {
	env, source := newClientBasicEnvironment(t)
	plan, err := env.Build(source.Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	input := &clientBasicBean{}
	var received Event
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) == 1 {
			received, _ = batch.New[0].Event()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if received.Underlying() != input {
		t.Fatalf("select wildcard identity = %#v, want %p", received.Underlying(), input)
	}
}

func TestClientBasicSelectClauseParity(t *testing.T) {
	env, source := newClientBasicEnvironment(t)
	plan, err := env.Build(Select(source,
		Alias("intPrimitive", Field[clientBasicBean, int]("intPrimitive")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var values []int
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "client basic select-clause result is not a row")
			}
			values = append(values, row.Get("intPrimitive").Any().(int))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientBasicBean{TheString: "E1", IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(values, []int{10}) {
		t.Fatalf("select-clause values = %#v", values)
	}
}
