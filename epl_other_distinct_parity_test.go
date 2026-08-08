package esper

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// distinctSupportBean mirrors SupportBean (theString, intPrimitive).
type distinctSupportBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

// distinctSupportBeanA mirrors SupportBean_A (id string).
type distinctSupportBeanA struct {
	ID string `esper:"id"`
}

// distinctSupportBeanN mirrors SupportBean_N (intPrimitive, intBoxed).
type distinctSupportBeanN struct {
	IntPrimitive int `esper:"intPrimitive"`
	IntBoxed     int `esper:"intBoxed"`
}

func newDistinctEnvironment(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[distinctSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[distinctSupportBeanA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[distinctSupportBeanN](env, "SupportBean_N"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[fcmEventWithManyArray](env, "SupportEventWithManyArray"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
}

func distinctSnapshotRows(t *testing.T, stmt *Statement) [][]string {
	t.Helper()
	snapshot, err := stmt.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	out := make([][]string, 0, len(snapshot.Results()))
	for _, result := range snapshot.Results() {
		out = append(out, distinctResultToStrings(result))
	}
	return out
}

// distinctResultToStrings reads the projected fields of a snapshot result,
// supporting both row-backed results (explicit projections) and event-backed
// results produced by wildcard "select *" queries.
func distinctResultToStrings(result Result) []string {
	var fields []FieldSpec
	if row, ok := result.Row(); ok {
		fields = row.Schema().Fields()
	} else if event, ok := result.Event(); ok {
		fields = event.Schema().Fields()
	} else {
		return nil
	}
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		v := result.Get(field.Name).Any()
		if v == nil {
			parts = append(parts, "<null>")
		} else {
			parts = append(parts, formatDistinctValue(v))
		}
	}
	return parts
}

func distinctRowsToStrings(rows []Row) [][]string {
	result := make([][]string, 0, len(rows))
	for _, row := range rows {
		fields := row.Schema().Fields()
		parts := make([]string, 0, len(fields))
		for _, field := range fields {
			v := row.Get(field.Name).Any()
			if v == nil {
				parts = append(parts, "<null>")
			} else {
				parts = append(parts, formatDistinctValue(v))
			}
		}
		result = append(result, parts)
	}
	return result
}

func formatDistinctValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return formatDistinctInt(val)
	case int64:
		return formatDistinctInt(int(val))
	case []int:
		parts := make([]string, len(val))
		for i, x := range val {
			parts[i] = formatDistinctInt(x)
		}
		return "[" + strings.Join(parts, ",") + "]"
	default:
		return formatDistinctAny(v)
	}
}

func formatDistinctInt(i int) string {
	if i == 0 {
		return "0"
	}
	negative := i < 0
	if negative {
		i = -i
	}
	var buf []byte
	for i > 0 {
		buf = append([]byte{byte('0' + i%10)}, buf...)
		i /= 10
	}
	if negative {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

func formatDistinctAny(v any) string {
	s := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(formatDistinctRepr(v), "\n", ""), "\t", " "))
	if s == "" {
		return "<null>"
	}
	return s
}

func formatDistinctRepr(v any) string {
	// Use fmt.Sprintf-compatible encoding via encodeKey for stable comparison
	return formatDistinctKey([]any{v})
}

func formatDistinctKey(values []any) string {
	parts := make([]string, 0, len(values))
	for _, v := range values {
		parts = append(parts, strings.TrimSpace(formatDistinctScalar(v)))
	}
	return strings.Join(parts, "|")
}

func formatDistinctScalar(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return formatDistinctInt(val)
	case int64:
		return formatDistinctInt(int(val))
	case []int:
		parts := make([]string, len(val))
		for i, x := range val {
			parts[i] = formatDistinctInt(x)
		}
		return "[" + strings.Join(parts, ",") + "]"
	case nil:
		return ""
	default:
		return formatDistinctGoString(v)
	}
}

func formatDistinctGoString(v any) string {
	// For struct types, encode by field via reflection
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Struct {
		parts := make([]string, 0, rv.NumField())
		for i := 0; i < rv.NumField(); i++ {
			parts = append(parts, formatDistinctScalar(rv.Field(i).Interface()))
		}
		return "{" + strings.Join(parts, ",") + "}"
	}
	return ""
}

