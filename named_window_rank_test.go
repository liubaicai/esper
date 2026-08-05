package esper

import (
	"context"
	"testing"
)

type namedWindowRankSupportBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type namedWindowRankDeleteSignal struct {
	ID string `esper:"id"`
}

type namedWindowRankExpected struct {
	id    string
	value int
	long  int64
}

func TestNamedWindowRankRemoveStreamMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[namedWindowRankSupportBean](env, "SupportBean")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[namedWindowRankDeleteSignal](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	key := Field[namedWindowRankSupportBean, string]("theString")
	value := Field[namedWindowRankSupportBean, int]("intPrimitive")
	longValue := Field[namedWindowRankSupportBean, int64]("longPrimitive")
	if _, err := CreateNamedWindow(env, "MyWindow", schema, NamedWindowRetention(
		RankWindowBy(3, []Expr{key}, Ascending(value)),
	)); err != nil {
		t.Fatal(err)
	}

	source := From[namedWindowRankSupportBean](env, "SupportBean")
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow("MyWindow",
		SetColumn("theString", key),
		SetColumn("intPrimitive", value),
		SetColumn("longPrimitive", longValue),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromNamedWindow(env, "MyWindow").Query(
		StatementName("s0"),
		WithOldStream(),
	))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[namedWindowRankDeleteSignal](env, "SupportBean_A")).DeleteFromNamedWindow(
		"MyWindow",
		Equal[string](NamedWindowField[string]("theString"), Field[namedWindowRankDeleteSignal, string]("id")),
	).Query(StatementName("delete")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	statement := consumerDeployment.Statements()[0]
	var batches []ResultBatch
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), deletePlan); err != nil {
		t.Fatal(err)
	}

	assertResult := func(result Result, want namedWindowRankExpected) {
		t.Helper()
		event, ok := result.Event()
		if !ok {
			t.Fatalf("rank named-window result is not an event: %#v", result)
		}
		got, ok := event.Underlying().(namedWindowRankSupportBean)
		if !ok {
			t.Fatalf("rank named-window result type = %T", event.Underlying())
		}
		if got.TheString != want.id || got.IntPrimitive != want.value || got.LongPrimitive != want.long {
			t.Fatalf("rank named-window result = %#v, want %#v", got, want)
		}
	}
	assertSnapshot := func(want ...namedWindowRankExpected) {
		t.Helper()
		snapshot, err := statement.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(snapshot.Results()) != len(want) {
			t.Fatalf("rank named-window snapshot length = %d, want %d (%#v)", len(snapshot.Results()), len(want), snapshot.Results())
		}
		for index, result := range snapshot.Results() {
			assertResult(result, want[index])
		}
	}
	assertBatch := func(wantNew []namedWindowRankExpected, wantOld []namedWindowRankExpected) {
		t.Helper()
		if len(batches) == 0 {
			t.Fatal("rank named-window emitted no batch")
		}
		batch := batches[len(batches)-1]
		if len(batch.New) != len(wantNew) || len(batch.Old) != len(wantOld) {
			t.Fatalf("rank named-window batch = %#v, want new=%d old=%d", batch, len(wantNew), len(wantOld))
		}
		for index, want := range wantNew {
			assertResult(batch.New[index], want)
		}
		for index, want := range wantOld {
			assertResult(batch.Old[index], want)
		}
	}

	send := func(event namedWindowRankSupportBean, wantOld []namedWindowRankExpected) {
		t.Helper()
		before := len(batches)
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		if len(batches) != before+1 {
			t.Fatalf("rank named-window insert batch count = %d, want %d", len(batches), before+1)
		}
		assertBatch([]namedWindowRankExpected{{id: event.TheString, value: event.IntPrimitive, long: event.LongPrimitive}}, wantOld)
	}
	remove := func(id string, wantOld namedWindowRankExpected) {
		t.Helper()
		before := len(batches)
		if err := engine.SendEvent(context.Background(), namedWindowRankDeleteSignal{ID: id}); err != nil {
			t.Fatal(err)
		}
		if len(batches) != before+1 {
			t.Fatalf("rank named-window delete batch count = %d, want %d", len(batches), before+1)
		}
		assertBatch(nil, []namedWindowRankExpected{wantOld})
	}

	send(namedWindowRankSupportBean{TheString: "E1", IntPrimitive: 10}, nil)
	assertSnapshot(namedWindowRankExpected{id: "E1", value: 10})
	send(namedWindowRankSupportBean{TheString: "E2", IntPrimitive: 50}, nil)
	assertSnapshot(
		namedWindowRankExpected{id: "E1", value: 10},
		namedWindowRankExpected{id: "E2", value: 50},
	)
	send(namedWindowRankSupportBean{TheString: "E3", IntPrimitive: 5}, nil)
	assertSnapshot(
		namedWindowRankExpected{id: "E3", value: 5},
		namedWindowRankExpected{id: "E1", value: 10},
		namedWindowRankExpected{id: "E2", value: 50},
	)
	send(namedWindowRankSupportBean{TheString: "E4", IntPrimitive: 5}, []namedWindowRankExpected{{id: "E2", value: 50}})
	assertSnapshot(
		namedWindowRankExpected{id: "E3", value: 5},
		namedWindowRankExpected{id: "E4", value: 5},
		namedWindowRankExpected{id: "E1", value: 10},
	)
	remove("E3", namedWindowRankExpected{id: "E3", value: 5})
	assertSnapshot(
		namedWindowRankExpected{id: "E4", value: 5},
		namedWindowRankExpected{id: "E1", value: 10},
	)
	remove("E4", namedWindowRankExpected{id: "E4", value: 5})
	assertSnapshot(namedWindowRankExpected{id: "E1", value: 10})
	remove("E1", namedWindowRankExpected{id: "E1", value: 10})
	assertSnapshot()

	send(namedWindowRankSupportBean{TheString: "E3", IntPrimitive: 100}, nil)
	assertSnapshot(namedWindowRankExpected{id: "E3", value: 100})
	send(namedWindowRankSupportBean{TheString: "E3", IntPrimitive: 101, LongPrimitive: 1}, []namedWindowRankExpected{{id: "E3", value: 100}})
	assertSnapshot(namedWindowRankExpected{id: "E3", value: 101, long: 1})
	remove("E3", namedWindowRankExpected{id: "E3", value: 101, long: 1})
	assertSnapshot()
}
