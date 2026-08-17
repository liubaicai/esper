package esper

import (
	"context"
	"testing"
)

// TestKeyContextPatternFilterNeverMatchesParity locks the never-match
// pattern of a keyed context verified against
// ContextKeySegmentedPatternFilter: `partition by theString` running `every
// (event1=SupportBean(no-X) -> event2=SupportBean(has-X))`. Because the
// partition key IS theString, event1 (no X) and event2 (contains X) can
// never occur within the same partition, so the pattern never fires and
// every event is silent. The Go runner normalizes the regression's
// stringContainsX UDF to the equivalent Contains predicate.
func TestKeyContextPatternFilterNeverMatchesParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegPatternBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	source := From[keySegPatternBean](env, "SupportBean")
	theString := Field[keySegPatternBean, string]("theString")
	if _, err := CreateKeyContext(env, "IndividualBean", theString); err != nil {
		t.Fatal(err)
	}
	pattern := PatternFrom(source, "event1", Not(Contains(theString, Literal("X")))).
		Every().
		FollowedBy("event2", Contains(theString, Literal("X")))
	query := pattern.Select(
		Alias("a_theString", TagField[string]("event1", "theString")),
		Alias("b_theString", TagField[string]("event2", "theString")),
	).Query(StatementName("s0"), WithContext("IndividualBean"))
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
	var matches int
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		matches += len(batch.New)
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
	send("F1", 0)
	send("F1", 0)
	send("X1", 0)
	send("X1", 0)
	if matches != 0 {
		t.Fatalf("matches = %d, want 0 (partition key prevents event1->event2 pairing)", matches)
	}
}