// assertDistinctRowsAnyOrder compares rows ignoring order.
func assertDistinctRowsAnyOrder(t *testing.T, actual [][]string, expected [][]string, label string) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("%s: got %d rows, want %d: %v", label, len(actual), len(expected), actual)
	}
	actCopy := make([]string, len(actual))
	expCopy := make([]string, len(expected))
	for i, row := range actual {
		actCopy[i] = strings.Join(row, "|")
	}
	for i, row := range expected {
		expCopy[i] = strings.Join(row, "|")
	}
	sort.Strings(actCopy)
	sort.Strings(expCopy)
	for i := range actCopy {
		if actCopy[i] != expCopy[i] {
			t.Fatalf("%s: got %v, want %v", label, actCopy, expCopy)
		}
	}
}

type distinctListener struct {
	lastNew []Row
	invoked bool
}

func subscribeDistinct(t *testing.T, stmt *Statement) *distinctListener {
	t.Helper()
	l := &distinctListener{}
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				l.lastNew = append(l.lastNew, row)
			}
		}
		l.invoked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return l
}

func (l *distinctListener) reset() { l.lastNew = nil; l.invoked = false }

func deployDistinct(t *testing.T, env *Environment, query Query) (*Engine, *Statement, *distinctListener) {
	t.Helper()
	plan, err := env.Build(query)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close(context.Background()) })
	stmt := deployment.Statements()[0]
	return engine, stmt, subscribeDistinct(t, stmt)
}

// TestDistinctSimpleColumnParity mirrors EPLOtherOutputSimpleColumn: distinct
// on selected columns with keepall, both single-stream and join.
func TestDistinctSimpleColumnParity(t *testing.T) {
	t.Run("single-stream", func(t *testing.T) {
		env, _ := newDistinctEnvironment(t)
		query := Select(
			From[distinctSupportBean](env, "SupportBean").Window(KeepAll()),
			Alias("theString", Field[distinctSupportBean, string]("theString")),
			Alias("intPrimitive", Field[distinctSupportBean, int]("intPrimitive")),
		).Query(StatementName("s0"), WithDistinct())
		engine, stmt, listener := deployDistinct(t, env, query)

		sb := func(theString string, intPrimitive int) distinctSupportBean {
			return distinctSupportBean{TheString: theString, IntPrimitive: intPrimitive}
		}

		sendAndAssert := func(event distinctSupportBean, expectedNew, expectedIter [][]string) {
			t.Helper()
			listener.reset()
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
			if !listener.invoked {
				t.Fatalf("listener not invoked for %v", event)
			}
			assertDistinctRowsAnyOrder(t, distinctRowsToStrings(listener.lastNew), expectedNew, "lastNew")
			assertDistinctRowsAnyOrder(t, distinctSnapshotRows(t, stmt), expectedIter, "iterator")
		}

		sendAndAssert(sb("E1", 1), [][]string{{"E1", "1"}}, [][]string{{"E1", "1"}})
		sendAndAssert(sb("E1", 1), [][]string{{"E1", "1"}}, [][]string{{"E1", "1"}})
		sendAndAssert(sb("E2", 1), [][]string{{"E2", "1"}}, [][]string{{"E1", "1"}, {"E2", "1"}})
		sendAndAssert(sb("E1", 2), [][]string{{"E1", "2"}}, [][]string{{"E1", "1"}, {"E2", "1"}, {"E1", "2"}})
		sendAndAssert(sb("E2", 2), [][]string{{"E2", "2"}}, [][]string{{"E1", "1"}, {"E2", "1"}, {"E1", "2"}, {"E2", "2"}})
		sendAndAssert(sb("E2", 2), [][]string{{"E2", "2"}}, [][]string{{"E1", "1"}, {"E2", "1"}, {"E1", "2"}, {"E2", "2"}})
		sendAndAssert(sb("E1", 1), [][]string{{"E1", "1"}}, [][]string{{"E1", "1"}, {"E2", "1"}, {"E1", "2"}, {"E2", "2"}})
	})

	t.Run("join", func(t *testing.T) {
		env, _ := newDistinctEnvironment(t)
		stream := From[distinctSupportBean](env, "SupportBean").Window(KeepAll())
		beanA := From[distinctSupportBeanA](env, "SupportBean_A").Window(KeepAll())
		query := Join(stream, beanA, OnEqual(
			Field[distinctSupportBean, string]("theString"),
			Field[distinctSupportBeanA, string]("id"),
		)).Select(
			SelectLeft("theString", Field[distinctSupportBean, string]("theString")),
			SelectLeft("intPrimitive", Field[distinctSupportBean, int]("intPrimitive")),
		).Query(StatementName("s0"), WithDistinct())
		engine, stmt, listener := deployDistinct(t, env, query)

		if err := engine.SendEvent(context.Background(), distinctSupportBeanA{ID: "E1"}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), distinctSupportBeanA{ID: "E2"}); err != nil {
			t.Fatal(err)
		}

		sb := func(theString string, intPrimitive int) distinctSupportBean {
			return distinctSupportBean{TheString: theString, IntPrimitive: intPrimitive}
		}

		sendAndAssert := func(event distinctSupportBean, expectedNew, expectedIter [][]string) {
			t.Helper()
			listener.reset()
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
			if !listener.invoked {
				t.Fatalf("listener not invoked for %v", event)
			}
			assertDistinctRowsAnyOrder(t, distinctRowsToStrings(listener.lastNew), expectedNew, "lastNew")
			assertDistinctRowsAnyOrder(t, distinctSnapshotRows(t, stmt), expectedIter, "iterator")
		}

		sendAndAssert(sb("E1", 1), [][]string{{"E1", "1"}}, [][]string{{"E1", "1"}})
		sendAndAssert(sb("E1", 1), [][]string{{"E1", "1"}}, [][]string{{"E1", "1"}})
		sendAndAssert(sb("E2", 1), [][]string{{"E2", "1"}}, [][]string{{"E1", "1"}, {"E2", "1"}})
		sendAndAssert(sb("E1", 2), [][]string{{"E1", "2"}}, [][]string{{"E1", "1"}, {"E2", "1"}, {"E1", "2"}})
		sendAndAssert(sb("E2", 2), [][]string{{"E2", "2"}}, [][]string{{"E1", "1"}, {"E2", "1"}, {"E1", "2"}, {"E2", "2"}})
		sendAndAssert(sb("E2", 2), [][]string{{"E2", "2"}}, [][]string{{"E1", "1"}, {"E2", "1"}, {"E1", "2"}, {"E2", "2"}})
		sendAndAssert(sb("E1", 1), [][]string{{"E1", "1"}}, [][]string{{"E1", "1"}, {"E2", "1"}, {"E1", "2"}, {"E2", "2"}})
	})
}

