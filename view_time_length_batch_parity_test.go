package esper

import (
	"context"
	"testing"
	"time"
)

func assertTimeLengthIR(t *testing.T, batches *[]ResultBatch, index int, newSyms, oldSyms []string) {
	t.Helper()
	if len(*batches) != index+1 {
		t.Fatalf("batches = %d, want %d", len(*batches), index+1)
	}
	batch := (*batches)[index]
	if len(batch.New) != len(newSyms) || len(batch.Old) != len(oldSyms) {
		t.Fatalf("batch %d = %d new/%d old, want %v/%v", index, len(batch.New), len(batch.Old), newSyms, oldSyms)
	}
	for i, sym := range newSyms {
		if got := batch.New[i].Get("symbol").Any(); got != sym {
			t.Fatalf("batch %d new[%d] = %v, want %s", index, i, got, sym)
		}
	}
	for i, sym := range oldSyms {
		if got := batch.Old[i].Get("symbol").Any(); got != sym {
			t.Fatalf("batch %d old[%d] = %v, want %s", index, i, got, sym)
		}
	}
}

// TestViewTimeLengthBatchSceneOneParity covers ViewTimeLengthBatchSceneOne:
// select irstream * from SupportMarketDataBean#time_length_batch(10 sec, 3).
// The batch flushes when three events accumulate or at the next 10-second
// deadline; the previous batch becomes the old stream, and an empty boundary
// after the old flush goes quiet.
func TestViewTimeLengthBatchSceneOneParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 1000)
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeLengthBatch(10*time.Second, 3))
	_, batches := deployViewMarketData(t, env, engine, source, "s0")

	send := func(sym string) { t.Helper(); sendViewMarketData(t, engine, sym, 0) }

	advanceViewTime(t, engine, 1000)
	send("E1")
	advanceViewTime(t, engine, 5000)
	send("E2")
	advanceViewTime(t, engine, 10999)
	if len(*batches) != 0 {
		t.Fatalf("t=10999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 11000)
	assertTimeLengthIR(t, batches, 0, []string{"E1", "E2"}, nil)

	advanceViewTime(t, engine, 12000)
	send("E3")
	send("E4")
	advanceViewTime(t, engine, 15000)
	send("E5")
	assertTimeLengthIR(t, batches, 1, []string{"E3", "E4", "E5"}, []string{"E1", "E2"})

	advanceViewTime(t, engine, 24999)
	if len(*batches) != 2 {
		t.Fatalf("t=24999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 25000)
	assertTimeLengthIR(t, batches, 2, nil, []string{"E3", "E4", "E5"})

	advanceViewTime(t, engine, 35000)
	if len(*batches) != 3 {
		t.Fatalf("t=35000 invoked listener: %#v", *batches)
	}
}

// TestViewTimeLengthBatchSceneTwoParity covers ViewTimeLengthBatchSceneTwo:
// the full 3-event and timer-driven batch lifecycle with interleaved empty
// boundaries, including re-anchoring after a long quiet gap.
func TestViewTimeLengthBatchSceneTwoParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	const start = 1000
	advanceViewTime(t, engine, start)
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeLengthBatch(10*time.Second, 3))
	_, batches := deployViewMarketData(t, env, engine, source, "s0")

	send := func(sym string) { t.Helper(); sendViewMarketData(t, engine, sym, 0) }

	send("E1")
	send("E2")
	if len(*batches) != 0 {
		t.Fatalf("early arrivals invoked listener: %#v", *batches)
	}
	send("E3")
	assertTimeLengthIR(t, batches, 0, []string{"E1", "E2", "E3"}, nil)

	send("E4")
	send("E5")
	send("E6")
	assertTimeLengthIR(t, batches, 1, []string{"E4", "E5", "E6"}, []string{"E1", "E2", "E3"})

	advanceViewTime(t, engine, start+9999)
	if len(*batches) != 2 {
		t.Fatalf("t=10999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, start+10000)
	assertTimeLengthIR(t, batches, 2, nil, []string{"E4", "E5", "E6"})

	advanceViewTime(t, engine, start+10100)
	send("E7")
	advanceViewTime(t, engine, start+19999)
	if len(*batches) != 3 {
		t.Fatalf("t=20999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, start+20000)
	assertTimeLengthIR(t, batches, 3, []string{"E7"}, nil)

	advanceViewTime(t, engine, start+29998)
	send("E8")
	send("E9")
	advanceViewTime(t, engine, start+30000)
	assertTimeLengthIR(t, batches, 4, []string{"E8", "E9"}, []string{"E7"})

	advanceViewTime(t, engine, start+39000)
	send("E10")
	send("E11")
	if len(*batches) != 5 {
		t.Fatalf("t=40000 invoked listener: %#v", *batches)
	}
	send("E12")
	assertTimeLengthIR(t, batches, 5, []string{"E10", "E11", "E12"}, []string{"E8", "E9"})

	advanceViewTime(t, engine, start+48999)
	send("E13")
	advanceViewTime(t, engine, start+49000)
	assertTimeLengthIR(t, batches, 6, []string{"E13"}, []string{"E10", "E11", "E12"})

	advanceViewTime(t, engine, start+59000)
	assertTimeLengthIR(t, batches, 7, nil, []string{"E13"})

	advanceViewTime(t, engine, start+69000)
	if len(*batches) != 8 {
		t.Fatalf("t=70000 invoked listener: %#v", *batches)
	}

	advanceViewTime(t, engine, start+90000)
	send("E14")
	advanceViewTime(t, engine, start+99999)
	if len(*batches) != 8 {
		t.Fatalf("t=100999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, start+100000)
	assertTimeLengthIR(t, batches, 8, []string{"E14"}, nil)
}

// TestViewTimeLengthBatchForceOutputParity covers ViewTimeLengthBatchForceOutputOne/Two:
// force_update delivers an empty callback at every boundary, including the
// all-empty boundary after the old stream flushes.
func TestViewTimeLengthBatchForceOutputParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 1000)
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeLengthBatchForce(10*time.Second, 3, true, false))
	_, batches := deployViewMarketData(t, env, engine, source, "s0")

	send := func(sym string) { t.Helper(); sendViewMarketData(t, engine, sym, 0) }

	advanceViewTime(t, engine, 1000)
	send("E1")
	advanceViewTime(t, engine, 5000)
	send("E2")
	advanceViewTime(t, engine, 11000)
	assertTimeLengthIR(t, batches, 0, []string{"E1", "E2"}, nil)

	advanceViewTime(t, engine, 12000)
	send("E3")
	send("E4")
	advanceViewTime(t, engine, 15000)
	send("E5")
	assertTimeLengthIR(t, batches, 1, []string{"E3", "E4", "E5"}, []string{"E1", "E2"})

	advanceViewTime(t, engine, 25000)
	assertTimeLengthIR(t, batches, 2, nil, []string{"E3", "E4", "E5"})

	advanceViewTime(t, engine, 35000)
	assertTimeLengthIR(t, batches, 3, nil, nil)
}

// TestViewTimeLengthBatchForceOutputSumParity covers ViewTimeLengthBatchForceOutputSum:
// aggregate sum over force_update reports the batch sum at each boundary and
// null once the batch empties.
func TestViewTimeLengthBatchForceOutputSumParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	const start = 1000
	advanceViewTime(t, engine, start)
	symbol := Field[viewUniqueMarketData, string]("symbol")
	price := Field[viewUniqueMarketData, float64]("price")
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeLengthBatchForce(10*time.Second, 3, true, false))
	plan, err := env.Build(source.Aggregate(Alias("s", Sum[float64](price))).Query(StatementName("s0"), WithOldStream()))
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
	_ = symbol

	sendViewMarketData(t, engine, "E1", 10)
	advanceViewTime(t, engine, start+10000)
	if len(*batches) != 1 || len((*batches)[0].New) != 1 || (*batches)[0].New[0].Get("s").Any() != 10.0 {
		t.Fatalf("t=11000: batch = %#v", (*batches)[0])
	}
	advanceViewTime(t, engine, start+20000)
	if len(*batches) != 2 || len((*batches)[1].New) != 1 {
		t.Fatalf("t=21000: batch = %#v", (*batches)[1])
	}
	if got := (*batches)[1].New[0].Get("s"); got.IsPresent() {
		t.Fatalf("t=21000 sum = %v, want null", got)
	}
}

// TestViewTimeLengthBatchStartEagerParity covers ViewTimeLengthBatchStartEager:
// start_eager seeds the first boundary at deployment time and implies
// force_update, so empty boundaries fire before any event arrives.
func TestViewTimeLengthBatchStartEagerParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	advanceViewTime(t, engine, 1000)
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeLengthBatchForce(10*time.Second, 3, false, true))
	_, batches := deployViewMarketData(t, env, engine, source, "s0")

	send := func(sym string) { t.Helper(); sendViewMarketData(t, engine, sym, 0) }

	advanceViewTime(t, engine, 10999)
	if len(*batches) != 0 {
		t.Fatalf("t=10999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, 11000)
	assertTimeLengthIR(t, batches, 0, nil, nil)

	advanceViewTime(t, engine, 21000)
	assertTimeLengthIR(t, batches, 1, nil, nil)

	advanceViewTime(t, engine, 22000)
	send("E1")
	send("E2")
	advanceViewTime(t, engine, 25000)
	send("E3")
	assertTimeLengthIR(t, batches, 2, []string{"E1", "E2", "E3"}, nil)

	advanceViewTime(t, engine, 35000)
	assertTimeLengthIR(t, batches, 3, nil, []string{"E1", "E2", "E3"})

	advanceViewTime(t, engine, 44999)
	send("E4")
	advanceViewTime(t, engine, 45000)
	assertTimeLengthIR(t, batches, 4, []string{"E4"}, nil)
}

// TestViewTimeLengthBatchForceOutputStartEagerSumParity covers
// ViewTimeLengthBatchForceOutputStartEagerSum: force_update + start_eager
// fires empty batches from deployment, then the accumulated sum at the first
// boundary after events arrive.
func TestViewTimeLengthBatchForceOutputStartEagerSumParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	const start = 1000
	advanceViewTime(t, engine, start)
	price := Field[viewUniqueMarketData, float64]("price")
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeLengthBatchForce(10*time.Second, 3, true, true))
	plan, err := env.Build(source.Aggregate(Alias("s", Sum[float64](price))).Query(StatementName("s0"), WithOldStream()))
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

	advanceViewTime(t, engine, start+9999)
	if len(*batches) != 0 {
		t.Fatalf("t=10999 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, start+10000)
	if len(*batches) != 1 || len((*batches)[0].New) != 1 || (*batches)[0].New[0].Get("s").IsPresent() {
		t.Fatalf("t=11000: batch = %#v", (*batches)[0])
	}

	advanceViewTime(t, engine, start+20000)
	if len(*batches) != 2 || len((*batches)[1].New) != 1 || (*batches)[1].New[0].Get("s").IsPresent() {
		t.Fatalf("t=21000: batch = %#v", (*batches)[1])
	}

	sendViewMarketData(t, engine, "E11", 11)
	sendViewMarketData(t, engine, "E12", 12)
	advanceViewTime(t, engine, start+30000)
	if len(*batches) != 3 || len((*batches)[2].New) != 1 || (*batches)[2].New[0].Get("s").Any() != 23.0 {
		t.Fatalf("t=31000: batch = %#v", (*batches)[2])
	}
}

// TestViewTimeLengthBatchForceOutputStartNoEagerSumParity covers
// ViewTimeLengthBatchForceOutputStartNoEagerSum: force_update without
// start_eager does not fire before the first event arms the batch.
func TestViewTimeLengthBatchForceOutputStartNoEagerSumParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	const start = 1000
	advanceViewTime(t, engine, start)
	price := Field[viewUniqueMarketData, float64]("price")
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeLengthBatchForce(10*time.Second, 3, true, false))
	plan, err := env.Build(source.Aggregate(Alias("s", Sum[float64](price))).Query(StatementName("s0"), WithOldStream()))
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

	advanceViewTime(t, engine, start+10000)
	advanceViewTime(t, engine, start+20000)
	if len(*batches) != 0 {
		t.Fatalf("force-update without start_eager fired: %#v", *batches)
	}
}

