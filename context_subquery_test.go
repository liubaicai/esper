package esper

import (
	"context"
	"testing"
	"time"
)

type contextSubqueryOuter struct {
	Symbol string `esper:"symbol"`
	ID     int64  `esper:"id"`
}

type contextSubqueryReference struct {
	ID    int64  `esper:"id"`
	Value string `esper:"value"`
}

func TestContextEventStreamSubqueryKeepsPartitionLocalLastEvent(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextSubqueryOuter](env, "ContextSubqueryOuter"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[contextSubqueryReference](env, "ContextSubqueryReference"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "context-subquery-by-symbol", Field[contextSubqueryOuter, string]("symbol")); err != nil {
		t.Fatal(err)
	}

	inner := Select(From[contextSubqueryReference](env, "ContextSubqueryReference")).Window(LastEvent())
	plan, err := env.Build(Select(
		From[contextSubqueryOuter](env, "ContextSubqueryOuter"),
		Alias("symbol", Field[contextSubqueryOuter, string]("symbol")),
		Alias("id", Field[contextSubqueryOuter, int64]("id")),
		Alias("value", SubqueryValue[string](
			inner,
			Field[contextSubqueryReference, string]("value"),
			Equal[int64](Field[contextSubqueryReference, int64]("id"), OuterField[int64]("id")),
		)),
	).Query(StatementName("context-subquery"), WithContext("context-subquery-by-symbol")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("context subquery result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendReference := func(id int64, value string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), contextSubqueryReference{ID: id, Value: value}); err != nil {
			t.Fatal(err)
		}
	}
	sendOuter := func(symbol string, id int64, wantValue string, wantNull bool) {
		t.Helper()
		before := len(rows)
		if err := engine.SendEvent(context.Background(), contextSubqueryOuter{Symbol: symbol, ID: id}); err != nil {
			t.Fatal(err)
		}
		if len(rows) != before+1 {
			t.Fatalf("context subquery rows after %s = %d, want %d", symbol, len(rows), before+1)
		}
		row := rows[len(rows)-1]
		if row.Get("symbol").Any() != symbol || row.Get("id").Any() != id {
			t.Fatalf("context subquery outer row = %#v", row.AsMap())
		}
		value := row.Get("value")
		if value.IsNull() != wantNull || (!wantNull && value.Any() != wantValue) {
			t.Fatalf("context subquery value for %s = %#v, want %q (null=%v)", symbol, value.Any(), wantValue, wantNull)
		}
	}

	// Existing partitions do not retroactively receive an inner event that
	// arrived before their creation, matching Esper's #lastevent behavior.
	sendReference(10, "s1")
	sendOuter("G1", 10, "", true)
	sendReference(10, "s2")
	sendOuter("G1", 10, "s2", false)
	sendOuter("G2", 10, "", true)
	sendReference(10, "s3")
	sendOuter("G2", 10, "s3", false)
	sendOuter("G3", 10, "", true)
	sendOuter("G1", 10, "s3", false)
}

func TestTemporalContextEventStreamSubqueryResetsAtCalendarBoundary(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextSubqueryOuter](env, "TemporalContextSubqueryOuter"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[contextSubqueryReference](env, "TemporalContextSubqueryReference"); err != nil {
		t.Fatal(err)
	}
	start, err := NewTimeOfDay(9, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	end, err := NewTimeOfDay(17, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateDailyTimeContext(env, "temporal-subquery", start, end); err != nil {
		t.Fatal(err)
	}

	inner := Select(From[contextSubqueryReference](env, "TemporalContextSubqueryReference")).Window(LastEvent())
	plan, err := env.Build(Select(
		From[contextSubqueryOuter](env, "TemporalContextSubqueryOuter"),
		Alias("symbol", Field[contextSubqueryOuter, string]("symbol")),
		Alias("id", Field[contextSubqueryOuter, int64]("id")),
		Alias("value", SubqueryValue[string](
			inner,
			Field[contextSubqueryReference, string]("value"),
			Equal[int64](Field[contextSubqueryReference, int64]("id"), OuterField[int64]("id")),
		)),
	).Query(StatementName("temporal-context-subquery"), WithContext("temporal-subquery")))
	if err != nil {
		t.Fatal(err)
	}

	location := time.FixedZone("temporal-subquery", 8*60*60)
	origin := time.Date(2024, time.May, 1, 8, 0, 0, 0, location)
	engine := NewEngine(env, WithStartTime(origin))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var rows []Row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("temporal context subquery result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	advance := func(at time.Time) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), at); err != nil {
			t.Fatal(err)
		}
	}
	sendReference := func(id int64, value string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), contextSubqueryReference{ID: id, Value: value}); err != nil {
			t.Fatal(err)
		}
	}
	sendOuter := func(symbol string, id int64, wantValue string, wantNull bool) {
		t.Helper()
		before := len(rows)
		if err := engine.SendEvent(context.Background(), contextSubqueryOuter{Symbol: symbol, ID: id}); err != nil {
			t.Fatal(err)
		}
		if len(rows) != before+1 {
			t.Fatalf("temporal context subquery rows after %s = %d, want %d", symbol, len(rows), before+1)
		}
		value := rows[len(rows)-1].Get("value")
		if value.IsNull() != wantNull || (!wantNull && value.Any() != wantValue) {
			t.Fatalf("temporal context subquery value for %s = %#v, want %q (null=%v)", symbol, value.Any(), wantValue, wantNull)
		}
	}

	// Events received while the daily context is inactive are not replayed
	// into the partition opened at the next calendar start.
	sendReference(1, "before-first-window")
	advance(time.Date(2024, time.May, 1, 9, 0, 0, 0, location))
	if statement.ContextPartitionCount() != 1 {
		t.Fatalf("temporal context did not open = %v", statement.ContextPartitions())
	}
	sendOuter("E1", 1, "", true)
	sendReference(1, "S01")
	sendOuter("E2", 1, "S01", false)

	advance(time.Date(2024, time.May, 1, 17, 0, 0, 0, location))
	if statement.ContextPartitionCount() != 0 {
		t.Fatalf("temporal context remained open at end = %v", statement.ContextPartitions())
	}
	sendReference(1, "between-windows")
	advance(time.Date(2024, time.May, 2, 9, 0, 0, 0, location))
	if statement.ContextPartitionCount() != 1 {
		t.Fatalf("temporal context did not reopen = %v", statement.ContextPartitions())
	}
	sendOuter("E3", 1, "", true)
	sendReference(1, "S02")
	sendOuter("E4", 1, "S02", false)
}

