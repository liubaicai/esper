package esper

import (
	"context"
	"testing"
	"time"
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

// TestPatternOperatorEveryFollowedByMatchesEsper covers PatternEveryFollowedBy:
// every a=SupportBean -> b=SupportBean(theString=a.theString). Each a-event
// spawns its own waiting branch; a later event with the same theString
// completes only the branch whose a carries that value.
func TestPatternOperatorEveryFollowedByMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[patternOpBean](env, "SupportBean")
	pattern := PatternFrom(source, "a", Literal[bool](true)).Every().Then(
		PatternFrom(source, "b", Equal[string](
			Field[patternOpBean, string]("theString"),
			TagField[string]("a", "theString"),
		)),
	)
	plan, err := env.Build(pattern.Select(
		Alias("c0", TagField[string]("a", "theString")),
		Alias("c1", TagField[int]("a", "intPrimitive")),
		Alias("c2", TagField[int]("b", "intPrimitive")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("pattern result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendPatternOpBean(t, engine, "E1", 1)
	if len(rows) != 0 {
		t.Fatalf("single a-event must not fire, rows = %#v", rows)
	}
	sendPatternOpBean(t, engine, "E2", 10)
	if len(rows) != 0 {
		t.Fatalf("different theString must not complete the waiting branch, rows = %#v", rows)
	}
	sendPatternOpBean(t, engine, "E1", 2)
	if len(rows) != 1 {
		t.Fatalf("correlated b-event should fire exactly once, rows = %#v", rows)
	}
	row := rows[0]
	if row.Get("c0").Any() != "E1" || row.Get("c1").Any() != 1 || row.Get("c2").Any() != 2 {
		t.Fatalf("row = (%v,%v,%v), want (E1,1,2)", row.Get("c0").Any(), row.Get("c1").Any(), row.Get("c2").Any())
	}
}

// TestPatternOperatorEveryFollowedByWithinMatchesEsper covers
// PatternEveryFollowedByWithin: every a=SupportBean -> b=SupportBean(theString=
// a.theString) where timer:within(10 sec). Each a-spawned branch expires 10
// seconds after its a-event; an expired branch cannot complete even when a
// later event carries the correlated value.
func TestPatternOperatorEveryFollowedByWithinMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[patternOpBean](env, "SupportBean")
	waiting := PatternFrom(source, "b", Equal[string](
		Field[patternOpBean, string]("theString"),
		TagField[string]("a", "theString"),
	)).Within(10 * time.Second)
	pattern := PatternFrom(source, "a", Literal[bool](true)).Every().Then(waiting)
	plan, err := env.Build(pattern.Select(
		Alias("c0", TagField[string]("a", "theString")),
		Alias("c1", TagField[int]("a", "intPrimitive")),
		Alias("c2", TagField[int]("b", "intPrimitive")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("pattern result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	advance := func(ms int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(ms).UTC()); err != nil {
			t.Fatal(err)
		}
	}

	advance(5000)
	sendPatternOpBean(t, engine, "E1", 1)
	if len(rows) != 0 {
		t.Fatalf("single a-event must not fire, rows = %#v", rows)
	}
	advance(8000)
	sendPatternOpBean(t, engine, "E2", 10)
	if len(rows) != 0 {
		t.Fatalf("different theString must not fire, rows = %#v", rows)
	}
	// t=15000 expires the branch started by E1 at t=5000 (10 second guard).
	advance(15000)
	sendPatternOpBean(t, engine, "E1", 2)
	if len(rows) != 0 {
		t.Fatalf("expired branch must not complete, rows = %#v", rows)
	}
	sendPatternOpBean(t, engine, "E2", 11)
	if len(rows) != 1 {
		t.Fatalf("live E2 branch should fire exactly once, rows = %#v", rows)
	}
	row := rows[0]
	if row.Get("c0").Any() != "E2" || row.Get("c1").Any() != 10 || row.Get("c2").Any() != 11 {
		t.Fatalf("row = (%v,%v,%v), want (E2,10,11)", row.Get("c0").Any(), row.Get("c1").Any(), row.Get("c2").Any())
	}
}

// TestPatternOperatorEveryWithAndMatchesEsper covers PatternEveryWithAnd:
// every (a=SupportBean(intPrimitive>0) and b=SupportBean(intPrimitive<0)).
// The and-attempt caches the first event on each side, ignores further
// same-side events, fires once when both sides have matched, and only then
// does the enclosing every spawn the next attempt.
func TestPatternOperatorEveryWithAndMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[patternOpBean](env, "SupportBean")
	pattern := PatternFrom(source, "a", Greater[int](
		Field[patternOpBean, int]("intPrimitive"), Literal(0),
	)).And(PatternFrom(source, "b", Less[int](
		Field[patternOpBean, int]("intPrimitive"), Literal(0),
	))).Every()
	plan, err := env.Build(pattern.Select(
		Alias("c0", TagField[string]("a", "theString")),
		Alias("c1", TagField[string]("b", "theString")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("pattern result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendPatternOpBean(t, engine, "E1", 1)
	sendPatternOpBean(t, engine, "E2", 1)
	if len(rows) != 0 {
		t.Fatalf("two positive-side events must not fire without a negative-side event, rows = %#v", rows)
	}
	sendPatternOpBean(t, engine, "E3", -1)
	if len(rows) != 1 {
		t.Fatalf("completing negative-side event should fire exactly once, rows = %#v", rows)
	}
	if rows[0].Get("c0").Any() != "E1" || rows[0].Get("c1").Any() != "E3" {
		t.Fatalf("first fire = (%v,%v), want (E1,E3)", rows[0].Get("c0").Any(), rows[0].Get("c1").Any())
	}
	sendPatternOpBean(t, engine, "E4", -2)
	if len(rows) != 1 {
		t.Fatalf("negative-side event alone must not fire the fresh attempt, rows = %#v", rows)
	}
	sendPatternOpBean(t, engine, "E5", 2)
	if len(rows) != 2 {
		t.Fatalf("positive-side event should complete the fresh attempt, rows = %#v", rows)
	}
	if rows[1].Get("c0").Any() != "E5" || rows[1].Get("c1").Any() != "E4" {
		t.Fatalf("second fire = (%v,%v), want (E5,E4)", rows[1].Get("c0").Any(), rows[1].Get("c1").Any())
	}
}

// TestPatternOperatorEveryAndNotMatchesEsper covers PatternEveryAndNot:
// every (timer:interval(6) and not SupportBean). A SupportBean cancels only
// the current attempt; the enclosing every immediately arms the next 6 second
// timer from the cancellation time.
func TestPatternOperatorEveryAndNotMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[patternOpBean](env, "SupportBean")
	pattern := TimerInterval(source, 6*time.Second).And(
		PatternFrom(source, "sb", Literal[bool](true)).Not(),
	).Every()
	plan, err := env.Build(pattern.Select(
		Alias("alert", Literal("No event within 6 seconds")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var alerts []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("pattern result is not a row: %#v", result)
			}
			alerts = append(alerts, row.Get("alert").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	advance := func(ms int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(ms).UTC()); err != nil {
			t.Fatal(err)
		}
	}

	advance(2000)
	sendPatternOpBean(t, engine, "E1", 1) // cancels the attempt armed at t=0
	advance(6000)
	advance(7000)
	advance(7999)
	if len(alerts) != 0 {
		t.Fatalf("cancelled attempt must not fire, alerts = %#v", alerts)
	}
	advance(8000) // 6 seconds after the t=2000 cancellation
	if len(alerts) != 1 || alerts[0] != "No event within 6 seconds" {
		t.Fatalf("alerts = %#v, want one alert at t=8000", alerts)
	}
	advance(12000)
	sendPatternOpBean(t, engine, "E2", 2) // cancels the attempt armed at t=8000
	advance(13000)
	sendPatternOpBean(t, engine, "E3", 3) // cancels the attempt armed at t=12000
	advance(18999)
	if len(alerts) != 1 {
		t.Fatalf("cancelled attempts must not fire, alerts = %#v", alerts)
	}
	advance(19000) // 6 seconds after the t=13000 cancellation
	if len(alerts) != 2 || alerts[1] != "No event within 6 seconds" {
		t.Fatalf("alerts = %#v, want second alert at t=19000", alerts)
	}
}
