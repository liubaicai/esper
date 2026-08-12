package esper

import (
	"context"
	"testing"
)

type joinTwentyStreamEvent struct {
	ID int `esper:"id"`
}

func TestTwentyStreamLastEventJoinMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinTwentyStreamEvent](env, "JoinTwentyStreamEvent"); err != nil {
		t.Fatal(err)
	}

	base := From[joinTwentyStreamEvent](env, "JoinTwentyStreamEvent")
	inputs := make([]JoinInput, 0, 20)
	for id := 0; id < 20; id++ {
		source := base.Filter(Equal[int](
			Field[joinTwentyStreamEvent, int]("id"),
			Literal(id),
		)).Window(LastEvent())
		inputs = append(inputs, JoinSource(source))
	}

	plan, err := env.Build(JoinMany(inputs...).Select(
		SelectFrom(0, "id", JoinField[int](0, "id")),
	).Query(StatementName("join-20-stream")))
	if err != nil {
		t.Fatal(err)
	}

	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())

	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for id := 0; id < 19; id++ {
		if err := engine.SendEvent(context.Background(), joinTwentyStreamEvent{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 0 {
		t.Fatalf("20-stream join emitted before all sources were ready: %#v", batches)
	}

	if err := engine.SendEvent(context.Background(), joinTwentyStreamEvent{ID: 19}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("20-stream join batches = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("id").Any() != 0 {
		t.Fatalf("20-stream join row = %#v", batches[0].New[0])
	}
}
