package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

// TestViewExternallyTimedWindowSceneOneParity covers ViewExternallyTimedWindowSceneOne:
// ext_timed(volume, 1 sec) expires rows by the external timestamp; each new
// event carries the external volume timestamp and old rows are evicted when
// the event timestamp passes expiry.
func TestViewExternallyTimedWindowSceneOneParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	volume := Field[viewUniqueMarketData, int64]("volume")
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(ExternallyTimed(volume, time.Second))
	stmt, batches := deployViewMarketData(t, env, engine, source, "s0")

	sendVol := func(sym string, vol int64) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportMarketDataBean", viewUniqueMarketData{Symbol: sym, Volume: vol}); err != nil {
			t.Fatal(err)
		}
	}

	// E1@500, E2@600
	sendVol("E1", 500)
	if len(*batches) != 1 || len((*batches)[0].New) != 1 || len((*batches)[0].Old) != 0 {
		t.Fatalf("E1: %#v", *batches)
	}
	sendVol("E2", 600)
	if len(*batches) != 2 || len((*batches)[1].New) != 1 || len((*batches)[1].Old) != 0 {
		t.Fatalf("E2: %#v", *batches)
	}

	// E3@1500 expires E1.
	sendVol("E3", 1500)
	if len(*batches) != 3 || len((*batches)[2].Old) != 1 || (*batches)[2].Old[0].Get("symbol").Any() != "E1" {
		t.Fatalf("E3: %#v", *batches)
	}
	// E4@1600 expires E2.
	sendVol("E4", 1600)
	if len(*batches) != 4 || len((*batches)[3].Old) != 1 || (*batches)[3].Old[0].Get("symbol").Any() != "E2" {
		t.Fatalf("E4: %#v", *batches)
	}
	// E5-E7 stay within the window.
	sendVol("E5", 1700)
	sendVol("E6", 1800)
	sendVol("E7", 1900)
	if got := snapshotSymbols(t, stmt); !reflect.DeepEqual(got, []string{"E3", "E4", "E5", "E6", "E7"}) {
		t.Fatalf("iterator = %v", got)
	}

	// E8@2700 expires E3/E4/E5.
	sendVol("E8", 2700)
	if len(*batches) != 8 || len((*batches)[7].Old) != 3 {
		t.Fatalf("E8: %#v", *batches)
	}
	for i, sym := range []string{"E3", "E4", "E5"} {
		if got := (*batches)[7].Old[i].Get("symbol").Any(); got != sym {
			t.Fatalf("E8 old[%d] = %v", i, got)
		}
	}
	// E9@3700 expires E6/E7/E8.
	sendVol("E9", 3700)
	if len(*batches) != 9 || len((*batches)[8].Old) != 3 {
		t.Fatalf("E9: %#v", *batches)
	}
}

// TestViewExternallyTimedWindowSceneTwoParity covers ViewExternallyTimedBatchSceneTwo
// (sliding ext_timed over SupportBean longPrimitive): rows expire by external
// timestamp in insertion-driven batches.
func TestViewExternallyTimedWindowSceneTwoParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	long := Field[viewParityBean, int64]("longPrimitive")
	source := From[viewParityBean](env, "SupportBean").Window(ExternallyTimed(long, 10*time.Second))
	plan, err := env.Build(Select(source, Alias("c0", Field[viewParityBean, string]("theString"))).Query(StatementName("s0"), WithOldStream()))
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
	sendBeanLong := func(name string, long int64) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBean", viewParityBean{TheString: name, LongPrimitive: long}); err != nil {
			t.Fatal(err)
		}
	}

	sendBeanLong("E1", 1000)
	if len(*batches) != 1 || (*batches)[0].New[0].Get("c0").Any() != "E1" {
		t.Fatalf("E1: %#v", *batches)
	}
	sendBeanLong("E2", 5000)
	if len(*batches) != 2 || (*batches)[1].New[0].Get("c0").Any() != "E2" {
		t.Fatalf("E2: %#v", *batches)
	}
	// E3@11000 expires E1.
	sendBeanLong("E3", 11000)
	if len(*batches) != 3 || len((*batches)[2].New) != 1 || len((*batches)[2].Old) != 1 || (*batches)[2].Old[0].Get("c0").Any() != "E1" {
		t.Fatalf("E3: %#v", *batches)
	}
	sendBeanLong("E4", 14000)
	if len(*batches) != 4 || (*batches)[3].New[0].Get("c0").Any() != "E4" {
		t.Fatalf("E4: %#v", *batches)
	}
	// E5@21000 expires E2/E3.
	sendBeanLong("E5", 21000)
	if len(*batches) != 5 || len((*batches)[4].Old) != 2 {
		t.Fatalf("E5: %#v", *batches)
	}
	// E6@24000 expires E4.
	sendBeanLong("E6", 24000)
	if len(*batches) != 6 || len((*batches)[5].Old) != 1 || (*batches)[5].Old[0].Get("c0").Any() != "E4" {
		t.Fatalf("E6: %#v", *batches)
	}
}

