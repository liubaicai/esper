package esper

import (
	"context"
	"testing"
)

type outputAfterBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestOutputAfterLastParity mirrors the shared output-after-last parity
// scenario (Java ResultSetAfterWithOutputLast): output after 4 events last
// every 2 events emits only the final aggregate row at the sixth event.
func TestOutputAfterLastParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[outputAfterBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[outputAfterBean](env, "SupportBean").Aggregate(
		Alias("thesum", Sum[int](Field[outputAfterBean, int]("intPrimitive"))),
	).Query(
		StatementName("s0"),
		WithOutput(OutputAfterEvents(4, OutputLastEveryEvents(2))),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
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
	for _, event := range []outputAfterBean{
		{TheString: "E1", IntPrimitive: 10},
		{TheString: "E2", IntPrimitive: 20},
		{TheString: "E3", IntPrimitive: 30},
		{TheString: "E4", IntPrimitive: 40},
		{TheString: "E5", IntPrimitive: 50},
		{TheString: "E6", IntPrimitive: 60},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("batches = %#v, want one row", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("thesum").Any() != 210 {
		t.Fatalf("output after row = %#v, want thesum=210", batches[0].New[0])
	}
}
