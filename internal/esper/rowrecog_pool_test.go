package esper

import (
	"context"
	"testing"
)

type rowRecogStatePoolEvent struct {
	Group string `esper:"group"`
	Phase int    `esper:"phase"`
	ID    int64  `esper:"id"`
}

func newRowRecogStatePoolPlan(t *testing.T, env *Environment, name string) Plan {
	t.Helper()
	stream := From[rowRecogStatePoolEvent](env, "RowRecogStatePoolEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(RowVar("P1"), RowVar("P2"))).
		PartitionBy(Field[rowRecogStatePoolEvent, string]("group")).
		Define("P1", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(1))).
		Define("P2", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(2))).
		Measures(Alias("id", TagField[int64]("P1", "id"))).
		Query(StatementName(name))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func newFilteredRowRecogStatePoolPlan(t *testing.T, env *Environment, name, group string) Plan {
	t.Helper()
	stream := From[rowRecogStatePoolEvent](env, "RowRecogStatePoolEvent").
		Filter(Equal[string](Field[rowRecogStatePoolEvent, string]("group"), Literal(group))).
		Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(RowVar("P1"), RowVar("P2"), RowVar("P3"))).
		Define("P1", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(1))).
		Define("P2", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(1))).
		Define("P3", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(2))).
		Measures(Alias("id", TagField[int64]("P1", "id"))).
		Query(StatementName(name))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func newRowRecogStatePoolTest(t *testing.T, options ...EngineOption) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[rowRecogStatePoolEvent](env, "RowRecogStatePoolEvent"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env, options...)
}

func sendRowRecogStatePoolEvent(t *testing.T, engine *Engine, group string, phase int, id int64) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), rowRecogStatePoolEvent{Group: group, Phase: phase, ID: id}); err != nil {
		t.Fatal(err)
	}
}

func TestRowRecogEngineWideMaxStatesPreventStart(t *testing.T) {
	env, engine := newRowRecogStatePoolTest(t, WithMatchRecognizeStateLimit(3, true))
	plan := newRowRecogStatePoolPlan(t, env, "rowrecog-state-prevent")
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	var limits []MatchRecognizeStateLimitEvent
	listener := MatchRecognizeStateLimitListenerFunc(func(event MatchRecognizeStateLimitEvent) {
		_ = engine.Now()
		limits = append(limits, event)
	})
	if err := engine.AddMatchRecognizeStateLimitListener(listener); err != nil {
		t.Fatal(err)
	}

	sendRowRecogStatePoolEvent(t, engine, "A", 1, 1)
	sendRowRecogStatePoolEvent(t, engine, "B", 1, 2)
	sendRowRecogStatePoolEvent(t, engine, "C", 1, 3)
	sendRowRecogStatePoolEvent(t, engine, "D", 1, 4)
	if len(limits) != 1 {
		t.Fatalf("limit events = %#v, want one overflow", limits)
	}
	limit := limits[0]
	if limit.MaxStates != 3 || limit.Counts[deployment.Statements()[0].ID()] != 3 {
		t.Fatalf("limit event = %#v, want max 3 and statement count 3", limit)
	}

	// D was rejected as a new start, but the row remains available to any
	// already-admitted branch. Its own partition therefore cannot match later.
	sendRowRecogStatePoolEvent(t, engine, "D", 2, 40)
	if len(*rows) != 0 {
		t.Fatalf("blocked D produced rows = %#v", *rows)
	}
	sendRowRecogStatePoolEvent(t, engine, "A", 2, 41)
	if len(*rows) != 1 || (*rows)[0].Get("id").Any() != int64(1) {
		t.Fatalf("admitted A match = %#v", *rows)
	}

	// The completed A branch releases one slot, so E is admitted without a
	// second condition event.
	sendRowRecogStatePoolEvent(t, engine, "E", 1, 5)
	if len(limits) != 1 {
		t.Fatalf("limit events after released slot = %#v", limits)
	}
}

