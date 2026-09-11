package esper

import (
	"context"
	"database/sql/driver"
	"reflect"
	"sync"
	"testing"
	"time"
)

// TestDatabaseTimeBatchMatchesJava covers EPLDatabaseTimeBatch (and the OM /
// Compile variants, which share runtestTimeBatch): a time_batch window over
// the event stream buffers the joining beans silently, the scheduled release
// re-polls the historical source once per released bean, and the joined rows
// leave as exactly one new-data batch at the boundary in release order.
func TestDatabaseTimeBatchMatchesJava(t *testing.T) {
	var pollMu sync.Mutex
	var pollKeys []int64
	polls := func() []int64 {
		pollMu.Lock()
		defer pollMu.Unlock()
		return append([]int64(nil), pollKeys...)
	}
	dbJoinSetHandler(func(_ string, args []any) (dbJoinSQLResult, error) {
		result := dbJoinSQLResult{columns: []string{"mybigint", "myint"}}
		key, ok := dbJoinToInt64(args[0])
		if !ok {
			return result, nil
		}
		pollMu.Lock()
		pollKeys = append(pollKeys, key)
		pollMu.Unlock()
		for _, row := range dbJoinMyTestTable {
			if row.mybigint == key {
				result.rows = append(result.rows, []driver.Value{row.mybigint, row.myint})
				break
			}
		}
		return result, nil
	})
	env := NewEnvironment()
	dbJoinRegisterSupportBean(t, env)
	schema, err := NewMapSchema("MyTestTable", []FieldSpec{
		FieldDef("mybigint", reflect.TypeOf(int64(0))),
		FieldDef("myint", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()
	provider, err := NewSQLHistoricalProvider(db, schema,
		"select mybigint, myint from mytesttable where ? = mybigint",
		func(request HistoricalRequest) any {
			return request.Trigger.Get("intPrimitive").Any()
		})
	if err != nil {
		t.Fatal(err)
	}
	hist := FromHistoricalOn[map[string]any](env, "MyDBWithRetain", "SupportBean", schema, provider)
	query := JoinMany(
		JoinSource(hist),
		JoinSource(From[dbJoinSupportBean](env, "SupportBean").Window(TimeBatch(10*time.Second))),
	).Select(
		SelectFrom(0, "myint", Field[map[string]any, int]("myint")),
	).Query(StatementName("s0"), WithOldStream())
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	origin := time.Unix(0, 0).UTC()
	engine := NewEngine(env, WithStartTime(origin))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	batches := dbTimeBatchSubscribeBatches(t, deployment.Statements()[0])

	snapshotRows := func() []int {
		snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return dbTimeBatchRowInts(dbTimeBatchRowMaps(snapshot.Results()))
	}

	send := func(value int) {
		if err := engine.Send(context.Background(), "SupportBean", dbJoinSupportBean{TheString: "E", IntPrimitive: value}); err != nil {
			t.Fatal(err)
		}
	}

	// Java's iterator accumulates the joined rows as beans buffer: the
	// statement iterator composes the batch window's current content and
	// re-polls the historical per buffered bean (runtestTimeBatch asserts
	// [[100]], [[100],[50]], [[100],[50],[20]] before any delivery), while
	// the buffered sends themselves poll nothing (the composer is deferred
	// to the boundary) and deliver nothing.
	if rows := snapshotRows(); len(rows) != 0 {
		t.Fatalf("snapshot before sends = %v, want empty", rows)
	}
	mark := len(polls())
	send(10)
	if got := polls()[mark:]; len(got) != 0 {
		t.Fatalf("polls driven by send = %v, want none", got)
	}
	if rows := snapshotRows(); !dbTimeBatchEqualInts(rows, []int{100}) {
		t.Fatalf("snapshot after SB(10) = %v, want [100]", rows)
	}
	if got := polls()[mark:]; !dbTimeBatchEqualInt64s(got, []int64{10}) {
		t.Fatalf("iterator poll keys after SB(10) = %v, want [10]", got)
	}
	send(5)
	if got := polls()[mark:]; len(got) != 1 {
		t.Fatalf("polls driven by send = %v, want none", got)
	}
	if rows := snapshotRows(); !dbTimeBatchEqualInts(rows, []int{100, 50}) {
		t.Fatalf("snapshot after SB(5) = %v, want [100 50]", rows)
	}
	if got := polls()[mark:]; !dbTimeBatchEqualInt64s(got, []int64{10, 10, 5}) {
		t.Fatalf("iterator poll keys after SB(5) = %v, want [10 10 5]", got)
	}
	send(2)
	if rows := snapshotRows(); !dbTimeBatchEqualInts(rows, []int{100, 50, 20}) {
		t.Fatalf("snapshot after SB(2) = %v, want [100 50 20]", rows)
	}
	if got := batches(); len(got) != 0 {
		t.Fatalf("batches before boundary = %d, want 0", len(got))
	}

	// The boundary releases the batch as new data; the flush re-polls the
	// historical exactly once per released bean and the joined rows leave
	// as one batch in release order.
	flushMark := len(polls())
	if err := engine.AdvanceTime(context.Background(), origin.Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
	if got := polls()[flushMark:]; !dbTimeBatchEqualInt64s(got, []int64{10, 5, 2}) {
		t.Fatalf("flush poll keys at 10s = %v, want [10 5 2]", got)
	}
	got := batches()
	if len(got) != 1 {
		t.Fatalf("batches at 10s = %d, want 1", len(got))
	}
	if newRows := dbTimeBatchRowInts(got[0].newRows); !dbTimeBatchEqualInts(newRows, []int{100, 50, 20}) {
		t.Fatalf("new rows at 10s = %v, want [100 50 20]", newRows)
	}
	// The flush emptied the batch window, so the iterator is empty right
	// after the delivery (runtestTimeBatch: iterator null at 10000).
	if rows := snapshotRows(); len(rows) != 0 {
		t.Fatalf("snapshot at 10s after flush = %v, want empty", rows)
	}

	// Subsequent sends buffer again: no output and no release-driven poll
	// until the next boundary, and the iterator accumulates the new cycle
	// ([[90]], then [[90],[80]]).
	mark = len(polls())
	send(9)
	if got := polls()[mark:]; len(got) != 0 {
		t.Fatalf("polls driven by send = %v, want none", got)
	}
	if rows := snapshotRows(); !dbTimeBatchEqualInts(rows, []int{90}) {
		t.Fatalf("snapshot after SB(9) = %v, want [90]", rows)
	}
	send(8)
	if rows := snapshotRows(); !dbTimeBatchEqualInts(rows, []int{90, 80}) {
		t.Fatalf("snapshot after SB(8) = %v, want [90 80]", rows)
	}
	if got := polls()[mark:]; !dbTimeBatchEqualInt64s(got, []int64{9, 9, 8}) {
		t.Fatalf("iterator poll keys between boundaries = %v, want [9 9 8]", got)
	}
	if got := batches(); len(got) != 1 {
		t.Fatalf("batches between boundaries = %d, want 1", len(got))
	}

	// The second boundary delivers the next cycle, re-polls per released
	// bean, and retires the previous cycle as remove-stream rows.
	flushMark = len(polls())
	if err := engine.AdvanceTime(context.Background(), origin.Add(20*time.Second)); err != nil {
		t.Fatal(err)
	}
	if got := polls()[flushMark:]; !dbTimeBatchEqualInt64s(got, []int64{9, 8}) {
		t.Fatalf("flush poll keys at 20s = %v, want [9 8]", got)
	}
	got = batches()
	if len(got) != 2 {
		t.Fatalf("batches at 20s = %d, want 2", len(got))
	}
	if newRows := dbTimeBatchRowInts(got[1].newRows); !dbTimeBatchEqualInts(newRows, []int{90, 80}) {
		t.Fatalf("new rows at 20s = %v, want [90 80]", newRows)
	}
	if oldRows := dbTimeBatchRowInts(got[1].oldRows); !dbTimeBatchEqualInts(oldRows, []int{100, 50, 20}) {
		t.Fatalf("old rows at 20s = %v, want [100 50 20]", oldRows)
	}
	if rows := snapshotRows(); len(rows) != 0 {
		t.Fatalf("snapshot at 20s after flush = %v, want empty", rows)
	}
}

// TestTimeBatchHistoricalJoinPairsReleasedRowWithOwnPoll pins the release
// pairing contract with a function-fed provider: every released bean drives
// exactly its own historical lookup, an unmatched released bean produces no
// row, and a completed batch never leaks its historical rows into a later
// cycle (the released rows must not cross with stale poll results).
func TestTimeBatchHistoricalJoinPairsReleasedRowWithOwnPoll(t *testing.T) {
	env := NewEnvironment()
	dbJoinRegisterSupportBean(t, env)
	schema, err := NewMapSchema("MyTestTable", []FieldSpec{
		FieldDef("mybigint", reflect.TypeOf(int64(0))),
		FieldDef("myint", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	provider := &timeBatchHistoricalProvider{schema: schema}
	hist := FromHistoricalOn[map[string]any](env, "MyTestTable", "SupportBean", schema, provider)
	query := JoinMany(
		JoinSource(hist),
		JoinSource(From[dbJoinSupportBean](env, "SupportBean").Window(TimeBatch(10*time.Second))),
	).Select(
		SelectFrom(0, "myint", Field[map[string]any, int]("myint")),
	).Query(StatementName("s0"), WithOldStream())
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	origin := time.Unix(0, 0).UTC()
	engine := NewEngine(env, WithStartTime(origin))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	batches := dbTimeBatchSubscribeBatches(t, deployment.Statements()[0])

	send := func(value int) {
		if err := engine.Send(context.Background(), "SupportBean", dbJoinSupportBean{TheString: "E", IntPrimitive: value}); err != nil {
			t.Fatal(err)
		}
	}
	send(10)
	send(5)
	send(2)
	send(77) // no historical row matches this key
	if got := batches(); len(got) != 0 {
		t.Fatalf("batches before boundary = %d, want 0", len(got))
	}
	if got := provider.Keys(); len(got) != 0 {
		t.Fatalf("provider polls before boundary = %v, want none", got)
	}

	if err := engine.AdvanceTime(context.Background(), origin.Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
	got := batches()
	if len(got) != 1 {
		t.Fatalf("batches at 10s = %d, want 1", len(got))
	}
	if newRows := dbTimeBatchRowInts(got[0].newRows); !dbTimeBatchEqualInts(newRows, []int{100, 50, 20}) {
		t.Fatalf("new rows at 10s = %v, want [100 50 20] (no duplicates, no row for 77)", newRows)
	}
	if keys := provider.Keys(); !dbTimeBatchEqualInt64s(keys, []int64{10, 5, 2, 77}) {
		t.Fatalf("provider poll keys at 10s = %v, want [10 5 2 77]", keys)
	}

	// The next cycle must see only its own poll results: the retired batch's
	// historical rows cascade out with their released beans.
	send(9)
	if err := engine.AdvanceTime(context.Background(), origin.Add(20*time.Second)); err != nil {
		t.Fatal(err)
	}
	got = batches()
	if len(got) != 2 {
		t.Fatalf("batches at 20s = %d, want 2", len(got))
	}
	if newRows := dbTimeBatchRowInts(got[1].newRows); !dbTimeBatchEqualInts(newRows, []int{90}) {
		t.Fatalf("new rows at 20s = %v, want [90] with no stale rows from the first batch", newRows)
	}
	if oldRows := dbTimeBatchRowInts(got[1].oldRows); !dbTimeBatchEqualInts(oldRows, []int{100, 50, 20}) {
		t.Fatalf("old rows at 20s = %v, want [100 50 20]", oldRows)
	}
	if keys := provider.Keys(); !dbTimeBatchEqualInt64s(keys, []int64{10, 5, 2, 77, 9}) {
		t.Fatalf("provider poll keys at 20s = %v, want [10 5 2 77 9]", keys)
	}
}

// TestTimeBatchHistoricalJoinSnapshotReadOnly pins the iterator contract's
// read-only boundary: snapshots re-drive the historical polls for the
// buffered batch without mutating join state (repeated snapshots are
// identical), without consuming or reshaping the pending batch (the boundary
// still delivers the full batch exactly once, with the previous cycle as
// remove-stream rows), and the snapshot is empty once the batch has flushed.
func TestTimeBatchHistoricalJoinSnapshotReadOnly(t *testing.T) {
	env := NewEnvironment()
	dbJoinRegisterSupportBean(t, env)
	schema, err := NewMapSchema("MyTestTable", []FieldSpec{
		FieldDef("mybigint", reflect.TypeOf(int64(0))),
		FieldDef("myint", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	provider := &timeBatchHistoricalProvider{schema: schema}
	hist := FromHistoricalOn[map[string]any](env, "MyTestTable", "SupportBean", schema, provider)
	query := JoinMany(
		JoinSource(hist),
		JoinSource(From[dbJoinSupportBean](env, "SupportBean").Window(TimeBatch(10*time.Second))),
	).Select(
		SelectFrom(0, "myint", Field[map[string]any, int]("myint")),
	).Query(StatementName("s0"), WithOldStream())
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	origin := time.Unix(0, 0).UTC()
	engine := NewEngine(env, WithStartTime(origin))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	batches := dbTimeBatchSubscribeBatches(t, deployment.Statements()[0])

	snapshotRows := func() []int {
		snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return dbTimeBatchRowInts(dbTimeBatchRowMaps(snapshot.Results()))
	}
	send := func(value int) {
		if err := engine.Send(context.Background(), "SupportBean", dbJoinSupportBean{TheString: "E", IntPrimitive: value}); err != nil {
			t.Fatal(err)
		}
	}
	send(10)
	send(5)
	send(2)

	// Repeated snapshots compose the same buffered view: the re-drive is
	// read-only and idempotent (the provider may be re-polled, exactly as
	// Esper's iterator re-executes the lookup per iteration).
	for iteration := 0; iteration < 3; iteration++ {
		if rows := snapshotRows(); !dbTimeBatchEqualInts(rows, []int{100, 50, 20}) {
			t.Fatalf("snapshot iteration %d = %v, want [100 50 20]", iteration, rows)
		}
	}
	if got := batches(); len(got) != 0 {
		t.Fatalf("batches after snapshots = %d, want 0 (snapshot must not deliver)", len(got))
	}

	// The buffered batch is untouched by the snapshots: the boundary still
	// releases the full batch exactly once.
	if err := engine.AdvanceTime(context.Background(), origin.Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
	got := batches()
	if len(got) != 1 {
		t.Fatalf("batches at 10s = %d, want 1", len(got))
	}
	if newRows := dbTimeBatchRowInts(got[0].newRows); !dbTimeBatchEqualInts(newRows, []int{100, 50, 20}) {
		t.Fatalf("new rows at 10s = %v, want [100 50 20]", newRows)
	}

	// The flushed window is empty, so snapshots stay empty until new beans
	// buffer, and the next cycle still delivers its own rows.
	if rows := snapshotRows(); len(rows) != 0 {
		t.Fatalf("snapshot after flush = %v, want empty", rows)
	}
	send(9)
	if rows := snapshotRows(); !dbTimeBatchEqualInts(rows, []int{90}) {
		t.Fatalf("snapshot after re-buffer = %v, want [90]", rows)
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(20*time.Second)); err != nil {
		t.Fatal(err)
	}
	got = batches()
	if len(got) != 2 {
		t.Fatalf("batches at 20s = %d, want 2", len(got))
	}
	if newRows := dbTimeBatchRowInts(got[1].newRows); !dbTimeBatchEqualInts(newRows, []int{90}) {
		t.Fatalf("new rows at 20s = %v, want [90]", newRows)
	}
	if oldRows := dbTimeBatchRowInts(got[1].oldRows); !dbTimeBatchEqualInts(oldRows, []int{100, 50, 20}) {
		t.Fatalf("old rows at 20s = %v, want [100 50 20]", oldRows)
	}
}

type dbTimeBatchCapturedBatch struct {
	newRows []map[string]any
	oldRows []map[string]any
}

func dbTimeBatchSubscribeBatches(t *testing.T, stmt *Statement) func() []dbTimeBatchCapturedBatch {
	t.Helper()
	var mu sync.Mutex
	var captured []dbTimeBatchCapturedBatch
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		entry := dbTimeBatchCapturedBatch{
			newRows: dbTimeBatchRowMaps(batch.New),
			oldRows: dbTimeBatchRowMaps(batch.Old),
		}
		mu.Lock()
		defer mu.Unlock()
		captured = append(captured, entry)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return func() []dbTimeBatchCapturedBatch {
		mu.Lock()
		defer mu.Unlock()
		return append([]dbTimeBatchCapturedBatch(nil), captured...)
	}
}

func dbTimeBatchRowMaps(results []Result) []map[string]any {
	rows := make([]map[string]any, 0, len(results))
	for _, result := range results {
		if row, ok := result.Row(); ok {
			rows = append(rows, row.AsMap())
		}
	}
	return rows
}

func dbTimeBatchRowInts(rows []map[string]any) []int {
	values := make([]int, 0, len(rows))
	for _, row := range rows {
		if v, ok := dbJoinToInt64(row["myint"]); ok {
			values = append(values, int(v))
		}
	}
	return values
}

func dbTimeBatchEqualInts(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func dbTimeBatchEqualInt64s(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// timeBatchHistoricalProvider mirrors the coordinator repro's function-fed
// provider: it polls rows keyed on the trigger bean's intPrimitive and
// records every lookup key it was called with.
type timeBatchHistoricalProvider struct {
	mu     sync.Mutex
	keys   []int64
	schema Schema
}

func (p *timeBatchHistoricalProvider) Poll(ctx context.Context, request HistoricalRequest) ([]Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var key int64
	if raw := request.Trigger.Get("intPrimitive").Any(); raw != nil {
		if converted, ok := dbJoinToInt64(raw); ok {
			key = converted
		}
	}
	p.mu.Lock()
	p.keys = append(p.keys, key)
	p.mu.Unlock()
	// Only the 1..10 key domain has rows (mirrors the mytesttable fixture);
	// an out-of-domain trigger polls but matches nothing.
	if key < 1 || key > 10 {
		return nil, nil
	}
	event, err := newEvent(p.schema, map[string]any{"mybigint": key, "myint": int(key * 10)}, request.Now)
	if err != nil {
		return nil, err
	}
	return []Event{event}, nil
}

func (p *timeBatchHistoricalProvider) Keys() []int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]int64(nil), p.keys...)
}
