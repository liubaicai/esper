package esper

import (
	"context"
	"testing"
)

type patternEveryA struct {
	ID string `esper:"id"`
}

type patternEveryB struct {
	ID string `esper:"id"`
}

type patternEveryOther struct {
	ID string `esper:"id"`
}

func newPatternEveryEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[patternEveryA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternEveryB](env, "SupportBean_B"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternEveryOther](env, "Other"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	return env, engine
}

// sendPatternEverySet sends the matching subset of the Java EventCollectionFactory
// mixed set in the same relative order (non-matching C1/D1/E1/F1/D2/G1/D3 are
// modeled as Other events; they cannot affect a b=SupportBean_B filter).
func sendPatternEverySet(t *testing.T, engine *Engine) {
	t.Helper()
	events := []struct {
		typeName string
		id       string
	}{
		{"A", "A1"}, {"B", "B1"}, {"O", "C1"}, {"B", "B2"}, {"A", "A2"}, {"O", "D1"},
		{"O", "E1"}, {"O", "F1"}, {"O", "D2"}, {"B", "B3"}, {"O", "G1"}, {"O", "D3"},
	}
	for _, event := range events {
		var err error
		switch event.typeName {
		case "A":
			err = engine.SendEvent(context.Background(), patternEveryA{ID: event.id})
		case "B":
			err = engine.SendEvent(context.Background(), patternEveryB{ID: event.id})
		default:
			err = engine.SendEvent(context.Background(), patternEveryOther{ID: event.id})
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

func countPatternEveryMatches(t *testing.T, env *Environment, engine *Engine, pattern PatternStream, name string) map[string]int {
	t.Helper()
	plan, err := env.Build(pattern.Select(
		Alias("c0", TagField[string]("b", "id")),
	).Query(StatementName(name)))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	counts := make(map[string]int)
	order := make([]string, 0)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			id := result.Get("c0").Any().(string)
			counts[id]++
			order = append(order, id)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendPatternEverySet(t, engine)
	t.Logf("%s match order = %v", name, order)
	return counts
}

// TestPatternOperatorEverySimpleMatchesEsper covers PatternEverySimple:
// select a.theString as c0 from pattern [every a=SupportBean] — every event
// matches once, and undeploy stops further matching.
func TestPatternOperatorEverySimpleMatchesEsper(t *testing.T) {
	env, engine := newPatternEveryEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	pattern := PatternFrom(From[patternEveryA](env, "SupportBean_A"), "a", Literal(true)).Every()
	plan, err := env.Build(pattern.Select(
		Alias("c0", TagField[string]("a", "id")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	stmt := deployment.Statements()[0]
	var matched []string
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			matched = append(matched, result.Get("c0").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"E1", "E2", "E3", "E4"} {
		if err := engine.SendEvent(context.Background(), patternEveryA{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	if len(matched) != 4 || matched[0] != "E1" || matched[3] != "E4" {
		t.Fatalf("matched = %v, want [E1 E2 E3 E4]", matched)
	}

	// Undeploy stops matching.
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), patternEveryA{ID: "E5"}); err != nil {
		t.Fatal(err)
	}
	if len(matched) != 4 {
		t.Fatalf("matched after undeploy = %v, want 4 entries", matched)
	}
}

// TestPatternOperatorEveryMultiplicityMatchesEsper covers PatternOp: nested
// every levels multiply matches — every(every(b)) fires B2 twice and B3 four
// times, every(every(every(b))) fires B2 three times and B3 nine times, and a
// plain (non-every) filter fires only on the first matching event.
func TestPatternOperatorEveryMultiplicityMatchesEsper(t *testing.T) {
	t.Run("every-single", func(t *testing.T) {
		env, engine := newPatternEveryEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		b := From[patternEveryB](env, "SupportBean_B")
		counts := countPatternEveryMatches(t, env, engine,
			PatternFrom(b, "b", Literal(true)).Every(), "every-single")
		if counts["B1"] != 1 || counts["B2"] != 1 || counts["B3"] != 1 {
			t.Fatalf("every b=B counts = %v, want B1/B2/B3 once each", counts)
		}
	})

	t.Run("no-every-fires-once", func(t *testing.T) {
		env, engine := newPatternEveryEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		b := From[patternEveryB](env, "SupportBean_B")
		counts := countPatternEveryMatches(t, env, engine,
			PatternFrom(b, "b", Literal(true)), "no-every")
		if counts["B1"] != 1 || counts["B2"] != 0 || counts["B3"] != 0 {
			t.Fatalf("b=B counts = %v, want only B1", counts)
		}
	})

	t.Run("nested-every-2", func(t *testing.T) {
		env, engine := newPatternEveryEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		b := From[patternEveryB](env, "SupportBean_B")
		counts := countPatternEveryMatches(t, env, engine,
			PatternFrom(b, "b", Literal(true)).Every().Every(), "nested-2")
		if counts["B1"] != 1 || counts["B2"] != 2 || counts["B3"] != 4 {
			t.Fatalf("every(every(b=B)) counts = %v, want B1x1 B2x2 B3x4", counts)
		}
	})

	t.Run("nested-every-3", func(t *testing.T) {
		env, engine := newPatternEveryEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		b := From[patternEveryB](env, "SupportBean_B")
		counts := countPatternEveryMatches(t, env, engine,
			PatternFrom(b, "b", Literal(true)).Every().Every().Every(), "nested-3")
		if counts["B1"] != 1 || counts["B2"] != 3 || counts["B3"] != 9 {
			t.Fatalf("every(every(every(b=B))) counts = %v, want B1x1 B2x3 B3x9", counts)
		}
	})

	t.Run("nested-every-4", func(t *testing.T) {
		env, engine := newPatternEveryEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		b := From[patternEveryB](env, "SupportBean_B")
		counts := countPatternEveryMatches(t, env, engine,
			PatternFrom(b, "b", Literal(true)).Every().Every().Every().Every(), "nested-4")
		if counts["B1"] != 1 || counts["B2"] != 4 || counts["B3"] != 16 {
			t.Fatalf("every^4(b=B) counts = %v, want B1x1 B2x4 B3x16", counts)
		}
	})
}
