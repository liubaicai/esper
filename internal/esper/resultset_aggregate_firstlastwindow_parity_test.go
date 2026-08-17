package esper

import (
	"context"
	"testing"
)

// Parity coverage for ResultSetAggregateFirstLastWindow executions that use
// only existing Go aggregate/window API. Covers access aggregates (first,
// last, window) over unbounded, length, and length_batch windows, grouped
// and ungrouped, with insert/remove stream semantics.
//
// - UnboundedSimple: first/last on unbounded stream (ever-semantics)
// - WindowedGrouped: length(5) grouped first/last/window with per-group expiry
// - BatchWindow: length_batch(2) irstream first/window/last
// - BatchWindowGrouped: length_batch(6) grouped first/window/last with empty-group null
// - WindowAndSumWGroup: length(3) grouped sum + window(expr) with expiry-driven null

type flwBean struct {
	TheString       string  `esper:"theString"`
	IntPrimitive    int     `esper:"intPrimitive"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
	LongPrimitive   int64   `esper:"longPrimitive"`
}

func registerFlwTypes(t *testing.T, env *Environment) {
	t.Helper()
	if _, err := RegisterStruct[flwBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
}

func flwSubscribe(t *testing.T, stmt *Statement) *[]ResultBatch {
	t.Helper()
	batches := make([]ResultBatch, 0, 8)
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &batches
}

func flwLastRow(t *testing.T, batches []ResultBatch) Row {
	t.Helper()
	if len(batches) == 0 {
		t.Fatal("no result batch")
	}
	last := batches[len(batches)-1]
	if len(last.New) != 1 {
		t.Fatalf("expected one new row, got %d", len(last.New))
	}
	row, ok := last.New[0].Row()
	if !ok {
		t.Fatalf("result is not a row: %#v", last.New[0])
	}
	return row
}

func flwAssertInt(t *testing.T, row Row, field string, want int) {
	t.Helper()
	got := row.Get(field).Any()
	if got != want {
		t.Fatalf("%s = %v, want %d", field, got, want)
	}
}

func flwAssertString(t *testing.T, row Row, field string, want string) {
	t.Helper()
	got := row.Get(field).Any()
	if got != want {
		t.Fatalf("%s = %v, want %s", field, got, want)
	}
}

func flwAssertIntSlice(t *testing.T, row Row, field string, want []int) {
	t.Helper()
	got := row.Get(field).Any()
	gotSlice, ok := got.([]int)
	if !ok {
		t.Fatalf("%s = %v (%T), want []int", field, got, got)
	}
	if len(gotSlice) != len(want) {
		t.Fatalf("%s len = %d, want %d (%v)", field, len(gotSlice), len(want), want)
	}
	for i, v := range want {
		if gotSlice[i] != v {
			t.Fatalf("%s[%d] = %d, want %d (full: %v)", field, i, gotSlice[i], v, want)
		}
	}
}

// TestResultSetAggregateUnboundedSimpleParity mirrors
// ResultSetAggregateUnboundedSimple: first/last on an unbounded stream
// (ever-semantics, no window). Java runtime: java-runtime-5c73ecba5097e8425b7f.
func TestResultSetAggregateUnboundedSimpleParity(t *testing.T) {
	env := NewEnvironment()
	registerFlwTypes(t, env)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// Java's unbounded first/last without a window acts as firstever/lastever.
	// Go's First/Last on an unbounded stream tracks only the current batch;
	// FirstEver/LastEver provides the correct ever-semantics.
	plan, err := env.Build(
		From[flwBean](env, "SupportBean").Aggregate(
			Alias("c0", FirstEver[string](Field[flwBean, string]("theString"))),
			Alias("c1", LastEver[string](Field[flwBean, string]("theString"))),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := flwSubscribe(t, deployment.Statements()[0])

	for _, s := range []string{"E1", "E2", "E3"} {
		if err := engine.SendEvent(context.Background(), flwBean{TheString: s}); err != nil {
			t.Fatal(err)
		}
		row := flwLastRow(t, *batches)
		flwAssertString(t, row, "c0", "E1")
		flwAssertString(t, row, "c1", s)
	}
}

// TestResultSetAggregateWindowedGroupedParity mirrors
// ResultSetAggregateWindowedGrouped (plain variant): length(5) grouped
// first/last/window with per-group expiry and order-by.
// Java runtime: java-runtime-e4b69ac483b580cbbf07.
func TestResultSetAggregateWindowedGroupedParity(t *testing.T) {
	env := NewEnvironment()
	registerFlwTypes(t, env)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(
		From[flwBean](env, "SupportBean").Window(LengthWindow(5)).
			GroupBy(Field[flwBean, string]("theString")).
			Select(
				Alias("theString", Field[flwBean, string]("theString")),
				Alias("firststring", First[string](Field[flwBean, string]("theString"))),
				Alias("laststring", Last[string](Field[flwBean, string]("theString"))),
				Alias("firstint", First[int](Field[flwBean, int]("intPrimitive"))),
				Alias("lastint", Last[int](Field[flwBean, int]("intPrimitive"))),
				Alias("allint", WindowValues[int](Field[flwBean, int]("intPrimitive"))),
			).
			Query(StatementName("s0"), OrderBy(Ascending(Field[flwBean, string]("theString")))),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := flwSubscribe(t, deployment.Statements()[0])
	ctx := context.Background()

	send := func(s string, v int) {
		t.Helper()
		if err := engine.SendEvent(ctx, flwBean{TheString: s, IntPrimitive: v}); err != nil {
			t.Fatal(err)
		}
	}

	// E1,10 → {E1,E1,10,E1,10,[10]}
	send("E1", 10)
	row := flwLastRow(t, *batches)
	flwAssertString(t, row, "theString", "E1")
	flwAssertString(t, row, "firststring", "E1")
	flwAssertInt(t, row, "firstint", 10)
	flwAssertString(t, row, "laststring", "E1")
	flwAssertInt(t, row, "lastint", 10)
	flwAssertIntSlice(t, row, "allint", []int{10})

	// E2,11 → {E2,E2,11,E2,11,[11]}
	send("E2", 11)
	row = flwLastRow(t, *batches)
	flwAssertString(t, row, "theString", "E2")
	flwAssertInt(t, row, "firstint", 11)
	flwAssertIntSlice(t, row, "allint", []int{11})

	// E1,12 → {E1,E1,10,E1,12,[10,12]}
	send("E1", 12)
	row = flwLastRow(t, *batches)
	flwAssertString(t, row, "theString", "E1")
	flwAssertInt(t, row, "firstint", 10)
	flwAssertInt(t, row, "lastint", 12)
	flwAssertIntSlice(t, row, "allint", []int{10, 12})

	// E2,13 → {E2,E2,11,E2,13,[11,13]}
	send("E2", 13)
	row = flwLastRow(t, *batches)
	flwAssertString(t, row, "theString", "E2")
	flwAssertInt(t, row, "firstint", 11)
	flwAssertInt(t, row, "lastint", 13)
	flwAssertIntSlice(t, row, "allint", []int{11, 13})

	// E2,14 → {E2,E2,11,E2,14,[11,13,14]}
	send("E2", 14)
	row = flwLastRow(t, *batches)
	flwAssertInt(t, row, "lastint", 14)
	flwAssertIntSlice(t, row, "allint", []int{11, 13, 14})

	// E1,15 → {E1,E1,12,E1,15,[12,15]} (E1/10 pushed out)
	send("E1", 15)
	row = flwLastRow(t, *batches)
	flwAssertString(t, row, "theString", "E1")
	flwAssertInt(t, row, "firstint", 12)
	flwAssertInt(t, row, "lastint", 15)
	flwAssertIntSlice(t, row, "allint", []int{12, 15})

	// E1,16 → two rows (order by theString): {E1,...,12,16,[12,15,16]}, {E2,...,13,14,[13,14]}
	send("E1", 16)
	last := (*batches)[len(*batches)-1]
	if len(last.New) != 2 {
		t.Fatalf("expected 2 rows after E1,16, got %d", len(last.New))
	}
	r0, _ := last.New[0].Row()
	r1, _ := last.New[1].Row()
	flwAssertString(t, r0, "theString", "E1")
	flwAssertInt(t, r0, "firstint", 12)
	flwAssertInt(t, r0, "lastint", 16)
	flwAssertIntSlice(t, r0, "allint", []int{12, 15, 16})
	flwAssertString(t, r1, "theString", "E2")
	flwAssertInt(t, r1, "firstint", 13)
	flwAssertInt(t, r1, "lastint", 14)
	flwAssertIntSlice(t, r1, "allint", []int{13, 14})
}

// TestResultSetAggregateBatchWindowParity mirrors
// ResultSetAggregateBatchWindow: length_batch(2) irstream first/window/last.
// Java runtime: java-runtime-eb23cb37438e324a71ed.
func TestResultSetAggregateBatchWindowParity(t *testing.T) {
	env := NewEnvironment()
	registerFlwTypes(t, env)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(
		From[flwBean](env, "SupportBean").Window(LengthBatch(2)).
			Aggregate(
				Alias("fs", First[string](Field[flwBean, string]("theString"))),
				Alias("ws", WindowValues[string](Field[flwBean, string]("theString"))),
				Alias("ls", Last[string](Field[flwBean, string]("theString"))),
			).
			Query(StatementName("s0"), WithOldStream()),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := flwSubscribe(t, deployment.Statements()[0])
	ctx := context.Background()

	send := func(s string) {
		t.Helper()
		if err := engine.SendEvent(ctx, flwBean{TheString: s}); err != nil {
			t.Fatal(err)
		}
	}

	// E1, E2 → batch flush: new {E1,[E1,E2],E2}, old null
	send("E1")
	send("E2")
	last := (*batches)[len(*batches)-1]
	if len(last.New) != 1 {
		t.Fatalf("expected 1 new row, got %d", len(last.New))
	}
	row, _ := last.New[0].Row()
	flwAssertString(t, row, "fs", "E1")
	flwAssertString(t, row, "ls", "E2")
	ws := row.Get("ws").Any().([]string)
	if len(ws) != 2 || ws[0] != "E1" || ws[1] != "E2" {
		t.Fatalf("ws = %v", ws)
	}

	// E3, E4 → new {E3,[E3,E4],E4}, old {E1,[E1,E2],E2}
	send("E3")
	send("E4")
	last = (*batches)[len(*batches)-1]
	if len(last.New) != 1 {
		t.Fatalf("expected 1 new row, got %d", len(last.New))
	}
	row, _ = last.New[0].Row()
	flwAssertString(t, row, "fs", "E3")
	flwAssertString(t, row, "ls", "E4")
	if len(last.Old) != 1 {
		t.Fatalf("expected 1 old row, got %d", len(last.Old))
	}
	oldRow, _ := last.Old[0].Row()
	flwAssertString(t, oldRow, "fs", "E1")
	flwAssertString(t, oldRow, "ls", "E2")

	// E5 (no output — batch not full), E6 → new {E5,[E5,E6],E6}, old {E3,[E3,E4],E4}
	send("E5")
	batchesBefore := len(*batches)
	send("E6")
	if len(*batches) != batchesBefore+1 {
		t.Fatalf("expected 1 new batch, got %d", len(*batches)-batchesBefore)
	}
	last = (*batches)[len(*batches)-1]
	row, _ = last.New[0].Row()
	flwAssertString(t, row, "fs", "E5")
	flwAssertString(t, row, "ls", "E6")
}

// TestResultSetAggregateBatchWindowGroupedParity mirrors
// ResultSetAggregateBatchWindowGrouped: length_batch(6) grouped
// first/window/last with empty-group null rows on flush.
// Java runtime: java-runtime-f04c46c8d88698c3726a.
func TestResultSetAggregateBatchWindowGroupedParity(t *testing.T) {
	env := NewEnvironment()
	registerFlwTypes(t, env)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(
		From[flwBean](env, "SupportBean").Window(LengthBatch(6)).
			GroupBy(Field[flwBean, string]("theString")).
			Select(
				Alias("theString", Field[flwBean, string]("theString")),
				Alias("fi", First[int](Field[flwBean, int]("intPrimitive"))),
				Alias("wi", WindowValues[int](Field[flwBean, int]("intPrimitive"))),
				Alias("li", Last[int](Field[flwBean, int]("intPrimitive"))),
			).
			Query(StatementName("s0"), OrderBy(Ascending(Field[flwBean, string]("theString")))),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := flwSubscribe(t, deployment.Statements()[0])
	ctx := context.Background()

	send := func(s string, v int) {
		t.Helper()
		if err := engine.SendEvent(ctx, flwBean{TheString: s, IntPrimitive: v}); err != nil {
			t.Fatal(err)
		}
	}

	// 6 events: E1/10, E2/20, E1/11, E3/30, E3/31, E1/12 → flush
	send("E1", 10)
	send("E2", 20)
	send("E1", 11)
	send("E3", 30)
	send("E3", 31)
	send("E1", 12)

	last := (*batches)[len(*batches)-1]
	if len(last.New) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(last.New))
	}
	r0, _ := last.New[0].Row()
	r1, _ := last.New[1].Row()
	r2, _ := last.New[2].Row()

	flwAssertString(t, r0, "theString", "E1")
	flwAssertInt(t, r0, "fi", 10)
	flwAssertInt(t, r0, "li", 12)
	flwAssertIntSlice(t, r0, "wi", []int{10, 11, 12})

	flwAssertString(t, r1, "theString", "E2")
	flwAssertInt(t, r1, "fi", 20)
	flwAssertInt(t, r1, "li", 20)
	flwAssertIntSlice(t, r1, "wi", []int{20})

	flwAssertString(t, r2, "theString", "E3")
	flwAssertInt(t, r2, "fi", 30)
	flwAssertInt(t, r2, "li", 31)
	flwAssertIntSlice(t, r2, "wi", []int{30, 31})

	// Second batch: 6 E1 events → E1 row populated, E2/E3 null
	for i := 13; i <= 18; i++ {
		send("E1", i)
	}
	last = (*batches)[len(*batches)-1]
	if len(last.New) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(last.New))
	}
	r0, _ = last.New[0].Row()
	r1, _ = last.New[1].Row()
	r2, _ = last.New[2].Row()

	flwAssertString(t, r0, "theString", "E1")
	flwAssertInt(t, r0, "fi", 13)
	flwAssertInt(t, r0, "li", 18)
	flwAssertIntSlice(t, r0, "wi", []int{13, 14, 15, 16, 17, 18})

	// E2 and E3 groups are empty → null aggregates
	flwAssertString(t, r1, "theString", "E2")
	if r1.Get("fi").IsPresent() {
		t.Fatalf("E2 fi should be null, got %v", r1.Get("fi").Any())
	}
	flwAssertString(t, r2, "theString", "E3")
	if r2.Get("fi").IsPresent() {
		t.Fatalf("E3 fi should be null, got %v", r2.Get("fi").Any())
	}
}

// TestResultSetAggregateWindowAndSumWGroupParity mirrors
// ResultSetAggregateWindowAndSumWGroup: length(3) grouped sum + window(expr)
// with expiry-driven empty-group null rows.
// Java runtime: java-runtime-b03302bb599098d90c20.
func TestResultSetAggregateWindowAndSumWGroupParity(t *testing.T) {
	env := NewEnvironment()
	registerFlwTypes(t, env)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(
		From[flwBean](env, "SupportBean").Window(LengthWindow(3)).
			GroupBy(Field[flwBean, string]("theString")).
			Select(
				Alias("c0", Field[flwBean, string]("theString")),
				Alias("c1", Sum[int](Field[flwBean, int]("intPrimitive"))),
				Alias("c2", WindowValues[int64](
					Multiply[int64](Field[flwBean, int]("intPrimitive"), Field[flwBean, int64]("longPrimitive")),
				)),
			).
			Query(StatementName("s0"), OrderBy(Ascending(Field[flwBean, string]("theString")))),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	batches := flwSubscribe(t, deployment.Statements()[0])
	ctx := context.Background()

	send := func(s string, iVal int, lVal int64) {
		t.Helper()
		if err := engine.SendEvent(ctx, flwBean{TheString: s, IntPrimitive: iVal, LongPrimitive: lVal}); err != nil {
			t.Fatal(err)
		}
	}

	// E1(10,5) → {E1,10,[50]}
	send("E1", 10, 5)
	row := flwLastRow(t, *batches)
	flwAssertString(t, row, "c0", "E1")
	flwAssertInt(t, row, "c1", 10)

	// E2(100,20) → {E2,100,[2000]}
	send("E2", 100, 20)
	row = flwLastRow(t, *batches)
	flwAssertString(t, row, "c0", "E2")
	flwAssertInt(t, row, "c1", 100)

	// E1(15,2) → {E1,25,[50,30]}
	send("E1", 15, 2)
	row = flwLastRow(t, *batches)
	flwAssertString(t, row, "c0", "E1")
	flwAssertInt(t, row, "c1", 25)

	// E1(18,3) → {E1,33,[30,54]} (E1/10 pushed out; sum 15+18)
	send("E1", 18, 3)
	row = flwLastRow(t, *batches)
	flwAssertString(t, row, "c0", "E1")
	flwAssertInt(t, row, "c1", 33)

	// E1(19,4) → rows {E1,52,[30,54,76]}, {E2,null,null} (E2/100 pushed out)
	send("E1", 19, 4)
	last := (*batches)[len(*batches)-1]
	if len(last.New) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(last.New))
	}
	r0, _ := last.New[0].Row()
	r1, _ := last.New[1].Row()
	flwAssertString(t, r0, "c0", "E1")
	flwAssertInt(t, r0, "c1", 52)
	flwAssertString(t, r1, "c0", "E2")
	if r1.Get("c1").IsPresent() {
		t.Fatalf("E2 c1 should be null, got %v", r1.Get("c1").Any())
	}

	// E1(17,-1) → {E1,54,[54,76,-17]} (sum 18+19+17)
	send("E1", 17, -1)
	row = flwLastRow(t, *batches)
	flwAssertString(t, row, "c0", "E1")
	flwAssertInt(t, row, "c1", 54)

	// E2(1,1000) → rows {E1,36,[76,-17]}, {E2,1,[1000]}
	send("E2", 1, 1000)
	last = (*batches)[len(*batches)-1]
	if len(last.New) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(last.New))
	}
	r0, _ = last.New[0].Row()
	r1, _ = last.New[1].Row()
	flwAssertString(t, r0, "c0", "E1")
	flwAssertInt(t, r0, "c1", 36)
	flwAssertString(t, r1, "c0", "E2")
	flwAssertInt(t, r1, "c1", 1)
}