func TestRowRecogEngineWideMaxStatesNoPreventStart(t *testing.T) {
	env, engine := newRowRecogStatePoolTest(t, WithMatchRecognizeStateLimit(3, false))
	plan := newRowRecogStatePoolPlan(t, env, "rowrecog-state-no-prevent")
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	var limits []MatchRecognizeStateLimitEvent
	if err := engine.AddMatchRecognizeStateLimitListener(MatchRecognizeStateLimitListenerFunc(func(event MatchRecognizeStateLimitEvent) {
		limits = append(limits, event)
	})); err != nil {
		t.Fatal(err)
	}

	sendRowRecogStatePoolEvent(t, engine, "A", 1, 1)
	sendRowRecogStatePoolEvent(t, engine, "B", 1, 2)
	sendRowRecogStatePoolEvent(t, engine, "C", 1, 3)
	sendRowRecogStatePoolEvent(t, engine, "D", 1, 4)
	if len(limits) != 1 || limits[0].Counts[deployment.Statements()[0].ID()] != 3 {
		t.Fatalf("first no-prevent overflow = %#v", limits)
	}
	sendRowRecogStatePoolEvent(t, engine, "E", 1, 5)
	if len(limits) != 2 || limits[1].Counts[deployment.Statements()[0].ID()] != 4 {
		t.Fatalf("second no-prevent overflow = %#v", limits)
	}

	sendRowRecogStatePoolEvent(t, engine, "D", 2, 40)
	if len(*rows) != 1 || (*rows)[0].Get("id").Any() != int64(4) {
		t.Fatalf("no-prevent D match = %#v", *rows)
	}
}

