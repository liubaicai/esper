package esper

import (
	"context"
	"testing"
	"time"
)

func TestGroupWindowKeepsIndependentLengthState(t *testing.T) {
	env, engine := newRuntimeTest(t)
	key := Field[runtimeTestTrade, string]("symbol")
	_, batches := deployViewTest(t, env, engine, From[runtimeTestTrade](env, "Trade").Window(GroupWindow(key, LengthWindow(2))), "group-window")
	for _, trade := range []runtimeTestTrade{{Symbol: "A", Price: 1}, {Symbol: "A", Price: 2}, {Symbol: "B", Price: 3}, {Symbol: "A", Price: 4}} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(*batches) != 4 || len((*batches)[3].Old) != 1 {
		t.Fatalf("group window batches = %#v", *batches)
	}
	old, ok := (*batches)[3].Old[0].Event()
	if !ok || old.Underlying().(runtimeTestTrade).Price != 1 {
		t.Fatalf("group window evicted = %#v", old)
	}
}

func TestGroupWindowExpiresEachPartitionOnVirtualTime(t *testing.T) {
	env, engine := newRuntimeTest(t)
	key := Field[runtimeTestTrade, string]("symbol")
	_, batches := deployViewTest(t, env, engine, From[runtimeTestTrade](env, "Trade").Window(GroupWindow(key, TimeWindow(time.Second))), "group-time-window")
	for _, symbol := range []string{"A", "B"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 3 || len((*batches)[2].Old) != 2 {
		t.Fatalf("group time expiry = %#v", *batches)
	}
}

func TestGroupWindowUsesArrayKeyIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[groupArrayTrade](env, "GroupArrayTrade"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	key := Field[groupArrayTrade, []int64]("coll")
	_, batches := deployViewTest(t, env, engine, From[groupArrayTrade](env, "GroupArrayTrade").Window(GroupWindow(key, LengthWindow(1))), "group-array-key")
	for _, trade := range []groupArrayTrade{
		{ID: "A", Coll: []int64{1}},
		{ID: "B", Coll: []int64{2}},
		{ID: "C", Coll: []int64{1}},
	} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(*batches) != 3 || len((*batches)[0].Old) != 0 || len((*batches)[1].Old) != 0 || len((*batches)[2].Old) != 1 {
		t.Fatalf("group array-key batches = %#v", *batches)
	}
	old, ok := (*batches)[2].Old[0].Event()
	if !ok || old.Underlying().(groupArrayTrade).ID != "A" {
		t.Fatalf("group array-key evicted = %#v", (*batches)[2].Old)
	}
}

type groupArrayTrade struct {
	ID   string  `esper:"id"`
	Coll []int64 `esper:"coll"`
}
