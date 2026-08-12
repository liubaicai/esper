package esper

import (
	"context"
	"testing"
)

type unaryMinusSupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func TestEPLOtherUnaryMinusMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[unaryMinusSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("v", 1.0); err != nil {
		t.Fatal(err)
	}

	stream := From[unaryMinusSupportBean](env, "SupportBean")
	plan, err := env.Build(Select(stream,
		Alias("c0", Negate[int](Field[unaryMinusSupportBean, int]("intPrimitive"))),
		Alias("c1", Negate[float64](VariableRef[float64]("v"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}

	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("unary-minus result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), unaryMinusSupportBean{TheString: "E1", IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("c0").Any() != -10 || rows[0].Get("c1").Any() != -1.0 {
		t.Fatalf("unary-minus rows = %#v", rows)
	}
	value, ok := engine.GetVariable("v")
	if !ok || !value.Equal(Present(1.0)) {
		t.Fatalf("variable v = %#v, present=%v", value, ok)
	}
}
