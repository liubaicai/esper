package esper

import (
	"context"
	"testing"
)

type subqueryLengthWindowS0 struct {
	ID int `esper:"id"`
}

type subqueryLengthWindowS1 struct {
	ID int `esper:"id"`
}

// TestSubqueryLengthWindowMaxParity mirrors the shared subquery-length-window
// parity scenario. The Java oracle (Esper 9.0.0 commit
// 9e1b9f1cc9117fea4bf33ab043762c045d73839c) projects max(id) from
// SupportBean_S1#length(3) for every SupportBean_S0 trigger: null on an empty
// window, then 100/200/200/200/190 as the window slides.
func TestSubqueryLengthWindowMaxParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[subqueryLengthWindowS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[subqueryLengthWindowS1](env, "SupportBean_S1"); err != nil {
		t.Fatal(err)
	}
	inner := From[subqueryLengthWindowS1](env, "SupportBean_S1").Window(LengthWindow(3)).AsRecord()
	query := Select(
		From[subqueryLengthWindowS0](env, "SupportBean_S0"),
		Alias("value", SubqueryValue[int](inner, Max[int](Field[any, int]("id")))),
	).Query(StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var values []any
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("subquery result is not a row: %#v", result)
			}
			value := row.Get("value")
			if value.IsNull() {
				values = append(values, nil)
			} else {
				values = append(values, value.Any())
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendS0 := func(id int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), subqueryLengthWindowS0{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	sendS1 := func(id int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), subqueryLengthWindowS1{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	sendS0(1)
	sendS1(100)
	sendS0(2)
	sendS1(200)
	sendS0(3)
	sendS1(190)
	sendS0(4)
	sendS1(180)
	sendS0(5)
	sendS1(170)
	sendS0(6)
	want := []any{nil, 100, 200, 200, 200, 190}
	if len(values) != len(want) {
		t.Fatalf("values = %#v, want %#v", values, want)
	}
	for index := range want {
		if values[index] != want[index] {
			t.Fatalf("values[%d] = %#v, want %#v (values %#v)", index, values[index], want[index], values)
		}
	}
}
