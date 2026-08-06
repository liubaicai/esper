package esper

import (
	"context"
	"testing"
)

type joinSelectClauseEvent struct {
	TheString    string  `esper:"theString"`
	DoubleBoxed  float64 `esper:"doubleBoxed"`
	IntPrimitive int     `esper:"intPrimitive"`
	IntBoxed     int     `esper:"intBoxed"`
}

func TestJoinSelectClauseTypesArithmeticAndSnapshotMatchEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinSelectClauseEvent](env, "JoinSelectS0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinSelectClauseEvent](env, "JoinSelectS1"); err != nil {
		t.Fatal(err)
	}
	s0 := From[joinSelectClauseEvent](env, "JoinSelectS0").
		Filter(Equal[string](Field[joinSelectClauseEvent, string]("theString"), Literal("s0"))).
		Window(LengthWindow(3))
	s1 := From[joinSelectClauseEvent](env, "JoinSelectS1").
		Filter(Equal[string](Field[joinSelectClauseEvent, string]("theString"), Literal("s1"))).
		Window(LengthWindow(3))
	dividend := Multiply[float64](
		Cast[int, float64](Field[joinSelectClauseEvent, int]("intPrimitive")),
		Cast[int, float64](Field[joinSelectClauseEvent, int]("intBoxed")),
	)
	plan, err := env.Build(Join(s0, s1, OnEqual(
		Field[joinSelectClauseEvent, float64]("doubleBoxed"),
		Field[joinSelectClauseEvent, float64]("doubleBoxed"),
	)).Select(
		SelectLeft("doubleBoxed", Field[joinSelectClauseEvent, float64]("doubleBoxed")),
		SelectRight("div", Divide[float64](dividend, Literal(2.0))),
	).Query(StatementName("join-select-clause")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "JoinSelectS0", joinSelectClauseEvent{
		TheString: "s0", DoubleBoxed: 1, IntPrimitive: 4, IntBoxed: 5,
	}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("join select left-only rows = %#v", rows)
	}
	if err := engine.Send(context.Background(), "JoinSelectS1", joinSelectClauseEvent{
		TheString: "s1", DoubleBoxed: 1, IntPrimitive: 3, IntBoxed: 2,
	}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("join select rows = %#v", rows)
	}
	if rows[0].Get("doubleBoxed").Any() != float64(1) || rows[0].Get("div").Any() != float64(3) {
		t.Fatalf("join select projection = %#v", rows[0].AsMap())
	}
	snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
	if err != nil || len(snapshot.Results()) != 1 {
		t.Fatalf("join select snapshot = %#v, err=%v", snapshot.Results(), err)
	}
	row, ok := snapshot.Results()[0].Row()
	if !ok || row.Get("div").Any() != float64(3) {
		t.Fatalf("join select snapshot row = %#v", snapshot.Results()[0])
	}
}

type joinStartStopEvent struct {
	Symbol string `esper:"symbol"`
	Volume int64  `esper:"volume"`
}

func TestJoinDeployLifecycleResetsWindowStateMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinStartStopEvent](env, "JoinStartStopS0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinStartStopEvent](env, "JoinStartStopS1"); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Join(
		From[joinStartStopEvent](env, "JoinStartStopS0").Window(LengthWindow(3)),
		From[joinStartStopEvent](env, "JoinStartStopS1").Window(LengthWindow(3)),
		OnEqual(
			Field[joinStartStopEvent, int64]("volume"),
			Field[joinStartStopEvent, int64]("volume"),
		),
	).Select(
		SelectLeft("left", JoinEventValue[Event](0)),
		SelectRight("right", JoinEventValue[Event](1)),
	).Query(StatementName("join-start-stop")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deploy, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deploy.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "JoinStartStopS0", joinStartStopEvent{Symbol: "IBM", Volume: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "JoinStartStopS1", joinStartStopEvent{Symbol: "CSCO", Volume: 10}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("initial lifecycle join rows = %#v", rows)
	}
	if err := deploy.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "JoinStartStopS0", joinStartStopEvent{Symbol: "IBM", Volume: 20}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "JoinStartStopS1", joinStartStopEvent{Symbol: "CSCO", Volume: 20}); err != nil {
		t.Fatal(err)
	}
	redeploy, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer redeploy.Undeploy(context.Background())
	if _, err := redeploy.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "JoinStartStopS0", joinStartStopEvent{Symbol: "IBM", Volume: 30}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "JoinStartStopS1", joinStartStopEvent{Symbol: "CSCO", Volume: 31}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("redeploy must not replay undeployed state: %#v", rows)
	}
	if err := engine.Send(context.Background(), "JoinStartStopS1", joinStartStopEvent{Symbol: "CSCO", Volume: 30}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("redeployed current state join rows = %#v", rows)
	}
}

func TestJoinRejectsSourcesWithoutViews(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinSelectClauseEvent](env, "JoinInvalidS0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinSelectClauseEvent](env, "JoinInvalidS1"); err != nil {
		t.Fatal(err)
	}
	query := Join(
		From[joinSelectClauseEvent](env, "JoinInvalidS0"),
		From[joinSelectClauseEvent](env, "JoinInvalidS1"),
		OnEqual(
			Field[joinSelectClauseEvent, float64]("doubleBoxed"),
			Field[joinSelectClauseEvent, float64]("doubleBoxed"),
		),
	).Select(
		SelectLeft("left", Field[joinSelectClauseEvent, string]("theString")),
		SelectRight("right", Field[joinSelectClauseEvent, string]("theString")),
	).Query(StatementName("join-invalid-no-view"))
	if _, err := env.Build(query); err == nil {
		t.Fatal("join without source views unexpectedly built")
	}
}