// TestDistinctBatchWindowParity mirrors EPLOtherBatchWindow: distinct with a
// length-batch window that flushes every 3 events.
func TestDistinctBatchWindowParity(t *testing.T) {
	env, _ := newDistinctEnvironment(t)
	query := Select(
		From[distinctSupportBean](env, "SupportBean").Window(LengthBatch(3)),
		Alias("theString", Field[distinctSupportBean, string]("theString")),
		Alias("intPrimitive", Field[distinctSupportBean, int]("intPrimitive")),
	).Query(StatementName("s0"), WithDistinct())
	engine, stmt, listener := deployDistinct(t, env, query)
	_ = stmt

	sb := func(theString string, intPrimitive int) distinctSupportBean {
		return distinctSupportBean{TheString: theString, IntPrimitive: intPrimitive}
	}

	// Batch 1: E1/1, E1/1 (duplicate), then E2/2 completes the batch
	listener.reset()
	_ = engine.SendEvent(context.Background(), sb("E1", 1))
	_ = engine.SendEvent(context.Background(), sb("E1", 1))
	if listener.invoked {
		t.Fatal("listener invoked before batch complete")
	}
	_ = engine.SendEvent(context.Background(), sb("E2", 2))
	assertDistinctRowsAnyOrder(t, distinctRowsToStrings(listener.lastNew), [][]string{{"E1", "1"}, {"E2", "2"}}, "batch1")

	// Batch 2: E2/2, E1/1, E2/2 (all duplicates of previous batch's distinct set)
	listener.reset()
	_ = engine.SendEvent(context.Background(), sb("E2", 2))
	_ = engine.SendEvent(context.Background(), sb("E1", 1))
	_ = engine.SendEvent(context.Background(), sb("E2", 2))
	assertDistinctRowsAnyOrder(t, distinctRowsToStrings(listener.lastNew), [][]string{{"E2", "2"}, {"E1", "1"}}, "batch2")

	// Batch 3: all E2/3
	listener.reset()
	_ = engine.SendEvent(context.Background(), sb("E2", 3))
	_ = engine.SendEvent(context.Background(), sb("E2", 3))
	_ = engine.SendEvent(context.Background(), sb("E2", 3))
	assertDistinctRowsAnyOrder(t, distinctRowsToStrings(listener.lastNew), [][]string{{"E2", "3"}}, "batch3")
}