// TestViewExternallyTimedWindowShortParity covers ViewExternallyTimedWinSceneShort:
// ext_timed(longPrimitive, 10 minutes) with boundary-timestamp expiry checks.
func TestViewExternallyTimedWindowShortParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	long := Field[viewParityBean, int64]("longPrimitive")
	source := From[viewParityBean](env, "SupportBean").Window(ExternallyTimed(long, 10*time.Minute))
	_, batches := deployViewParity(t, env, engine, source, "s0")

	sendBeanLong := func(name string, long int64) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBean", viewParityBean{TheString: name, LongPrimitive: long}); err != nil {
			t.Fatal(err)
		}
	}

	sendBeanLong("E0", 0)
	// At 599999 the window still holds E0 (expiry 600000).
	sendBeanLong("E1", 600000-1)
	if len(*batches) != 2 || len((*batches)[1].Old) != 0 {
		t.Fatalf("E1 before expiry: %#v", *batches)
	}
	// At 600001 E0 expires.
	sendBeanLong("E2", 600001)
	if len(*batches) != 3 || len((*batches)[2].Old) != 1 || (*batches)[2].Old[0].Get("theString").Any() != "E0" {
		t.Fatalf("E2 at expiry: %#v", *batches)
	}
}

// TestViewExternallyTimedWindowMonthScopedParity covers ViewExternallyTimedTimedMonthScoped:
// ext_timed(longPrimitive, 1 month) with calendar-scoped expiry (rstream).
func TestViewExternallyTimedWindowMonthScopedParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	long := Field[viewParityBean, int64]("longPrimitive")
	source := From[viewParityBean](env, "SupportBean").Window(ExternallyTimedCalendar(long, 0, 1, 0))
	plan, err := env.Build(source.Query(StatementName("s0"), WithRemoveStreamOnly()))
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
	sendBeanLong := func(name string, long int64) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBean", viewParityBean{TheString: name, LongPrimitive: long}); err != nil {
			t.Fatal(err)
		}
	}

	feb := time.Date(2002, 2, 1, 9, 0, 0, 0, time.UTC).UnixMilli()
	mar := time.Date(2002, 3, 1, 9, 0, 0, 0, time.UTC).UnixMilli()
	sendBeanLong("E1", feb)
	sendBeanLong("E2", mar-1)
	if len(*batches) != 0 {
		t.Fatalf("arrivals before month expiry invoked listener: %#v", *batches)
	}
	// E3@Mar-1 expires E1 (rstream as new).
	sendBeanLong("E3", mar)
	if len(*batches) != 1 || len((*batches)[0].New) != 1 || (*batches)[0].New[0].Get("theString").Any() != "E1" {
		t.Fatalf("E3: %#v", *batches)
	}
}

