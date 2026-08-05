package esper

import (
	"context"
	"testing"
)

func TestPatternDiscardPartialsOnMatchClearsOverlappingStartsMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	pattern := PatternFrom(base, "a", StartsWith(Field[runtimeTestTrade, string]("symbol"), Literal("A"))).Every().
		FollowedBy("b", StartsWith(Field[runtimeTestTrade, string]("symbol"), Literal("A")))
	plan, err := env.Build(pattern.Select(
		Alias("a", TagField[string]("a", "symbol")),
		Alias("b", TagField[string]("b", "symbol")),
	).Query(StatementName("discard-partials"), DiscardPartialsOnMatch()))
	if err != nil {
		t.Fatal(err)
	}
	if !plan.query.discardPartialsOnMatch {
		t.Fatal("discard-partials query option was not retained in the plan")
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("discard-partials result = %#v, want Row", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"A1", "A2", "A3", "A4"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("discard-partials rows = %#v, want two non-overlapping matches", rows)
	}
	if rows[0].Get("a").Any() != "A1" || rows[0].Get("b").Any() != "A2" || rows[1].Get("a").Any() != "A3" || rows[1].Get("b").Any() != "A4" {
		t.Fatalf("discard-partials rows = %#v, want A1/A2 and A3/A4", rows)
	}
}

func TestPatternSuppressOverlappingMatchesHidesRepeatedEventMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	pattern := PatternFrom(base, "a", StartsWith(Field[runtimeTestTrade, string]("symbol"), Literal("A"))).Every().
		FollowedBy("b", StartsWith(Field[runtimeTestTrade, string]("symbol"), Literal("B"))).
		FollowedBy("c", Equal[float64](Field[runtimeTestTrade, float64]("price"), TagField[float64]("a", "price")))
	plan, err := env.Build(pattern.Select(
		Alias("a", TagField[string]("a", "symbol")),
		Alias("b", TagField[string]("b", "symbol")),
		Alias("c", TagField[string]("c", "symbol")),
	).Query(StatementName("suppress-overlapping"), SuppressOverlappingMatches()))
	if err != nil {
		t.Fatal(err)
	}
	if !plan.query.suppressOverlappingMatches {
		t.Fatal("suppress-overlapping query option was not retained in the plan")
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("suppress-overlapping result = %#v, want Row", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "A1", Price: 1},
		{Symbol: "A2", Price: 2},
		{Symbol: "A3", Price: 2},
		{Symbol: "B1"},
		{Symbol: "C1", Price: 2},
		{Symbol: "C2", Price: 1},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("suppress-overlapping rows = %#v, want one same-batch match and one later match", rows)
	}
	if rows[0].Get("a").Any() != "A2" || rows[0].Get("b").Any() != "B1" || rows[0].Get("c").Any() != "C1" {
		t.Fatalf("suppress-overlapping first row = %#v, want A2/B1/C1", rows[0])
	}
	if rows[1].Get("a").Any() != "A1" || rows[1].Get("b").Any() != "B1" || rows[1].Get("c").Any() != "C2" {
		t.Fatalf("suppress-overlapping second row = %#v, want A1/B1/C2", rows[1])
	}
}

func TestPatternConsumptionPoliciesRejectNonPatternQueries(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	for name, option := range map[string]QueryOption{
		"discard-partials":     DiscardPartialsOnMatch(),
		"suppress-overlapping": SuppressOverlappingMatches(),
	} {
		query := Select(base, Alias("symbol", Field[runtimeTestTrade, string]("symbol"))).Query(option)
		if _, err := env.Build(query); err == nil {
			t.Fatalf("%s was accepted for a non-pattern query", name)
		}
	}
}

func TestPatternConsumptionPoliciesRejectContextQueriesAndChangePlanIdentity(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	pattern := PatternFrom(base, "a", StartsWith(Field[runtimeTestTrade, string]("symbol"), Literal("A")))
	if _, err := env.RegisterContext("consumption-context", Field[runtimeTestTrade, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	patternQuery := pattern.Select(Alias("symbol", TagField[string]("a", "symbol")))
	if _, err := env.Build(patternQuery.Query(WithContext("consumption-context"), DiscardPartialsOnMatch())); err == nil {
		t.Fatal("discard-partials was accepted for a context pattern query")
	}
	plain, err := env.Build(patternQuery.Query(StatementName("consumption-identity")))
	if err != nil {
		t.Fatal(err)
	}
	policy, err := env.Build(patternQuery.Query(StatementName("consumption-identity"), SuppressOverlappingMatches()))
	if err != nil {
		t.Fatal(err)
	}
	if plain.Hash() == policy.Hash() {
		t.Fatalf("consumption policy did not change plan hash: %s", plain.Hash())
	}
}