func TestInitiatedContextEventStreamSubqueryReleasesPartitionState(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextLifecycleEvent](env, "InitiatedSubqueryLifecycle"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[contextSubqueryOuter](env, "InitiatedSubqueryOuter"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[contextSubqueryReference](env, "InitiatedSubqueryReference"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateInitiatedTerminatedContext(
		env,
		"initiated-subquery",
		Literal("A"),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("start")),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("end")),
	); err != nil {
		t.Fatal(err)
	}

	inner := Select(From[contextSubqueryReference](env, "InitiatedSubqueryReference")).Window(LastEvent())
	plan, err := env.Build(Select(
		From[contextSubqueryOuter](env, "InitiatedSubqueryOuter"),
		Alias("symbol", Field[contextSubqueryOuter, string]("symbol")),
		Alias("id", Field[contextSubqueryOuter, int64]("id")),
		Alias("value", SubqueryValue[string](
			inner,
			Field[contextSubqueryReference, string]("value"),
			Equal[int64](Field[contextSubqueryReference, int64]("id"), OuterField[int64]("id")),
		)),
	).Query(StatementName("initiated-context-subquery"), WithContext("initiated-subquery")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var rows []Row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("initiated context subquery result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendLifecycle := func(id, kind string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), contextLifecycleEvent{ID: id, Kind: kind}); err != nil {
			t.Fatal(err)
		}
	}
	sendReference := func(id int64, value string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), contextSubqueryReference{ID: id, Value: value}); err != nil {
			t.Fatal(err)
		}
	}
	sendOuter := func(symbol string, id int64, wantValue string, wantNull bool) {
		t.Helper()
		before := len(rows)
		if err := engine.SendEvent(context.Background(), contextSubqueryOuter{Symbol: symbol, ID: id}); err != nil {
			t.Fatal(err)
		}
		if len(rows) != before+1 {
			t.Fatalf("initiated context subquery rows after %s = %d, want %d", symbol, len(rows), before+1)
		}
		value := rows[len(rows)-1].Get("value")
		if value.IsNull() != wantNull || (!wantNull && value.Any() != wantValue) {
			t.Fatalf("initiated context subquery value for %s = %#v, want %q (null=%v)", symbol, value.Any(), wantValue, wantNull)
		}
	}

	// No active initiated partition exists before the start boundary.
	sendReference(1, "before-start")
	sendLifecycle("A", "start")
	if statement.ContextPartitionCount() != 1 {
		t.Fatalf("initiated context did not start = %v", statement.ContextPartitions())
	}
	sendOuter("E1", 1, "", true)
	sendReference(1, "S01")
	sendOuter("E2", 1, "S01", false)

	sendLifecycle("A", "end")
	if statement.ContextPartitionCount() != 0 {
		t.Fatalf("initiated context did not terminate = %v", statement.ContextPartitions())
	}
	sendReference(1, "between-starts")
	sendLifecycle("A", "start")
	if statement.ContextPartitionCount() != 1 {
		t.Fatalf("initiated context did not restart = %v", statement.ContextPartitions())
	}
	sendOuter("E3", 1, "", true)
}