// TestViewExternallyTimedWindowPrevParity covers ViewExternallyTimedWindowPrev:
// prev/prevtail/prevcount/prevwindow over the external-timestamp window.
func TestViewExternallyTimedWindowPrevParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	symbol := Field[viewUniqueMarketData, string]("symbol")
	volume := Field[viewUniqueMarketData, int64]("volume")
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(ExternallyTimed(volume, time.Second))
	plan, err := env.Build(Select(source,
		Alias("symbol", symbol),
		Alias("prev1", Prev[string](1, symbol)),
		Alias("prevTail0", PrevTail[string](0, symbol)),
		Alias("prevTail1", PrevTail[string](1, symbol)),
		Alias("prevCountSym", PrevCount[string](symbol)),
		Alias("prevWindowSym", PrevWindow[string](symbol)),
	).Query(StatementName("s0"), WithOldStream()))
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
	sendVol := func(sym string, vol int64) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportMarketDataBean", viewUniqueMarketData{Symbol: sym, Volume: vol}); err != nil {
			t.Fatal(err)
		}
	}

	// E1@500: prev1 null, prevTail0=E1, count 1, window [E1].
	sendVol("E1", 500)
	row := (*batches)[0].New[0]
	if row.Get("prev1").IsPresent() || row.Get("prevTail0").Any() != "E1" || row.Get("prevCountSym").Any() != int64(1) {
		t.Fatalf("E1 row = %#v", row)
	}
	if got := row.Get("prevWindowSym").Any(); !reflect.DeepEqual(got, []string{"E1"}) {
		t.Fatalf("E1 prevWindow = %#v", got)
	}

	// E2@600: prev1=E1, prevTail0=E1, prevTail1=E2, count 2, window [E2 E1].
	sendVol("E2", 600)
	row = (*batches)[1].New[0]
	if row.Get("prev1").Any() != "E1" || row.Get("prevTail0").Any() != "E1" || row.Get("prevTail1").Any() != "E2" || row.Get("prevCountSym").Any() != int64(2) {
		t.Fatalf("E2 row = %#v", row)
	}
	if got := row.Get("prevWindowSym").Any(); !reflect.DeepEqual(got, []string{"E2", "E1"}) {
		t.Fatalf("E2 prevWindow = %#v", got)
	}

	// E3@1500 expires E1: prev1=E2, prevTail0=E2, prevTail1=E3, count 2, window [E3 E2].
	sendVol("E3", 1500)
	row = (*batches)[2].New[0]
	if row.Get("prev1").Any() != "E2" || row.Get("prevTail0").Any() != "E2" || row.Get("prevTail1").Any() != "E3" || row.Get("prevCountSym").Any() != int64(2) {
		t.Fatalf("E3 row = %#v", row)
	}
	if got := row.Get("prevWindowSym").Any(); !reflect.DeepEqual(got, []string{"E3", "E2"}) {
		t.Fatalf("E3 prevWindow = %#v", got)
	}
}

// TestViewExternallyTimedBatchSceneOneParity covers ViewExternallyTimedBatchSceneOne:
// ext_timed_batch(longPrimitive, 10 sec) flushes at anchored 10-second
// boundaries derived from the first event timestamp.
func TestViewExternallyTimedBatchSceneOneParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	long := Field[viewParityBean, int64]("longPrimitive")
	source := From[viewParityBean](env, "SupportBean").Window(ExternallyTimedBatch(long, 10*time.Second))
	plan, err := env.Build(Select(source, Alias("c0", Field[viewParityBean, string]("theString"))).Query(StatementName("s0"), WithOldStream()))
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
	sendBeanLong := func(name string, long int64) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBean", viewParityBean{TheString: name, LongPrimitive: long}); err != nil {
			t.Fatal(err)
		}
	}

	// E1@1000 anchors; E2@5000 pending.
	sendBeanLong("E1", 1000)
	if len(*batches) != 0 {
		t.Fatalf("E1 invoked listener: %#v", *batches)
	}
	sendBeanLong("E2", 5000)
	if len(*batches) != 0 {
		t.Fatalf("E2 invoked listener: %#v", *batches)
	}
	// E3@11000 crosses the boundary: new=[E1,E2].
	sendBeanLong("E3", 11000)
	if len(*batches) != 1 || len((*batches)[0].New) != 2 || len((*batches)[0].Old) != 0 {
		t.Fatalf("E3: %#v", *batches)
	}
	// E4@0 out-of-order pending; E5@21000 flushes new=[E3,E4], old=[E1,E2].
	sendBeanLong("E4", 0)
	if len(*batches) != 1 {
		t.Fatalf("E4 invoked listener: %#v", *batches)
	}
	sendBeanLong("E5", 21000)
	if len(*batches) != 2 || len((*batches)[1].New) != 2 || len((*batches)[1].Old) != 2 {
		t.Fatalf("E5: %#v", *batches)
	}
	// E6@31000 flushes new=[E5], old=[E3,E4]; E7@41000 flushes new=[E6], old=[E5].
	sendBeanLong("E6", 31000)
	if len(*batches) != 3 || len((*batches)[2].New) != 1 || (*batches)[2].New[0].Get("c0").Any() != "E5" || len((*batches)[2].Old) != 2 {
		t.Fatalf("E6: %#v", *batches)
	}
	sendBeanLong("E7", 41000)
	if len(*batches) != 4 || len((*batches)[3].New) != 1 || (*batches)[3].New[0].Get("c0").Any() != "E6" || len((*batches)[3].Old) != 1 {
		t.Fatalf("E7: %#v", *batches)
	}
}

