package esper

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func collectPatternConsumptionRows(t *testing.T, env *Environment, engine *Engine, pattern PatternStream, name string) (*[]Row, *Deployment) {
	t.Helper()
	plan, err := env.Build(pattern.Select(
		Alias("a", TagField[string]("a", "symbol")),
		Alias("b", TagField[string]("b", "symbol")),
		Alias("c", TagField[string]("c", "symbol")),
	).Query(StatementName(name)))
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
				return fmt.Errorf("pattern consumption result = %#v, want Row", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &rows, deployment
}

func patternConsumptionRowKey(row Row) string {
	return fmt.Sprintf("%v|%v|%v", row.Get("a").Any(), row.Get("b").Any(), row.Get("c").Any())
}

func TestPatternFilterConsumeFollowedByMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	pattern := PatternFrom(base, "a", Literal(true)).Every().
		FollowedBy("b", Literal(true)).Consume(1)
	rows, deployment := collectPatternConsumptionRows(t, env, engine, pattern, "pattern-filter-consume-followed-by")
	defer deployment.Undeploy(context.Background())

	for _, symbol := range []string{"E1", "E2", "E3", "E4", "E5", "E6"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"E1|E2|<nil>", "E3|E4|<nil>", "E5|E6|<nil>"}
	if len(*rows) != len(want) {
		t.Fatalf("followed-by @consume rows = %#v, want %v", *rows, want)
	}
	for index, expected := range want {
		if got := patternConsumptionRowKey((*rows)[index]); got != expected {
			t.Fatalf("followed-by @consume row[%d] = %q, want %q", index, got, expected)
		}
	}
}

func TestPatternFilterConsumeAndPreservesSuppressedBranch(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	left := PatternFrom(base, "a", Literal(true))
	right := PatternFrom(base, "b", Equal[float64](Field[runtimeTestTrade, float64]("price"), Literal(10.0))).Consume(2)
	rows, deployment := collectPatternConsumptionRows(t, env, engine, left.And(right), "pattern-filter-consume-and")
	defer deployment.Undeploy(context.Background())

	for _, event := range []runtimeTestTrade{
		{Symbol: "E1", Price: 10},
		{Symbol: "E2", Price: 20},
		{Symbol: "E3", Price: 1},
		{Symbol: "E4", Price: 1},
		{Symbol: "E5", Price: 10},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"E2|E1|<nil>", "E3|E5|<nil>"}
	if len(*rows) != len(want) {
		t.Fatalf("and @consume rows = %#v, want %v", *rows, want)
	}
	for index, expected := range want {
		if got := patternConsumptionRowKey((*rows)[index]); got != expected {
			t.Fatalf("and @consume row[%d] = %q, want %q", index, got, expected)
		}
	}
}

func TestPatternFilterConsumeAndSceneTwoMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	a := PatternFrom(base, "a", Literal(true))
	b := PatternFrom(base, "b", StartsWith(Field[runtimeTestTrade, string]("symbol"), Literal("A"))).Consume()
	pattern := a.And(b)
	plan, err := env.Build(pattern.Select(
		Alias("a", TagField[string]("a", "symbol")),
		Alias("b", TagField[string]("b", "symbol")),
	).Query(StatementName("pattern-filter-consume-scene-two")))
	if err != nil {
		t.Fatal(err)
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
			if ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("scene-two first event rows = %#v, want none", rows)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "X"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("a").Any() != "X" || rows[0].Get("b").Any() != "A" {
		t.Fatalf("scene-two rows = %#v, want X/A", rows)
	}
}