// TestViewTimeLengthBatchPrevPriorParity covers ViewTimeLengthBatchPreviousAndPrior:
// prev(1)/prior(1) inside the completed batch prefix evaluate per row.
func TestViewTimeLengthBatchPrevPriorParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	const start = 1000
	advanceViewTime(t, engine, start)
	price := Field[viewUniqueMarketData, float64]("price")
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeLengthBatch(10*time.Second, 3))
	plan, err := env.Build(Select(source,
		Alias("price", price),
		Alias("prevPrice", Prev[float64](1, price)),
		Alias("priorPrice", Prior[float64](0, price)),
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

	sendViewMarketData(t, engine, "E0", 0)
	sendViewMarketData(t, engine, "E1", 1)
	if len(*batches) != 0 {
		t.Fatalf("early arrivals invoked listener: %#v", *batches)
	}
	sendViewMarketData(t, engine, "E2", 2)
	if len(*batches) != 1 || len((*batches)[0].New) != 3 {
		t.Fatalf("batch = %#v", (*batches)[0])
	}
	rows := (*batches)[0].New
	if rows[0].Get("price").Any() != 0.0 || rows[0].Get("prevPrice").IsPresent() || rows[0].Get("priorPrice").IsPresent() {
		t.Fatalf("row0 = %#v", rows[0])
	}
	if rows[1].Get("price").Any() != 1.0 || rows[1].Get("prevPrice").Any() != 0.0 || rows[1].Get("priorPrice").Any() != 0.0 {
		t.Fatalf("row1 = %#v", rows[1])
	}
	if rows[2].Get("price").Any() != 2.0 || rows[2].Get("prevPrice").Any() != 1.0 || rows[2].Get("priorPrice").Any() != 1.0 {
		t.Fatalf("row2 = %#v", rows[2])
	}
}

// TestViewTimeLengthBatchGroupByStartEagerParity covers
// ViewTimeLengthBatchGroupBySumStartEager: group-by sums flush at the seeded
// boundary ordered by symbol.
func TestViewTimeLengthBatchGroupByStartEagerParity(t *testing.T) {
	env, engine := newViewMarketDataEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	const start = 1000
	advanceViewTime(t, engine, start)
	symbol := Field[viewUniqueMarketData, string]("symbol")
	price := Field[viewUniqueMarketData, float64]("price")
	source := From[viewUniqueMarketData](env, "SupportMarketDataBean").Window(TimeLengthBatchForce(5*time.Second, 10, false, true))
	plan, err := env.Build(source.GroupBy(symbol).Select(
		Alias("symbol", symbol),
		Alias("s", Sum[float64](price)),
	).Query(StatementName("s0"), WithOldStream(), OrderBy(Ascending(symbol))))
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

	advanceViewTime(t, engine, start+4000)
	if len(*batches) != 0 {
		t.Fatalf("t=5000 invoked listener: %#v", *batches)
	}
	advanceViewTime(t, engine, start+6000)
	if len(*batches) != 1 {
		t.Fatalf("t=7000: batches = %d, want 1", len(*batches))
	}

	advanceViewTime(t, engine, start+7000)
	sendViewMarketData(t, engine, "S1", 10)
	advanceViewTime(t, engine, start+8000)
	sendViewMarketData(t, engine, "S2", 77)
	advanceViewTime(t, engine, start+9000)
	sendViewMarketData(t, engine, "S1", 1)
	advanceViewTime(t, engine, start+10000)
	if len(*batches) != 1 {
		t.Fatalf("t=11000: batches = %d, want 1", len(*batches))
	}
	advanceViewTime(t, engine, start+11000)
	if len(*batches) != 2 || len((*batches)[1].New) != 2 {
		t.Fatalf("t=12000: batch = %#v", (*batches)[1])
	}
	rows := (*batches)[1].New
	if rows[0].Get("symbol").Any() != "S1" || rows[0].Get("s").Any() != 11.0 {
		t.Fatalf("S1 row = %#v", rows[0])
	}
	if rows[1].Get("symbol").Any() != "S2" || rows[1].Get("s").Any() != 77.0 {
		t.Fatalf("S2 row = %#v", rows[1])
	}
}