// TestViewExternallyTimedBatchNoReferenceParity covers ViewExternallyTimedBatchedNoReference:
// ext_timed_batch without reference anchors at the first event and flushes at
// each anchored boundary, with out-of-order events accumulating.
func TestViewExternallyTimedBatchNoReferenceParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	long := Field[viewParityBean, int64]("longPrimitive")
	source := From[viewParityBean](env, "SupportBean").Window(ExternallyTimedBatch(long, time.Minute))
	plan, err := env.Build(Select(source, Alias("c0", Field[viewParityBean, string]("theString"))).Query(StatementName("s0"), WithOldStream()))
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
	sendBeanLong := func(name string, long int64) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBean", viewParityBean{TheString: name, LongPrimitive: long}); err != nil {
			t.Fatal(err)
		}
	}

	base := time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC).UnixMilli()
	ms := func(h, m, s int) int64 { return time.Date(2000, 1, 1, h, m, s, 0, time.UTC).UnixMilli() }
	_ = base
	_ = ms

	// E1 8:00:00 anchors; E2/E3 accumulate; E4 8:01:00 flushes E1-E3.
	sendBeanLong("E1", ms(8, 0, 0))
	sendBeanLong("E2", ms(8, 0, 30))
	sendBeanLong("E3", ms(8, 0, 59))
	if len(*batches) != 0 {
		t.Fatalf("early arrivals invoked listener: %#v", *batches)
	}
	sendBeanLong("E4", ms(8, 1, 0))
	if len(*batches) != 1 || len((*batches)[0].New) != 3 || len((*batches)[0].Old) != 0 {
		t.Fatalf("E4: %#v", *batches)
	}
	// E5/E6 pending; E7 8:02:00 flushes E4-E6, old E1-E3.
	sendBeanLong("E5", ms(8, 1, 2))
	sendBeanLong("E6", ms(8, 1, 5))
	sendBeanLong("E7", ms(8, 2, 0))
	if len(*batches) != 2 || len((*batches)[1].New) != 3 || len((*batches)[1].Old) != 3 {
		t.Fatalf("E7: %#v", *batches)
	}
	// E8 8:03:59 flushes E7, old E4-E6; E9 same ts pending; E10 8:04:00 flushes E8/E9, old E7.
	sendBeanLong("E8", ms(8, 3, 59))
	if len(*batches) != 3 || len((*batches)[2].New) != 1 || (*batches)[2].New[0].Get("c0").Any() != "E7" || len((*batches)[2].Old) != 3 {
		t.Fatalf("E8: %#v", *batches)
	}
	sendBeanLong("E9", ms(8, 3, 59))
	if len(*batches) != 3 {
		t.Fatalf("E9 invoked listener: %#v", *batches)
	}
	sendBeanLong("E10", ms(8, 4, 0))
	if len(*batches) != 4 || len((*batches)[3].New) != 2 || len((*batches)[3].Old) != 1 {
		t.Fatalf("E10: %#v", *batches)
	}
	// E11 8:06:30 flushes E10, old E8/E9; E12 pending; E13 8:07:00.001 flushes E11/E12, old E10.
	sendBeanLong("E11", ms(8, 6, 30))
	if len(*batches) != 5 || len((*batches)[4].New) != 1 || (*batches)[4].New[0].Get("c0").Any() != "E10" || len((*batches)[4].Old) != 2 {
		t.Fatalf("E11: %#v", *batches)
	}
	sendBeanLong("E12", ms(8, 6, 59))
	if len(*batches) != 5 {
		t.Fatalf("E12 invoked listener: %#v", *batches)
	}
	sendBeanLong("E13", ms(8, 7, 0)+1)
	if len(*batches) != 6 || len((*batches)[5].New) != 2 || len((*batches)[5].Old) != 1 {
		t.Fatalf("E13: %#v", *batches)
	}
	if got := (*batches)[5].New[0].Get("c0").Any(); got != "E11" {
		t.Fatalf("E13 new[0] = %v, want E11", got)
	}
	if got := (*batches)[5].New[1].Get("c0").Any(); got != "E12" {
		t.Fatalf("E13 new[1] = %v, want E12", got)
	}
}

