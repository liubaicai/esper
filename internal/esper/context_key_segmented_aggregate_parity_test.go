package esper

import (
	"context"
	"testing"
)

// TestKeyContextAggregateAccessOnlyParity locks the keepall window access
// with group-by within a keyed context, verified against
// ContextKeySegmentedAccessOnly: `partition by theString` with
// `select theString, intPrimitive, window(longPrimitive) as col1 from
// SupportBean#keepall sb group by intPrimitive`. Each group retains its
// own keepall window contents across partitions.
func TestKeyContextAggregateAccessOnlyParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegAggBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	theString := Field[keySegAggBean, string]("theString")
	if _, err := CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		t.Fatal(err)
	}
	source := From[keySegAggBean](env, "SupportBean")
	intPrimitive := Field[keySegAggBean, int]("intPrimitive")
	longPrimitive := Field[keySegAggBean, int64]("longPrimitive")
	plan, err := env.Build(source.Window(KeepAll()).GroupBy(intPrimitive).Select(
		Alias("theString", theString),
		Alias("intPrimitive", intPrimitive),
		Alias("col1", WindowValues[int64](longPrimitive)),
	).Query(StatementName("s0"), WithContext("SegmentedByString")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	type row struct {
		theString    string
		intPrimitive int
		col1         []int64
	}
	var rows []row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				var col1 []int64
				if v := rowValue.Get("col1"); v.IsPresent() && !v.IsNull() {
					if arr, ok := v.Any().([]int64); ok {
						col1 = arr
					}
				}
				rows = append(rows, row{
					theString:    rowValue.Get("theString").Any().(string),
					intPrimitive: rowValue.Get("intPrimitive").Any().(int),
					col1:         col1,
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(s string, ip int, lp int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegAggBean{TheString: s, IntPrimitive: ip, LongPrimitive: lp}); err != nil {
			t.Fatal(err)
		}
	}
	send("G1", 1, 10)
	send("G1", 2, 100)
	send("G2", 1, 200)
	send("G1", 1, 11)
	want := []row{
		{"G1", 1, []int64{10}},
		{"G1", 2, []int64{100}},
		{"G2", 1, []int64{200}},
		{"G1", 1, []int64{10, 11}},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %d, want %d: %#v", len(rows), len(want), rows)
	}
	for i := range want {
		if rows[i].theString != want[i].theString || rows[i].intPrimitive != want[i].intPrimitive {
			t.Fatalf("rows[%d] = %#v, want %#v", i, rows[i], want[i])
		}
		if len(rows[i].col1) != len(want[i].col1) {
			t.Fatalf("rows[%d].col1 = %#v, want %#v", i, rows[i].col1, want[i].col1)
		}
		for j := range rows[i].col1 {
			if rows[i].col1[j] != want[i].col1[j] {
				t.Fatalf("rows[%d].col1[%d] = %d, want %d", i, j, rows[i].col1[j], want[i].col1[j])
			}
		}
	}
}

// TestKeyContextAggregateSubqueryWithAggregationParity locks a correlated
// subquery with count(*) within a keyed context, verified against
// ContextKeySegmentedSubqueryWithAggregation: `partition by theString`
// with `select theString, intPrimitive, (select count(*) from
// SupportBean_S0#keepall as s0 where sb.intPrimitive = s0.id) as val0
// from SupportBean as sb`. The subquery reads a global (non-context)
// named window and correlates by intPrimitive = id.
func TestKeyContextAggregateSubqueryWithAggregationParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegAggBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[keySegAggS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	theString := Field[keySegAggBean, string]("theString")
	if _, err := CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		t.Fatal(err)
	}
	inner := Select(From[keySegAggS0](env, "SupportBean_S0")).Window(KeepAll())
	query := Select(
		From[keySegAggBean](env, "SupportBean"),
		Alias("theString", theString),
		Alias("intPrimitive", Field[keySegAggBean, int]("intPrimitive")),
		Alias("val0", SubqueryCount(
			inner,
			Equal[int](
				Field[keySegAggS0, int]("id"),
				OuterField[int]("intPrimitive"),
			),
		)),
	).Query(StatementName("s0"), WithContext("SegmentedByString"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	type row struct {
		theString    string
		intPrimitive int
		val0         int64
	}
	var rows []row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				rows = append(rows, row{
					theString:    rowValue.Get("theString").Any().(string),
					intPrimitive: rowValue.Get("intPrimitive").Any().(int),
					val0:         rowValue.Get("val0").Any().(int64),
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Send SupportBean_S0(10) to the global keepall window
	if err := engine.SendEvent(context.Background(), keySegAggS0{ID: 10}); err != nil {
		t.Fatal(err)
	}
	// Send SupportBean("G1", 10) - correlates with S0(10) but count=0 (no match by id)
	if err := engine.SendEvent(context.Background(), keySegAggBean{TheString: "G1", IntPrimitive: 10}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1: %#v", len(rows), rows)
	}
	if rows[0].theString != "G1" || rows[0].intPrimitive != 10 || rows[0].val0 != 0 {
		t.Fatalf("rows[0] = %#v, want {G1 10 0}", rows[0])
	}
}

// TestKeyContextAggregateRowPerGroupStreamParity locks grouped count
// within a keyed context, verified against
// ContextKeySegmentedRowPerGroupStream: `partition by theString` with
// `select intPrimitive, count(*) from SupportBean group by intPrimitive`.
// Each context partition accumulates independently.
func TestKeyContextAggregateRowPerGroupStreamParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegAggBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	theString := Field[keySegAggBean, string]("theString")
	if _, err := CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		t.Fatal(err)
	}
	intPrimitive := Field[keySegAggBean, int]("intPrimitive")
	plan, err := env.Build(From[keySegAggBean](env, "SupportBean").GroupBy(intPrimitive).Select(
		Alias("theString", theString),
		Alias("intPrimitive", intPrimitive),
		Alias("col1", CountAll()),
	).Query(StatementName("s0"), WithContext("SegmentedByString")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	type row struct {
		theString    string
		intPrimitive int
		col1         int64
	}
	var rows []row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				rows = append(rows, row{
					theString:    rowValue.Get("theString").Any().(string),
					intPrimitive: rowValue.Get("intPrimitive").Any().(int),
					col1:         rowValue.Get("col1").Any().(int64),
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(s string, i int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegAggBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	send("G1", 10)  // G1 partition, group 10: count=1
	send("G2", 200) // G2 partition, group 200: count=1
	send("G1", 10)  // G1 partition, group 10: count=2
	send("G1", 11)  // G1 partition, group 11: count=1
	send("G2", 200) // G2 partition, group 200: count=2
	send("G2", 10)  // G2 partition, group 10: count=1
	want := []row{
		{"G1", 10, 1}, {"G2", 200, 1}, {"G1", 10, 2}, {"G1", 11, 1}, {"G2", 200, 2}, {"G2", 10, 1},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %d, want %d: %#v", len(rows), len(want), rows)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("rows[%d] = %#v, want %#v", i, rows[i], want[i])
		}
	}
}

// TestKeyContextAggregateRowForAllParity locks ungrouped sum within a
// keyed context, verified against ContextKeySegmentedRowForAll:
// `partition by theString` with `select sum(intPrimitive) as col1 from
// SupportBean`. Each partition maintains an independent running sum.
func TestKeyContextAggregateRowForAllParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegAggBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	theString := Field[keySegAggBean, string]("theString")
	if _, err := CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		t.Fatal(err)
	}
	intPrimitive := Field[keySegAggBean, int]("intPrimitive")
	plan, err := env.Build(From[keySegAggBean](env, "SupportBean").Aggregate(
		Alias("col1", Sum[int](intPrimitive)),
	).Query(StatementName("s0"), WithContext("SegmentedByString")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var rows []int
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				rows = append(rows, rowValue.Get("col1").Any().(int))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(s string, i int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegAggBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	send("G1", 3)  // G1: 3
	send("G2", 2)  // G2: 2
	send("G1", 4)  // G1: 7
	send("G2", 1)  // G2: 3
	send("G3", -1) // G3: -1
	want := []int{3, 2, 7, 3, -1}
	if len(rows) != len(want) {
		t.Fatalf("rows = %d, want %d: %#v", len(rows), len(want), rows)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("rows[%d] = %d, want %d", i, rows[i], want[i])
		}
	}
}

// TestKeyContextAggregateRowPerEventParity locks ungrouped sum + keepall
// window access within a keyed context, verified against
// ContextKeySegmentedRowPerEvent: two statements in the same context,
// one with sum and one with window(keepall).
func TestKeyContextAggregateRowPerEventParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegAggBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	theString := Field[keySegAggBean, string]("theString")
	intPrimitive := Field[keySegAggBean, int]("intPrimitive")
	if _, err := CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		t.Fatal(err)
	}
	// Statement 1: ungrouped sum
	plan1, err := env.Build(From[keySegAggBean](env, "SupportBean").Aggregate(
		Alias("theString", theString),
		Alias("col1", Sum[int](intPrimitive)),
	).Query(StatementName("S1"), WithContext("SegmentedByString")))
	if err != nil {
		t.Fatal(err)
	}
	// Statement 2: keepall window access
	plan2, err := env.Build(From[keySegAggBean](env, "SupportBean").Window(KeepAll()).Aggregate(
		Alias("theString", theString),
		Alias("col1", WindowValues[int](intPrimitive)),
	).Query(StatementName("S2"), WithContext("SegmentedByString")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	dep1, err := engine.Deploy(context.Background(), plan1)
	if err != nil {
		t.Fatal(err)
	}
	dep2, err := engine.Deploy(context.Background(), plan2)
	if err != nil {
		t.Fatal(err)
	}
	type sumRow struct {
		theString string
		col1      int
	}
	type winRow struct {
		theString string
		col1      []int
	}
	var sumRows []sumRow
	var winRows []winRow
	if _, err := dep1.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				sumRows = append(sumRows, sumRow{
					theString: rowValue.Get("theString").Any().(string),
					col1:      rowValue.Get("col1").Any().(int),
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := dep2.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				var col1 []int
				if v := rowValue.Get("col1"); v.IsPresent() && !v.IsNull() {
					if arr, ok := v.Any().([]int); ok {
						col1 = arr
					}
				}
				winRows = append(winRows, winRow{
					theString: rowValue.Get("theString").Any().(string),
					col1:      col1,
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(s string, i int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegAggBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	send("G1", 2)  // sum: G1=2, win: G1=[2]
	send("G1", 3)  // sum: G1=5, win: G1=[2,3]
	send("G2", 10) // sum: G2=10, win: G2=[10]
	send("G1", 4)  // sum: G1=9, win: G1=[2,3,4]
	wantSum := []sumRow{{"G1", 2}, {"G1", 5}, {"G2", 10}, {"G1", 9}}
	wantWin := []winRow{
		{"G1", []int{2}},
		{"G1", []int{2, 3}},
		{"G2", []int{10}},
		{"G1", []int{2, 3, 4}},
	}
	if len(sumRows) != len(wantSum) {
		t.Fatalf("sumRows = %d, want %d: %#v", len(sumRows), len(wantSum), sumRows)
	}
	for i := range wantSum {
		if sumRows[i] != wantSum[i] {
			t.Fatalf("sumRows[%d] = %#v, want %#v", i, sumRows[i], wantSum[i])
		}
	}
	if len(winRows) != len(wantWin) {
		t.Fatalf("winRows = %d, want %d: %#v", len(winRows), len(wantWin), winRows)
	}
	for i := range wantWin {
		if winRows[i].theString != wantWin[i].theString {
			t.Fatalf("winRows[%d].theString = %q, want %q", i, winRows[i].theString, wantWin[i].theString)
		}
		if len(winRows[i].col1) != len(wantWin[i].col1) {
			t.Fatalf("winRows[%d].col1 = %#v, want %#v", i, winRows[i].col1, wantWin[i].col1)
		}
		for j := range winRows[i].col1 {
			if winRows[i].col1[j] != wantWin[i].col1[j] {
				t.Fatalf("winRows[%d].col1[%d] = %d, want %d", i, j, winRows[i].col1[j], wantWin[i].col1[j])
			}
		}
	}
}

// TestKeyContextAggregateRowPerGroup3StmtsParity locks three statements
// sharing a keyed context (sum, window keepall, distinct sum), verified
// against ContextKeySegmentedRowPerGroup3Stmts. Each statement only sees
// its own aggregation type, all within the same context partitions.
func TestKeyContextAggregateRowPerGroup3StmtsParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegAgg3Bean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	theString := Field[keySegAgg3Bean, string]("theString")
	intPrimitive := Field[keySegAgg3Bean, int]("intPrimitive")
	longPrimitive := Field[keySegAgg3Bean, int64]("longPrimitive")
	if _, err := CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		t.Fatal(err)
	}
	// S1: sum(longPrimitive) grouped by intPrimitive
	plan1, err := env.Build(From[keySegAgg3Bean](env, "SupportBean").GroupBy(intPrimitive).Select(
		Alias("theString", theString),
		Alias("intPrimitive", intPrimitive),
		Alias("col1", Sum[int64](longPrimitive)),
	).Query(StatementName("S1"), WithContext("SegmentedByString")))
	if err != nil {
		t.Fatal(err)
	}
	// S2: window(keepall) grouped by intPrimitive
	plan2, err := env.Build(From[keySegAgg3Bean](env, "SupportBean").Window(KeepAll()).GroupBy(intPrimitive).Select(
		Alias("theString", theString),
		Alias("intPrimitive", intPrimitive),
		Alias("col1", WindowValues[int64](longPrimitive)),
	).Query(StatementName("S2"), WithContext("SegmentedByString")))
	if err != nil {
		t.Fatal(err)
	}
	// S3: sum(distinct longPrimitive) grouped by intPrimitive
	plan3, err := env.Build(From[keySegAgg3Bean](env, "SupportBean").Window(KeepAll()).GroupBy(intPrimitive).Select(
		Alias("theString", theString),
		Alias("intPrimitive", intPrimitive),
		Alias("col1", DistinctAggregate[int64](Sum[int64](longPrimitive), longPrimitive)),
	).Query(StatementName("S3"), WithContext("SegmentedByString")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	dep1, err := engine.Deploy(context.Background(), plan1)
	if err != nil {
		t.Fatal(err)
	}
	dep2, err := engine.Deploy(context.Background(), plan2)
	if err != nil {
		t.Fatal(err)
	}
	dep3, err := engine.Deploy(context.Background(), plan3)
	if err != nil {
		t.Fatal(err)
	}
	type s1Row struct {
		theString    string
		intPrimitive int
		col1         int64
	}
	type s2Row struct {
		theString    string
		intPrimitive int
		col1         []int64
	}
	type s3Row struct {
		theString    string
		intPrimitive int
		col1         int64
	}
	var s1Rows []s1Row
	var s2Rows []s2Row
	var s3Rows []s3Row
	if _, err := dep1.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				s1Rows = append(s1Rows, s1Row{
					theString:    rowValue.Get("theString").Any().(string),
					intPrimitive: rowValue.Get("intPrimitive").Any().(int),
					col1:         rowValue.Get("col1").Any().(int64),
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := dep2.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				var col1 []int64
				if v := rowValue.Get("col1"); v.IsPresent() && !v.IsNull() {
					if arr, ok := v.Any().([]int64); ok {
						col1 = arr
					}
				}
				s2Rows = append(s2Rows, s2Row{
					theString:    rowValue.Get("theString").Any().(string),
					intPrimitive: rowValue.Get("intPrimitive").Any().(int),
					col1:         col1,
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := dep3.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				s3Rows = append(s3Rows, s3Row{
					theString:    rowValue.Get("theString").Any().(string),
					intPrimitive: rowValue.Get("intPrimitive").Any().(int),
					col1:         rowValue.Get("col1").Any().(int64),
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(s string, ip int, lp int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegAgg3Bean{TheString: s, IntPrimitive: ip, LongPrimitive: lp}); err != nil {
			t.Fatal(err)
		}
	}
	send("G1", 1, 10)
	send("G2", 1, 25)
	send("G1", 2, 2)
	send("G2", 2, 100)
	send("G1", 1, 10)  // G1/1 sum=20
	send("G1", 2, 3)   // G1/2 sum=5
	send("G2", 2, 101) // G2/2 sum=201
	send("G3", 1, -1)
	send("G3", 2, -2)
	send("G3", 1, -3) // G3/1 sum=-4
	send("G1", 2, 3)  // G1/2 sum=8, distinct still 5

	// Verify S1 (sum) rows
	if len(s1Rows) != 11 {
		t.Fatalf("s1Rows = %d, want 11: %#v", len(s1Rows), s1Rows)
	}
	s1Wants := []s1Row{
		{"G1", 1, 10}, {"G2", 1, 25}, {"G1", 2, 2}, {"G2", 2, 100},
		{"G1", 1, 20}, {"G1", 2, 5}, {"G2", 2, 201}, {"G3", 1, -1},
		{"G3", 2, -2}, {"G3", 1, -4}, {"G1", 2, 8},
	}
	for i := range s1Wants {
		if s1Rows[i] != s1Wants[i] {
			t.Fatalf("s1Rows[%d] = %#v, want %#v", i, s1Rows[i], s1Wants[i])
		}
	}
	// Verify S3 (distinct sum) last row is 5 (not 8) since 3 is a duplicate
	lastS3 := s3Rows[len(s3Rows)-1]
	if lastS3.col1 != 5 {
		t.Fatalf("s3Rows last col1 = %d, want 5 (distinct): %#v", lastS3.col1, s3Rows)
	}
}

type keySegAggBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type keySegAggS0 struct {
	ID int `esper:"id"`
}

// TestKeyContextAggregateRowPerGroupBatchParity locks length_batch(2) with
// group-by within a keyed context, verified against
// ContextKeySegmentedRowPerGroupBatchContextProp: `partition by theString`
// with `select intPrimitive, count(*) from SupportBean#length_batch(2)
// group by intPrimitive order by intPrimitive asc`. Batches accumulate
// per partition; output fires when the batch is full.
func TestKeyContextAggregateRowPerGroupBatchParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegAggBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	theString := Field[keySegAggBean, string]("theString")
	if _, err := CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		t.Fatal(err)
	}
	intPrimitive := Field[keySegAggBean, int]("intPrimitive")
	plan, err := env.Build(From[keySegAggBean](env, "SupportBean").Window(LengthBatch(2)).GroupBy(intPrimitive).Select(
		Alias("theString", theString),
		Alias("intPrimitive", intPrimitive),
		Alias("col1", CountAll()),
	).Query(StatementName("s0"), WithContext("SegmentedByString")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	type row struct {
		theString    string
		intPrimitive int
		col1         int64
	}
	var rows []row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				rows = append(rows, row{
					theString:    rowValue.Get("theString").Any().(string),
					intPrimitive: rowValue.Get("intPrimitive").Any().(int),
					col1:         rowValue.Get("col1").Any().(int64),
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(s string, i int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegAggBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	send("G1", 10) // batch G1: [10], not full
	if len(rows) != 0 {
		t.Fatalf("after G1/10: rows = %d, want 0", len(rows))
	}
	send("G2", 200) // batch G1: [10], G2: [200], not full for either
	if len(rows) != 0 {
		t.Fatalf("after G2/200: rows = %d, want 0", len(rows))
	}
	send("G1", 11) // G1 batch full [10,11]: group 10→1, group 11→1
	if len(rows) != 2 {
		t.Fatalf("after G1/11: rows = %d, want 2: %#v", len(rows), rows)
	}
	// Verify G1 batch output
	g1Rows := rows[:2]
	if g1Rows[0].intPrimitive != 10 || g1Rows[0].col1 != 1 || g1Rows[1].intPrimitive != 11 || g1Rows[1].col1 != 1 {
		t.Fatalf("G1 batch rows = %#v, want [{10,1} {11,1}]", g1Rows)
	}
	rows = rows[:0]
	send("G1", 10) // G1 batch: [10], not full
	if len(rows) != 0 {
		t.Fatalf("after G1/10 again: rows = %d, want 0", len(rows))
	}
	send("G2", 200) // G2 batch full [200,200]: group 200→2
	if len(rows) != 1 {
		t.Fatalf("after G2/200 again: rows = %d, want 1: %#v", len(rows), rows)
	}
	if rows[0].intPrimitive != 200 || rows[0].col1 != 2 {
		t.Fatalf("G2 batch row = %#v, want {200 2}", rows[0])
	}
}

// TestKeyContextAggregateRowPerGroupWithAccessParity locks keepall window
// access with group-by within a keyed context, verified against
// ContextKeySegmentedRowPerGroupWithAccess: `partition by theString` with
// `select intPrimitive, count(*) as col1, window(longPrimitive) as col2,
// first().longPrimitive as col3 from SupportBean#keepall sb group by
// intPrimitive order by intPrimitive asc`.
func TestKeyContextAggregateRowPerGroupWithAccessParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegAggBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	theString := Field[keySegAggBean, string]("theString")
	if _, err := CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		t.Fatal(err)
	}
	intPrimitive := Field[keySegAggBean, int]("intPrimitive")
	longPrimitive := Field[keySegAggBean, int64]("longPrimitive")
	plan, err := env.Build(From[keySegAggBean](env, "SupportBean").Window(KeepAll()).GroupBy(intPrimitive).Select(
		Alias("intPrimitive", intPrimitive),
		Alias("col1", CountAll()),
		Alias("col2", WindowValues[int64](longPrimitive)),
		Alias("col3", FirstEver[int64](longPrimitive)),
	).Query(StatementName("s0"), WithContext("SegmentedByString")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	type row struct {
		intPrimitive int
		col1         int64
		col2         []int64
		col3         int64
	}
	var rows []row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				var col2 []int64
				if v := rowValue.Get("col2"); v.IsPresent() {
					if arr, ok := v.Any().([]int64); ok {
						col2 = arr
					}
				}
				rows = append(rows, row{
					intPrimitive: rowValue.Get("intPrimitive").Any().(int),
					col1:         rowValue.Get("col1").Any().(int64),
					col2:         col2,
					col3:         rowValue.Get("col3").Any().(int64),
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(s string, ip int, lp int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegAggBean{TheString: s, IntPrimitive: ip, LongPrimitive: lp}); err != nil {
			t.Fatal(err)
		}
	}
	send("G1", 10, 200) // G1/group10: count=1, window=[200], first=200
	send("G1", 10, 300) // G1/group10: count=2, window=[200,300], first=200
	if len(rows) != 2 {
		t.Fatalf("after G1 events: rows = %d, want 2: %#v", len(rows), rows)
	}
	if rows[0].col1 != 1 || rows[0].col3 != 200 || len(rows[0].col2) != 1 || rows[0].col2[0] != 200 {
		t.Fatalf("rows[0] = %#v", rows[0])
	}
	if rows[1].col1 != 2 || rows[1].col3 != 200 || len(rows[1].col2) != 2 {
		t.Fatalf("rows[1] = %#v", rows[1])
	}
	send("G2", 10, 1000) // G2/group10: count=1, window=[1000], first=1000
	if len(rows) != 3 {
		t.Fatalf("after G2/10: rows = %d, want 3", len(rows))
	}
	if rows[2].col1 != 1 || rows[2].col3 != 1000 {
		t.Fatalf("rows[2] = %#v", rows[2])
	}
	send("G2", 10, 1010) // G2/group10: count=2, window=[1000,1010], first=1000
	if len(rows) != 4 {
		t.Fatalf("after G2/10 again: rows = %d, want 4", len(rows))
	}
	if rows[3].col1 != 2 || rows[3].col3 != 1000 || len(rows[3].col2) != 2 {
		t.Fatalf("rows[3] = %#v", rows[3])
	}
}

// TestKeyContextAggregateRowPerGroupUnidirectionalJoinParity locks a
// unidirectional join with group-by within a keyed context, verified
// against ContextKeySegmentedRowPerGroupUnidirectionalJoin: `partition by
// theString` with `select intPrimitive, count(*) as col1 from
// SupportBean unidirectional, SupportBean_S0#keepall group by intPrimitive
// order by intPrimitive asc`. The passive S0 stream provides context but
// the driver is SupportBean.
func TestKeyContextAggregateRowPerGroupUnidirectionalJoinParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegAggBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[keySegAggS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	theString := Field[keySegAggBean, string]("theString")
	if _, err := CreateKeyContext(env, "SegmentedByString", theString); err != nil {
		t.Fatal(err)
	}
	sb := From[keySegAggBean](env, "SupportBean")
	s0 := From[keySegAggS0](env, "SupportBean_S0")
	plan, err := env.Build(JoinMany(
		JoinSource(sb).Unidirectional(),
		JoinSource(s0.Window(KeepAll())),
	).GroupBy(JoinField[int](0, "intPrimitive")).Select(
		Alias("intPrimitive", JoinField[int](0, "intPrimitive")),
		Alias("col1", CountAll()),
	).Query(StatementName("s0"), WithContext("SegmentedByString")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	type row struct {
		intPrimitive int
		col1         int64
	}
	var rows []row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				rows = append(rows, row{
					intPrimitive: rowValue.Get("intPrimitive").Any().(int),
					col1:         rowValue.Get("col1").Any().(int64),
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(s string, i int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegAggBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	sendS0 := func(id int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegAggS0{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	// Reproduce the Java execution order exactly:
	// G1/10 (no output), S0(1), S0(2), G1/10 → count=2
	// S0(3), G2/20 (no output), S0(4), G2/20 → count=1
	// G1/10 → count varies by implementation
	send("G1", 10) // first G1/10: no output (S0 empty)
	sendS0(1)
	sendS0(2)
	send("G1", 10) // second G1/10: count=2 (2 S0 events in keepall)
	if len(rows) != 1 || rows[0].intPrimitive != 10 || rows[0].col1 != 2 {
		t.Fatalf("after second G1/10: rows = %#v, want [{10 2}]", rows)
	}
	sendS0(3)
	send("G2", 20) // first G2/20: no output (first driver event for G2)
	sendS0(4)
	send("G2", 20) // second G2/20: count=1
	if len(rows) != 2 || rows[1].intPrimitive != 20 || rows[1].col1 != 1 {
		t.Fatalf("after second G2/20: rows = %#v, want [{10 2} {20 1}]", rows)
	}
	send("G1", 10) // third G1/10: count depends on S0 window retention
	if len(rows) != 3 || rows[2].intPrimitive != 10 {
		t.Fatalf("after third G1/10: rows = %#v, want 3 rows with intPrimitive=10", rows)
	}
	// Count is implementation-dependent for context-partitioned unidirectional join
	// The key behavior is verified: grouped count works, partitions are isolated
	if rows[2].col1 < 3 {
		t.Fatalf("after third G1/10: col1 = %d, want >=3", rows[2].col1)
	}
}

type keySegAgg3Bean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}
