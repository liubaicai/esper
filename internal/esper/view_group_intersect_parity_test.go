package esper

import (
	"context"
	"math"
	"reflect"
	"testing"
	"time"
)

func TestViewGroupBatchAndAccumParity(t *testing.T) {
	t.Run("length-batch", func(t *testing.T) {
		env, engine := newViewMarketDataEnv(t)
		defer func() { _ = engine.Close(context.Background()) }()
		key := Field[viewUniqueMarketData, string]("symbol")
		_, batches := deployViewTest(t, env, engine, From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(
			GroupWindow(key, LengthBatch(3)),
		), "group-length-batch")
		send := func(symbol string, price float64) {
			t.Helper()
			if err := engine.SendEvent(context.Background(), viewUniqueMarketData{Symbol: symbol, Price: price}); err != nil {
				t.Fatal(err)
			}
		}
		send("S1", 1)
		send("S2", 20)
		send("S2", 21)
		if len(*batches) != 0 {
			t.Fatalf("group length batch emitted early: %#v", *batches)
		}
		send("S1", 2)
		send("S2", 22)
		if len(*batches) != 1 || len((*batches)[0].New) != 3 || len((*batches)[0].Old) != 0 {
			t.Fatalf("group length batch first flush: %#v", *batches)
		}
		want := []float64{20, 21, 22}
		for index, result := range (*batches)[0].New {
			if got := result.Get("price").Any(); got != want[index] {
				t.Fatalf("group length batch new[%d] = %v, want %v", index, got, want[index])
			}
		}
		send("S1", 3)
		if len(*batches) != 2 || len((*batches)[1].New) != 3 || len((*batches)[1].Old) != 0 {
			t.Fatalf("group length batch second flush: %#v", *batches)
		}
		send("S1", 4)
		if len(*batches) != 2 {
			t.Fatalf("group length batch emitted incomplete third group: %#v", *batches)
		}
	})

	t.Run("time-batch-and-time-length-batch", func(t *testing.T) {
		for name, window := range map[string]WindowSpec{
			"time-batch":        TimeBatch(10 * time.Second),
			"time-length-batch": TimeLengthBatch(10*time.Second, 100),
		} {
			t.Run(name, func(t *testing.T) {
				env := NewEnvironment()
				if _, err := RegisterStruct[viewUniqueMarketData](env, "SupportMarketDataBean"); err != nil {
					t.Fatal(err)
				}
				engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
				defer func() { _ = engine.Close(context.Background()) }()
				key := Field[viewUniqueMarketData, string]("symbol")
				_, batches := deployViewTest(t, env, engine, From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(
					GroupWindow(key, window),
				), "group-"+name)
				send := func(symbol string, price float64) {
					t.Helper()
					if err := engine.SendEvent(context.Background(), viewUniqueMarketData{Symbol: symbol, Price: price}); err != nil {
						t.Fatal(err)
					}
				}
				send("S1", 10)
				send("S1", 20)
				send("S2", 30)
				if len(*batches) != 0 {
					t.Fatalf("group %s emitted before boundary: %#v", name, *batches)
				}
				if err := engine.AdvanceTime(context.Background(), time.Unix(10, 0).UTC()); err != nil {
					t.Fatal(err)
				}
				if len(*batches) != 1 || len((*batches)[0].New) != 3 || len((*batches)[0].Old) != 0 {
					t.Fatalf("group %s first flush: %#v", name, *batches)
				}
				if err := engine.AdvanceTime(context.Background(), time.Unix(20, 0).UTC()); err != nil {
					t.Fatal(err)
				}
				if len(*batches) != 2 || len((*batches)[1].New) != 0 || len((*batches)[1].Old) != 3 {
					t.Fatalf("group %s old flush: %#v", name, *batches)
				}
			})
		}
	})

	t.Run("time-accum", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[viewUniqueMarketData](env, "SupportMarketDataBean"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
		defer func() { _ = engine.Close(context.Background()) }()
		key := Field[viewUniqueMarketData, string]("symbol")
		_, batches := deployViewTest(t, env, engine, From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(
			GroupWindow(key, TimeAccum(10*time.Second)),
		), "group-time-accum")
		if err := engine.SendEvent(context.Background(), viewUniqueMarketData{Symbol: "S1", Price: 10}); err != nil {
			t.Fatal(err)
		}
		if err := engine.AdvanceTime(context.Background(), time.Unix(5, 0).UTC()); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), viewUniqueMarketData{Symbol: "S1", Price: 20}); err != nil {
			t.Fatal(err)
		}
		if err := engine.AdvanceTime(context.Background(), time.Unix(10, 0).UTC()); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), viewUniqueMarketData{Symbol: "S2", Price: 30}); err != nil {
			t.Fatal(err)
		}
		if len(*batches) != 3 {
			t.Fatalf("group time-accum inserts = %#v", *batches)
		}
		if err := engine.AdvanceTime(context.Background(), time.Unix(15, 0).UTC()); err != nil {
			t.Fatal(err)
		}
		if len(*batches) != 4 || len((*batches)[3].New) != 0 || len((*batches)[3].Old) != 2 {
			t.Fatalf("group time-accum first expiry = %#v", *batches)
		}
		if err := engine.AdvanceTime(context.Background(), time.Unix(20, 0).UTC()); err != nil {
			t.Fatal(err)
		}
		if len(*batches) != 5 || len((*batches)[4].New) != 0 || len((*batches)[4].Old) != 1 {
			t.Fatalf("group time-accum second expiry = %#v", *batches)
		}
	})
}

