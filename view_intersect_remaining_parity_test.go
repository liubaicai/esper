package esper

import (
	"context"
	"reflect"
	"testing"
)

func TestViewIntersectFirstAndLengthMatrixParity(t *testing.T) {
	for name, windows := range map[string]CompositeWindowSpec{
		"unique-first-length": IntersectWindows(FirstLength(3), Unique(Field[viewParityBean, string]("theString"))),
		"first-unique-length": IntersectWindows(FirstUnique(Field[viewParityBean, string]("theString")), FirstLength(3)),
		"length-one-unique":   IntersectWindows(LengthWindow(1), Unique(Field[viewParityBean, string]("theString"))),
	} {
		t.Run(name, func(t *testing.T) {
			env, engine := newViewParityEnv(t)
			defer func() { _ = engine.Close(context.Background()) }()
			statement, batches := deployViewParity(t, env, engine, From[viewParityBean](env, "SupportBean").Window(windows), "intersect-"+name)
			send := func(name string, value int) {
				t.Helper()
				sendViewBean(t, engine, name, value)
			}
			send("E1", 1)
			send("E2", 2)
			send("E1", 3)
			snapshot, err := statement.Snapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if name == "length-one-unique" {
				if got := intersectViewNames(snapshot.Results()); !reflect.DeepEqual(got, []string{"E1"}) {
					t.Fatalf("length-one unique snapshot = %#v", got)
				}
			} else if got := intersectViewNames(snapshot.Results()); !sameStringSet(got, []string{"E1", "E2"}) {
				t.Fatalf("%s snapshot = %#v", name, got)
			}
			if len(*batches) < 2 {
				t.Fatalf("%s did not emit each accepted insert: %#v", name, *batches)
			}
			send("E3", 30)
			if name != "length-one-unique" {
				send("E4", 40)
				if snapshot, err = statement.Snapshot(context.Background()); err != nil {
					t.Fatal(err)
				}
				if got := intersectViewNames(snapshot.Results()); len(got) != 3 {
					t.Fatalf("%s first-length cap = %#v", name, got)
				}
			}
		})
	}
}

