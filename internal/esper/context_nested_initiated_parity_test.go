package esper

import (
	"context"
	"testing"
)

type contextNestedInitS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

func newContextNestedInitEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[contextNestedParityBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[contextNestedInitS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	return env
}

func contextNestedInitRows(t *testing.T, stmt *Statement) func() []Row {
	t.Helper()
	var rows []Row
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return func() []Row { return append([]Row(nil), rows...) }
}

// TestContextNestedInitiatedParity covers ContextNestedInvalid and
// ContextNestedPartitionedOverPatternInitiated.
func TestContextNestedInitiatedParity(t *testing.T) {
	t.Run("invalid", func(t *testing.T) {
		env, _ := newContextNestedParityEnvironment(t)
		outer, err := NewKeyContext("Outer", Field[contextNestedParityBean, string]("theString"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := env.RegisterContext(outer.Name(), outer.Keys()...); err != nil {
			t.Fatal(err)
		}
		inner, err := NewKeyContext("Inner", Field[contextNestedParityBean, int]("intPrimitive"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CreateNestedContext(env, "Nested", "Outer", inner); err != nil {
			t.Fatal(err)
		}
		if _, err := CreateNestedContext(env, "Nested", "Outer", inner); err == nil {
			t.Fatal("duplicate nested context name was accepted")
		}
	})

	t.Run("partitioned-over-pattern-initiated", func(t *testing.T) {
		env := newContextNestedInitEnvironment(t)
		outer, err := NewKeyContext("Outer", Field[contextNestedParityBean, string]("theString"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := env.RegisterContext(outer.Name(), outer.Keys()...); err != nil {
			t.Fatal(err)
		}
		inner, err := NewInitiatedTerminatedContext("Inner", Literal("global"),
			Equal[int](Field[contextNestedParityBean, int]("intPrimitive"), Literal(1)),
			Equal[int](Field[contextNestedParityBean, int]("intPrimitive"), Literal(2)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CreateNestedContext(env, "Nested", "Outer", inner); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		plan, err := env.Build(From[contextNestedParityBean](env, "SupportBean").GroupBy(
			Field[contextNestedParityBean, string]("theString"),
		).Select(
			Alias("theString", Field[contextNestedParityBean, string]("theString")),
			Alias("theSum", Sum[int64](Field[contextNestedParityBean, int64]("longPrimitive"))),
		).Query(StatementName("s0"), WithContext("Nested"), WithOutput(OutputWhenTerminated())))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := contextNestedInitRows(t, deployment.Statements()[0])
		send := func(s string, i int, l int64) {
			t.Helper()
			if err := engine.SendEvent(context.Background(), contextNestedParityBean{TheString: s, IntPrimitive: i, LongPrimitive: l}); err != nil {
				t.Fatal(err)
			}
		}
		send("A", 0, 1)
		send("B", 0, 2)
		send("C", 1, 3)
		send("D", 1, 4)
		send("A", 0, 5)
		send("C", 0, 6)
		send("C", 2, -10)
		got := rows()
		if len(got) != 1 || got[0].Get("theString").Any() != "C" || got[0].Get("theSum").Any() != int64(-1) {
			t.Fatalf("partitioned-over-initiated rows = %#v", got)
		}
	})
}
