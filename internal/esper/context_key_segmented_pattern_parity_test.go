package esper

import (
	"context"
	"testing"
)

type keySegPatternBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestKeyContextPatternEveryLeftLegParity locks the left-leg every
// semantics of a keyed context pattern verified against
// ContextKeySegmentedPattern: `partition by theString` running `every
// a=SupportBean -> b=SupportBean(intPrimitive=a.intPrimitive+1)`. The
// every binds to the left event expression, so every a-event spawns its
// own concurrent b-wait within its partition: G1's 10 waits for 11 and
// G2's 20 waits for 21 independently, later matches chain (21->22,
// 22->23), and the pattern stays alive after each match.
func TestKeyContextPatternEveryLeftLegParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegPatternBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	source := From[keySegPatternBean](env, "SupportBean")
	theString := Field[keySegPatternBean, string]("theString")
	if _, err := CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		t.Fatal(err)
	}
	pattern := PatternFrom(source, "a", Literal(true)).
		Every().
		FollowedBy("b", Equal[int](
			TagField[int]("b", "intPrimitive"),
			Add[int](TagField[int]("a", "intPrimitive"), Literal(1))))
	query := pattern.Select(
		Alias("a", PatternEvent("a")),
		Alias("b", PatternEvent("b")),
	).Query(StatementName("s0"), WithContext("SegmentedByString"))
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
	type pair struct {
		aString, bString string
		aValue, bValue   int
	}
	var matches []pair
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				a, _ := row.Get("a").Any().(Event)
				b, _ := row.Get("b").Any().(Event)
				matches = append(matches, pair{
					aString: a.Get("theString").Any().(string),
					bString: b.Get("theString").Any().(string),
					aValue:  a.Get("intPrimitive").Any().(int),
					bValue:  b.Get("intPrimitive").Any().(int),
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(s string, i int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegPatternBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	send("G1", 10)
	send("G1", 20)
	send("G2", 10)
	send("G2", 20)
	if len(matches) != 0 {
		t.Fatalf("before any completion matches = %#v, want none", matches)
	}
	send("G2", 21)
	if len(matches) != 1 || matches[0] != (pair{"G2", "G2", 20, 21}) {
		t.Fatalf("after G2/21 matches = %#v, want [{G2 G2 20 21}]", matches)
	}
	send("G1", 11)
	if len(matches) != 2 || matches[1] != (pair{"G1", "G1", 10, 11}) {
		t.Fatalf("after G1/11 matches = %#v, want second {G1 G1 10 11}", matches)
	}
	send("G2", 22)
	send("G2", 23)
	if len(matches) != 4 || matches[2] != (pair{"G2", "G2", 21, 22}) || matches[3] != (pair{"G2", "G2", 22, 23}) {
		t.Fatalf("after G2/23 matches = %#v, want chained {21 22} {22 23}", matches)
	}
	send("G1", 12)
	send("G1", 13)
	if len(matches) != 6 || matches[4] != (pair{"G1", "G1", 11, 12}) || matches[5] != (pair{"G1", "G1", 12, 13}) {
		t.Fatalf("after G1/13 matches = %#v, want chained {11 12} {12 13}", matches)
	}
}
