package esper

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

type viewUnionBean struct {
	TheString       string  `esper:"theString"`
	IntPrimitive    int     `esper:"intPrimitive"`
	IntBoxed        int     `esper:"intBoxed"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
}

type viewUnionS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

type viewUnionS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

func newViewUnionEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[viewUnionBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	return env, engine
}

func sendViewUnionBean(t *testing.T, engine *Engine, event viewUnionBean) {
	t.Helper()
	if err := engine.Send(context.Background(), "SupportBean", event); err != nil {
		t.Fatal(err)
	}
}

func deployViewUnionParity(t *testing.T, env *Environment, engine *Engine, stream Stream[viewUnionBean], name string) (*Statement, *[]ResultBatch) {
	t.Helper()
	plan, err := env.Build(stream.Query(StatementName(name), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := new([]ResultBatch)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		*batches = append(*batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return deployment.Statements()[0], batches
}

func unionSnapshotStrings(t *testing.T, statement *Statement, field string) []string {
	t.Helper()
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	values := make([]string, 0, len(snapshot.Results()))
	for _, result := range snapshot.Results() {
		value, ok := result.Get(field).Any().(string)
		if !ok {
			t.Fatalf("snapshot field %s = %#v", field, result.Get(field))
		}
		values = append(values, value)
	}
	return values
}

func lastBatchNewStrings(batches *[]ResultBatch) []string {
	if len(*batches) == 0 {
		return nil
	}
	batch := (*batches)[len(*batches)-1]
	values := make([]string, 0, len(batch.New))
	for _, result := range batch.New {
		if value, ok := result.Get("theString").Any().(string); ok {
			values = append(values, value)
		}
	}
	return values
}

func lastBatchOldStrings(batches *[]ResultBatch) []string {
	if len(*batches) == 0 {
		return nil
	}
	batch := (*batches)[len(*batches)-1]
	values := make([]string, 0, len(batch.Old))
	for _, result := range batch.Old {
		if value, ok := result.Get("theString").Any().(string); ok {
			values = append(values, value)
		}
	}
	return values
}

// TestViewUnionFirstUniqueAndFirstLengthParity covers
// ViewUnionFirstUniqueAndFirstLength: firstlength(3)+firstunique(theString)
// with retain-union retains an event while either child holds it and drops
// events that both children reject silently.
func TestViewUnionFirstUniqueAndFirstLengthParity(t *testing.T) {
	for name, windows := range map[string]CompositeWindowSpec{
		"first-length-then-first-unique": UnionWindows(FirstLength(3), FirstUnique(Field[viewUnionBean, string]("theString"))),
		"first-unique-then-first-length": UnionWindows(FirstUnique(Field[viewUnionBean, string]("theString")), FirstLength(3)),
	} {
		t.Run(name, func(t *testing.T) {
			env, engine := newViewUnionEnv(t)
			defer func() { _ = engine.Close(context.Background()) }()
			statement, batches := deployViewUnionParity(t, env, engine,
				From[viewUnionBean](env, "SupportBean").Window(windows), "union-"+name)
			send := func(theString string, intPrimitive int) {
				t.Helper()
				sendViewUnionBean(t, engine, viewUnionBean{TheString: theString, IntPrimitive: intPrimitive})
			}
			send("E1", 1)
			if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E1"}) {
				t.Fatalf("after E1/1 snapshot = %v", got)
			}
			if got := lastBatchNewStrings(batches); !reflect.DeepEqual(got, []string{"E1"}) {
				t.Fatalf("after E1/1 new = %v", got)
			}
			send("E1", 2)
			if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E1", "E1"}) {
				t.Fatalf("after E1/2 snapshot = %v", got)
			}
			if got := lastBatchNewStrings(batches); !reflect.DeepEqual(got, []string{"E1"}) {
				t.Fatalf("after E1/2 new = %v", got)
			}
			send("E2", 1)
			if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E1", "E1", "E2"}) {
				t.Fatalf("after E2/1 snapshot = %v", got)
			}
			if got := lastBatchNewStrings(batches); !reflect.DeepEqual(got, []string{"E2"}) {
				t.Fatalf("after E2/1 new = %v", got)
			}
			before := len(*batches)
			send("E2", 3)
			if len(*batches) != before {
				t.Fatalf("E2/3 must be silent (both children full/dropped), batches=%d", len(*batches))
			}
			if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E1", "E1", "E2"}) {
				t.Fatalf("after E2/3 snapshot = %v", got)
			}
			send("E3", 3)
			if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E1", "E1", "E2", "E3"}) {
				t.Fatalf("after E3/3 snapshot = %v", got)
			}
			if got := lastBatchNewStrings(batches); !reflect.DeepEqual(got, []string{"E3"}) {
				t.Fatalf("after E3/3 new = %v", got)
			}
			before = len(*batches)
			send("E3", 4)
			if len(*batches) != before {
				t.Fatalf("E3/4 must be silent, batches=%d", len(*batches))
			}
		})
	}
}

// TestViewUnionBatchWindowParity covers ViewUnionBatchWindow:
// length_batch(3)+unique(intPrimitive) retain-union. The unique child emits
// each arriving event as new; the batch child flushes the previous batch as
// old every three events, and only events no longer retained by either child
// leave the union.
func TestViewUnionBatchWindowParity(t *testing.T) {
	env, engine := newViewUnionEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	statement, batches := deployViewUnionParity(t, env, engine,
		From[viewUnionBean](env, "SupportBean").Window(UnionWindows(
			LengthBatch(3),
			Unique(Field[viewUnionBean, int]("intPrimitive")),
		)), "union-length-batch")
	send := func(theString string, intPrimitive int) {
		t.Helper()
		sendViewUnionBean(t, engine, viewUnionBean{TheString: theString, IntPrimitive: intPrimitive})
	}
	events := []viewUnionBean{
		{TheString: "E1", IntPrimitive: 1},
		{TheString: "E2", IntPrimitive: 2},
		{TheString: "E3", IntPrimitive: 3},
		{TheString: "E4", IntPrimitive: 4},
		{TheString: "E5", IntPrimitive: 4},
		{TheString: "E6", IntPrimitive: 4},
		{TheString: "E7", IntPrimitive: 5},
		{TheString: "E8", IntPrimitive: 6},
		{TheString: "E9", IntPrimitive: 7},
	}
	wantSnapshots := [][]string{
		{"E1"},
		{"E1", "E2"},
		{"E1", "E2", "E3"},
		{"E1", "E2", "E3", "E4"},
		{"E1", "E2", "E3", "E4", "E5"},
		{"E1", "E2", "E3", "E4", "E5", "E6"},
		{"E1", "E2", "E3", "E4", "E5", "E6", "E7"},
		{"E1", "E2", "E3", "E4", "E5", "E6", "E7", "E8"},
		{"E1", "E2", "E3", "E6", "E7", "E8", "E9"},
	}
	for index, event := range events {
		send(event.TheString, event.IntPrimitive)
		if got := lastBatchNewStrings(batches); !reflect.DeepEqual(got, []string{event.TheString}) {
			t.Fatalf("after %s new = %v", event.TheString, got)
		}
		if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, wantSnapshots[index]) {
			t.Fatalf("snapshot after %s = %v, want %v", event.TheString, got, wantSnapshots[index])
		}
		switch event.TheString {
		case "E7", "E8", "E9":
			// The first batch (E1-E3) leaves the batch child at E6 but stays
			// in the unique child; only E4/E5 leave the union at E9 (E6 is
			// still retained by the unique child).
			if event.TheString == "E9" {
				if got := lastBatchOldStrings(batches); !sameStringSet(got, []string{"E4", "E5"}) {
					t.Fatalf("after E9 old = %v", got)
				}
			}
		}
	}
}

// TestViewUnionDerivedValueParity covers ViewUnionAndDerivedValue: the Java
// #uni(doublePrimitive) derived view is expressed in Go as a Sum aggregate
// over the union window, which exposes the same observable "total".
func TestViewUnionDerivedValueParity(t *testing.T) {
	env, engine := newViewUnionEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	plan, err := env.Build(From[viewUnionBean](env, "SupportBean").Window(UnionWindows(
		Unique(Field[viewUnionBean, int]("intPrimitive")),
		Unique(Field[viewUnionBean, int]("intBoxed")),
	)).Aggregate(
		Alias("total", Sum[float64](Field[viewUnionBean, float64]("doublePrimitive"))),
	).Query(StatementName("s0")))
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
	send := func(theString string, intPrimitive, intBoxed int, doublePrimitive float64) {
		t.Helper()
		sendViewUnionBean(t, engine, viewUnionBean{TheString: theString, IntPrimitive: intPrimitive, IntBoxed: intBoxed, DoublePrimitive: doublePrimitive})
	}
	send("E1", 1, 10, 100)
	send("E2", 2, 20, 50)
	send("E3", 1, 20, 20)
	if !reflect.DeepEqual(totals, []float64{100, 150, 170}) {
		t.Fatalf("totals = %v, want [100 150 170]", totals)
	}
}

// TestViewUnionGroupByParity covers ViewUnionGroupBy:
// #groupwin(intPrimitive)#length(2)#unique(intBoxed) retain-union. The Java
// plan exposes two union children: groupwin(intPrimitive)#length(2) and
// groupwin(intPrimitive)#unique(intBoxed) (the unique view is grouped by the
// same groupwin key), so Go expresses the same shape with two GroupWindow
// children.
func TestViewUnionGroupByParity(t *testing.T) {
	env, engine := newViewUnionEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	statement, batches := deployViewUnionParity(t, env, engine,
		From[viewUnionBean](env, "SupportBean").Window(UnionWindows(
			GroupWindow(Field[viewUnionBean, int]("intPrimitive"), LengthWindow(2)),
			GroupWindow(Field[viewUnionBean, int]("intPrimitive"), Unique(Field[viewUnionBean, int]("intBoxed"))),
		)), "union-group")
	send := func(theString string, intPrimitive, intBoxed int) {
		t.Helper()
		sendViewUnionBean(t, engine, viewUnionBean{TheString: theString, IntPrimitive: intPrimitive, IntBoxed: intBoxed})
	}
	send("E1", 1, 10)
	if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E1"}) {
		t.Fatalf("after E1 snapshot = %v", got)
	}
	send("E2", 2, 10)
	if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E1", "E2"}) {
		t.Fatalf("after E2 snapshot = %v", got)
	}
	send("E3", 1, 20)
	if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E1", "E2", "E3"}) {
		t.Fatalf("after E3 snapshot = %v", got)
	}
	send("E4", 1, 30)
	if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E1", "E2", "E3", "E4"}) {
		t.Fatalf("after E4 snapshot = %v", got)
	}
	send("E5", 2, 10)
	if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E1", "E2", "E3", "E4", "E5"}) {
		t.Fatalf("after E5 snapshot = %v", got)
	}
	send("E6", 1, 20)
	if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E1", "E2", "E4", "E5", "E6"}) {
		t.Fatalf("after E6 snapshot = %v", got)
	}
	if got := lastBatchOldStrings(batches); !sameStringSet(got, []string{"E3"}) {
		t.Fatalf("after E6 old = %v", got)
	}
	send("E7", 1, 10)
	if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E2", "E4", "E5", "E6", "E7"}) {
		t.Fatalf("after E7 snapshot = %v", got)
	}
	if got := lastBatchOldStrings(batches); !sameStringSet(got, []string{"E1"}) {
		t.Fatalf("after E7 old = %v", got)
	}
	send("E8", 2, 10)
	if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E4", "E5", "E6", "E7", "E8"}) {
		t.Fatalf("after E8 snapshot = %v", got)
	}
	if got := lastBatchOldStrings(batches); !sameStringSet(got, []string{"E2"}) {
		t.Fatalf("after E8 old = %v", got)
	}
}

// TestViewUnionTwoUniqueParity covers ViewUnionTwoUnique with the full
// E1-E10 replacement trajectory from the Java execution.
func TestViewUnionTwoUniqueParity(t *testing.T) {
	env, engine := newViewUnionEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	statement, batches := deployViewUnionParity(t, env, engine,
		From[viewUnionBean](env, "SupportBean").Window(UnionWindows(
			Unique(Field[viewUnionBean, int]("intPrimitive")),
			Unique(Field[viewUnionBean, int]("intBoxed")),
		)), "union-two-unique")
	send := func(theString string, intPrimitive, intBoxed int) {
		t.Helper()
		sendViewUnionBean(t, engine, viewUnionBean{TheString: theString, IntPrimitive: intPrimitive, IntBoxed: intBoxed})
	}
	sequence := []struct {
		event viewUnionBean
		want  []string
		old   []string
	}{
		{viewUnionBean{TheString: "E1", IntPrimitive: 1, IntBoxed: 10}, []string{"E1"}, nil},
		{viewUnionBean{TheString: "E2", IntPrimitive: 2, IntBoxed: 10}, []string{"E1", "E2"}, nil},
		{viewUnionBean{TheString: "E3", IntPrimitive: 1, IntBoxed: 20}, []string{"E2", "E3"}, []string{"E1"}},
		{viewUnionBean{TheString: "E4", IntPrimitive: 1, IntBoxed: 20}, []string{"E2", "E4"}, []string{"E3"}},
		{viewUnionBean{TheString: "E5", IntPrimitive: 2, IntBoxed: 30}, []string{"E2", "E4", "E5"}, nil},
		{viewUnionBean{TheString: "E6", IntPrimitive: 3, IntBoxed: 10}, []string{"E4", "E5", "E6"}, []string{"E2"}},
		{viewUnionBean{TheString: "E7", IntPrimitive: 3, IntBoxed: 30}, []string{"E4", "E5", "E6", "E7"}, nil},
		{viewUnionBean{TheString: "E8", IntPrimitive: 4, IntBoxed: 10}, []string{"E4", "E5", "E7", "E8"}, []string{"E6"}},
		{viewUnionBean{TheString: "E9", IntPrimitive: 3, IntBoxed: 50}, []string{"E4", "E5", "E7", "E8", "E9"}, nil},
		{viewUnionBean{TheString: "E10", IntPrimitive: 2, IntBoxed: 30}, []string{"E4", "E8", "E9", "E10"}, []string{"E5", "E7"}},
	}
	for _, step := range sequence {
		send(step.event.TheString, step.event.IntPrimitive, step.event.IntBoxed)
		if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, step.want) {
			t.Fatalf("snapshot after %s = %v, want %v", step.event.TheString, got, step.want)
		}
		if len(step.old) > 0 {
			if got := lastBatchOldStrings(batches); !sameStringSet(got, step.old) {
				t.Fatalf("old after %s = %v, want %v", step.event.TheString, got, step.old)
			}
		}
	}
}

// TestViewUnionThreeUniqueParity covers ViewUnionThreeUnique.
func TestViewUnionThreeUniqueParity(t *testing.T) {
	env, engine := newViewUnionEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	statement, _ := deployViewUnionParity(t, env, engine,
		From[viewUnionBean](env, "SupportBean").Window(UnionWindows(
			Unique(Field[viewUnionBean, int]("intPrimitive")),
			Unique(Field[viewUnionBean, int]("intBoxed")),
			Unique(Field[viewUnionBean, float64]("doublePrimitive")),
		)), "union-three-unique")
	send := func(theString string, intPrimitive, intBoxed int, doublePrimitive float64) {
		t.Helper()
		sendViewUnionBean(t, engine, viewUnionBean{TheString: theString, IntPrimitive: intPrimitive, IntBoxed: intBoxed, DoublePrimitive: doublePrimitive})
	}
	send("E1", 1, 10, 100)
	send("E2", 2, 10, 200)
	send("E3", 2, 20, 100)
	send("E4", 1, 30, 300)
	if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E2", "E3", "E4"}) {
		t.Fatalf("after E4 snapshot = %v", got)
	}
}

// TestViewUnionSortedParity covers ViewUnionSorted:
// sort(2,intPrimitive) union sort(2,intBoxed) retain-union.
func TestViewUnionSortedParity(t *testing.T) {
	env, engine := newViewUnionEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	statement, batches := deployViewUnionParity(t, env, engine,
		From[viewUnionBean](env, "SupportBean").Window(UnionWindows(
			SortWindow(2, Ascending(Field[viewUnionBean, int]("intPrimitive"))),
			SortWindow(2, Ascending(Field[viewUnionBean, int]("intBoxed"))),
		)), "union-sorted")
	send := func(theString string, intPrimitive, intBoxed int) {
		t.Helper()
		sendViewUnionBean(t, engine, viewUnionBean{TheString: theString, IntPrimitive: intPrimitive, IntBoxed: intBoxed})
	}
	sequence := []struct {
		event viewUnionBean
		want  []string
	}{
		{viewUnionBean{TheString: "E1", IntPrimitive: 1, IntBoxed: 10}, []string{"E1"}},
		{viewUnionBean{TheString: "E2", IntPrimitive: 2, IntBoxed: 9}, []string{"E1", "E2"}},
		{viewUnionBean{TheString: "E3", IntPrimitive: 0, IntBoxed: 0}, []string{"E1", "E2", "E3"}},
		{viewUnionBean{TheString: "E4", IntPrimitive: -1, IntBoxed: -1}, []string{"E3", "E4"}},
		{viewUnionBean{TheString: "E5", IntPrimitive: 1, IntBoxed: 1}, []string{"E3", "E4"}},
		{viewUnionBean{TheString: "E6", IntPrimitive: 0, IntBoxed: 0}, []string{"E4", "E6"}},
	}
	for _, step := range sequence {
		send(step.event.TheString, step.event.IntPrimitive, step.event.IntBoxed)
		if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, step.want) {
			t.Fatalf("snapshot after %s = %v, want %v", step.event.TheString, got, step.want)
		}
	}
	// E4 expels E1/E2 from both children.
	if len(*batches) < 4 || len((*batches)[3].Old) != 2 {
		t.Fatalf("E4 old rows = %v", (*batches)[3].Old)
	}
	// E5 enters both children and is removed by both (same event), so it
	// appears as old and new once.
	if len(*batches) < 5 || len((*batches)[4].Old) != 1 || len((*batches)[4].New) != 1 {
		t.Fatalf("E5 batch = %#v", (*batches)[4])
	}
	// E6 expels E3 from both children.
	if len(*batches) < 6 || len((*batches)[5].Old) != 1 {
		t.Fatalf("E6 old rows = %v", (*batches)[5].Old)
	}
}

// TestViewUnionTimeWinParity covers ViewUnionTimeWin and
// ViewUnionTimeWinSODA: unique(intPrimitive) union time(10s) in both orders.
func TestViewUnionTimeWinParity(t *testing.T) {
	for name, windows := range map[string]CompositeWindowSpec{
		"unique-time": UnionWindows(
			Unique(Field[viewUnionBean, int]("intPrimitive")),
			TimeWindow(10*time.Second),
		),
		"time-unique": UnionWindows(
			TimeWindow(10*time.Second),
			Unique(Field[viewUnionBean, int]("intPrimitive")),
		),
	} {
		t.Run(name, func(t *testing.T) {
			env, initial := newViewUnionEnv(t)
			if err := initial.Close(context.Background()); err != nil {
				t.Fatal(err)
			}
			origin := time.Unix(0, 0).UTC()
			engine := NewEngine(env, WithStartTime(origin))
			defer func() { _ = engine.Close(context.Background()) }()
			statement, batches := deployViewUnionParity(t, env, engine,
				From[viewUnionBean](env, "SupportBean").Window(windows), "union-"+name)
			advance := func(milliseconds int) {
				t.Helper()
				if err := engine.AdvanceTime(context.Background(), origin.Add(time.Duration(milliseconds)*time.Millisecond)); err != nil {
					t.Fatal(err)
				}
			}
			send := func(theString string, intPrimitive int) {
				t.Helper()
				sendViewUnionBean(t, engine, viewUnionBean{TheString: theString, IntPrimitive: intPrimitive})
			}
			advance(1000)
			send("E1", 1)
			advance(2000)
			send("E2", 2)
			advance(3000)
			send("E3", 1)
			if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E1", "E2", "E3"}) {
				t.Fatalf("at 3s snapshot = %v", got)
			}
			advance(4000)
			send("E4", 3)
			send("E5", 1)
			if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E1", "E2", "E3", "E4", "E5"}) {
				t.Fatalf("at 4s snapshot = %v", got)
			}
			send("E6", 3)
			if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E1", "E2", "E3", "E4", "E5", "E6"}) {
				t.Fatalf("at 4s after E6 snapshot = %v", got)
			}
			advance(5000)
			send("E7", 4)
			send("E8", 4)
			advance(6000)
			send("E9", 4)
			if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E1", "E2", "E3", "E4", "E5", "E6", "E7", "E8", "E9"}) {
				t.Fatalf("at 6s snapshot = %v", got)
			}
			before := len(*batches)
			advance(10999)
			if len(*batches) != before {
				t.Fatalf("unexpected batch before 11s: %d", len(*batches))
			}
			advance(11000)
			if got := lastBatchOldStrings(batches); !reflect.DeepEqual(got, []string{"E1"}) {
				t.Fatalf("at 11s old = %v", got)
			}
			if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E2", "E3", "E4", "E5", "E6", "E7", "E8", "E9"}) {
				t.Fatalf("at 11s snapshot = %v", got)
			}
			advance(12999)
			advance(13000)
			if got := lastBatchOldStrings(batches); !reflect.DeepEqual(got, []string{"E3"}) {
				t.Fatalf("at 13s old = %v", got)
			}
			if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E2", "E4", "E5", "E6", "E7", "E8", "E9"}) {
				t.Fatalf("at 13s snapshot = %v", got)
			}
			advance(14000)
			if got := lastBatchOldStrings(batches); !reflect.DeepEqual(got, []string{"E4"}) {
				t.Fatalf("at 14s old = %v", got)
			}
			advance(15000)
			if got := lastBatchOldStrings(batches); !sameStringSet(got, []string{"E7", "E8"}) {
				t.Fatalf("at 15s old = %v", got)
			}
			if got := unionSnapshotStrings(t, statement, "theString"); !sameStringSet(got, []string{"E2", "E5", "E6", "E9"}) {
				t.Fatalf("at 15s snapshot = %v", got)
			}
		})
	}
}

// TestViewUnionPatternParity covers ViewUnionPattern: completed pattern
// matches retained by unique(a.id) union unique(b.id).
func TestViewUnionPatternParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[viewUnionS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[viewUnionS1](env, "SupportBean_S1"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "PatternUnion", []FieldSpec{
		FieldDef("string", reflect.TypeOf("")),
		FieldDef("a_id", reflect.TypeOf(0)),
		FieldDef("b_id", reflect.TypeOf(0)),
	}); err != nil {
		t.Fatal(err)
	}
	a := From[viewUnionS0](env, "SupportBean_S0")
	b := From[viewUnionS1](env, "SupportBean_S1")
	pattern := PatternFrom(a, "a", Literal(true)).Then(PatternFrom(b, "b", Literal(true))).Every()
	producerPlan, err := env.Build(pattern.Select(
		Alias("string", Concat(TagField[string]("a", "p00"), TagField[string]("b", "p10"))),
		Alias("a_id", TagField[int]("a", "id")),
		Alias("b_id", TagField[int]("b", "id")),
	).InsertInto("PatternUnion", StatementName("producer")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromAny(env, "PatternUnion").Window(UnionWindows(
		Unique(Field[any, int]("a_id")),
		Unique(Field[any, int]("b_id")),
	)).Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.DeployPlans(context.Background(), []Plan{producerPlan, consumerPlan})
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[1]
	var batches []ResultBatch
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendS0 := func(id int, p00 string) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBean_S0", viewUnionS0{ID: id, P00: p00}); err != nil {
			t.Fatal(err)
		}
	}
	sendS1 := func(id int, p10 string) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBean_S1", viewUnionS1{ID: id, P10: p10}); err != nil {
			t.Fatal(err)
		}
	}
	patternStrings := func() []string {
		t.Helper()
		snapshot, err := statement.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		values := make([]string, 0, len(snapshot.Results()))
		for _, result := range snapshot.Results() {
			values = append(values, result.Get("string").Any().(string))
		}
		return values
	}
	sendS0(1, "E1")
	sendS1(2, "E2")
	if got := patternStrings(); !sameStringSet(got, []string{"E1E2"}) {
		t.Fatalf("first match snapshot = %v", got)
	}
	sendS0(10, "E3")
	sendS1(20, "E4")
	if got := patternStrings(); !sameStringSet(got, []string{"E1E2", "E3E4"}) {
		t.Fatalf("second match snapshot = %v", got)
	}
	sendS0(1, "E5")
	sendS1(2, "E6")
	if got := patternStrings(); !sameStringSet(got, []string{"E3E4", "E5E6"}) {
		t.Fatalf("third match snapshot = %v", got)
	}
	last := batches[len(batches)-1]
	var oldStrings []string
	for _, result := range last.Old {
		oldStrings = append(oldStrings, result.Get("string").Any().(string))
	}
	if !sameStringSet(oldStrings, []string{"E1E2"}) {
		t.Fatalf("third match old = %v", oldStrings)
	}
}

// TestViewUnionSubselectParity covers ViewUnionSubselect: a subquery over
// length(2) union unique(intPrimitive) retains the union contents.
func TestViewUnionSubselectParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[viewUnionBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[viewUnionS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	inner := Select(From[viewUnionBean](env, "SupportBean").Window(UnionWindows(
		LengthWindow(2),
		Unique(Field[viewUnionBean, int]("intPrimitive")),
	)))
	outer := From[viewUnionS0](env, "SupportBean_S0").Filter(SubqueryExists(
		inner,
		Equal[string](Field[viewUnionBean, string]("theString"), OuterField[string]("p00")),
	))
	plan, err := env.Build(outer.Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var invoked int
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		invoked += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(theString string, intPrimitive int) {
		t.Helper()
		sendViewUnionBean(t, engine, viewUnionBean{TheString: theString, IntPrimitive: intPrimitive})
	}
	sendS0 := func(id int, p00 string) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBean_S0", viewUnionS0{ID: id, P00: p00}); err != nil {
			t.Fatal(err)
		}
	}
	send("E1", 1)
	send("E2", 2)
	send("E3", 3)
	send("E4", 2)
	send("E5", 1)
	sendS0(1, "E1")
	if invoked != 0 {
		t.Fatalf("E1 unexpectedly matched: %d", invoked)
	}
	sendS0(1, "E2")
	if invoked != 0 {
		t.Fatalf("E2 unexpectedly matched: %d", invoked)
	}
	sendS0(1, "E3")
	if invoked != 1 {
		t.Fatalf("E3 match count = %d", invoked)
	}
	sendS0(1, "E4")
	if invoked != 2 {
		t.Fatalf("E4 match count = %d", invoked)
	}
	sendS0(1, "E5")
	if invoked != 3 {
		t.Fatalf("E5 match count = %d", invoked)
	}
}

// TestViewUnionInvalidParity covers ViewUnionInvalid: multiple groupwin
// declarations and a group/merge mismatch are rejected at Build.
func TestViewUnionInvalidParity(t *testing.T) {
	env, _ := newViewUnionEnv(t)
	theString := Field[viewUnionBean, string]("theString")
	intPrimitive := Field[viewUnionBean, int]("intPrimitive")
	// Java rejects a second groupwin declaration inside one window chain
	// ("Multiple groupwin-declarations are not supported"); the typed API
	// expresses the same boundary as a nested group window inside a grouped
	// retention. Java's group/merge mismatch is unrepresentable because Go
	// has no merge window view.
	_, err := env.Build(From[viewUnionBean](env, "SupportBean").Window(
		GroupWindow(theString, GroupWindow(intPrimitive, Unique(theString))),
	).Query(StatementName("invalid-nested-group")))
	if err == nil || !strings.Contains(err.Error(), "group") {
		t.Fatalf("nested groupwin error = %v", err)
	}
}

// TestViewUnionNamedWindowFirstUniqueAndFirstLengthParity covers
// ViewUnionFirstUniqueAndLengthOnDelete: a named window with firstunique+
// firstlength union retention and an on-delete trigger.
func TestViewUnionNamedWindowFirstUniqueAndFirstLengthParity(t *testing.T) {
	env, engine := newViewUnionEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	if _, err := RegisterStruct[viewUnionS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema missing")
	}
	if _, err := CreateNamedWindow(env, "MyWindowOne", schema, NamedWindowRetention(UnionWindows(
		FirstUnique(Field[viewUnionBean, string]("theString")),
		FirstLength(3),
	))); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(OnEvent(From[viewUnionBean](env, "SupportBean")).InsertIntoNamedWindow(
		"MyWindowOne",
		SetColumn("theString", Field[viewUnionBean, string]("theString")),
		SetColumn("intPrimitive", Field[viewUnionBean, int]("intPrimitive")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[viewUnionS0](env, "SupportBean_S0")).DeleteFromNamedWindow(
		"MyWindowOne",
		Equal[string](NamedWindowField[string]("theString"), Field[viewUnionS0, string]("p00")),
	).Query(StatementName("delete")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromNamedWindow(env, "MyWindowOne").Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.DeployPlans(context.Background(), []Plan{insertPlan, deletePlan, consumerPlan}); err != nil {
		t.Fatal(err)
	}
	send := func(theString string, intPrimitive int) {
		t.Helper()
		sendViewUnionBean(t, engine, viewUnionBean{TheString: theString, IntPrimitive: intPrimitive})
	}
	send("E1", 1)
	if got := namedWindowUnionStrings(t, engine, "MyWindowOne"); !sameStringSet(got, []string{"E1"}) {
		t.Fatalf("after E1/1 snapshot = %v", got)
	}
	send("E1", 99)
	if got := namedWindowUnionStrings(t, engine, "MyWindowOne"); !sameStringSet(got, []string{"E1", "E1"}) {
		t.Fatalf("after E1/99 snapshot = %v", got)
	}
	send("E2", 2)
	if got := namedWindowUnionStrings(t, engine, "MyWindowOne"); !sameStringSet(got, []string{"E1", "E1", "E2"}) {
		t.Fatalf("after E2 snapshot = %v", got)
	}
	if err := engine.Send(context.Background(), "SupportBean_S0", viewUnionS0{ID: 1, P00: "E1"}); err != nil {
		t.Fatal(err)
	}
	if got := namedWindowUnionStrings(t, engine, "MyWindowOne"); !sameStringSet(got, []string{"E2"}) {
		t.Fatalf("after delete E1 snapshot = %v", got)
	}
	send("E1", 3)
	if got := namedWindowUnionStrings(t, engine, "MyWindowOne"); !sameStringSet(got, []string{"E1", "E2"}) {
		t.Fatalf("after E1/3 snapshot = %v", got)
	}
}

func namedWindowUnionStrings(t *testing.T, engine *Engine, name string) []string {
	t.Helper()
	window, ok := engine.NamedWindow(name)
	if !ok {
		t.Fatalf("named window %s missing", name)
	}
	snapshot, err := window.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	values := make([]string, 0, len(snapshot))
	for _, event := range snapshot {
		values = append(values, event.Get("theString").Any().(string))
	}
	return values
}

// TestViewUnionNamedWindowTimeWinParity covers ViewUnionTimeWinNamedWindow
// and ViewUnionTimeWinNamedWindowDelete: named-window union of time(10s) and
// unique(intPrimitive) with insert, on-delete and time expiry.
func TestViewUnionNamedWindowTimeWinParity(t *testing.T) {
	env, initial := newViewUnionEnv(t)
	if err := initial.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[viewUnionS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema missing")
	}
	if _, err := CreateNamedWindow(env, "MyWindowThree", schema, NamedWindowRetention(UnionWindows(
		TimeWindow(10*time.Second),
		Unique(Field[viewUnionBean, int]("intPrimitive")),
	))); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(OnEvent(From[viewUnionBean](env, "SupportBean")).InsertIntoNamedWindow(
		"MyWindowThree",
		SetColumn("theString", Field[viewUnionBean, string]("theString")),
		SetColumn("intPrimitive", Field[viewUnionBean, int]("intPrimitive")),
		SetColumn("intBoxed", Field[viewUnionBean, int]("intBoxed")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[viewUnionS0](env, "SupportBean_S0")).DeleteFromNamedWindow(
		"MyWindowThree",
		Equal[int](NamedWindowField[int]("intBoxed"), Field[viewUnionS0, int]("id")),
	).Query(StatementName("delete")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromNamedWindow(env, "MyWindowThree").Query(StatementName("s0"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	origin := time.Unix(0, 0).UTC()
	engine := NewEngine(env, WithStartTime(origin))
	defer func() { _ = engine.Close(context.Background()) }()
	if _, err := engine.DeployPlans(context.Background(), []Plan{insertPlan, deletePlan, consumerPlan}); err != nil {
		t.Fatal(err)
	}
	advance := func(milliseconds int) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), origin.Add(time.Duration(milliseconds)*time.Millisecond)); err != nil {
			t.Fatal(err)
		}
	}
	send := func(theString string, intPrimitive, intBoxed int) {
		t.Helper()
		sendViewUnionBean(t, engine, viewUnionBean{TheString: theString, IntPrimitive: intPrimitive, IntBoxed: intBoxed})
	}
	sendS0 := func(id int) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBean_S0", viewUnionS0{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	advance(1000)
	send("E1", 1, 10)
	if got := namedWindowUnionStrings(t, engine, "MyWindowThree"); !sameStringSet(got, []string{"E1"}) {
		t.Fatalf("after E1 snapshot = %v", got)
	}
	advance(2000)
	send("E2", 2, 20)
	if got := namedWindowUnionStrings(t, engine, "MyWindowThree"); !sameStringSet(got, []string{"E1", "E2"}) {
		t.Fatalf("after E2 snapshot = %v", got)
	}
	sendS0(20)
	if got := namedWindowUnionStrings(t, engine, "MyWindowThree"); !sameStringSet(got, []string{"E1"}) {
		t.Fatalf("after delete E2 snapshot = %v", got)
	}
	advance(3000)
	send("E3", 3, 30)
	send("E4", 3, 40)
	if got := namedWindowUnionStrings(t, engine, "MyWindowThree"); !sameStringSet(got, []string{"E1", "E3", "E4"}) {
		t.Fatalf("after E3/E4 snapshot = %v", got)
	}
	advance(4000)
	send("E5", 4, 50)
	send("E6", 4, 50)
	if got := namedWindowUnionStrings(t, engine, "MyWindowThree"); !sameStringSet(got, []string{"E1", "E3", "E4", "E5", "E6"}) {
		t.Fatalf("after E5/E6 snapshot = %v", got)
	}
	sendS0(20)
	if got := namedWindowUnionStrings(t, engine, "MyWindowThree"); !sameStringSet(got, []string{"E1", "E3", "E4", "E5", "E6"}) {
		t.Fatalf("delete no-op snapshot = %v", got)
	}
	sendS0(50)
	if got := namedWindowUnionStrings(t, engine, "MyWindowThree"); !sameStringSet(got, []string{"E1", "E3", "E4"}) {
		t.Fatalf("after delete E5/E6 snapshot = %v", got)
	}
	advance(12999)
	advance(13000)
	if got := namedWindowUnionStrings(t, engine, "MyWindowThree"); !sameStringSet(got, []string{"E1", "E4"}) {
		t.Fatalf("after 13s expiry snapshot = %v", got)
	}
	advance(10000000)
	if got := namedWindowUnionStrings(t, engine, "MyWindowThree"); !sameStringSet(got, []string{"E1", "E4"}) {
		t.Fatalf("after final expiry snapshot = %v", got)
	}
}

// TestViewUnionNamedWindowTimeWinNoDeleteParity covers
// ViewUnionTimeWinNamedWindow: the full shared tryAssertionTimeWinUnique
// trajectory against a named window with time(10s) union unique(intPrimitive).
func TestViewUnionNamedWindowTimeWinNoDeleteParity(t *testing.T) {
	env, initial := newViewUnionEnv(t)
	if err := initial.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema missing")
	}
	if _, err := CreateNamedWindow(env, "MyWindowTwo", schema, NamedWindowRetention(UnionWindows(
		TimeWindow(10*time.Second),
		Unique(Field[viewUnionBean, int]("intPrimitive")),
	))); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(OnEvent(From[viewUnionBean](env, "SupportBean")).InsertIntoNamedWindow(
		"MyWindowTwo",
		SetColumn("theString", Field[viewUnionBean, string]("theString")),
		SetColumn("intPrimitive", Field[viewUnionBean, int]("intPrimitive")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	origin := time.Unix(0, 0).UTC()
	engine := NewEngine(env, WithStartTime(origin))
	defer func() { _ = engine.Close(context.Background()) }()
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	advance := func(milliseconds int) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), origin.Add(time.Duration(milliseconds)*time.Millisecond)); err != nil {
			t.Fatal(err)
		}
	}
	send := func(theString string, intPrimitive int) {
		t.Helper()
		sendViewUnionBean(t, engine, viewUnionBean{TheString: theString, IntPrimitive: intPrimitive})
	}
	advance(1000)
	send("E1", 1)
	if got := namedWindowUnionStrings(t, engine, "MyWindowTwo"); !sameStringSet(got, []string{"E1"}) {
		t.Fatalf("after E1 snapshot = %v", got)
	}
	advance(2000)
	send("E2", 2)
	advance(3000)
	send("E3", 1)
	if got := namedWindowUnionStrings(t, engine, "MyWindowTwo"); !sameStringSet(got, []string{"E1", "E2", "E3"}) {
		t.Fatalf("at 3s snapshot = %v", got)
	}
	advance(4000)
	send("E4", 3)
	send("E5", 1)
	if got := namedWindowUnionStrings(t, engine, "MyWindowTwo"); !sameStringSet(got, []string{"E1", "E2", "E3", "E4", "E5"}) {
		t.Fatalf("at 4s snapshot = %v", got)
	}
	send("E6", 3)
	if got := namedWindowUnionStrings(t, engine, "MyWindowTwo"); !sameStringSet(got, []string{"E1", "E2", "E3", "E4", "E5", "E6"}) {
		t.Fatalf("at 4s after E6 snapshot = %v", got)
	}
	advance(5000)
	send("E7", 4)
	send("E8", 4)
	advance(6000)
	send("E9", 4)
	if got := namedWindowUnionStrings(t, engine, "MyWindowTwo"); !sameStringSet(got, []string{"E1", "E2", "E3", "E4", "E5", "E6", "E7", "E8", "E9"}) {
		t.Fatalf("at 6s snapshot = %v", got)
	}
	advance(10999)
	if got := namedWindowUnionStrings(t, engine, "MyWindowTwo"); !sameStringSet(got, []string{"E1", "E2", "E3", "E4", "E5", "E6", "E7", "E8", "E9"}) {
		t.Fatalf("at 10999 snapshot = %v", got)
	}
	advance(11000)
	if got := namedWindowUnionStrings(t, engine, "MyWindowTwo"); !sameStringSet(got, []string{"E2", "E3", "E4", "E5", "E6", "E7", "E8", "E9"}) {
		t.Fatalf("at 11s snapshot = %v", got)
	}
	advance(12999)
	advance(13000)
	if got := namedWindowUnionStrings(t, engine, "MyWindowTwo"); !sameStringSet(got, []string{"E2", "E4", "E5", "E6", "E7", "E8", "E9"}) {
		t.Fatalf("at 13s snapshot = %v", got)
	}
	advance(14000)
	if got := namedWindowUnionStrings(t, engine, "MyWindowTwo"); !sameStringSet(got, []string{"E2", "E5", "E6", "E7", "E8", "E9"}) {
		t.Fatalf("at 14s snapshot = %v", got)
	}
	advance(15000)
	if got := namedWindowUnionStrings(t, engine, "MyWindowTwo"); !sameStringSet(got, []string{"E2", "E5", "E6", "E9"}) {
		t.Fatalf("at 15s snapshot = %v", got)
	}
	advance(1000000)
	if got := namedWindowUnionStrings(t, engine, "MyWindowTwo"); !sameStringSet(got, []string{"E2", "E5", "E6", "E9"}) {
		t.Fatalf("final snapshot = %v", got)
	}
}

func TestViewUnionHelpers(t *testing.T) {
	if !sameStringSet([]string{"b", "a"}, []string{"a", "b"}) {
		t.Fatal("sameStringSet order-insensitive")
	}
	if sameStringSet([]string{"a"}, []string{"a", "b"}) {
		t.Fatal("sameStringSet length mismatch")
	}
	if got := fmt.Sprintf("%d", 20*10); got != "200" {
		t.Fatalf("unexpected arithmetic %s", got)
	}
}
