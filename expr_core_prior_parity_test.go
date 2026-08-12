package esper

import (
	"context"
	"testing"
)

type priorParityBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func priorParityEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[priorParityBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env)
}

func sendPriorParity(t *testing.T, engine *Engine, name string, value int) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), priorParityBean{TheString: name, IntPrimitive: value}); err != nil {
		t.Fatal(err)
	}
}

func priorRowValue(row Row, field string) any {
	return row.Get(field).Any()
}

// ExprCorePriorUnboundSceneTwo. Java prior(1, intPrimitive) and prior(2,
// intPrimitive) map to Go Prior(0, ...) and Prior(1, ...) respectively.
func TestExprCorePriorUnboundSceneTwoParity(t *testing.T) {
	env, engine := priorParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	input := From[priorParityBean](env, "SupportBean")
	plan, err := env.Build(Select(input,
		Alias("c0", Field[priorParityBean, string]("theString")),
		Alias("c1", Prior[int](0, Field[priorParityBean, int]("intPrimitive"))),
		Alias("c2", Prior[int](1, Field[priorParityBean, int]("intPrimitive"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	rows := make([]Row, 0, 5)
	_, _ = dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	})
	for i, value := range []int{10, 11, 12, 13, 14} {
		sendPriorParity(t, engine, string(rune('A'+i)), value)
	}
	if len(rows) != 5 {
		t.Fatalf("unbound rows = %d, want 5", len(rows))
	}
	wantPrevious := []any{nil, 10, 11, 12, 13}
	wantTwoBack := []any{nil, nil, 10, 11, 12}
	for i, row := range rows {
		if got := priorRowValue(row, "c1"); got != wantPrevious[i] {
			t.Fatalf("unbound row %d c1 = %v, want %v", i, got, wantPrevious[i])
		}
		if got := priorRowValue(row, "c2"); got != wantTwoBack[i] {
			t.Fatalf("unbound row %d c2 = %v, want %v", i, got, wantTwoBack[i])
		}
	}
}

// ExprCorePriorBoundedMultiple. The length window retains only two events;
// old-stream rows retain the projection emitted when each event entered.
func TestExprCorePriorBoundedMultipleParity(t *testing.T) {
	env, engine := priorParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	input := From[priorParityBean](env, "SupportBean").Window(LengthWindow(2))
	plan, err := env.Build(Select(input,
		Alias("c0", Field[priorParityBean, string]("theString")),
		Alias("c1", Prior[int](0, Field[priorParityBean, int]("intPrimitive"))),
		Alias("c2", Prior[int](1, Field[priorParityBean, int]("intPrimitive"))),
	).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	batches := make([]ResultBatch, 0, 5)
	_, _ = dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	})
	for i, value := range []int{10, 11, 12, 13, 14} {
		sendPriorParity(t, engine, string(rune('A'+i)), value)
	}
	if len(batches) != 5 {
		t.Fatalf("bounded batches = %d, want 5", len(batches))
	}
	wantNew := []struct{ c1, c2 any }{{nil, nil}, {10, nil}, {11, 10}, {12, 11}, {13, 12}}
	for i, batch := range batches {
		if len(batch.New) != 1 {
			t.Fatalf("bounded batch %d new = %#v", i, batch.New)
		}
		row, ok := batch.New[0].Row()
		if !ok {
			t.Fatalf("bounded batch %d new result is not a row", i)
		}
		if got := priorRowValue(row, "c1"); got != wantNew[i].c1 {
			t.Fatalf("bounded batch %d c1 = %v, want %v", i, got, wantNew[i].c1)
		}
		if got := priorRowValue(row, "c2"); got != wantNew[i].c2 {
			t.Fatalf("bounded batch %d c2 = %v, want %v", i, got, wantNew[i].c2)
		}
		if i >= 2 {
			if len(batch.Old) != 1 {
				t.Fatalf("bounded batch %d old = %#v, want one row", i, batch.Old)
			}
			old, ok := batch.Old[0].Row()
			if !ok {
				t.Fatalf("bounded batch %d old result is not a row", i)
			}
			wantOldC1 := []any{nil, 10, 11}[i-2]
			wantOldC2 := []any{nil, nil, 10}[i-2]
			if got := priorRowValue(old, "c1"); got != wantOldC1 || priorRowValue(old, "c2") != wantOldC2 {
				t.Fatalf("bounded batch %d old = (%v,%v), want (%v,%v)", i, got, priorRowValue(old, "c2"), wantOldC1, wantOldC2)
			}
		}
	}
}

// ExprCorePriorBoundedSingle is the one-offset bounded execution tracked
// separately by Java.
func TestExprCorePriorBoundedSingleParity(t *testing.T) {
	env, engine := priorParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	input := From[priorParityBean](env, "SupportBean").Window(LengthWindow(2))
	plan, err := env.Build(Select(input,
		Alias("c0", Field[priorParityBean, string]("theString")),
		Alias("c1", Prior[int](0, Field[priorParityBean, int]("intPrimitive"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	var previous []any
	_, _ = dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				previous = append(previous, priorRowValue(row, "c1"))
			}
		}
		return nil
	})
	for i, value := range []int{10, 11, 12} {
		sendPriorParity(t, engine, string(rune('A'+i)), value)
	}
	want := []any{nil, 10, 11}
	if len(previous) != len(want) {
		t.Fatalf("bounded single rows = %d, want %d", len(previous), len(want))
	}
	for i := range want {
		if previous[i] != want[i] {
			t.Fatalf("bounded single row %d = %v, want %v", i, previous[i], want[i])
		}
	}
}