// TestDistinctWildcardParity mirrors EPLOtherBeanEventWildcardThisProperty:
// distinct on wildcard (*) deduplicates identical events.
func TestDistinctWildcardParity(t *testing.T) {
	env, _ := newDistinctEnvironment(t)
	query := From[distinctSupportBeanA](env, "SupportBean_A").Window(KeepAll()).
		Query(StatementName("s0"), WithDistinct())
	engine, stmt, listener := deployDistinct(t, env, query)

	send := func(id string, expectedIter [][]string, expectInvoked bool) {
		t.Helper()
		listener.reset()
		_ = engine.SendEvent(context.Background(), distinctSupportBeanA{ID: id})
		assertDistinctRowsAnyOrder(t, distinctSnapshotRows(t, stmt), expectedIter, "iterator "+id)
		if expectInvoked && !listener.invoked {
			t.Fatalf("listener not invoked for %s", id)
		}
		if !expectInvoked && listener.invoked {
			t.Fatalf("listener invoked unexpectedly for %s", id)
		}
	}

	send("E1", [][]string{{"E1"}}, true)
	send("E2", [][]string{{"E1"}, {"E2"}}, true)
	send("E1", [][]string{{"E1"}, {"E2"}}, true) // duplicate, still fires in Go distinct model
}

// TestDistinctWildcardPlusColsParity mirrors EPLOtherBeanEventWildcardPlusCols:
// distinct on wildcard plus computed columns.
func TestDistinctWildcardPlusColsParity(t *testing.T) {
	env, _ := newDistinctEnvironment(t)
	query := Select(
		From[distinctSupportBeanN](env, "SupportBean_N").Window(KeepAll()),
		Alias("intPrimitive", Field[distinctSupportBeanN, int]("intPrimitive")),
		Alias("val1", Modulo[int](Field[distinctSupportBeanN, int]("intBoxed"), Literal(5))),
		Alias("val2", Field[distinctSupportBeanN, int]("intBoxed")),
	).Query(StatementName("s0"), WithDistinct())
	engine, stmt, _ := deployDistinct(t, env, query)

	send := func(ip, ib int, expectedIter [][]string) {
		t.Helper()
		_ = engine.SendEvent(context.Background(), distinctSupportBeanN{IntPrimitive: ip, IntBoxed: ib})
		assertDistinctRowsAnyOrder(t, distinctSnapshotRows(t, stmt), expectedIter, "iterator")
	}

	send(1, 8, [][]string{{"1", "3", "8"}})
	send(1, 3, [][]string{{"1", "3", "8"}, {"1", "3", "3"}})
	send(1, 8, [][]string{{"1", "3", "8"}, {"1", "3", "3"}}) // duplicate of first
}

