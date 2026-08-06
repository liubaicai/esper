package esper

import (
	"context"
	"testing"
)

type joinSingleOpA struct {
	ID    string `esper:"id"`
	Value string `esper:"value"`
}

type joinSingleOpB struct {
	ID    string `esper:"id"`
	Value string `esper:"value"`
}

type joinSingleOpC struct {
	ID    string `esper:"id"`
	Value string `esper:"value"`
}

func TestThreeStreamSingleOperationJoinMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinSingleOpA](env, "JoinSingleOpA"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinSingleOpB](env, "JoinSingleOpB"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinSingleOpC](env, "JoinSingleOpC"); err != nil {
		t.Fatal(err)
	}

	plan, err := env.Build(JoinMany(
		JoinSource(From[joinSingleOpA](env, "JoinSingleOpA").Window(LengthWindow(3))),
		JoinSource(From[joinSingleOpB](env, "JoinSingleOpB").Window(LengthWindow(3))),
		JoinSource(From[joinSingleOpC](env, "JoinSingleOpC").Window(LengthWindow(3))),
	).On(
		OnSourcesEqual(0, Field[joinSingleOpA, string]("id"), 1, Field[joinSingleOpB, string]("id")),
		OnSourcesEqual(1, Field[joinSingleOpB, string]("id"), 2, Field[joinSingleOpC, string]("id")),
		OnSourcesEqual(0, Field[joinSingleOpA, string]("id"), 2, Field[joinSingleOpC, string]("id")),
	).Select(
		SelectFrom(0, "a", JoinEventValue[Event](0)),
		SelectFrom(1, "b", JoinEventValue[Event](1)),
		SelectFrom(2, "c", JoinEventValue[Event](2)),
	).Query(StatementName("join-single-op-3-stream")))
	if err != nil {
		t.Fatal(err)
	}

	engine := env.NewEngine()
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

	a := func(id string) joinSingleOpA { return joinSingleOpA{ID: id, Value: "A-" + id} }
	b := func(id string) joinSingleOpB { return joinSingleOpB{ID: id, Value: "B-" + id} }
	c := func(id string) joinSingleOpC { return joinSingleOpC{ID: id, Value: "C-" + id} }
	send := func(event any) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	assertRow := func(index int, id string) {
		t.Helper()
		if index >= len(rows) {
			t.Fatalf("three-stream join rows = %#v, missing row %d", rows, index)
		}
		row := rows[index]
		for _, source := range []struct {
			name string
			want any
		}{
			{name: "a", want: a(id)},
			{name: "b", want: b(id)},
			{name: "c", want: c(id)},
		} {
			event, ok := row.Get(source.name).Any().(Event)
			if !ok {
				t.Fatalf("three-stream join %s projection = %#v", source.name, row.AsMap())
			}
			if eventValue := event.Underlying(); eventValue != source.want {
				t.Fatalf("three-stream join %s event = %#v, want %#v", source.name, eventValue, source.want)
			}
		}
	}

	send(a("0"))
	send(b("0"))
	if len(rows) != 0 {
		t.Fatalf("incomplete first tuple emitted: %#v", rows)
	}
	send(c("0"))
	if len(rows) != 1 {
		t.Fatalf("first complete tuple rows = %#v", rows)
	}
	assertRow(0, "0")

	send(a("1"))
	send(b("2"))
	send(c("3"))
	send(c("1"))
	if len(rows) != 1 {
		t.Fatalf("partial second tuple emitted: %#v", rows)
	}
	send(b("1"))
	if len(rows) != 2 {
		t.Fatalf("second complete tuple rows = %#v", rows)
	}
	assertRow(1, "1")

	send(a("4"))
	send(a("5"))
	send(b("4"))
	send(b("3"))
	if len(rows) != 2 {
		t.Fatalf("partial third tuple emitted: %#v", rows)
	}
	send(c("4"))
	if len(rows) != 3 {
		t.Fatalf("third complete tuple rows = %#v", rows)
	}
	assertRow(2, "4")
}