func TestPatternFilterConsumeOrSelectsHighestLevelAndFansOutTies(t *testing.T) {
	cases := []struct {
		name     string
		levels   []int
		wantRows []string
	}{
		{name: "left-wins", levels: []int{2, 1}, wantRows: []string{"A|<nil>|<nil>"}},
		{name: "right-wins", levels: []int{1, 2}, wantRows: []string{"<nil>|<nil>|B"}},
		{name: "equal-level-fanout", levels: []int{2, 2}, wantRows: []string{"A|<nil>|<nil>", "<nil>|<nil>|A"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env, engine := newRuntimeTest(t)
			base := From[runtimeTestTrade](env, "Trade")
			left := PatternFrom(base, "a", Literal(true)).Consume(testCase.levels[0])
			right := PatternFrom(base, "c", Literal(true)).Consume(testCase.levels[1])
			rows, deployment := collectPatternConsumptionRows(t, env, engine, left.Or(right), "pattern-filter-consume-or-"+testCase.name)
			defer deployment.Undeploy(context.Background())
			if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
				t.Fatal(err)
			}
			if testCase.name == "right-wins" {
				// The right branch uses tag c; its projected value is kept in
				// the third column to make the selected alternative explicit.
				testCase.wantRows = []string{"<nil>|<nil>|A"}
			}
			if len(*rows) != len(testCase.wantRows) {
				t.Fatalf("or @consume rows = %#v, want %v", *rows, testCase.wantRows)
			}
			seen := make(map[string]struct{}, len(*rows))
			for _, row := range *rows {
				seen[patternConsumptionRowKey(row)] = struct{}{}
			}
			for _, expected := range testCase.wantRows {
				if _, ok := seen[expected]; !ok {
					t.Fatalf("or @consume rows = %#v, missing %q", *rows, expected)
				}
			}
		})
	}
}

func TestPatternFilterConsumeNestedOrMatrixMatchesEsper(t *testing.T) {
	cases := []struct {
		name     string
		levels   [3]int
		wantRows []string
	}{
		{name: "c-highest", levels: [3]int{1, 2, 3}, wantRows: []string{"<nil>|<nil>|E"}},
		{name: "b-and-c-tie", levels: [3]int{1, 2, 2}, wantRows: []string{"<nil>|E|<nil>", "<nil>|<nil>|E"}},
		{name: "all-tie", levels: [3]int{2, 2, 2}, wantRows: []string{"E|<nil>|<nil>", "<nil>|E|<nil>", "<nil>|<nil>|E"}},
		{name: "a-and-b-tie", levels: [3]int{2, 2, 1}, wantRows: []string{"E|<nil>|<nil>", "<nil>|E|<nil>"}},
		{name: "a-and-c-tie", levels: [3]int{2, 1, 2}, wantRows: []string{"E|<nil>|<nil>", "<nil>|<nil>|E"}},
		{name: "zero-keeps-all", levels: [3]int{0, 0, 0}, wantRows: []string{"E|<nil>|<nil>", "<nil>|E|<nil>", "<nil>|<nil>|E"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env, engine := newRuntimeTest(t)
			base := From[runtimeTestTrade](env, "Trade")
			a := PatternFrom(base, "a", Literal(true)).Consume(testCase.levels[0])
			b := PatternFrom(base, "b", Literal(true)).Consume(testCase.levels[1])
			c := PatternFrom(base, "c", Literal(true)).Consume(testCase.levels[2])
			rows, deployment := collectPatternConsumptionRows(t, env, engine, a.Or(b).Or(c), "pattern-filter-consume-nested-or-"+testCase.name)
			defer deployment.Undeploy(context.Background())
			if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E"}); err != nil {
				t.Fatal(err)
			}
			if len(*rows) != len(testCase.wantRows) {
				t.Fatalf("nested or @consume rows = %#v, want %v", *rows, testCase.wantRows)
			}
			seen := make(map[string]struct{}, len(*rows))
			for _, row := range *rows {
				seen[patternConsumptionRowKey(row)] = struct{}{}
			}
			for _, expected := range testCase.wantRows {
				if _, ok := seen[expected]; !ok {
					t.Fatalf("nested or @consume rows = %#v, missing %q", *rows, expected)
				}
			}
		})
	}
}

