package esper

import (
	"context"
	"testing"
)

func TestPatternFollowedByMaxLimitsActiveSubexpressionsMatchesEsper(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	symbol := func(want string) Expression[bool] {
		return StartsWith(Field[runtimeTestTrade, string]("symbol"), Literal(want))
	}
	pattern := PatternFrom(base, "a", symbol("A")).Every().
		FollowedBy("b", symbol("B")).
		FollowedByMax(2, "c", symbol("C"))
	plan, err := env.Build(pattern.Select(
		Alias("a", TagField[string]("a", "symbol")),
		Alias("b", TagField[string]("b", "symbol")),
		Alias("c", TagField[string]("c", "symbol")),
	).Query(StatementName("pattern-followed-by-max")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("followed-by max result = %#v, want Row", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, event := range []runtimeTestTrade{
		{Symbol: "A1"}, {Symbol: "A2"}, {Symbol: "A3"},
		{Symbol: "B"}, {Symbol: "C"},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("nested followed-by max rows = %#v, want two", rows)
	}
	assertPatternFollowedByMaxRow(t, rows[0], "A1", "B", "C")
	assertPatternFollowedByMaxRow(t, rows[1], "A2", "B", "C")

	// The completed B->C subexpressions release capacity. A later batch may
	// therefore accept two new A starts, while the third is again rejected.
	for _, event := range []runtimeTestTrade{
		{Symbol: "A4"}, {Symbol: "A5"}, {Symbol: "A6"},
		{Symbol: "B"}, {Symbol: "C"},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 4 {
		t.Fatalf("followed-by max capacity reuse rows = %#v, want four", rows)
	}
	assertPatternFollowedByMaxRow(t, rows[2], "A4", "B", "C")
	assertPatternFollowedByMaxRow(t, rows[3], "A5", "B", "C")
}

func TestPatternFollowedByMaxSimpleMatchesEsper(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	pattern := PatternFrom(base, "a", StartsWith(Field[runtimeTestTrade, string]("symbol"), Literal("A"))).Every().
		FollowedByMax(2, "b", StartsWith(Field[runtimeTestTrade, string]("symbol"), Literal("B")))
	plan, err := env.Build(pattern.Select(
		Alias("a", TagField[string]("a", "symbol")),
		Alias("b", TagField[string]("b", "symbol")),
	).Query(StatementName("pattern-followed-by-max-simple")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("simple followed-by max result = %#v, want Row", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "A1"}, {Symbol: "A2"}, {Symbol: "A3"}, {Symbol: "B1"},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 2 || rows[0].Get("a").Any() != "A1" || rows[1].Get("a").Any() != "A2" {
		t.Fatalf("simple followed-by max rows = %#v, want A1/A2 only", rows)
	}
}

func TestPatternFollowedByMaxRejectsNonPositiveMaximum(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	pattern := PatternFrom(base, "a", Literal[bool](true)).FollowedByMax(0, "b", Literal[bool](true))
	if _, err := env.Build(pattern.Select(Alias("a", TagField[string]("a", "symbol"))).Query()); err == nil {
		t.Fatal("non-positive followed-by maximum was accepted")
	}
}

func assertPatternFollowedByMaxRow(t *testing.T, row Row, wantA, wantB, wantC string) {
	t.Helper()
	if got := row.Get("a").Any(); got != wantA {
		t.Fatalf("followed-by max a = %#v, want %q", got, wantA)
	}
	if got := row.Get("b").Any(); got != wantB {
		t.Fatalf("followed-by max b = %#v, want %q", got, wantB)
	}
	if got := row.Get("c").Any(); got != wantC {
		t.Fatalf("followed-by max c = %#v, want %q", got, wantC)
	}
}
