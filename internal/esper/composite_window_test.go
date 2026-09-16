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

// TestNamedWindowIntersectUniqueFirstLengthEvictionParity pins the Java
// IntersectAsymetricView contract on a named window with
// #unique(intPrimitive)#firstlength(2) retain-intersection (verified against
// Esper 9.0.0 @9e1b9f1cc9117fea4bf33ab043762c045d73839c):
//
//	E1@1, E2@2, E3@1 (unique replaces E1; firstlength is full and drops E3),
//	E4@4.
//
// Java posts E3@1 as new=null old=[E3@1,E1@1] — the dropped incoming joins
// oldData under hasRemovestreamData — and pushes both removals to every
// child, so the evicted E1@1 frees the firstlength slot and E4@4 is
// admitted (new=[E4@4], iterator [E2@2,E4@4]). Without the eviction
// fan-out the stale E1@1 copy blocks E4@4 silently.
func TestNamedWindowIntersectUniqueFirstLengthEvictionParity(t *testing.T) {
	env := NewEnvironment()
	type bean struct {
		TheString    string `esper:"theString"`
		IntPrimitive int    `esper:"intPrimitive"`
	}
	if _, err := RegisterStruct[bean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	rowSchema, err := RegisterStruct[bean](env, "WRow")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "W", rowSchema,
		NamedWindowRetention(IntersectWindows(
			Unique(Field[bean, int]("intPrimitive")),
			FirstLength(2),
		))); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	window, ok := engine.NamedWindow("W")
	if !ok {
		t.Fatal("named window W not found")
	}
	type delta struct {
		newKeys []int
		oldKeys []int
	}
	var deltas []delta
	keys := func(events []Event) []int {
		if len(events) == 0 {
			return nil
		}
		out := make([]int, 0, len(events))
		for _, event := range events {
			row, ok := event.Underlying().(bean)
			if !ok {
				t.Fatalf("unexpected underlying %#v", event.Underlying())
			}
			out = append(out, row.IntPrimitive)
		}
		return out
	}
	if _, err := window.Subscribe(func(_ context.Context, d NamedWindowDelta) error {
		deltas = append(deltas, delta{newKeys: keys(d.New), oldKeys: keys(d.Old)})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	insert := func(theString string, intPrimitive int) {
		t.Helper()
		if err := engine.InsertNamedWindow(context.Background(), "W", bean{TheString: theString, IntPrimitive: intPrimitive}); err != nil {
			t.Fatal(err)
		}
	}
	insert("E1", 1)
	insert("E2", 2)
	insert("E3", 1)
	insert("E4", 4)

	want := []delta{
		{newKeys: []int{1}},
		{newKeys: []int{2}},
		{oldKeys: []int{1, 1}}, // dropped incoming E3@1 first, then evicted E1@1
		{newKeys: []int{4}},
	}
	if !reflect.DeepEqual(deltas, want) {
		t.Fatalf("deltas = %#v, want %#v", deltas, want)
	}
	snapshot, err := window.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := keys(snapshot); !reflect.DeepEqual(got, []int{2, 4}) {
		t.Fatalf("window contents = %v, want [2 4]", got)
	}
}
