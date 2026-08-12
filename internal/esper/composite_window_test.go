package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestUnionWindowRetainsEventUntilAllChildrenExpire(t *testing.T) {
	env, engine := newRuntimeTest(t)
	_, batches := deployViewTest(t, env, engine, From[runtimeTestTrade](env, "Trade").Window(UnionWindows(LengthWindow(1), TimeWindow(time.Second))), "union-window")
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 3 || len((*batches)[1].New) != 1 || len((*batches)[2].Old) != 1 {
		t.Fatalf("union batches = %#v", *batches)
	}
	old, ok := (*batches)[2].Old[0].Event()
	if !ok || old.Underlying().(runtimeTestTrade).Symbol != "A" {
		t.Fatalf("union expiry = %#v", old)
	}
}

func TestIntersectWindowRequiresEveryChild(t *testing.T) {
	env, engine := newRuntimeTest(t)
	_, batches := deployViewTest(t, env, engine, From[runtimeTestTrade](env, "Trade").Window(IntersectWindows(LengthWindow(1), TimeWindow(time.Second))), "intersect-window")
	for _, symbol := range []string{"A", "B"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if len(*batches) != 2 || len((*batches)[1].New) != 1 || len((*batches)[1].Old) != 1 {
		t.Fatalf("intersect insert = %#v", *batches)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 3 || len((*batches)[2].Old) != 1 {
		t.Fatalf("intersect expiry = %#v", *batches)
	}
}

func TestCompositeUniqueWindowsUseArrayKeyContent(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[compositeArrayTrade](env, "CompositeArrayTrade"); err != nil {
		t.Fatal(err)
	}
	one := Field[compositeArrayTrade, []int64]("one")
	two := Field[compositeArrayTrade, []int64]("two")
	unionEngine := NewEngine(env)
	unionStatement, unionBatches := deployViewTest(t, env, unionEngine, From[compositeArrayTrade](env, "CompositeArrayTrade").Window(UnionWindows(Unique(one), Unique(two))), "array-union")
	intersectEngine := NewEngine(env)
	intersectStatement, intersectBatches := deployViewTest(t, env, intersectEngine, From[compositeArrayTrade](env, "CompositeArrayTrade").Window(IntersectWindows(Unique(one), Unique(two))), "array-intersect")
	events := []compositeArrayTrade{
		{ID: "E0", One: []int64{1, 2}, Two: []int64{3, 4}},
		{ID: "E1", One: []int64{1, 2}, Two: []int64{3, 4}},
		{ID: "E2", One: []int64{10, 20}, Two: []int64{30}},
		{ID: "E3", One: []int64{1, 2}, Two: []int64{40}},
	}
	for _, event := range events {
		if err := unionEngine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		if err := intersectEngine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	unionSnapshot, err := unionStatement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	intersectSnapshot, err := intersectStatement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := compositeArrayTradeIDs(unionSnapshot.Results()), []string{"E1", "E2", "E3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("array union ids = %#v, want %#v, batches=%#v", got, want, *unionBatches)
	}
	if got, want := compositeArrayTradeIDs(intersectSnapshot.Results()), []string{"E2", "E3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("array intersect ids = %#v, want %#v, batches=%#v", got, want, *intersectBatches)
	}
}

func compositeArrayTradeIDs(results []Result) []string {
	ids := make([]string, 0, len(results))
	for _, result := range results {
		event, ok := result.Event()
		if !ok {
			continue
		}
		trade, ok := event.Underlying().(compositeArrayTrade)
		if ok {
			ids = append(ids, trade.ID)
		}
	}
	return ids
}

type compositeArrayTrade struct {
	ID  string  `esper:"id"`
	One []int64 `esper:"one"`
	Two []int64 `esper:"two"`
}
