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

// distinctTriggerS0 mirrors SupportBean_S0 (id int).
type distinctTriggerS0 struct {
	ID int `esper:"id"`
}

// distinctTriggerS1 mirrors SupportBean_S1 (id int).
type distinctTriggerS1 struct {
	ID int `esper:"id"`
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
	if _, err := RegisterStruct[distinctTriggerS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[distinctTriggerS1](env, "SupportBean_S1"); err != nil {
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

// TestDistinctBatchWindowJoinParity mirrors EPLOtherBatchWindowJoin: select
// distinct theString, intPrimitive from SupportBean#length_batch(3) a,
// SupportBean_A#keepall b where a.theString = b.id. The keepall window
// preloads E1/E2; each completed three-event batch flushes the joined rows
// deduplicated by distinct in first-seen batch order.
func TestDistinctBatchWindowJoinParity(t *testing.T) {
	env, _ := newDistinctEnvironment(t)
	beans := From[distinctSupportBean](env, "SupportBean").Window(LengthBatch(3))
	beanA := From[distinctSupportBeanA](env, "SupportBean_A").Window(KeepAll())
	query := Join(beans, beanA, OnEqual(
		Field[distinctSupportBean, string]("theString"),
		Field[distinctSupportBeanA, string]("id"),
	)).Select(
		SelectLeft("theString", Field[distinctSupportBean, string]("theString")),
		SelectLeft("intPrimitive", Field[distinctSupportBean, int]("intPrimitive")),
	).Query(StatementName("s0"), WithDistinct())
	engine, _, listener := deployDistinct(t, env, query)

	sb := func(theString string, intPrimitive int) distinctSupportBean {
		return distinctSupportBean{TheString: theString, IntPrimitive: intPrimitive}
	}
	assertOrdered := func(got [][]string, want [][]string, label string) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: got %v, want ordered %v", label, got, want)
		}
	}

	if err := engine.SendEvent(context.Background(), distinctSupportBeanA{ID: "E1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), distinctSupportBeanA{ID: "E2"}); err != nil {
		t.Fatal(err)
	}

	// Batch 1: E1/1, E1/1 (silent, batch not full), E2/2 completes it; the
	// joined duplicates collapse to {E1,1}, {E2,2} in batch order.
	listener.reset()
	_ = engine.SendEvent(context.Background(), sb("E1", 1))
	_ = engine.SendEvent(context.Background(), sb("E1", 1))
	if listener.invoked {
		t.Fatal("listener invoked before batch complete")
	}
	_ = engine.SendEvent(context.Background(), sb("E2", 2))
	assertOrdered(distinctRowsToStrings(listener.lastNew), [][]string{{"E1", "1"}, {"E2", "2"}}, "batch1")

	// Batch 2: E2/2, E1/1, E2/2 — first-seen batch order is E2 then E1.
	listener.reset()
	_ = engine.SendEvent(context.Background(), sb("E2", 2))
	_ = engine.SendEvent(context.Background(), sb("E1", 1))
	_ = engine.SendEvent(context.Background(), sb("E2", 2))
	assertOrdered(distinctRowsToStrings(listener.lastNew), [][]string{{"E2", "2"}, {"E1", "1"}}, "batch2")

	// Batch 3: all E2/3.
	listener.reset()
	_ = engine.SendEvent(context.Background(), sb("E2", 3))
	_ = engine.SendEvent(context.Background(), sb("E2", 3))
	_ = engine.SendEvent(context.Background(), sb("E2", 3))
	assertOrdered(distinctRowsToStrings(listener.lastNew), [][]string{{"E2", "3"}}, "batch3")
}

// distinctJoinPatternWildcardQuery builds the shared shape of
// EPLOtherDistinctWildcardJoinPatternOne/Two: a unidirectional SupportBean
// (intPrimitive=0) driver inner-joined to a retained every-distinct pattern
// pair (fooA intPrimitive=1 -> wooA intPrimitive=2 within 1 hour, kept in a
// 1-hour pattern window) on fooB.longPrimitive = fooA.longPrimitive, with a
// distinct wildcard projection of both source events.
func distinctJoinPatternWildcardQuery(env *Environment, orderByWooA bool) Query {
	base := From[distinctSupportBean](env, "SupportBean")
	intIs := func(want int) Expression[bool] {
		return Equal[int](Field[distinctSupportBean, int]("intPrimitive"), Literal(want))
	}
	key := Field[distinctSupportBean, string]("theString")
	pattern := PatternFrom(base, "fooA", intIs(1)).EveryDistinct(key).
		Then(PatternFrom(base, "wooA", intIs(2)).EveryDistinct(key)).
		Within(time.Hour)
	options := []QueryOption{StatementName("s0"), WithDistinct()}
	if orderByWooA {
		options = append(options, OrderBy(Ascending(JoinPatternField[string](1, "wooA", "theString"))))
	}
	return JoinMany(
		JoinSource(base.Filter(intIs(0))).Unidirectional(),
		JoinPatternSource(pattern).Window(TimeWindow(time.Hour)),
	).On(OnSourcesEqual(
		0, Field[distinctSupportBean, int64]("longPrimitive"),
		1, JoinPatternField[int64](1, "fooA", "longPrimitive"),
	)).Select(
		SelectSourceEvent(0, "fooB"),
		SelectSourceEvent(1, "fooWooPair"),
	).Query(options...)
}

// distinctJoinPatternWildcardRow reads one wildcard join row as the Java
// subscriber does: fooB's theString plus the fooWooPair map's fooA/wooA
// fragment theString values.
func distinctJoinPatternWildcardRow(t *testing.T, row Row) (fooB, fooA, wooA string) {
	t.Helper()
	fooBEvent, ok := row.Get("fooB").Any().(Event)
	if !ok {
		t.Fatalf("fooB is not an event: %#v", row.Get("fooB").Any())
	}
	pairEvent, ok := row.Get("fooWooPair").Any().(Event)
	if !ok {
		t.Fatalf("fooWooPair is not an event: %#v", row.Get("fooWooPair").Any())
	}
	fooAEvent, ok := pairEvent.Get("fooA").Any().(Event)
	if !ok {
		t.Fatalf("fooWooPair.fooA is not an event: %#v", pairEvent.Get("fooA").Any())
	}
	wooAEvent, ok := pairEvent.Get("wooA").Any().(Event)
	if !ok {
		t.Fatalf("fooWooPair.wooA is not an event: %#v", pairEvent.Get("wooA").Any())
	}
	return fooBEvent.Get("theString").Any().(string),
		fooAEvent.Get("theString").Any().(string),
		wooAEvent.Get("theString").Any().(string)
}

// TestDistinctWildcardJoinPatternOneParity mirrors
// EPLOtherDistinctWildcardJoinPatternOne: the every-distinct pattern pair
// accumulates matches silently (unidirectional join), and the driver event
// fires the listener once with the joined distinct wildcard rows. Java
// asserts invocation only; the Go test additionally records the Esper
// every-distinct sequence semantics (each captured fooA pairs with every
// later distinct wooA) as the expected row multiset.
func TestDistinctWildcardJoinPatternOneParity(t *testing.T) {
	env, _ := newDistinctEnvironment(t)
	engine, _, listener := deployDistinct(t, env, distinctJoinPatternWildcardQuery(env, false))

	send := func(theString string, intPrimitive int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), distinctSupportBean{
			TheString: theString, IntPrimitive: intPrimitive, LongPrimitive: 10,
		}); err != nil {
			t.Fatal(err)
		}
	}
	send("E1", 1)
	send("E1", 2)
	send("E2", 1)
	send("E2", 2)
	send("E3", 1)
	send("E3", 2)
	if listener.invoked {
		t.Fatalf("unidirectional join fired without a driver event: %d rows", len(listener.lastNew))
	}

	listener.reset()
	send("Query", 0)
	if !listener.invoked {
		t.Fatal("listener not invoked by the driver event")
	}
	got := make([][]string, 0, len(listener.lastNew))
	for _, row := range listener.lastNew {
		fooB, fooA, wooA := distinctJoinPatternWildcardRow(t, row)
		got = append(got, []string{fooB, fooA, wooA})
	}
	want := [][]string{
		{"Query", "E1", "E1"},
		{"Query", "E1", "E2"},
		{"Query", "E2", "E2"},
		{"Query", "E1", "E3"},
		{"Query", "E2", "E3"},
		{"Query", "E3", "E3"},
	}
	sort.Slice(got, func(i, j int) bool { return strings.Join(got[i], "|") < strings.Join(got[j], "|") })
	sort.Slice(want, func(i, j int) bool { return strings.Join(want[i], "|") < strings.Join(want[j], "|") })
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("join pattern one rows: got %v, want %v", got, want)
	}
}