// TestDistinctMapEventWildcardParity mirrors EPLOtherMapEventWildcard: distinct
// on a map event type with k1/v1 fields.
func TestDistinctMapEventWildcardParity(t *testing.T) {
	env := NewEnvironment()
	schema, err := NewMapSchema("MyMapTypeKVDistinct", []FieldSpec{
		FieldDef("k1", reflect.TypeOf("")),
		FieldDef("v1", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(schema); err != nil {
		t.Fatal(err)
	}
	query := FromAny(env, "MyMapTypeKVDistinct").Window(KeepAll()).Select(
		Alias("k1", Field[map[string]any, string]("k1")),
		Alias("v1", Field[map[string]any, int]("v1")),
	).Query(StatementName("s0"), WithDistinct())
	engine := NewEngine(env)
	t.Cleanup(func() { _ = engine.Close(context.Background()) })
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	stmt := deployment.Statements()[0]

	send := func(k1 string, v1 int, expectedIter [][]string) {
		t.Helper()
		if err := engine.SendRecord(context.Background(), "MyMapTypeKVDistinct", map[string]any{"k1": k1, "v1": v1}); err != nil {
			t.Fatal(err)
		}
		assertDistinctRowsAnyOrder(t, distinctSnapshotRows(t, stmt), expectedIter, "iterator")
	}

	send("E1", 1, [][]string{{"E1", "1"}})
	send("E2", 2, [][]string{{"E1", "1"}, {"E2", "2"}})
	send("E1", 1, [][]string{{"E1", "1"}, {"E2", "2"}}) // duplicate
}

// TestDistinctOutputLimitEveryParity mirrors EPLOtherOutputLimitEveryColumn:
// distinct with output every 3 events.
func TestDistinctOutputLimitEveryParity(t *testing.T) {
	env, _ := newDistinctEnvironment(t)
	query := Select(
		From[distinctSupportBean](env, "SupportBean").Window(KeepAll()),
		Alias("theString", Field[distinctSupportBean, string]("theString")),
		Alias("intPrimitive", Field[distinctSupportBean, int]("intPrimitive")),
	).Query(StatementName("s0"), WithDistinct(), WithOutput(OutputEvery(3)))
	engine, _, listener := deployDistinct(t, env, query)

	sb := func(theString string, intPrimitive int) distinctSupportBean {
		return distinctSupportBean{TheString: theString, IntPrimitive: intPrimitive}
	}

	// Two events, no output yet
	listener.reset()
	_ = engine.SendEvent(context.Background(), sb("E1", 1))
	_ = engine.SendEvent(context.Background(), sb("E1", 1))
	if listener.invoked {
		t.Fatal("listener invoked before 3 events")
	}

	// Third event triggers output: {E1,1}, {E2,2}
	_ = engine.SendEvent(context.Background(), sb("E2", 2))
	assertDistinctRowsAnyOrder(t, distinctRowsToStrings(listener.lastNew), [][]string{{"E1", "1"}, {"E2", "2"}}, "batch1")

	// Next batch: E2/2, E1/1, E2/2 -> {E2,2}, {E1,1}
	listener.reset()
	_ = engine.SendEvent(context.Background(), sb("E2", 2))
	_ = engine.SendEvent(context.Background(), sb("E1", 1))
	_ = engine.SendEvent(context.Background(), sb("E2", 2))
	assertDistinctRowsAnyOrder(t, distinctRowsToStrings(listener.lastNew), [][]string{{"E2", "2"}, {"E1", "1"}}, "batch2")

	// Next batch: all E2/3
	listener.reset()
	_ = engine.SendEvent(context.Background(), sb("E2", 3))
	_ = engine.SendEvent(context.Background(), sb("E2", 3))
	_ = engine.SendEvent(context.Background(), sb("E2", 3))
	assertDistinctRowsAnyOrder(t, distinctRowsToStrings(listener.lastNew), [][]string{{"E2", "3"}}, "batch3")
}

// TestDistinctOutputSnapshotParity mirrors EPLOtherOutputRateSnapshotColumn:
// distinct with output snapshot every 3 events.
func TestDistinctOutputSnapshotParity(t *testing.T) {
	env, _ := newDistinctEnvironment(t)
	query := Select(
		From[distinctSupportBean](env, "SupportBean").Window(KeepAll()),
		Alias("theString", Field[distinctSupportBean, string]("theString")),
		Alias("intPrimitive", Field[distinctSupportBean, int]("intPrimitive")),
	).Query(StatementName("s0"), WithDistinct(), WithOutput(OutputSnapshotEveryEvents(3)),
		OrderBy(Ascending(Field[distinctSupportBean, string]("theString"))))
	engine, _, listener := deployDistinct(t, env, query)

	sb := func(theString string, intPrimitive int) distinctSupportBean {
		return distinctSupportBean{TheString: theString, IntPrimitive: intPrimitive}
	}

	// Two events, no output yet
	listener.reset()
	_ = engine.SendEvent(context.Background(), sb("E1", 1))
	_ = engine.SendEvent(context.Background(), sb("E1", 1))
	if listener.invoked {
		t.Fatal("listener invoked before 3 events")
	}

	// Third event: snapshot = {E1,1}, {E2,2}
	_ = engine.SendEvent(context.Background(), sb("E2", 2))
	if !listener.invoked {
		t.Fatal("listener not invoked at 3 events")
	}
	assertRowsOrdered(t, distinctRowsToStrings(listener.lastNew), [][]string{{"E1", "1"}, {"E2", "2"}}, "snapshot1")

	// Events 4-6: E2/2, E1/1, E2/2 -> snapshot still {E1,1}, {E2,2}
	listener.reset()
	_ = engine.SendEvent(context.Background(), sb("E2", 2))
	_ = engine.SendEvent(context.Background(), sb("E1", 1))
	_ = engine.SendEvent(context.Background(), sb("E2", 2))
	assertRowsOrdered(t, distinctRowsToStrings(listener.lastNew), [][]string{{"E1", "1"}, {"E2", "2"}}, "snapshot2")

	// Events 7-9: E3/3, E1/1, E2/2 -> snapshot {E1,1}, {E2,2}, {E3,3}
	listener.reset()
	_ = engine.SendEvent(context.Background(), sb("E3", 3))
	_ = engine.SendEvent(context.Background(), sb("E1", 1))
	_ = engine.SendEvent(context.Background(), sb("E2", 2))
	assertRowsOrdered(t, distinctRowsToStrings(listener.lastNew), [][]string{{"E1", "1"}, {"E2", "2"}, {"E3", "3"}}, "snapshot3")
}

func assertRowsOrdered(t *testing.T, actual, expected [][]string, label string) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("%s: got %d rows, want %d: %v", label, len(actual), len(expected), actual)
	}
	for i := range actual {
		if strings.Join(actual[i], "|") != strings.Join(expected[i], "|") {
			t.Fatalf("%s: row %d got %v want %v", label, i, actual[i], expected[i])
		}
	}
}

// TestDistinctIterateMultikeyWArrayParity mirrors
// EPLOtherDistinctIterateMultikeyWArray: distinct on array fields via the
// iterator snapshot.
func TestDistinctIterateMultikeyWArrayParity(t *testing.T) {
	env, _ := newDistinctEnvironment(t)
	query := Select(
		From[fcmEventWithManyArray](env, "SupportEventWithManyArray").Window(KeepAll()),
		Alias("intOne", Field[fcmEventWithManyArray, []int]("intOne")),
	).Query(StatementName("s0"), WithDistinct())
	engine, stmt, _ := deployDistinct(t, env, query)

	query2 := Select(
		From[fcmEventWithManyArray](env, "SupportEventWithManyArray").Window(KeepAll()),
		Alias("intOne", Field[fcmEventWithManyArray, []int]("intOne")),
		Alias("intTwo", Field[fcmEventWithManyArray, []int]("intTwo")),
	).Query(StatementName("s1"), WithDistinct())
	plan2, err := env.Build(query2)
	if err != nil {
		t.Fatal(err)
	}
	dep2, err := engine.Deploy(context.Background(), plan2)
	if err != nil {
		t.Fatal(err)
	}
	stmt2 := dep2.Statements()[0]

	data := []fcmEventWithManyArray{
		{ID: "id", IntOne: []int{1, 2}, IntTwo: []int{3, 4}},
		{ID: "id", IntOne: []int{3, 4}, IntTwo: []int{1, 2}},
		{ID: "id", IntOne: []int{1, 2}, IntTwo: []int{3, 5}},
		{ID: "id", IntOne: []int{3, 4}, IntTwo: []int{1, 2}}, // dup of #2
		{ID: "id", IntOne: []int{1, 2}, IntTwo: []int{3, 4}}, // dup of #1
	}
	for _, e := range data {
		_ = engine.SendEvent(context.Background(), e)
	}

	assertDistinctRowsAnyOrder(t, distinctSnapshotRows(t, stmt),
		[][]string{{"[1,2]"}, {"[3,4]"}}, "s0 distinct intOne")

	assertDistinctRowsAnyOrder(t, distinctSnapshotRows(t, stmt2),
		[][]string{{"[1,2]", "[3,4]"}, {"[3,4]", "[1,2]"}, {"[1,2]", "[3,5]"}},
		"s1 distinct intOne,intTwo")
}

// TestDistinctOutputLimitMultikeyWArrayParity mirrors the two output-limit
// multikey-with-array variants using virtual clock.
func TestDistinctOutputLimitMultikeyWArrayParity(t *testing.T) {
	t.Run("single-array", func(t *testing.T) {
		env, _ := newDistinctEnvironment(t)
		query := Select(
			From[fcmEventWithManyArray](env, "SupportEventWithManyArray"),
			Alias("intOne", Field[fcmEventWithManyArray, []int]("intOne")),
		).Query(StatementName("s0"), WithDistinct(), WithOutput(OutputEveryTime(time.Second)))
		engine, _, listener := deployDistinct(t, env, query)

		data := []fcmEventWithManyArray{
			{ID: "id", IntOne: []int{1, 2}},
			{ID: "id", IntOne: []int{2, 1}},
			{ID: "id", IntOne: []int{2, 3}},
			{ID: "id", IntOne: []int{1, 2}}, // dup
			{ID: "id", IntOne: []int{1, 2}}, // dup
		}
		for _, e := range data {
			_ = engine.SendEvent(context.Background(), e)
		}
		_ = engine.AdvanceTime(context.Background(), time.Unix(0, 0).UTC().Add(time.Second))
		assertDistinctRowsAnyOrder(t, distinctRowsToStrings(listener.lastNew),
			[][]string{{"[1,2]"}, {"[2,1]"}, {"[2,3]"}}, "single-array output")
	})

	t.Run("two-array", func(t *testing.T) {
		env, _ := newDistinctEnvironment(t)
		query := Select(
			From[fcmEventWithManyArray](env, "SupportEventWithManyArray"),
			Alias("intOne", Field[fcmEventWithManyArray, []int]("intOne")),
			Alias("intTwo", Field[fcmEventWithManyArray, []int]("intTwo")),
		).Query(StatementName("s0"), WithDistinct(), WithOutput(OutputEveryTime(time.Second)))
		engine, _, listener := deployDistinct(t, env, query)

		data := []fcmEventWithManyArray{
			{ID: "id", IntOne: []int{1, 2}, IntTwo: []int{3, 4}},
			{ID: "id", IntOne: []int{3, 4}, IntTwo: []int{1, 2}},
			{ID: "id", IntOne: []int{1, 2}, IntTwo: []int{3, 5}},
			{ID: "id", IntOne: []int{3, 4}, IntTwo: []int{1, 2}}, // dup
			{ID: "id", IntOne: []int{1, 2}, IntTwo: []int{3, 4}}, // dup
		}
		for _, e := range data {
			_ = engine.SendEvent(context.Background(), e)
		}
		_ = engine.AdvanceTime(context.Background(), time.Unix(0, 0).UTC().Add(time.Second))
		assertDistinctRowsAnyOrder(t, distinctRowsToStrings(listener.lastNew),
			[][]string{{"[1,2]", "[3,4]"}, {"[3,4]", "[1,2]"}, {"[1,2]", "[3,5]"}},
			"two-array output")
	})
}

// TestDistinctBatchWindowInsertIntoParity mirrors EPLOtherBatchWindowInsertInto:
// distinct in an insert-into stream fed from a length-batch window.
func TestDistinctBatchWindowInsertIntoParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[distinctSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	t.Cleanup(func() { _ = engine.Close(context.Background()) })

	streamSchema, err := NewMapSchema("MyStream", []FieldSpec{
		FieldDef("theString", reflect.TypeOf("")),
		FieldDef("intPrimitive", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(streamSchema); err != nil {
		t.Fatal(err)
	}

	// Insert into MyStream with distinct from length_batch(3)
	insertQuery := Select(
		From[distinctSupportBean](env, "SupportBean").Window(LengthBatch(3)),
		Alias("theString", Field[distinctSupportBean, string]("theString")),
		Alias("intPrimitive", Field[distinctSupportBean, int]("intPrimitive")),
	).InsertInto("MyStream", StatementName("insert"), WithDistinct())
	if _, err := engine.Deploy(context.Background(), mustBuild(t, env, insertQuery)); err != nil {
		t.Fatal(err)
	}

	// Consume MyStream
	consumeQuery := FromAny(env, "MyStream").Window(KeepAll()).Select(
		Alias("theString", Field[map[string]any, string]("theString")),
		Alias("intPrimitive", Field[map[string]any, int]("intPrimitive")),
	).Query(StatementName("s0"))
	dep, err := engine.Deploy(context.Background(), mustBuild(t, env, consumeQuery))
	if err != nil {
		t.Fatal(err)
	}
	stmt := dep.Statements()[0]
	listener := subscribeDistinct(t, stmt)

	sb := func(theString string, intPrimitive int) distinctSupportBean {
		return distinctSupportBean{TheString: theString, IntPrimitive: intPrimitive}
	}

	// Batch 1: E1/1, E1/1, E1/1 -> distinct = {E1,1}
	listener.reset()
	_ = engine.SendEvent(context.Background(), sb("E1", 1))
	_ = engine.SendEvent(context.Background(), sb("E1", 1))
	_ = engine.SendEvent(context.Background(), sb("E1", 1))
	if !listener.invoked {
		t.Fatal("batch1: listener not invoked")
	}
	assertDistinctRowsAnyOrder(t, distinctRowsToStrings(listener.lastNew), [][]string{{"E1", "1"}}, "batch1")

	// Batch 2: E2/2, E3/3, E2/2 -> distinct = {E2,2}, {E3,3}
	listener.reset()
	_ = engine.SendEvent(context.Background(), sb("E2", 2))
	_ = engine.SendEvent(context.Background(), sb("E3", 3))
	_ = engine.SendEvent(context.Background(), sb("E2", 2))
	if !listener.invoked {
		t.Fatal("batch2: listener not invoked")
	}
	assertDistinctRowsAnyOrder(t, distinctRowsToStrings(listener.lastNew), [][]string{{"E2", "2"}, {"E3", "3"}}, "batch2")
}

func mustBuild(t *testing.T, env *Environment, q Query) Plan {
	t.Helper()
	plan, err := env.Build(q)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	return plan
}

// fafResultRows converts fire-and-forget Result values into Rows so the shared
// row-to-string helpers can compare them.
func fafResultRows(results []Result) []Row {
	rows := make([]Row, 0, len(results))
	for _, result := range results {
		if row, ok := result.Row(); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

// TestDistinctFAFMultikeyWArrayParity mirrors EPLOtherDistinctFireAndForgetMultikeyWArray:
// fire-and-forget distinct over a named window keyed by int arrays.
func TestDistinctFAFMultikeyWArrayParity(t *testing.T) {
	env, engine := newDistinctEnvironment(t)
	schema, ok := env.Schema("SupportEventWithManyArray")
	if !ok {
		t.Fatal("SupportEventWithManyArray schema missing")
	}
	if _, err := CreateNamedWindow(env, "MyWindow", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	source := FromNamedWindow(env, "MyWindow")
	ctx := context.Background()

	send := func(intOne, intTwo []int) {
		if err := engine.InsertNamedWindow(ctx, "MyWindow", fcmEventWithManyArray{IntOne: intOne, IntTwo: intTwo}); err != nil {
			t.Fatal(err)
		}
	}
	send([]int{1, 2}, []int{3, 4})
	send([]int{3, 4}, []int{1, 2})
	send([]int{1, 2}, []int{3, 5})
	send([]int{3, 4}, []int{1, 2})
	send([]int{1, 2}, []int{3, 4})

	planOne, err := env.Build(source.Select(
		Alias("intOne", Field[any, []int]("intOne")),
	).Query(WithDistinct()))
	if err != nil {
		t.Fatal(err)
	}
	resultOne, err := engine.ExecuteFireAndForget(ctx, planOne)
	if err != nil {
		t.Fatal(err)
	}
	assertDistinctRowsAnyOrder(t, distinctRowsToStrings(fafResultRows(resultOne.Results())), [][]string{{"[1,2]"}, {"[3,4]"}}, "distinct intOne")

	planTwo, err := env.Build(source.Select(
		Alias("intOne", Field[any, []int]("intOne")),
		Alias("intTwo", Field[any, []int]("intTwo")),
	).Query(WithDistinct()))
	if err != nil {
		t.Fatal(err)
	}
	resultTwo, err := engine.ExecuteFireAndForget(ctx, planTwo)
	if err != nil {
		t.Fatal(err)
	}
	assertDistinctRowsAnyOrder(t, distinctRowsToStrings(fafResultRows(resultTwo.Results())), [][]string{
		{"[1,2]", "[3,4]"}, {"[3,4]", "[1,2]"}, {"[1,2]", "[3,5]"},
	}, "distinct intOne,intTwo")
}
