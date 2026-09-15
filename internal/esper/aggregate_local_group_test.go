package esper

import (
	"context"
	"math"
	"reflect"
	"testing"
)

type localGroupEvent struct {
	ID    string `esper:"id"`
	Group string `esper:"group"`
	Level int    `esper:"level"`
	Value int64  `esper:"value"`
}

type localGroupDeleteEvent struct {
	ID string `esper:"id"`
}

func newLocalGroupTest(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[localGroupEvent](env, "LocalGroupEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[localGroupDeleteEvent](env, "LocalGroupDeleteEvent"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env)
}

func TestLocalGroupByMultiKeyTrace(t *testing.T) {
	env, engine := newLocalGroupTest(t)
	group := Field[localGroupEvent, string]("group")
	level := Field[localGroupEvent, int]("level")
	value := Field[localGroupEvent, int64]("value")
	plan, err := env.Build(From[localGroupEvent](env, "LocalGroupEvent").Aggregate(
		Alias("pairSum", LocalGroupBy[int64](Sum[int64](value), group, level)),
		Alias("groupSum", LocalGroupBy[int64](Sum[int64](value), group)),
		Alias("levelSum", LocalGroupBy[int64](Sum[int64](value), level)),
		Alias("total", Sum[int64](value)),
	).Query(StatementName("local-group-multi-key")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var latest Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			latest, _ = batch.New[len(batch.New)-1].Row()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	expected := []struct {
		event                     localGroupEvent
		pair, group, level, total int64
	}{
		{localGroupEvent{"E1-1", "E1", 1, 10}, 10, 10, 10, 10},
		{localGroupEvent{"E2-2", "E2", 2, 11}, 11, 11, 11, 21},
		{localGroupEvent{"E1-2", "E1", 2, 12}, 12, 22, 23, 33},
		{localGroupEvent{"E1-1b", "E1", 1, 13}, 23, 35, 23, 46},
		{localGroupEvent{"E2-1", "E2", 1, 14}, 14, 25, 37, 60},
	}
	for index, item := range expected {
		if err := engine.SendEvent(context.Background(), item.event); err != nil {
			t.Fatal(err)
		}
		if latest.Get("pairSum").Any() != item.pair || latest.Get("groupSum").Any() != item.group || latest.Get("levelSum").Any() != item.level || latest.Get("total").Any() != item.total {
			t.Fatalf("local group row %d = %#v", index, latest.AsMap())
		}
	}
}

func TestLocalGroupByOuterGroupsShareCrossGroupState(t *testing.T) {
	env, engine := newLocalGroupTest(t)
	group := Field[localGroupEvent, string]("group")
	level := Field[localGroupEvent, int]("level")
	value := Field[localGroupEvent, int64]("value")
	event := EventValue[localGroupEvent]()
	plan, err := env.Build(From[localGroupEvent](env, "LocalGroupEvent").Window(KeepAll()).GroupBy(group, level).Select(
		Alias("group", group),
		Alias("level", level),
		Alias("groupSum", LocalGroupBy[int64](Sum[int64](value), group)),
		Alias("levelSum", LocalGroupBy[int64](Sum[int64](value), level)),
		Alias("allSum", LocalGroupBy[int64](Sum[int64](value))),
		Alias("groupValues", LocalGroupBy[[]localGroupEvent](WindowValues[localGroupEvent](event), group)),
		Alias("levelValues", LocalGroupBy[[]localGroupEvent](WindowValues[localGroupEvent](event), level)),
		Alias("allValues", LocalGroupBy[[]localGroupEvent](WindowValues[localGroupEvent](event))),
	).Query(StatementName("local-group-cross-outer")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var latest Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			latest, _ = batch.New[len(batch.New)-1].Row()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	events := []localGroupEvent{
		{ID: "1", Group: "E1", Level: 10, Value: 100},
		{ID: "2", Group: "E1", Level: 20, Value: 202},
		{ID: "3", Group: "E2", Level: 10, Value: 303},
		{ID: "4", Group: "E1", Level: 10, Value: 404},
		{ID: "5", Group: "E2", Level: 10, Value: 505},
	}
	expected := []struct {
		group, level               any
		groupSum, levelSum, allSum int64
		groupValues, levelValues   []localGroupEvent
		allValues                  []localGroupEvent
	}{
		{"E1", 10, 100, 100, 100, []localGroupEvent{events[0]}, []localGroupEvent{events[0]}, []localGroupEvent{events[0]}},
		{"E1", 20, 302, 202, 302, []localGroupEvent{events[0], events[1]}, []localGroupEvent{events[1]}, []localGroupEvent{events[0], events[1]}},
		{"E2", 10, 303, 403, 605, []localGroupEvent{events[2]}, []localGroupEvent{events[0], events[2]}, []localGroupEvent{events[0], events[1], events[2]}},
		{"E1", 10, 706, 807, 1009, []localGroupEvent{events[0], events[1], events[3]}, []localGroupEvent{events[0], events[2], events[3]}, []localGroupEvent{events[0], events[1], events[2], events[3]}},
		{"E2", 10, 808, 1312, 1514, []localGroupEvent{events[2], events[4]}, []localGroupEvent{events[0], events[2], events[3], events[4]}, events},
	}
	for index, event := range events {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		if latest.Get("group").Any() != expected[index].group || latest.Get("level").Any() != expected[index].level ||
			latest.Get("groupSum").Any() != expected[index].groupSum || latest.Get("levelSum").Any() != expected[index].levelSum || latest.Get("allSum").Any() != expected[index].allSum {
			t.Fatalf("cross-group local aggregate row %d = %#v", index, latest.AsMap())
		}
		for _, name := range []string{"groupValues", "levelValues", "allValues"} {
			if got, ok := latest.Get(name).Any().([]localGroupEvent); !ok {
				t.Fatalf("cross-group local aggregate %d %s type = %#v", index, name, latest.Get(name).Any())
			} else {
				var want []localGroupEvent
				switch name {
				case "groupValues":
					want = expected[index].groupValues
				case "levelValues":
					want = expected[index].levelValues
				case "allValues":
					want = expected[index].allValues
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("cross-group local aggregate row %d %s = %#v, want %#v", index, name, got, want)
				}
			}
		}
	}
}

func TestLocalGroupByNamedWindowDeleteRecomputesOuterAndLocalState(t *testing.T) {
	env, _ := newLocalGroupTest(t)
	schema, ok := env.Schema("LocalGroupEvent")
	if !ok {
		t.Fatal("local group schema is missing")
	}
	if _, err := CreateNamedWindow(env, "local-group-window", schema); err != nil {
		t.Fatal(err)
	}
	source := From[localGroupEvent](env, "LocalGroupEvent")
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow("local-group-window",
		SetColumn("id", Field[localGroupEvent, string]("id")),
		SetColumn("group", Field[localGroupEvent, string]("group")),
		SetColumn("level", Field[localGroupEvent, int]("level")),
		SetColumn("value", Field[localGroupEvent, int64]("value")),
	).Query(StatementName("local-group-insert")))
	if err != nil {
		t.Fatal(err)
	}
	group := Field[any, string]("group")
	level := Field[any, int]("level")
	value := Field[any, int64]("value")
	aggregatePlan, err := env.Build(FromNamedWindow(env, "local-group-window").GroupBy(group, level).Select(
		Alias("group", group),
		Alias("level", level),
		Alias("groupSum", LocalGroupBy[int64](Sum[int64](value), group)),
		Alias("levelSum", LocalGroupBy[int64](Sum[int64](value), level)),
		Alias("allSum", LocalGroupBy[int64](Sum[int64](value))),
	).Query(StatementName("local-group-delete-query")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[localGroupDeleteEvent](env, "LocalGroupDeleteEvent")).DeleteFromNamedWindow(
		"local-group-window",
		Equal[string](NamedWindowField[string]("id"), Field[localGroupDeleteEvent, string]("id")),
	).Query(StatementName("local-group-delete")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), aggregatePlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), deletePlan); err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 5)
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
	for _, event := range []localGroupEvent{
		{ID: "1", Group: "E1", Level: 10, Value: 100},
		{ID: "2", Group: "E1", Level: 20, Value: 202},
		{ID: "3", Group: "E2", Level: 10, Value: 303},
		{ID: "4", Group: "E1", Level: 10, Value: 404},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.SendEvent(context.Background(), localGroupDeleteEvent{ID: "1"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("local group delete rows = %d", len(rows))
	}
	last := rows[len(rows)-1]
	if last.Get("group").Any() != "E1" || last.Get("level").Any() != 10 || last.Get("groupSum").Any() != int64(606) || last.Get("levelSum").Any() != int64(707) || last.Get("allSum").Any() != int64(909) {
		t.Fatalf("local group delete row = %#v", last.AsMap())
	}
}

func TestLocalGroupByRejectsInvalidKeys(t *testing.T) {
	env, _ := newLocalGroupTest(t)
	value := Field[localGroupEvent, int64]("value")
	group := Field[localGroupEvent, string]("group")
	if _, err := env.Build(From[localGroupEvent](env, "LocalGroupEvent").Aggregate(
		Alias("bad", LocalGroupBy[int64](Sum[int64](value), nil)),
	).Query(StatementName("local-group-nil-key"))); err == nil {
		t.Fatal("local group aggregate with nil key unexpectedly built")
	}
	if _, err := env.Build(From[localGroupEvent](env, "LocalGroupEvent").Aggregate(
		Alias("bad", LocalGroupBy[int64](Sum[int64](value), CountAll())),
	).Query(StatementName("local-group-aggregate-key"))); err == nil {
		t.Fatal("local group aggregate with aggregate key unexpectedly built")
	}
	if _, err := env.Build(From[localGroupEvent](env, "LocalGroupEvent").Aggregate(
		Alias("good", LocalGroupBy[int64](Sum[int64](value), group)),
	).Query(StatementName("local-group-valid-key"))); err != nil {
		t.Fatalf("valid local group key rejected: %v", err)
	}
}

// TestNamedWindowDeleteStreamSelectionGate pins the stream-selection gate on
// named-window removals: a plain query is istream-only (Java's default stream
// selection), so a deleted row produces NO callback, while an explicit
// irstream query observes the removal as an old-only row computed over the
// post-removal state.
func TestNamedWindowDeleteStreamSelectionGate(t *testing.T) {
	env, _ := newLocalGroupTest(t)
	schema, ok := env.Schema("LocalGroupEvent")
	if !ok {
		t.Fatal("local group schema is missing")
	}
	if _, err := CreateNamedWindow(env, "stream-gate-window", schema); err != nil {
		t.Fatal(err)
	}
	source := From[localGroupEvent](env, "LocalGroupEvent")
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow("stream-gate-window",
		SetColumn("id", Field[localGroupEvent, string]("id")),
		SetColumn("group", Field[localGroupEvent, string]("group")),
		SetColumn("level", Field[localGroupEvent, int]("level")),
		SetColumn("value", Field[localGroupEvent, int64]("value")),
	).Query(StatementName("stream-gate-insert")))
	if err != nil {
		t.Fatal(err)
	}
	id := Field[any, string]("id")
	value := Field[any, int64]("value")
	newOnlyPlan, err := env.Build(FromNamedWindow(env, "stream-gate-window").Aggregate(
		Alias("id", id),
		Alias("c0", Sum[int64](value)),
	).Query(StatementName("stream-gate-new-only")))
	if err != nil {
		t.Fatal(err)
	}
	iStreamPlan, err := env.Build(FromNamedWindow(env, "stream-gate-window").Aggregate(
		Alias("id", id),
		Alias("c0", Sum[int64](value)),
	).Query(StatementName("stream-gate-irstream"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[localGroupDeleteEvent](env, "LocalGroupDeleteEvent")).DeleteFromNamedWindow(
		"stream-gate-window",
		Equal[string](NamedWindowField[string]("id"), Field[localGroupDeleteEvent, string]("id")),
	).Query(StatementName("stream-gate-delete")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	newOnlyDeployment, err := engine.Deploy(context.Background(), newOnlyPlan)
	if err != nil {
		t.Fatal(err)
	}
	iStreamDeployment, err := engine.Deploy(context.Background(), iStreamPlan)
	if err != nil {
		t.Fatal(err)
	}
	for _, plan := range []Plan{insertPlan, deletePlan} {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	type observed struct {
		newRows, oldRows int
		oldSums          []int64
	}
	newOnlyBatches := make([]observed, 0, 2)
	iStreamBatches := make([]observed, 0, 2)
	collect := func(target *[]observed) Listener {
		return func(_ context.Context, batch ResultBatch) error {
			entry := observed{newRows: len(batch.New), oldRows: len(batch.Old)}
			for _, result := range batch.Old {
				if row, ok := result.Row(); ok {
					sum, _ := row.Get("c0").Any().(int64)
					entry.oldSums = append(entry.oldSums, sum)
				}
			}
			*target = append(*target, entry)
			return nil
		}
	}
	if _, err := newOnlyDeployment.Statements()[0].Subscribe(collect(&newOnlyBatches)); err != nil {
		t.Fatal(err)
	}
	if _, err := iStreamDeployment.Statements()[0].Subscribe(collect(&iStreamBatches)); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := engine.Send(ctx, "LocalGroupEvent", localGroupEvent{ID: "A", Group: "E1", Level: 1, Value: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(ctx, "LocalGroupEvent", localGroupEvent{ID: "B", Group: "E2", Level: 2, Value: 15}); err != nil {
		t.Fatal(err)
	}
	if len(newOnlyBatches) != 2 || len(iStreamBatches) != 2 {
		t.Fatalf("insert batches = %d/%d, want two per statement", len(newOnlyBatches), len(iStreamBatches))
	}
	if err := engine.Send(ctx, "LocalGroupDeleteEvent", localGroupDeleteEvent{ID: "A"}); err != nil {
		t.Fatal(err)
	}
	if len(newOnlyBatches) != 2 {
		t.Fatalf("istream-only query observed %d extra batches on a named-window delete: %#v", len(newOnlyBatches)-2, newOnlyBatches[2:])
	}
	if len(iStreamBatches) != 3 {
		t.Fatalf("irstream query batches = %d, want three", len(iStreamBatches))
	}
	removal := iStreamBatches[2]
	if removal.newRows != 0 || removal.oldRows != 1 || len(removal.oldSums) != 1 || removal.oldSums[0] != 15 {
		t.Fatalf("removal batch = %#v, want one old row carrying the post-removal sum 15", removal)
	}
	if err := engine.Send(ctx, "LocalGroupEvent", localGroupEvent{ID: "C", Group: "E3", Level: 3, Value: 20}); err != nil {
		t.Fatal(err)
	}
	if len(newOnlyBatches) != 3 || len(iStreamBatches) != 4 {
		t.Fatalf("post-delete insert batches = %d/%d, want three and four", len(newOnlyBatches), len(iStreamBatches))
	}
}

// TestLocalGroupByUncoveredKeyRoutesToRowPerEvent pins Esper's routing rule
// for local group-by keys: when a key is not covered by the outer group-by
// list, the statement is row-per-event even though every projection is an
// aggregate (Esper's localGroupByMatchesGroupBy check), so a snapshot carries
// one row per retained event while a covered key set keeps the fully
// aggregated single row.
func TestLocalGroupByUncoveredKeyRoutesToRowPerEvent(t *testing.T) {
	env, _ := newLocalGroupTest(t)
	group := Field[localGroupEvent, string]("group")
	level := Field[localGroupEvent, int]("level")
	uncovered := From[localGroupEvent](env, "LocalGroupEvent").Window(KeepAll()).Aggregate(
		Alias("c0", LocalGroupBy[int](Sum[int](level), group)),
	).Query(StatementName("uncovered-local-group"))
	covered := From[localGroupEvent](env, "LocalGroupEvent").Window(KeepAll()).Aggregate(
		Alias("c0", LocalGroupBy[int](Sum[int](level))),
	).Query(StatementName("covered-local-group"))
	uncoveredPlan, err := env.Build(uncovered)
	if err != nil {
		t.Fatal(err)
	}
	coveredPlan, err := env.Build(covered)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	uncoveredDeployment, err := engine.Deploy(context.Background(), uncoveredPlan)
	if err != nil {
		t.Fatal(err)
	}
	coveredDeployment, err := engine.Deploy(context.Background(), coveredPlan)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, event := range []localGroupEvent{
		{ID: "1", Group: "E1", Level: 10, Value: 100},
		{ID: "2", Group: "E2", Level: 20, Value: 200},
		{ID: "3", Group: "E2", Level: 30, Value: 300},
	} {
		if err := engine.Send(ctx, "LocalGroupEvent", event); err != nil {
			t.Fatal(err)
		}
	}
	read := func(deployment *Deployment) []int {
		statement := deployment.Statements()[0]
		result, err := statement.Snapshot(ctx)
		if err != nil {
			t.Fatal(err)
		}
		values := make([]int, 0, len(result.Batch.New))
		for _, entry := range result.Batch.New {
			row, ok := entry.Row()
			if !ok {
				t.Fatalf("snapshot result is not a row: %#v", entry)
			}
			value, _ := row.Get("c0").Any().(int)
			values = append(values, value)
		}
		return values
	}
	if got := read(uncoveredDeployment); !reflect.DeepEqual(got, []int{10, 50, 50}) {
		t.Fatalf("uncovered local group key snapshot = %v, want one row per retained event [10 50 50]", got)
	}
	if got := read(coveredDeployment); !reflect.DeepEqual(got, []int{60}) {
		t.Fatalf("covered local group key snapshot = %v, want the fully aggregated row [60]", got)
	}
}

// localGroupArrayEvent mirrors the Java support bean SupportThreeArrayEvent:
// three array-typed properties used as local group-by keys.
type localGroupArrayEvent struct {
	ID          string    `esper:"id"`
	Value       int32     `esper:"value"`
	IntArray    []int32   `esper:"intArray"`
	LongArray   []int64   `esper:"longArray"`
	DoubleArray []float64 `esper:"doubleArray"`
}

// TestLocalGroupByArrayKeyDeepContentEquality pins Esper's array-typed local
// group-by key semantics: keys compare by deep CONTENT (a distinct array
// instance with equal content shares the group), each array type keeps its own
// level, and the multi-key tuple delegates to the same content comparison.
func TestLocalGroupByArrayKeyDeepContentEquality(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[localGroupArrayEvent](env, "LocalGroupArrayEvent"); err != nil {
		t.Fatal(err)
	}
	value := Field[localGroupArrayEvent, int32]("value")
	intArray := Field[localGroupArrayEvent, []int32]("intArray")
	longArray := Field[localGroupArrayEvent, []int64]("longArray")
	doubleArray := Field[localGroupArrayEvent, []float64]("doubleArray")
	sum := func(keys ...Expr) AggregateExpression[int32] {
		return LocalGroupBy[int32](Sum[int32](value), keys...)
	}
	plan, err := env.Build(From[localGroupArrayEvent](env, "LocalGroupArrayEvent").Aggregate(
		Alias("c0", sum(intArray)),
		Alias("c1", sum(longArray)),
		Alias("c2", sum(doubleArray)),
		Alias("c3", sum(intArray, longArray, doubleArray)),
		Alias("c4", Sum[int32](value)),
	).Query(StatementName("local-group-array-keys")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var latest Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			latest, _ = batch.New[len(batch.New)-1].Row()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// E4 carries fresh []int32{1} and []int64{20} instances whose CONTENT
	// matches earlier events, so c0 joins E1/E5 and c1 joins E2/E7; c2 keeps a
	// separate double[] level and c3 is the three-key tuple.
	events := []localGroupArrayEvent{
		{ID: "E1", Value: 10, IntArray: []int32{1}, LongArray: []int64{10}, DoubleArray: []float64{100}},
		{ID: "E2", Value: 11, IntArray: []int32{2}, LongArray: []int64{20}, DoubleArray: []float64{200}},
		{ID: "E3", Value: 12, IntArray: []int32{3}, LongArray: []int64{10}, DoubleArray: []float64{300}},
		{ID: "E4", Value: 13, IntArray: []int32{1}, LongArray: []int64{20}, DoubleArray: []float64{200}},
		{ID: "E5", Value: 14, IntArray: []int32{1}, LongArray: []int64{10}, DoubleArray: []float64{100}},
		{ID: "E6", Value: 15, IntArray: []int32{3}, LongArray: []int64{20}, DoubleArray: []float64{300}},
		{ID: "E7", Value: 16, IntArray: []int32{2}, LongArray: []int64{20}, DoubleArray: []float64{200}},
	}
	expected := []struct{ c0, c1, c2, c3, c4 int32 }{
		{10, 10, 10, 10, 10},
		{11, 11, 11, 11, 21},
		{12, 22, 12, 12, 33},
		{23, 24, 24, 13, 46},
		{37, 36, 24, 24, 60},
		{27, 39, 27, 15, 75},
		{27, 55, 40, 27, 91},
	}
	for index, event := range events {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		got := latest
		if got.Get("c0").Any() != expected[index].c0 || got.Get("c1").Any() != expected[index].c1 ||
			got.Get("c2").Any() != expected[index].c2 || got.Get("c3").Any() != expected[index].c3 ||
			got.Get("c4").Any() != expected[index].c4 {
			t.Fatalf("array-key local group row %d = %#v, want %#v", index, got.AsMap(), expected[index])
		}
	}
}

// TestEnumMethodsOverLocalGroupAggregate pins the readback of a local group
// through the window(*)/window(value) accessor methods as windowed by the
// statement `#keepall` window: firstOf() over an empty key list reads the
// statement-wide level while a keyed list reads the current event's group, and
// first(*) projected to a property resolves the earliest event of that level.
func TestEnumMethodsOverLocalGroupAggregate(t *testing.T) {
	env, engine := newLocalGroupTest(t)
	group := Field[localGroupEvent, string]("group")
	level := Field[localGroupEvent, int]("level")
	plan, err := env.Build(From[localGroupEvent](env, "LocalGroupEvent").Window(KeepAll()).GroupBy(group, level).Select(
		Alias("group", group),
		Alias("level", level),
		Alias("firstEventGlobal", EnumFirstOf[Event](LocalGroupBy[[]Event](WindowEvents()))),
		Alias("firstEventKeyed", EnumFirstOf[Event](LocalGroupBy[[]Event](WindowEvents(), group))),
		Alias("firstValueGlobal", EnumFirstOf[int](LocalGroupBy[[]int](WindowValues[int](level)))),
		Alias("firstValueKeyed", EnumFirstOf[int](LocalGroupBy[[]int](WindowValues[int](level), group))),
		Alias("firstLevelGlobal", NestedField[int](LocalGroupBy[Event](FirstEventValue()), "level")),
		Alias("firstLevelKeyed", NestedField[int](LocalGroupBy[Event](FirstEventValue(), group), "level")),
	).Query(StatementName("local-group-enum-methods")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var latest Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			latest, _ = batch.New[len(batch.New)-1].Row()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// The third event carries a DIFFERENT level so every column discriminates:
	// the statement-wide window keeps 10 (the first of [10,20,30]) while the
	// keyed local group for E2 keeps 30, and the same holds for first(*).
	events := []localGroupEvent{
		{ID: "A", Group: "E1", Level: 10, Value: 100},
		{ID: "B", Group: "E1", Level: 20, Value: 200},
		{ID: "C", Group: "E2", Level: 30, Value: 300},
	}
	expected := []struct {
		globalID, keyedID string
		globalValue       int
		keyedValue        int
		globalLevel       int
		keyedLevel        int
	}{
		{globalID: "A", keyedID: "A", globalValue: 10, keyedValue: 10, globalLevel: 10, keyedLevel: 10},
		{globalID: "A", keyedID: "A", globalValue: 10, keyedValue: 10, globalLevel: 10, keyedLevel: 10},
		{globalID: "A", keyedID: "C", globalValue: 10, keyedValue: 30, globalLevel: 10, keyedLevel: 30},
	}
	for index, event := range events {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		want := expected[index]
		globalEvent, ok := latest.Get("firstEventGlobal").Any().(Event)
		if !ok {
			t.Fatalf("enum-method row %d firstEventGlobal = %#v", index, latest.Get("firstEventGlobal").Any())
		}
		keyedEvent, ok := latest.Get("firstEventKeyed").Any().(Event)
		if !ok {
			t.Fatalf("enum-method row %d firstEventKeyed = %#v", index, latest.Get("firstEventKeyed").Any())
		}
		globalID, _ := globalEvent.Get("id").Any().(string)
		keyedID, _ := keyedEvent.Get("id").Any().(string)
		if globalID != want.globalID || keyedID != want.keyedID ||
			latest.Get("firstValueGlobal").Any() != want.globalValue || latest.Get("firstValueKeyed").Any() != want.keyedValue ||
			latest.Get("firstLevelGlobal").Any() != want.globalLevel || latest.Get("firstLevelKeyed").Any() != want.keyedLevel {
			t.Fatalf("enum-method row %d = %#v, want %#v", index, latest.AsMap(), want)
		}
	}
}

type localGroupFloatKeyEvent struct {
	ID         string    `esper:"id"`
	Key        float64   `esper:"key"`
	Array      []float64 `esper:"array"`
	FloatKey   float32   `esper:"floatKey"`
	FloatArray []float32 `esper:"floatArray"`
	ObjectKey  []any     `esper:"objectKey"`
	Value      int32     `esper:"value"`
}

// TestLocalGroupKeyFloatBitSemantics pins Esper's floating-point local group
// key semantics: Java compares a boxed Double/Float key with Double.equals/
// Float.equals and a double[]/float[]/Object[] key component with
// Arrays.equals, i.e. by doubleToLongBits/floatToIntBits and
// doubleToLongBits on the elements. Those canonicalize EVERY NaN payload to a
// single bit pattern, so two NaNs with different payloads are the same key,
// while -0.0 and 0.0 are different keys. Go's == and reflect.DeepEqual get both
// of those backwards.
func TestLocalGroupKeyFloatBitSemantics(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[localGroupFloatKeyEvent](env, "LocalGroupFloatKeyEvent"); err != nil {
		t.Fatal(err)
	}
	key := Field[localGroupFloatKeyEvent, float64]("key")
	array := Field[localGroupFloatKeyEvent, []float64]("array")
	floatKey := Field[localGroupFloatKeyEvent, float32]("floatKey")
	floatArray := Field[localGroupFloatKeyEvent, []float32]("floatArray")
	objectKey := Field[localGroupFloatKeyEvent, []any]("objectKey")
	value := Field[localGroupFloatKeyEvent, int32]("value")
	plan, err := env.Build(From[localGroupFloatKeyEvent](env, "LocalGroupFloatKeyEvent").Aggregate(
		Alias("scalarSum", LocalGroupBy[int32](Sum[int32](value), key)),
		Alias("arraySum", LocalGroupBy[int32](Sum[int32](value), array)),
		Alias("floatScalarSum", LocalGroupBy[int32](Sum[int32](value), floatKey)),
		Alias("floatArraySum", LocalGroupBy[int32](Sum[int32](value), floatArray)),
		Alias("objectSum", LocalGroupBy[int32](Sum[int32](value), objectKey)),
	).Query(StatementName("local-group-float-bits")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var latest Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			latest, _ = batch.New[len(batch.New)-1].Row()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Two distinct NaN payloads per width: Java's key comparison canonicalizes
	// both to the same NaN key, so rows 3 and 4 must land in ONE group.
	nanDouble := math.NaN()
	nanDoubleOther := math.Float64frombits(0xfff8000000000000)
	nanFloat := math.Float32frombits(0x7fc00000)
	nanFloatOther := math.Float32frombits(0x7fc00001)
	negativeZero := math.Copysign(0, -1)
	events := []localGroupFloatKeyEvent{
		{ID: "A", Key: 0, Array: []float64{0}, FloatKey: 0, FloatArray: []float32{0}, ObjectKey: []any{float64(0)}, Value: 1},
		{ID: "B", Key: negativeZero, Array: []float64{negativeZero}, FloatKey: float32(negativeZero), FloatArray: []float32{float32(negativeZero)}, ObjectKey: []any{negativeZero}, Value: 2},
		{ID: "C", Key: nanDouble, Array: []float64{nanDouble}, FloatKey: nanFloat, FloatArray: []float32{nanFloat}, ObjectKey: []any{nanDouble}, Value: 4},
		{ID: "D", Key: nanDoubleOther, Array: []float64{nanDoubleOther}, FloatKey: nanFloatOther, FloatArray: []float32{nanFloatOther}, ObjectKey: []any{nanDoubleOther}, Value: 8},
	}
	expected := []struct{ scalar, array, floatScalar, floatArray, object int32 }{
		{1, 1, 1, 1, 1},      // 0.0 opens a group in every key path
		{2, 2, 2, 2, 2},      // -0.0 is a distinct key in every key path
		{4, 4, 4, 4, 4},      // NaN opens a group in every key path
		{12, 12, 12, 12, 12}, // a NaN with another payload is the SAME key
	}
	for index, event := range events {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		columns := map[string]int32{
			"scalarSum":      expected[index].scalar,
			"arraySum":       expected[index].array,
			"floatScalarSum": expected[index].floatScalar,
			"floatArraySum":  expected[index].floatArray,
			"objectSum":      expected[index].object,
		}
		for name, want := range columns {
			if got := latest.Get(name).Any(); got != want {
				t.Fatalf("float key row %d %s = %#v, want %d", index, name, got, want)
			}
		}
	}
}