// TestViewExternallyTimedBatchWithRefParity covers ViewExternallyTimedBatchedWithRefTime:
// ext_timed_batch with an explicit reference point uses reference-anchored
// boundaries regardless of event timestamps.
func TestViewExternallyTimedBatchWithRefParity(t *testing.T) {
	env, engine := newViewParityEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	long := Field[viewParityBean, int64]("longPrimitive")
	source := From[viewParityBean](env, "SupportBean").Window(ExternallyTimedBatchWithReference(long, time.Minute, 5000))
	plan, err := env.Build(Select(source, Alias("c0", Field[viewParityBean, string]("theString"))).Query(StatementName("s0"), WithOldStream()))
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
	sendBeanLong := func(name string, long int64) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportBean", viewParityBean{TheString: name, LongPrimitive: long}); err != nil {
			t.Fatal(err)
		}
	}
	ms := func(h, m, s int) int64 { return time.Date(2000, 1, 1, h, m, s, 0, time.UTC).UnixMilli() }

	// Reference 5000 => boundaries at ...5000/65000/125000... E1/E2 before 65000 pending.
	sendBeanLong("E1", ms(8, 0, 0))
	sendBeanLong("E2", ms(8, 0, 4))
	if len(*batches) != 0 {
		t.Fatalf("E1/E2 invoked listener: %#v", *batches)
	}
	// E3 8:00:05 crosses boundary 65000: new=[E1,E2].
	sendBeanLong("E3", ms(8, 0, 5))
	if len(*batches) != 1 || len((*batches)[0].New) != 2 || len((*batches)[0].Old) != 0 {
		t.Fatalf("E3: %#v", *batches)
	}
	// Out-of-order E4/E5/E6 pending; E7 8:01:05 crosses: new=[E3,E4,E5,E6], old=[E1,E2].
	sendBeanLong("E4", ms(8, 0, 4))
	sendBeanLong("E5", ms(7, 0, 0))
	sendBeanLong("E6", ms(8, 1, 4))
	sendBeanLong("E7", ms(8, 1, 5))
	if len(*batches) != 2 || len((*batches)[1].New) != 4 || len((*batches)[1].Old) != 2 {
		t.Fatalf("E7: %#v", *batches)
	}
	// E8 8:03:55 crosses 125000? Actually 8:03:55 = 125000+... check boundary 185000.
	sendBeanLong("E8", ms(8, 3, 55))
	if len(*batches) != 3 || len((*batches)[2].New) != 1 || (*batches)[2].New[0].Get("c0").Any() != "E7" || len((*batches)[2].Old) != 4 {
		t.Fatalf("E8: %#v", *batches)
	}
	// E9 0:00 + E10 8:04:04 pending; E11 8:04:05 crosses 245000: new=[E8,E9,E10], old=[E7].
	sendBeanLong("E9", ms(0, 0, 0))
	sendBeanLong("E10", ms(8, 4, 4))
	sendBeanLong("E11", ms(8, 4, 5))
	if len(*batches) != 4 || len((*batches)[3].New) != 3 || len((*batches)[3].Old) != 1 {
		t.Fatalf("E11: %#v", *batches)
	}
}

