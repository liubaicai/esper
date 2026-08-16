package esper

import (
	"context"
	"testing"
)

type keySegMatchRecognizeBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

// TestKeyContextMatchRecognizePerPartitionStateParity locks the per-
// partition row-recognition state of a keyed context verified against
// ContextKeySegmentedMatchRecognize: `partition by theString` with a
// match_recognize pattern (A B) where A has intPrimitive=1 and B has
// intPrimitive=2, measuring each row's longPrimitive. Each partition keeps
// its own recognition state: A's and B's events each complete an A-B match
// within their own partition, never crossing.
func TestKeyContextMatchRecognizePerPartitionStateParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegMatchRecognizeBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	source := From[keySegMatchRecognizeBean](env, "SupportBean")
	theString := Field[keySegMatchRecognizeBean, string]("theString")
	if _, err := CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		t.Fatal(err)
	}
	query := source.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		Define("A", Equal[int](TagField[int]("A", "intPrimitive"), Literal(1))).
		Define("B", Equal[int](TagField[int]("B", "intPrimitive"), Literal(2))).
		Measures(
			Alias("a", TagField[int64]("A", "longPrimitive")),
			Alias("b", TagField[int64]("B", "longPrimitive")),
		).
		Query(StatementName("s0"), WithContext("SegmentedByString"))
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
	statement := deployment.Statements()[0]
	type pair struct{ a, b int64 }
	var matches []pair
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				matches = append(matches, pair{
					a: row.Get("a").Any().(int64),
					b: row.Get("b").Any().(int64),
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(s string, i int, l int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegMatchRecognizeBean{TheString: s, IntPrimitive: i, LongPrimitive: l}); err != nil {
			t.Fatal(err)
		}
	}
	send("A", 1, 10)
	send("B", 1, 30)
	send("A", 2, 20)
	if len(matches) != 1 || matches[0] != (pair{10, 20}) {
		t.Fatalf("after A completion matches = %#v, want [{10 20}]", matches)
	}
	send("B", 2, 40)
	if len(matches) != 2 || matches[1] != (pair{30, 40}) {
		t.Fatalf("after B completion matches = %#v, want [{10 20} {30 40}]", matches)
	}
	send("A", 1, 50)
	send("A", 2, 60)
	if len(matches) != 3 || matches[2] != (pair{50, 60}) {
		t.Fatalf("after second A cycle matches = %#v, want third {50 60}", matches)
	}
}