// TestDistinctWildcardJoinPatternTwoParity mirrors
// EPLOtherDistinctWildcardJoinPatternTwo: same statement plus
// order by fooWooPair.wooA.theString asc, delivered to a multi-row
// subscriber as one insert batch of two rows.
func TestDistinctWildcardJoinPatternTwoParity(t *testing.T) {
	env, _ := newDistinctEnvironment(t)
	engine, _, listener := deployDistinct(t, env, distinctJoinPatternWildcardQuery(env, true))

	send := func(theString string, intPrimitive int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), distinctSupportBean{
			TheString: theString, IntPrimitive: intPrimitive, LongPrimitive: 10,
		}); err != nil {
			t.Fatal(err)
		}
	}
	send("E1", 1)
	send("E2", 2)
	send("E3", 2)
	if listener.invoked {
		t.Fatalf("unidirectional join fired without a driver event: %d rows", len(listener.lastNew))
	}

	listener.reset()
	send("Query", 0)
	if !listener.invoked {
		t.Fatal("listener not invoked by the driver event")
	}
	if len(listener.lastNew) != 2 {
		t.Fatalf("driver batch row count = %d, want 2", len(listener.lastNew))
	}
	firstFooB, firstFooA, firstWooA := distinctJoinPatternWildcardRow(t, listener.lastNew[0])
	secondFooB, secondFooA, secondWooA := distinctJoinPatternWildcardRow(t, listener.lastNew[1])
	if firstFooB != "Query" || firstFooA != "E1" || firstWooA != "E2" {
		t.Fatalf("first ordered row = (%s,%s,%s), want (Query,E1,E2)", firstFooB, firstFooA, firstWooA)
	}
	if secondFooB != "Query" || secondFooA != "E1" || secondWooA != "E3" {
		t.Fatalf("second ordered row = (%s,%s,%s), want (Query,E1,E3)", secondFooB, secondFooA, secondWooA)
	}
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