// TestViewExternallyTimedBatchRefWithPrevParity covers ViewExternallyTimedBatchRefWithPrev:
// prev-family projections over an ext_timed_batch flush, with per-row batch
// prefix semantics.
func TestViewExternallyTimedBatchRefWithPrevParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	symbol := Field[viewUniqueMarketData, string]("symbol")
	price := Field[viewUniqueMarketData, float64]("price")
	volume := Field[viewUniqueMarketData, int64]("volume")
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(ExternallyTimedBatchWithReference(volume, 10*time.Second, 0))
	plan, err := env.Build(Select(source,
		Alias("currSymbol", symbol),
		Alias("prev0Symbol", Prev[string](0, symbol)),
		Alias("prev0Price", Prev[float64](0, price)),
		Alias("prev1Symbol", Prev[string](1, symbol)),
		Alias("prev1Price", Prev[float64](1, price)),
		Alias("prev2Symbol", Prev[string](2, symbol)),
		Alias("prev2Price", Prev[float64](2, price)),
		Alias("prevTail0Symbol", PrevTail[string](0, symbol)),
		Alias("prevTail0Price", PrevTail[float64](0, price)),
		Alias("prevTail1Symbol", PrevTail[string](1, symbol)),
		Alias("prevTail1Price", PrevTail[float64](1, price)),
		Alias("prevCountPrice", PrevCount[float64](price)),
		Alias("prevWindowPrice", PrevWindow[float64](price)),
	).Query(StatementName("s0"), WithOldStream()))
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
	sendVol := func(sym string, p float64, vol int64) {
		t.Helper()
		if err := engine.Send(context.Background(), "SupportMarketDataBean", viewUniqueMarketData{Symbol: sym, Price: p, Volume: vol}); err != nil {
			t.Fatal(err)
		}
	}

	sendVol("A", 1, 1000)
	sendVol("B", 2, 1001)
	sendVol("C", 3, 1002)
	if len(*batches) != 0 {
		t.Fatalf("early arrivals invoked listener: %#v", *batches)
	}
	// D@10000 crosses boundary 0+10s: flush A/B/C as new (reference 0, batch 10 seconds).
	sendVol("D", 4, 10000)
	if len(*batches) != 1 || len((*batches)[0].New) != 3 {
		t.Fatalf("D: %#v", *batches)
	}
	rows := (*batches)[0].New
	if rows[0].Get("currSymbol").Any() != "A" || rows[0].Get("prev0Symbol").Any() != "A" || rows[0].Get("prev0Price").Any() != 1.0 {
		t.Fatalf("A row = %#v", rows[0])
	}
	if rows[1].Get("currSymbol").Any() != "B" || rows[1].Get("prev1Symbol").Any() != "A" || rows[1].Get("prev1Price").Any() != 1.0 {
		t.Fatalf("B row = %#v", rows[1])
	}
	if rows[2].Get("currSymbol").Any() != "C" || rows[2].Get("prev2Symbol").Any() != "A" || rows[2].Get("prev2Price").Any() != 1.0 {
		t.Fatalf("C row = %#v", rows[2])
	}
	if got := rows[0].Get("prevWindowPrice").Any(); !reflect.DeepEqual(got, []float64{1.0}) {
		t.Fatalf("A prevWindow = %#v", got)
	}
	if got := rows[2].Get("prevWindowPrice").Any(); !reflect.DeepEqual(got, []float64{3.0, 2.0, 1.0}) {
		t.Fatalf("C prevWindow = %#v", got)
	}
}