func TestRowRecogEngineWideMaxStatesUndeployReleasesPool(t *testing.T) {
	env, engine := newRowRecogStatePoolTest(t, WithMatchRecognizeStateLimit(3, true))
	firstPlan := newRowRecogStatePoolPlan(t, env, "rowrecog-state-first")
	first, err := engine.Deploy(context.Background(), firstPlan)
	if err != nil {
		t.Fatal(err)
	}
	var limits []MatchRecognizeStateLimitEvent
	if err := engine.AddMatchRecognizeStateLimitListener(MatchRecognizeStateLimitListenerFunc(func(event MatchRecognizeStateLimitEvent) {
		limits = append(limits, event)
	})); err != nil {
		t.Fatal(err)
	}
	for _, event := range []rowRecogStatePoolEvent{{Group: "A", Phase: 1, ID: 1}, {Group: "B", Phase: 1, ID: 2}, {Group: "C", Phase: 1, ID: 3}} {
		sendRowRecogStatePoolEvent(t, engine, event.Group, event.Phase, event.ID)
	}

	secondPlan := newRowRecogStatePoolPlan(t, env, "rowrecog-state-second")
	second, err := engine.Deploy(context.Background(), secondPlan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Undeploy(context.Background(), first.ID()); err != nil {
		t.Fatal(err)
	}
	limits = nil
	for _, event := range []rowRecogStatePoolEvent{{Group: "D", Phase: 1, ID: 4}, {Group: "E", Phase: 1, ID: 5}, {Group: "F", Phase: 1, ID: 6}} {
		sendRowRecogStatePoolEvent(t, engine, event.Group, event.Phase, event.ID)
	}
	if len(limits) != 0 {
		t.Fatalf("state-limit events after undeploy = %#v", limits)
	}
	if len(second.Statements()) != 1 {
		t.Fatalf("second deployment statements = %#v", second.Statements())
	}
}

func TestRowRecogEngineWideMaxStatesAcrossStatementsAndContext(t *testing.T) {
	env, engine := newRowRecogStatePoolTest(t, WithMatchRecognizeStateLimit(3, true))
	first, err := engine.Deploy(context.Background(), newFilteredRowRecogStatePoolPlan(t, env, "rowrecog-state-A", "A"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.Deploy(context.Background(), newFilteredRowRecogStatePoolPlan(t, env, "rowrecog-state-B", "B"))
	if err != nil {
		t.Fatal(err)
	}
	firstRows := collectRowRecogRows(t, first)
	secondRows := collectRowRecogRows(t, second)
	var limits []MatchRecognizeStateLimitEvent
	if err := engine.AddMatchRecognizeStateLimitListener(MatchRecognizeStateLimitListenerFunc(func(event MatchRecognizeStateLimitEvent) {
		limits = append(limits, event)
	})); err != nil {
		t.Fatal(err)
	}
	sendRowRecogStatePoolEvent(t, engine, "A", 1, 1)
	sendRowRecogStatePoolEvent(t, engine, "B", 1, 2)
	sendRowRecogStatePoolEvent(t, engine, "A", 1, 3)
	sendRowRecogStatePoolEvent(t, engine, "B", 1, 4)
	if len(limits) != 1 {
		t.Fatalf("cross-statement limit events = %#v", limits)
	}
	counts := limits[0].Counts
	if counts[first.Statements()[0].ID()] != 2 || counts[second.Statements()[0].ID()] != 1 {
		t.Fatalf("cross-statement counts = %#v", counts)
	}
	sendRowRecogStatePoolEvent(t, engine, "B", 2, 40)
	if len(*secondRows) != 1 || (*secondRows)[0].Get("id").Any() != int64(2) {
		t.Fatalf("second statement match = %#v", *secondRows)
	}

	if _, err := CreateKeyContext(env, "rowrecog-state-context", Field[rowRecogStatePoolEvent, string]("group")); err != nil {
		t.Fatal(err)
	}
	contextStream := From[rowRecogStatePoolEvent](env, "RowRecogStatePoolEvent").Window(KeepAll())
	contextQuery := contextStream.MatchRecognize(RowSequence(RowVar("P1"), RowVar("P2"))).
		Define("P1", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(1))).
		Define("P2", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(2))).
		Measures(Alias("id", TagField[int64]("P1", "id"))).
		Query(StatementName("rowrecog-state-context-statement"), WithContext("rowrecog-state-context"))
	contextPlan, err := env.Build(contextQuery)
	if err != nil {
		t.Fatal(err)
	}
	contextDeployment, err := engine.Deploy(context.Background(), contextPlan)
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range []string{"C", "D"} {
		sendRowRecogStatePoolEvent(t, engine, group, 1, int64(len(group)+10))
	}
	if len(limits) != 2 {
		t.Fatalf("context overflow events = %#v", limits)
	}
	contextCounts := limits[1].Counts
	if contextCounts[contextDeployment.Statements()[0].ID()] != 1 {
		t.Fatalf("context statement count = %#v", contextCounts)
	}
	if contextDeployment.Statements()[0].ContextPartitionCount() != 2 {
		t.Fatalf("context partition count = %d", contextDeployment.Statements()[0].ContextPartitionCount())
	}
	if err := engine.Undeploy(context.Background(), contextDeployment.ID()); err != nil {
		t.Fatal(err)
	}
	if len(*firstRows) != 0 {
		t.Fatalf("first statement unexpectedly emitted = %#v", *firstRows)
	}
}

func TestRowRecogEngineWideMaxStatesNamedWindowRemoval(t *testing.T) {
	env, _ := newRowRecogStatePoolTest(t)
	schema, ok := env.Schema("RowRecogStatePoolEvent")
	if !ok {
		t.Fatal("state-pool schema is missing")
	}
	if _, err := CreateNamedWindow(env, "rowrecog-state-window", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithMatchRecognizeStateLimit(3, true))
	query := FromNamedWindow(env, "rowrecog-state-window").
		MatchRecognize(RowSequence(RowVar("P1"), RowVar("P2"))).
		PartitionBy(Field[rowRecogStatePoolEvent, string]("group")).
		Define("P1", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(1))).
		Define("P2", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(2))).
		Measures(Alias("id", TagField[int64]("P1", "id"))).
		Query(StatementName("rowrecog-state-window-statement"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	var limits []MatchRecognizeStateLimitEvent
	if err := engine.AddMatchRecognizeStateLimitListener(MatchRecognizeStateLimitListenerFunc(func(event MatchRecognizeStateLimitEvent) {
		limits = append(limits, event)
	})); err != nil {
		t.Fatal(err)
	}
	for _, event := range []rowRecogStatePoolEvent{{Group: "A", Phase: 1, ID: 1}, {Group: "B", Phase: 1, ID: 2}, {Group: "C", Phase: 1, ID: 3}, {Group: "D", Phase: 1, ID: 4}} {
		if err := engine.InsertNamedWindow(context.Background(), "rowrecog-state-window", event); err != nil {
			t.Fatal(err)
		}
	}
	if len(limits) != 1 {
		t.Fatalf("named-window overflow events = %#v", limits)
	}

	trigger := From[rowRecogStatePoolEvent](env, "RowRecogStatePoolEvent")
	deletePlan, err := env.Build(OnEvent(trigger).DeleteFromNamedWindow(
		"rowrecog-state-window",
		Equal[string](NamedWindowField[string]("group"), Field[rowRecogStatePoolEvent, string]("group")),
	).Query(StatementName("rowrecog-state-window-delete")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), deletePlan); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), rowRecogStatePoolEvent{Group: "A", Phase: 99, ID: 50}); err != nil {
		t.Fatal(err)
	}
	limits = nil
	if err := engine.InsertNamedWindow(context.Background(), "rowrecog-state-window", rowRecogStatePoolEvent{Group: "E", Phase: 1, ID: 5}); err != nil {
		t.Fatal(err)
	}
	if len(limits) != 0 {
		t.Fatalf("named-window removal did not free a slot = %#v", limits)
	}
	if len(*rows) != 0 {
		t.Fatalf("named-window state test emitted unexpected rows = %#v", *rows)
	}
}