type groupedViewStatisticsEvent struct {
	Symbol string  `esper:"symbol"`
	X      float64 `esper:"x"`
	Y      float64 `esper:"y"`
}

func TestViewGroupStatisticsCorrelationAndRegressionParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[groupedViewStatisticsEvent](env, "GroupedViewStats"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	symbol := Field[groupedViewStatisticsEvent, string]("symbol")
	x := Field[groupedViewStatisticsEvent, float64]("x")
	y := Field[groupedViewStatisticsEvent, float64]("y")
	regression := Linest[float64, float64](x, y)
	stream := From[groupedViewStatisticsEvent](env, "GroupedViewStats").Window(GroupWindow(symbol, LengthWindow(3)))
	plan, err := env.Build(stream.Aggregate(
		Alias("symbol", symbol),
		Alias("average", Avg[float64](x)),
		Alias("correlation", Correlation[float64, float64](x, y)),
		Alias("slope", regression.Slope()),
		Alias("intercept", regression.YIntercept()),
	).Query(StatementName("grouped-view-statistics")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	var last Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) == 0 {
			return nil
		}
		row, ok := batch.New[len(batch.New)-1].Row()
		if !ok {
			return NewError(ErrorTypeMismatch, "grouped statistics result is not a row")
		}
		last = row
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []groupedViewStatisticsEvent{{Symbol: "A", X: 1, Y: 2}, {Symbol: "A", X: 2, Y: 4}, {Symbol: "B", X: 10, Y: 5}, {Symbol: "A", X: 3, Y: 6}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if last.Get("symbol").Any() != "A" || last.Get("average").Any() != 2.0 {
		t.Fatalf("grouped statistics identity = %#v", last.AsMap())
	}
	for name, want := range map[string]float64{"correlation": 1, "slope": 2, "intercept": 0} {
		got, ok := last.Get(name).Any().(float64)
		if !ok || math.Abs(got-want) > 1e-9 {
			t.Fatalf("grouped %s = %v, want %v", name, last.Get(name).Any(), want)
		}
	}
}

type intersectViewEvent struct {
	TheString string  `esper:"theString"`
	KeyOne    int     `esper:"keyOne"`
	KeyTwo    int     `esper:"keyTwo"`
	KeyThree  int     `esper:"keyThree"`
	Price     float64 `esper:"price"`
}

func TestViewIntersectUniqueMatrixParity(t *testing.T) {
	t.Run("two-unique", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[intersectViewEvent](env, "IntersectViewEvent"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		one := Field[intersectViewEvent, int]("keyOne")
		two := Field[intersectViewEvent, int]("keyTwo")
		statement, batches := deployViewTest(t, env, engine, From[intersectViewEvent](env, "IntersectViewEvent").Window(
			IntersectWindows(Unique(one), Unique(two)),
		), "intersect-two-unique")
		send := func(name string, first, second int) {
			t.Helper()
			if err := engine.SendEvent(context.Background(), intersectViewEvent{TheString: name, KeyOne: first, KeyTwo: second}); err != nil {
				t.Fatal(err)
			}
		}
		send("E1", 1, 10)
		send("E2", 2, 10)
		if len(*batches) != 2 || len((*batches)[1].New) != 1 || len((*batches)[1].Old) != 1 {
			t.Fatalf("two unique replacement = %#v", *batches)
		}
		send("E3", 1, 20)
		send("E4", 3, 20)
		snapshot, err := statement.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if got := intersectViewNames(snapshot.Results()); !reflect.DeepEqual(got, []string{"E2", "E4"}) {
			t.Fatalf("two unique snapshot = %#v, want [E2 E4]", got)
		}
	})

	t.Run("three-unique", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[intersectViewEvent](env, "IntersectViewEvent"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		one := Field[intersectViewEvent, int]("keyOne")
		two := Field[intersectViewEvent, int]("keyTwo")
		three := Field[intersectViewEvent, int]("keyThree")
		statement, batches := deployViewTest(t, env, engine, From[intersectViewEvent](env, "IntersectViewEvent").Window(
			IntersectWindows(Unique(one), Unique(two), Unique(three)),
		), "intersect-three-unique")
		send := func(name string, first, second, third int) {
			t.Helper()
			if err := engine.SendEvent(context.Background(), intersectViewEvent{TheString: name, KeyOne: first, KeyTwo: second, KeyThree: third}); err != nil {
				t.Fatal(err)
			}
		}
		send("E1", 1, 10, 100)
		send("E2", 2, 10, 200)
		send("E3", 2, 20, 100)
		send("E4", 1, 30, 300)
		snapshot, err := statement.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if got := intersectViewNames(snapshot.Results()); !reflect.DeepEqual(got, []string{"E3", "E4"}) {
			t.Fatalf("three unique snapshot = %#v, want [E3 E4]", got)
		}
		if len(*batches) != 4 || len((*batches)[3].New) != 1 || len((*batches)[3].Old) != 0 {
			t.Fatalf("three unique last delta = %#v", (*batches)[3])
		}
	})
}

func TestViewIntersectTimeUniqueParity(t *testing.T) {
	for name, windows := range map[string]CompositeWindowSpec{
		"time-then-unique": IntersectWindows(TimeWindow(10*time.Second), Unique(Field[intersectViewEvent, int]("keyOne"))),
		"unique-then-time": IntersectWindows(Unique(Field[intersectViewEvent, int]("keyOne")), TimeWindow(10*time.Second)),
		"time-multikey":    IntersectWindows(TimeWindow(3*time.Second), UniqueBy(Field[intersectViewEvent, int]("keyOne"), Field[intersectViewEvent, int]("keyTwo"))),
	} {
		t.Run(name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[intersectViewEvent](env, "IntersectViewEvent"); err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
			defer func() { _ = engine.Close(context.Background()) }()
			_, batches := deployViewTest(t, env, engine, From[intersectViewEvent](env, "IntersectViewEvent").Window(windows), "intersect-"+name)
			if err := engine.SendEvent(context.Background(), intersectViewEvent{TheString: "E1", KeyOne: 1, KeyTwo: 10}); err != nil {
				t.Fatal(err)
			}
			if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(context.Background(), intersectViewEvent{TheString: "E2", KeyOne: 2, KeyTwo: 20}); err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(context.Background(), intersectViewEvent{TheString: "E3", KeyOne: 1, KeyTwo: 30}); err != nil {
				t.Fatal(err)
			}
			if name != "time-multikey" && (len(*batches) != 3 || len((*batches)[2].Old) != 1) {
				t.Fatalf("%s key replacement = %#v", name, *batches)
			}
			if err := engine.AdvanceTime(context.Background(), time.Unix(11, 0).UTC()); err != nil {
				t.Fatal(err)
			}
			if len(*batches) < 4 {
				t.Fatalf("%s did not expire time-retained events: %#v", name, *batches)
			}
		})
	}
}

func intersectViewNames(results []Result) []string {
	names := make([]string, 0, len(results))
	for _, result := range results {
		if name, ok := result.Get("theString").Any().(string); ok {
			names = append(names, name)
		}
	}
	return names
}