func TestPatternFilterConsumeContextKeepsHighestLevelSemantics(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if _, err := CreateKeyContext(env, "pattern-consume-context", Literal("constant")); err != nil {
		t.Fatal(err)
	}
	base := From[runtimeTestTrade](env, "Trade")
	pattern := PatternFrom(base, "a", Literal(true)).Every().
		FollowedBy("b", Literal(true)).Consume(1)
	plan, err := env.Build(pattern.Select(
		Alias("aPrice", TagField[float64]("a", "price")),
		Alias("bPrice", TagField[float64]("b", "price")),
	).Query(StatementName("pattern-filter-consume-context"), WithContext("pattern-consume-context")))
	if err != nil {
		t.Fatal(err)
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
			if ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "S", Price: 1}, {Symbol: "S", Price: 2}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 1 || rows[0].Get("aPrice").Any() != float64(1) || rows[0].Get("bPrice").Any() != float64(2) {
		t.Fatalf("context @consume rows = %#v, want 1/2", rows)
	}
}

func TestPatternFilterConsumeEveryBranchesKeepSameLevelFanout(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	a := PatternFrom(base, "a", Literal(true)).Consume(1).Every()
	b := PatternFrom(base, "b", Literal(true)).Consume(2).Every()
	c := PatternFrom(base, "c", Literal(true)).Consume(2).Every()
	rows, deployment := collectPatternConsumptionRows(t, env, engine, a.Or(b).Or(c), "pattern-filter-consume-every-or")
	defer deployment.Undeploy(context.Background())
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E"}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 2 {
		t.Fatalf("every-branch @consume rows = %#v, want two level-2 rows", *rows)
	}
	seen := map[string]struct{}{}
	for _, row := range *rows {
		seen[patternConsumptionRowKey(row)] = struct{}{}
	}
	for _, expected := range []string{"<nil>|E|<nil>", "<nil>|<nil>|E"} {
		if _, ok := seen[expected]; !ok {
			t.Fatalf("every-branch @consume rows = %#v, missing %q", *rows, expected)
		}
	}
}

func TestPatternFilterConsumeUnannotatedFiltersYieldOnlyWithoutConsumptionMatch(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	a := PatternFrom(base, "a", Equal[float64](Field[runtimeTestTrade, float64]("price"), Literal(11.0))).Consume(1).Every()
	b := PatternFrom(base, "b", Literal(true)).Every()
	rows, deployment := collectPatternConsumptionRows(t, env, engine, a.Or(b), "pattern-filter-consume-unannotated")
	defer deployment.Undeploy(context.Background())
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E1", Price: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E2", Price: 11}); err != nil {
		t.Fatal(err)
	}
	want := []string{"<nil>|E1|<nil>", "E2|<nil>|<nil>"}
	if len(*rows) != len(want) {
		t.Fatalf("unannotated @consume rows = %#v, want %v", *rows, want)
	}
	seen := make(map[string]struct{}, len(*rows))
	for _, row := range *rows {
		seen[patternConsumptionRowKey(row)] = struct{}{}
	}
	for _, expected := range want {
		if _, ok := seen[expected]; !ok {
			t.Fatalf("unannotated @consume rows = %#v, missing %q", *rows, expected)
		}
	}
}

func TestPatternFilterConsumeZeroAndNegativeValidation(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	selection := Alias("a", TagField[string]("a", "symbol"))
	plain, err := env.Build(PatternFrom(base, "a", Literal(true)).Select(
		selection,
	).Query(StatementName("pattern-filter-consume-identity")))
	if err != nil {
		t.Fatalf("plain pattern unexpectedly failed: %v", err)
	}
	valid, err := env.Build(PatternFrom(base, "a", Literal(true)).Consume(0).Select(
		selection,
	).Query(StatementName("pattern-filter-consume-identity")))
	if err != nil {
		t.Fatalf("consume(0) unexpectedly failed: %v", err)
	}
	if plain.Hash() == valid.Hash() {
		t.Fatalf("consume(0) annotation did not change plan hash: %s", valid.Hash())
	}
	if _, err := env.Build(PatternFrom(base, "a", Literal(true)).Consume(-1).Select(
		Alias("a", TagField[string]("a", "symbol")),
	).Query(StatementName("pattern-filter-consume-negative"))); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("negative consume level error = %v, want ErrorInvalidRule", err)
	}
	if _, err := env.Build(PatternFrom(base, "a", Literal(true)).Consume(0, 1).Select(
		Alias("a", TagField[string]("a", "symbol")),
	).Query(StatementName("pattern-filter-consume-arity"))); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("consume arity error = %v, want ErrorInvalidRule", err)
	}
}

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