// TestDistinctOnSelectMultikeyWArrayParity mirrors
// EPLOtherDistinctOnSelectMultikeyWArray: on-trigger select distinct from a
// named window keyed by int arrays.
func TestDistinctOnSelectMultikeyWArrayParity(t *testing.T) {
	env, engine := newDistinctEnvironment(t)
	schema, ok := env.Schema("SupportEventWithManyArray")
	if !ok {
		t.Fatal("SupportEventWithManyArray schema missing")
	}
	if _, err := CreateNamedWindow(env, "MyWindow", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// Populate the window
	data := []fcmEventWithManyArray{
		{ID: "id", IntOne: []int{1, 2}, IntTwo: []int{3, 4}},
		{ID: "id", IntOne: []int{3, 4}, IntTwo: []int{1, 2}},
		{ID: "id", IntOne: []int{1, 2}, IntTwo: []int{3, 5}},
		{ID: "id", IntOne: []int{3, 4}, IntTwo: []int{1, 2}},
		{ID: "id", IntOne: []int{1, 2}, IntTwo: []int{3, 4}},
	}
	for _, e := range data {
		if err := engine.InsertNamedWindow(ctx, "MyWindow", e); err != nil {
			t.Fatal(err)
		}
	}

	// s0: on SupportBean_S0 select distinct intOne from MyWindow
	windowS0 := FromNamedWindow(env, "MyWindow")
	s0Plan, err := env.Build(OnEvent(From[distinctTriggerS0](env, "SupportBean_S0")).SelectFromNamedWindow(
		"MyWindow", nil,
		Alias("intOne", NamedWindowField[[]int]("intOne")),
	).Query(StatementName("s0"), WithDistinct()))
	if err != nil {
		t.Fatal(err)
	}
	s0Dep, err := engine.Deploy(ctx, s0Plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s0Dep.Undeploy(ctx) })
	var s0Rows []Row
	if _, err := s0Dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				s0Rows = append(s0Rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// s1: on SupportBean_S1 select distinct intOne, intTwo from MyWindow
	_ = windowS0
	s1Plan, err := env.Build(OnEvent(From[distinctTriggerS1](env, "SupportBean_S1")).SelectFromNamedWindow(
		"MyWindow", nil,
		Alias("intOne", NamedWindowField[[]int]("intOne")),
		Alias("intTwo", NamedWindowField[[]int]("intTwo")),
	).Query(StatementName("s1"), WithDistinct()))
	if err != nil {
		t.Fatal(err)
	}
	s1Dep, err := engine.Deploy(ctx, s1Plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s1Dep.Undeploy(ctx) })
	var s1Rows []Row
	if _, err := s1Dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				s1Rows = append(s1Rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Fire S0 trigger
	if err := engine.SendEvent(ctx, distinctTriggerS0{ID: 0}); err != nil {
		t.Fatal(err)
	}
	assertDistinctRowsAnyOrder(t, distinctRowsToStrings(s0Rows), [][]string{{"[1,2]"}, {"[3,4]"}}, "s0 distinct intOne")

	// Fire S1 trigger
	if err := engine.SendEvent(ctx, distinctTriggerS1{ID: 0}); err != nil {
		t.Fatal(err)
	}
	assertDistinctRowsAnyOrder(t, distinctRowsToStrings(s1Rows), [][]string{
		{"[1,2]", "[3,4]"}, {"[3,4]", "[1,2]"}, {"[1,2]", "[3,5]"},
	}, "s1 distinct intOne,intTwo")
}

// TestDistinctVariantStreamParity mirrors EPLOtherDistinctVariantStream:
// distinct over a variant stream with array keys.
func TestDistinctVariantStreamParity(t *testing.T) {
	env, engine := newDistinctEnvironment(t)
	manyArraySchema, ok := env.Schema("SupportEventWithManyArray")
	if !ok {
		t.Fatal("SupportEventWithManyArray schema missing")
	}
	if _, err := RegisterVariant(env, "MyVariant", manyArraySchema); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// Insert into variant: route SupportEventWithManyArray events to MyVariant
	insertPlan, err := env.Build(From[fcmEventWithManyArray](env, "SupportEventWithManyArray").InsertInto(
		"MyVariant", StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(ctx, insertPlan); err != nil {
		t.Fatal(err)
	}

	// s1: select distinct intOne from MyVariant#keepall
	s1Plan, err := env.Build(FromAny(env, "MyVariant").Window(KeepAll()).Select(
		Alias("intOne", Field[any, []int]("intOne")),
	).Query(StatementName("s1"), WithDistinct()))
	if err != nil {
		t.Fatal(err)
	}
	s1Dep, err := engine.Deploy(ctx, s1Plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s1Dep.Undeploy(ctx) })

	// s2: select distinct intOne, intTwo from MyVariant#keepall
	s2Plan, err := env.Build(FromAny(env, "MyVariant").Window(KeepAll()).Select(
		Alias("intOne", Field[any, []int]("intOne")),
		Alias("intTwo", Field[any, []int]("intTwo")),
	).Query(StatementName("s2"), WithDistinct()))
	if err != nil {
		t.Fatal(err)
	}
	s2Dep, err := engine.Deploy(ctx, s2Plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s2Dep.Undeploy(ctx) })

	// Send events
	data := []fcmEventWithManyArray{
		{ID: "id", IntOne: []int{1, 2}, IntTwo: []int{3, 4}},
		{ID: "id", IntOne: []int{3, 4}, IntTwo: []int{1, 2}},
		{ID: "id", IntOne: []int{1, 2}, IntTwo: []int{3, 5}},
		{ID: "id", IntOne: []int{3, 4}, IntTwo: []int{1, 2}},
		{ID: "id", IntOne: []int{1, 2}, IntTwo: []int{3, 4}},
	}
	for _, e := range data {
		if err := engine.SendEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
	}

	// Java asserts iterator lengths: s0=3, s1=2, s2=3
	// s0 (wildcard distinct *) = 3 unique events
	// s1 (distinct intOne) = 2
	// s2 (distinct intOne, intTwo) = 3
	assertDistinctRowsAnyOrder(t, distinctSnapshotRows(t, s1Dep.Statements()[0]),
		[][]string{{"[1,2]"}, {"[3,4]"}}, "s1 distinct intOne")
	assertDistinctRowsAnyOrder(t, distinctSnapshotRows(t, s2Dep.Statements()[0]),
		[][]string{{"[1,2]", "[3,4]"}, {"[3,4]", "[1,2]"}, {"[1,2]", "[3,5]"}},
		"s2 distinct intOne,intTwo")
}

// TestDistinctSubqueryParity mirrors EPLOtherSubquery: distinct inside an IN
// subquery.
func TestDistinctSubqueryParity(t *testing.T) {
	env, _ := newDistinctEnvironment(t)
	query := Select(
		From[distinctSupportBean](env, "SupportBean").Filter(
			SubqueryIn[string](
				Field[distinctSupportBean, string]("theString"),
				From[distinctSupportBeanA](env, "SupportBean_A").Window(KeepAll()).AsRecord(),
				Field[distinctSupportBeanA, string]("id"),
			),
		),
		Alias("theString", Field[distinctSupportBean, string]("theString")),
		Alias("intPrimitive", Field[distinctSupportBean, int]("intPrimitive")),
	).Query(StatementName("s0"))
	engine, _, listener := deployDistinct(t, env, query)

	sb := func(theString string, intPrimitive int) distinctSupportBean {
		return distinctSupportBean{TheString: theString, IntPrimitive: intPrimitive}
	}

	// Send A("E1"), then Bean("E1",2) -> fires
	if err := engine.SendEvent(context.Background(), distinctSupportBeanA{ID: "E1"}); err != nil {
		t.Fatal(err)
	}
	listener.reset()
	if err := engine.SendEvent(context.Background(), sb("E1", 2)); err != nil {
		t.Fatal(err)
	}
	if !listener.invoked {
		t.Fatal("listener not invoked for E1/2")
	}
	assertDistinctRowsAnyOrder(t, distinctRowsToStrings(listener.lastNew), [][]string{{"E1", "2"}}, "first match")

	// Send another A("E1") (duplicate), then Bean("E1",3) -> still fires (distinct dedups subquery but IN matches)
	if err := engine.SendEvent(context.Background(), distinctSupportBeanA{ID: "E1"}); err != nil {
		t.Fatal(err)
	}
	listener.reset()
	if err := engine.SendEvent(context.Background(), sb("E1", 3)); err != nil {
		t.Fatal(err)
	}
	if !listener.invoked {
		t.Fatal("listener not invoked for E1/3")
	}
	assertDistinctRowsAnyOrder(t, distinctRowsToStrings(listener.lastNew), [][]string{{"E1", "3"}}, "second match")
}

// TestDistinctOnDemandFAFParity mirrors EPLOtherOnDemandAndOnSelect: FAF
// distinct + on-select distinct over a named window.
func TestDistinctOnDemandAndOnSelectParity(t *testing.T) {
	env, engine := newDistinctEnvironment(t)
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema missing")
	}
	if _, err := CreateNamedWindow(env, "MyWindow", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	for _, e := range []distinctSupportBean{
		{TheString: "E1", IntPrimitive: 1},
		{TheString: "E1", IntPrimitive: 2},
		{TheString: "E2", IntPrimitive: 2},
		{TheString: "E1", IntPrimitive: 1},
	} {
		if err := engine.InsertNamedWindow(ctx, "MyWindow", e); err != nil {
			t.Fatal(err)
		}
	}

	// FAF: select distinct theString, intPrimitive from MyWindow order by theString, intPrimitive
	window := FromNamedWindow(env, "MyWindow")
	fafPlan, err := env.Build(window.Select(
		Alias("theString", Field[any, string]("theString")),
		Alias("intPrimitive", Field[any, int]("intPrimitive")),
	).Query(WithDistinct(),
		OrderBy(Ascending(ResultField[string]("theString"))),
		OrderBy(Ascending(ResultField[int]("intPrimitive"))),
	))
	if err != nil {
		t.Fatal(err)
	}
	fafResult, err := engine.ExecuteFireAndForget(ctx, fafPlan)
	if err != nil {
		t.Fatal(err)
	}
	// Java expects: {E1,1}, {E1,2}, {E2,2} ordered
	fafRows := distinctRowsToStrings(fafResultRows(fafResult.Results()))
	if len(fafRows) != 3 {
		t.Fatalf("FAF distinct: got %d rows, want 3: %v", len(fafRows), fafRows)
	}
	expectedFAF := [][]string{{"E1", "1"}, {"E1", "2"}, {"E2", "2"}}
	for i, row := range fafRows {
		if strings.Join(row, "|") != strings.Join(expectedFAF[i], "|") {
			t.Fatalf("FAF row %d: got %v, want %v", i, row, expectedFAF[i])
		}
	}
}
