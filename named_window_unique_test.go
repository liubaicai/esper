package esper

import (
	"context"
	"reflect"
	"testing"
)

func TestNamedWindowArrayUniqueRetentionMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[namedWindowArrayTrade](env, "NamedWindowArrayTrade")
	if err != nil {
		t.Fatal(err)
	}
	const windowName = "named-array-unique"
	key := Field[namedWindowArrayTrade, []int64]("coll")
	if _, err := CreateNamedWindow(env, windowName, schema, NamedWindowRetention(Unique(key))); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	plan, err := env.Build(FromNamedWindow(env, windowName).Query(StatementName("named-array-unique-consumer"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	send := func(event namedWindowArrayTrade, wantOld string) {
		t.Helper()
		if err := engine.InsertNamedWindow(context.Background(), windowName, event); err != nil {
			t.Fatal(err)
		}
		if len(batches) == 0 {
			t.Fatalf("named window did not emit for %#v", event)
		}
		batch := batches[len(batches)-1]
		if len(batch.New) != 1 {
			t.Fatalf("named window new for %#v = %#v", event, batch)
		}
		if wantOld == "" {
			if len(batch.Old) != 0 {
				t.Fatalf("named window unexpected old for %#v = %#v", event, batch.Old)
			}
			return
		}
		if len(batch.Old) != 1 || namedWindowArrayID(batch.Old[0]) != wantOld {
			t.Fatalf("named window old for %#v = %#v, want %q", event, batch.Old, wantOld)
		}
	}

	send(namedWindowArrayTrade{ID: "E0", Coll: []int64{1, 2}}, "")
	send(namedWindowArrayTrade{ID: "E1", Coll: []int64{1, 2}}, "E0")
	send(namedWindowArrayTrade{ID: "E2", Coll: []int64{2, 1}}, "")
	send(namedWindowArrayTrade{ID: "E3", Coll: []int64{2, 2}}, "")
	send(namedWindowArrayTrade{ID: "E4", Coll: []int64{2, 2, 2}}, "")
	send(namedWindowArrayTrade{ID: "E5", Coll: nil}, "")
	send(namedWindowArrayTrade{ID: "E6", Coll: []int64{1}}, "")
	send(namedWindowArrayTrade{ID: "E7", Coll: nil}, "E5")
	send(namedWindowArrayTrade{ID: "E10", Coll: []int64{1, 2}}, "E1")
	send(namedWindowArrayTrade{ID: "E11", Coll: []int64{2, 2}}, "E3")
	send(namedWindowArrayTrade{ID: "E12", Coll: []int64{2, 1}}, "E2")
	send(namedWindowArrayTrade{ID: "E13", Coll: []int64{2, 2, 2}}, "E4")
	send(namedWindowArrayTrade{ID: "E14", Coll: nil}, "E7")
	send(namedWindowArrayTrade{ID: "E15", Coll: []int64{1}}, "E6")
	send(namedWindowArrayTrade{ID: "E16", Coll: nil}, "E14")

	snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []namedWindowArrayTrade{
		{ID: "E10", Coll: []int64{1, 2}},
		{ID: "E12", Coll: []int64{2, 1}},
		{ID: "E11", Coll: []int64{2, 2}},
		{ID: "E13", Coll: []int64{2, 2, 2}},
		{ID: "E16", Coll: nil},
		{ID: "E15", Coll: []int64{1}},
	}
	if len(snapshot.Results()) != len(want) {
		ids := make([]string, 0, len(snapshot.Results()))
		for _, result := range snapshot.Results() {
			ids = append(ids, namedWindowArrayID(result))
		}
		t.Fatalf("named array unique snapshot ids = %v, want %#v", ids, want)
	}
	for index, result := range snapshot.Results() {
		event, ok := result.Event()
		if !ok || !reflect.DeepEqual(event.Underlying(), want[index]) {
			t.Fatalf("named array unique snapshot[%d] = %#v, want %#v", index, event.Underlying(), want[index])
		}
	}
}

type namedWindowArrayTrade struct {
	ID   string  `esper:"id"`
	Coll []int64 `esper:"coll"`
}

func namedWindowArrayID(result Result) string {
	event, ok := result.Event()
	if !ok {
		return ""
	}
	trade, ok := event.Underlying().(namedWindowArrayTrade)
	if !ok {
		return ""
	}
	return trade.ID
}