func TestViewIntersectBatchAndDerivedAggregateParity(t *testing.T) {
	t.Run("length-batch-unique", func(t *testing.T) {
		env, engine := newViewParityEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		one := Field[viewParityBean, int]("intPrimitive")
		statement, batches := deployViewParity(t, env, engine, From[viewParityBean](env, "SupportBean").Window(
			IntersectWindows(LengthBatch(3), Unique(one)),
		), "intersect-length-batch")
		for _, event := range []viewParityBean{{TheString: "E1", IntPrimitive: 1}, {TheString: "E2", IntPrimitive: 2}, {TheString: "E3", IntPrimitive: 3}} {
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
		if len(*batches) != 1 || len((*batches)[0].New) != 3 || len((*batches)[0].Old) != 0 {
			t.Fatalf("length-batch unique first flush = %#v", *batches)
		}
		if snapshot, err := statement.Snapshot(context.Background()); err != nil || len(snapshot.Results()) != 0 {
			t.Fatalf("length-batch snapshot after flush = %#v, err=%v", snapshot.Results(), err)
		}
		for _, event := range []viewParityBean{{TheString: "E4", IntPrimitive: 4}, {TheString: "E5", IntPrimitive: 4}, {TheString: "E6", IntPrimitive: 5}} {
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
		if len(*batches) != 2 || len((*batches)[1].New) != 2 || len((*batches)[1].Old) != 3 {
			t.Fatalf("length-batch unique replacement flush = %#v", *batches)
		}
	})

	t.Run("unique-length-batch", func(t *testing.T) {
		env, engine := newViewParityEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		one := Field[viewParityBean, int]("intPrimitive")
		_, batches := deployViewParity(t, env, engine, From[viewParityBean](env, "SupportBean").Window(
			IntersectWindows(Unique(one), LengthBatch(3)),
		), "intersect-unique-length-batch")
		for _, event := range []viewParityBean{{TheString: "E1", IntPrimitive: 1}, {TheString: "E2", IntPrimitive: 2}, {TheString: "E1", IntPrimitive: 3}, {TheString: "E3", IntPrimitive: 4}} {
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
		if len(*batches) != 1 || len((*batches)[0].New) != 3 {
			t.Fatalf("unique-length-batch flush = %#v", *batches)
		}
	})

	t.Run("derived-sum", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[intersectViewEvent](env, "IntersectViewEvent"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		one := Field[intersectViewEvent, int]("keyOne")
		two := Field[intersectViewEvent, int]("keyTwo")
		three := Field[intersectViewEvent, int]("keyThree")
		price := Field[intersectViewEvent, float64]("price")
		stream := From[intersectViewEvent](env, "IntersectViewEvent").Window(IntersectWindows(Unique(one), Unique(two), Unique(three)))
		plan, err := env.Build(stream.Aggregate(Alias("total", Sum[float64](price))).Query(StatementName("intersect-derived-sum")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = deployment.Undeploy(context.Background()) }()
		var totals []float64
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				totals = append(totals, result.Get("total").Any().(float64))
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		for _, event := range []intersectViewEvent{
			{TheString: "E1", KeyOne: 1, KeyTwo: 10, KeyThree: 100, Price: 100},
			{TheString: "E2", KeyOne: 2, KeyTwo: 20, KeyThree: 200, Price: 50},
			{TheString: "E3", KeyOne: 1, KeyTwo: 20, KeyThree: 100, Price: 20},
		} {
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
		if !reflect.DeepEqual(totals, []float64{100, 150, 20}) {
			t.Fatalf("intersect derived totals = %#v, want [100 150 20]", totals)
		}
	})
}

func TestViewIntersectGroupedAndSortedParity(t *testing.T) {
	t.Run("grouped", func(t *testing.T) {
		env, engine := newViewParityEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		group := Field[viewParityBean, int]("intPrimitive")
		unique := Field[viewParityBean, int64]("longPrimitive")
		statement, batches := deployViewParity(t, env, engine, From[viewParityBean](env, "SupportBean").Window(
			GroupWindow(group, IntersectWindows(LengthWindow(2), Unique(unique))),
		), "intersect-grouped")
		for _, event := range []viewParityBean{
			{TheString: "E1", IntPrimitive: 1, LongPrimitive: 10},
			{TheString: "E2", IntPrimitive: 2, LongPrimitive: 10},
			{TheString: "E3", IntPrimitive: 1, LongPrimitive: 20},
			{TheString: "E4", IntPrimitive: 1, LongPrimitive: 30},
			{TheString: "E5", IntPrimitive: 2, LongPrimitive: 10},
		} {
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
		snapshot, err := statement.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if got := intersectViewNames(snapshot.Results()); !reflect.DeepEqual(got, []string{"E3", "E4", "E5"}) {
			t.Fatalf("grouped intersect snapshot = %#v", got)
		}
		if len(*batches) != 5 || len((*batches)[4].Old) != 1 {
			t.Fatalf("grouped intersect last delta = %#v", (*batches)[4])
		}
	})

	t.Run("sorted", func(t *testing.T) {
		env, engine := newViewParityEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		first := Field[viewParityBean, int]("intPrimitive")
		second := Field[viewParityBean, int64]("longPrimitive")
		statement, batches := deployViewParity(t, env, engine, From[viewParityBean](env, "SupportBean").Window(
			IntersectWindows(SortWindow(2, Ascending(first)), SortWindow(2, Ascending(second))),
		), "intersect-sorted")
		for _, event := range []viewParityBean{
			{TheString: "E1", IntPrimitive: 1, LongPrimitive: 10},
			{TheString: "E2", IntPrimitive: 2, LongPrimitive: 9},
			{TheString: "E3", IntPrimitive: 0, LongPrimitive: 0},
		} {
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
		snapshot, err := statement.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if got := intersectViewNames(snapshot.Results()); !reflect.DeepEqual(got, []string{"E3"}) {
			t.Fatalf("sorted intersect snapshot = %#v", got)
		}
		if len(*batches) != 3 || len((*batches)[2].New) != 1 || len((*batches)[2].Old) != 2 {
			t.Fatalf("sorted intersect replacement = %#v", (*batches)[2])
		}
	})
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[string]int, len(left))
	for _, value := range left {
		counts[value]++
	}
	for _, value := range right {
		counts[value]--
		if counts[value] < 0 {
			return false
		}
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}
