package esper

import (
	"context"
	"testing"
)

// TestPatternSubqueryFilterAndNamedWindowVariantsMatchesEsper covers the
// three branches not exercised by TestPatternSubqueryInLastEventMatchesEsper:
// an event-stream filter, a named-window filter, and a named-window pattern.
// All branches use the exact tryAssertion sequence from
// EPLSubselectWithinPattern.EPLSubselectFilterPatternNamedWindowNoAlias.
func TestPatternSubqueryFilterAndNamedWindowVariantsMatchesEsper(t *testing.T) {
	scenarios := []struct {
		name        string
		pattern     bool
		namedWindow bool
	}{
		{name: "event-stream-filter"},
		{name: "named-window-filter", namedWindow: true},
		{name: "named-window-pattern", pattern: true, namedWindow: true},
	}
	for _, scenario := range scenarios {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			runPatternSubqueryFilterVariant(t, scenario.pattern, scenario.namedWindow)
		})
	}
}

func runPatternSubqueryFilterVariant(t *testing.T, pattern, namedWindow bool) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[patternSubqueryCandidate](env, "PatternSubqueryVariantCandidate"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternSubqueryReference](env, "PatternSubqueryVariantReference"); err != nil {
		t.Fatal(err)
	}

	var inner RecordStream
	var referenceProjection Expression[string]
	if namedWindow {
		schema, ok := env.Schema("PatternSubqueryVariantReference")
		if !ok {
			t.Fatal("named-window reference schema is missing")
		}
		if _, err := CreateNamedWindow(env, "PatternSubqueryVariantWindow", schema, NamedWindowRetention(LastEvent())); err != nil {
			t.Fatal(err)
		}
		inner = FromNamedWindow(env, "PatternSubqueryVariantWindow")
		referenceProjection = Field[any, string]("symbol")
	} else {
		inner = Select(From[patternSubqueryReference](env, "PatternSubqueryVariantReference")).Window(LastEvent())
		referenceProjection = Field[patternSubqueryReference, string]("symbol")
	}

	predicate := SubqueryIn[string](
		Field[patternSubqueryCandidate, string]("symbol"),
		inner,
		referenceProjection,
	)
	var query Query
	if pattern {
		query = PatternFrom(
			From[patternSubqueryCandidate](env, "PatternSubqueryVariantCandidate"),
			"candidate",
			predicate,
		).Every().Select(
			Alias("id", TagField[int64]("candidate", "id")),
		).Query(StatementName("pattern-subquery-named-window"))
	} else {
		query = Select(
			From[patternSubqueryCandidate](env, "PatternSubqueryVariantCandidate").Filter(predicate),
			Alias("id", Field[patternSubqueryCandidate, int64]("id")),
		).Query(StatementName("subquery-named-window-filter"))
	}
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var insertDeployment *Deployment
	if namedWindow {
		insertPlan, buildErr := env.Build(OnEvent(
			From[patternSubqueryReference](env, "PatternSubqueryVariantReference"),
		).InsertIntoNamedWindow(
			"PatternSubqueryVariantWindow",
			SetColumn("id", Field[patternSubqueryReference, int64]("id")),
			SetColumn("symbol", Field[patternSubqueryReference, string]("symbol")),
		).Query(StatementName("subquery-named-window-insert")))
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		insertDeployment, err = engine.Deploy(context.Background(), insertPlan)
		if err != nil {
			t.Fatal(err)
		}
	}

	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("pattern subquery variant result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendCandidate := func(id int64, symbol string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), patternSubqueryCandidate{ID: id, Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	sendReference := func(id int64, symbol string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), patternSubqueryReference{ID: id, Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	assertIDs := func(want ...int64) {
		t.Helper()
		if len(rows) != len(want) {
			t.Fatalf("pattern subquery variant rows = %#v, want ids %#v", rows, want)
		}
		for index, wantID := range want {
			if got := rows[index].Get("id").Any(); got != wantID {
				t.Fatalf("pattern subquery variant row %d id = %#v, want %d", index, got, wantID)
			}
		}
	}

	sendCandidate(1, "A")
	sendReference(2, "A")
	sendCandidate(3, "B")
	sendReference(4, "C")
	assertIDs()

	sendCandidate(5, "C")
	assertIDs(5)

	sendCandidate(6, "A")
	sendCandidate(7, "D")
	sendReference(8, "E")
	sendCandidate(9, "C")
	assertIDs(5)

	sendCandidate(10, "E")
	assertIDs(5, 10)

	if insertDeployment != nil {
		if err := engine.Undeploy(context.Background(), insertDeployment.ID()); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.Undeploy(context.Background(), deployment.ID()); err != nil {
		t.Fatal(err)
	}
}