func TestContextNamedWindowSubqueryUsesGlobalState(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextSubqueryOuter](env, "ContextNamedWindowOuter"); err != nil {
		t.Fatal(err)
	}
	referenceSchema, err := RegisterStruct[contextSubqueryReference](env, "ContextNamedWindowReference")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "context-global-references", referenceSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "context-named-window", Field[contextSubqueryOuter, string]("symbol")); err != nil {
		t.Fatal(err)
	}

	inner := FromNamedWindow(env, "context-global-references")
	plan, err := env.Build(Select(
		From[contextSubqueryOuter](env, "ContextNamedWindowOuter"),
		Alias("symbol", Field[contextSubqueryOuter, string]("symbol")),
		Alias("id", Field[contextSubqueryOuter, int64]("id")),
		Alias("value", SubqueryValue[string](
			inner,
			Field[contextSubqueryReference, string]("value"),
			Equal[int64](Field[contextSubqueryReference, int64]("id"), OuterField[int64]("id")),
		)),
	).Query(StatementName("context-named-window-subquery"), WithContext("context-named-window")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("context named-window subquery result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendOuter := func(symbol string, id int64, wantValue string, wantNull bool) {
		t.Helper()
		before := len(rows)
		if err := engine.SendEvent(context.Background(), contextSubqueryOuter{Symbol: symbol, ID: id}); err != nil {
			t.Fatal(err)
		}
		if len(rows) != before+1 {
			t.Fatalf("context named-window subquery rows after %s = %d, want %d", symbol, len(rows), before+1)
		}
		value := rows[len(rows)-1].Get("value")
		if value.IsNull() != wantNull || (!wantNull && value.Any() != wantValue) {
			t.Fatalf("context named-window subquery value for %s = %#v, want %q (null=%v)", symbol, value.Any(), wantValue, wantNull)
		}
	}

	// The global Named Window is not replayed into a Context-local registry;
	// each outer event evaluates against the current global snapshot instead.
	sendOuter("G1", 10, "", true)
	if err := engine.InsertNamedWindow(context.Background(), "context-global-references", contextSubqueryReference{ID: 10, Value: "S01"}); err != nil {
		t.Fatal(err)
	}
	sendOuter("G1", 10, "S01", false)
	sendOuter("G2", 10, "S01", false)
	if err := engine.InsertNamedWindow(context.Background(), "context-global-references", contextSubqueryReference{ID: 20, Value: "S02"}); err != nil {
		t.Fatal(err)
	}
	sendOuter("G1", 20, "S02", false)
	sendOuter("G2", 20, "S02", false)
}

func TestContextSubquerySnapshotPreservesPartitionRegistry(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextSubqueryOuter](env, "ContextSnapshotOuter"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[contextSubqueryReference](env, "ContextSnapshotReference"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "context-subquery-snapshot", Field[contextSubqueryOuter, string]("symbol")); err != nil {
		t.Fatal(err)
	}

	inner := Select(From[contextSubqueryReference](env, "ContextSnapshotReference")).Window(LastEvent())
	query := Select(
		From[contextSubqueryOuter](env, "ContextSnapshotOuter").Window(LastEvent()),
		Alias("symbol", Field[contextSubqueryOuter, string]("symbol")),
		Alias("value", SubqueryValue[string](
			inner,
			Field[contextSubqueryReference, string]("value"),
			Equal[int64](Field[contextSubqueryReference, int64]("id"), OuterField[int64]("id")),
		)),
	).Query(StatementName("context-subquery-snapshot"), WithContext("context-subquery-snapshot"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]

	if err := engine.SendEvent(context.Background(), contextSubqueryOuter{Symbol: "A", ID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), contextSubqueryReference{ID: 1, Value: "A1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), contextSubqueryOuter{Symbol: "A", ID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), contextSubqueryOuter{Symbol: "B", ID: 1}); err != nil {
		t.Fatal(err)
	}

	keys := statement.ContextPartitionKeys()
	if len(keys) != 2 {
		t.Fatalf("context subquery snapshot partition keys = %v", keys)
	}
	selected, err := statement.SnapshotWithSelector(context.Background(), SelectContextPartitions(keys[0]))
	if err != nil {
		t.Fatal(err)
	}
	if len(selected.Results()) != 1 {
		t.Fatalf("selected context subquery snapshot = %#v, keys=%v", selected.Results(), keys)
	}
	selectedRow, ok := selected.Results()[0].Row()
	if !ok || selectedRow.Get("symbol").Any() != "A" || selectedRow.Get("value").Any() != "A1" {
		t.Fatalf("selected context subquery snapshot row = %#v", selected.Results()[0])
	}

	all, err := statement.SnapshotWithSelector(context.Background(), ContextPartitionSelectorAll{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Results()) != 2 {
		t.Fatalf("all context subquery snapshot = %#v", all.Results())
	}
	for _, result := range all.Results() {
		row, ok := result.Row()
		if !ok {
			t.Fatalf("all context subquery snapshot row = %#v", result)
		}
		if row.Get("symbol").Any() == "A" {
			if row.Get("value").Any() != "A1" {
				t.Fatalf("all context subquery A snapshot row = %#v", result)
			}
		} else if row.Get("symbol").Any() == "B" && !row.Get("value").IsNull() {
			t.Fatalf("all context subquery B snapshot row = %#v", result)
		}
	}
}
