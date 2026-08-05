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

func TestNamedWindowArrayUniqueRetentionSupportsMerge(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[namedWindowArrayTrade](env, "NamedWindowMergeArrayTrade")
	if err != nil {
		t.Fatal(err)
	}
	const windowName = "named-array-unique-merge"
	key := Field[namedWindowArrayTrade, []int64]("coll")
	if _, err := CreateNamedWindow(env, windowName, schema, NamedWindowRetention(Unique(key))); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	source := From[namedWindowArrayTrade](env, "NamedWindowMergeArrayTrade")
	namedColl := NamedWindowField[[]int64]("coll")
	match := makeExpr[bool]("array-equal", "array-equal(named.coll,coll)", []*exprNode{namedColl.node(), key.node()}, func(ctx EvalContext) Value {
		return Present(reflect.DeepEqual(namedColl.eval(ctx).Any(), key.eval(ctx).Any()))
	})
	plan, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen(windowName, match,
		WhenMatched(Literal(true), SetColumn("id", Field[namedWindowArrayTrade, string]("id"))),
		WhenNotMatched(Literal(true)),
	).Query(StatementName("named-array-unique-merge")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	var batches []NamedWindowDelta
	window, ok := engine.NamedWindow(windowName)
	if !ok {
		t.Fatal("unique merge named window is missing")
	}
	if _, err := window.Subscribe(func(_ context.Context, delta NamedWindowDelta) error {
		batches = append(batches, delta)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(event namedWindowArrayTrade) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	send(namedWindowArrayTrade{ID: "E0", Coll: []int64{1, 2}})
	send(namedWindowArrayTrade{ID: "E1", Coll: []int64{1, 2}})
	send(namedWindowArrayTrade{ID: "E2", Coll: []int64{2, 1}})
	if len(batches) != 3 || len(batches[0].New) != 1 || len(batches[0].Old) != 0 ||
		len(batches[1].New) != 1 || len(batches[1].Old) != 1 || len(batches[2].New) != 1 || len(batches[2].Old) != 0 {
		t.Fatalf("unique merge batches = %#v", batches)
	}
	if got := namedWindowArrayEventID(batches[1].Old[0]); got != "E0" {
		t.Fatalf("unique merge replacement old = %q, want E0", got)
	}
	if got := namedWindowArrayEventID(batches[1].New[0]); got != "E1" {
		t.Fatalf("unique merge replacement new = %q, want E1", got)
	}
	events, err := window.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || namedWindowArrayEventID(events[0]) != "E1" || namedWindowArrayEventID(events[1]) != "E2" {
		t.Fatalf("unique merge snapshot = %#v", events)
	}
}

func TestNamedWindowArrayUniqueMergeCollisionDispatch(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[namedWindowArrayTrade](env, "NamedWindowMergeCollisionTrade")
	if err != nil {
		t.Fatal(err)
	}
	const windowName = "named-array-unique-merge-collision"
	key := Field[namedWindowArrayTrade, []int64]("coll")
	if _, err := CreateNamedWindow(env, windowName, schema, NamedWindowRetention(Unique(key))); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	source := From[namedWindowArrayTrade](env, "NamedWindowMergeCollisionTrade")
	plan, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen(windowName, Literal(false),
		WhenNotMatched(Literal(true)),
	).Query(StatementName("named-array-unique-merge-collision")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow(windowName)
	if !ok {
		t.Fatal("unique merge collision named window is missing")
	}
	var batches []NamedWindowDelta
	if _, err := window.Subscribe(func(_ context.Context, delta NamedWindowDelta) error {
		batches = append(batches, delta)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []namedWindowArrayTrade{
		{ID: "E0", Coll: []int64{1, 2}},
		{ID: "E1", Coll: []int64{1, 2}},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 2 || len(batches[0].New) != 1 || len(batches[0].Old) != 0 ||
		len(batches[1].New) != 1 || len(batches[1].Old) != 1 || namedWindowArrayEventID(batches[1].Old[0]) != "E0" || namedWindowArrayEventID(batches[1].New[0]) != "E1" {
		t.Fatalf("unique merge collision batches = %#v", batches)
	}
}

func TestNamedWindowFirstUniqueMergeIgnoresDuplicate(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[namedWindowArrayTrade](env, "NamedWindowFirstMergeTrade")
	if err != nil {
		t.Fatal(err)
	}
	const windowName = "named-array-first-unique-merge"
	key := Field[namedWindowArrayTrade, []int64]("coll")
	if _, err := CreateNamedWindow(env, windowName, schema, NamedWindowRetention(FirstUnique(key))); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	source := From[namedWindowArrayTrade](env, "NamedWindowFirstMergeTrade")
	plan, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen(windowName, Literal(false),
		WhenNotMatched(Literal(true)),
	).Query(StatementName("named-array-first-unique-merge")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow(windowName)
	if !ok {
		t.Fatal("first unique merge named window is missing")
	}
	var batches []NamedWindowDelta
	if _, err := window.Subscribe(func(_ context.Context, delta NamedWindowDelta) error {
		batches = append(batches, delta)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []namedWindowArrayTrade{
		{ID: "E0", Coll: []int64{1, 2}},
		{ID: "E1", Coll: []int64{1, 2}},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 1 || len(batches[0].New) != 1 || len(batches[0].Old) != 0 {
		t.Fatalf("first unique merge duplicate batches = %#v", batches)
	}
	events, err := window.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || namedWindowArrayEventID(events[0]) != "E0" {
		t.Fatalf("first unique merge duplicate snapshot = %#v", events)
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

func namedWindowArrayEventID(event Event) string {
	trade, ok := event.Underlying().(namedWindowArrayTrade)
	if !ok {
		return ""
	}
	return trade.ID
}
