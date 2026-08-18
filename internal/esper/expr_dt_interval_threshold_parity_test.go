package esper

import (
	"context"
	"math"
	"testing"
)

// Parity coverage for the threshold-form date-time interval executions of
// ExprDTIntervalOps:
//
// - ExprDTIntervalBeforeWVariable: before(b, somenumber) with a variable
//   threshold (Go binds the constant directly, like the variable case in
//   filter-val slices).
// - ExprDTIntervalBeforeInSelectClause: before(...) projected as select
//   columns rather than a where-filter.
// - ExprDTIntervalBeforeWhereClauseWithBean: before(b, 1 millisecond) and
//   before(b, 1 millisecond, 1000000000L) threshold forms.
// - ExprDTIntervalTimePeriodWYearNonConst: before(b, 1 <unit>) across all
//   time units, all false at identical instants except microseconds (whose
//   0.001ms threshold admits delta 0).
//
// Expected values cross-checked against the Java oracle trace
// testdata/parity/dt-interval-ops-threshold.trace.json (24 records, zero
// mismatch).

const dtThresholdSeed int64 = 1022749200000 // 2002-05-30T09:00:00.000 UTC

// TestExprDTIntervalThresholdParity covers the threshold (delta-range)
// forms of before/after, including the swapped-range and negative-range
// variants, using the same A/B sequences as the Java oracle scenario.
func TestExprDTIntervalThresholdParity(t *testing.T) {
	cases := []struct {
		name string
		op   IntervalComputer
		b    dtIntervalEvent
		send []dtIntervalEvent
		want []bool
	}{
		{"before-w-variable", BeforeThreshold(1, math.MaxInt64), dtIntervalEvent{dtThresholdSeed, dtThresholdSeed},
			[]dtIntervalEvent{
				{dtThresholdSeed, dtThresholdSeed},
				{dtThresholdSeed - 1, dtThresholdSeed - 1},
				{dtThresholdSeed - 1000, dtThresholdSeed - 1000},
			}, []bool{false, true, true}},
		{"before-where-with-bean", BeforeThreshold(1, math.MaxInt64), dtIntervalEvent{dtThresholdSeed, dtThresholdSeed},
			[]dtIntervalEvent{
				{dtThresholdSeed - 1000, dtThresholdSeed - 1000},
				{dtThresholdSeed - 1, dtThresholdSeed - 1},
				{dtThresholdSeed, dtThresholdSeed},
				{dtThresholdSeed + 1, dtThresholdSeed + 1},
			}, []bool{true, true, false, false}},
		{"before-two-thresholds", BeforeThreshold(1, 1000000000), dtIntervalEvent{dtThresholdSeed, dtThresholdSeed},
			[]dtIntervalEvent{
				{dtThresholdSeed - 1000, dtThresholdSeed - 1000},
				{dtThresholdSeed - 1, dtThresholdSeed - 1},
				{dtThresholdSeed, dtThresholdSeed},
				{dtThresholdSeed + 1, dtThresholdSeed + 1},
			}, []bool{true, true, false, false}},
		{"before-swapped-thresholds", BeforeThreshold(1000000000, 1), dtIntervalEvent{dtThresholdSeed, dtThresholdSeed},
			[]dtIntervalEvent{
				{dtThresholdSeed - 1, dtThresholdSeed - 1},
				{dtThresholdSeed, dtThresholdSeed},
			}, []bool{true, false}},
		{"after-threshold", AfterThreshold(1, 1000000000), dtIntervalEvent{dtThresholdSeed, dtThresholdSeed + 1000},
			[]dtIntervalEvent{
				{dtThresholdSeed + 1000, dtThresholdSeed + 1500},
				{dtThresholdSeed + 1001, dtThresholdSeed + 1500},
			}, []bool{false, true}},
		// after(b, -100, -500): Java swaps to [-500,-100]; delta = ls-re must
		// fall within [S+500, S+900] given re = S+1000.
		{"after-negative-thresholds", AfterThreshold(-100, -500), dtIntervalEvent{dtThresholdSeed, dtThresholdSeed + 1000},
			[]dtIntervalEvent{
				{dtThresholdSeed + 499, dtThresholdSeed + 600},
				{dtThresholdSeed + 500, dtThresholdSeed + 600},
				{dtThresholdSeed + 900, dtThresholdSeed + 950},
				{dtThresholdSeed + 901, dtThresholdSeed + 950},
			}, []bool{false, true, true, false}},
	}

	env := newDTIntervalEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pred := Interval(tc.op,
				IntervalBounds{Start: JoinField[int64](0, "st"), End: JoinField[int64](0, "en")},
				IntervalBounds{Start: JoinField[int64](1, "st"), End: JoinField[int64](1, "en")},
			)
			plan, err := env.Build(
				JoinMany(
					JoinSource(From[dtIntervalEvent](env, "A")).Window(LengthWindow(1)),
					JoinSource(From[dtIntervalEvent](env, "B")).Window(LengthWindow(1)),
				).Select(
					SelectFrom(0, "a_st", JoinField[int64](0, "st")),
				).Where(pred).Query(StatementName("s0")),
			)
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = deployment.Undeploy(context.Background()) }()
			fired := subscribeRebool(t, deployment, "s0")

			if err := engine.Send(context.Background(), "B", tc.b); err != nil {
				t.Fatal(err)
			}
			_ = fired() // drain B-only send

			for i, ev := range tc.send {
				if err := engine.Send(context.Background(), "A", ev); err != nil {
					t.Fatal(err)
				}
				if got := fired(); got != tc.want[i] {
					t.Fatalf("%s step %d (a=%d,b=%d): fired=%v want %v", tc.name, i, ev.Start, ev.End, got, tc.want[i])
				}
			}
		})
	}
}

// TestExprDTIntervalBeforeInSelectClauseParity covers
// ExprDTIntervalBeforeInSelectClause: the interval comparison projected as
// select columns (c0/c1) instead of a where-filter. Both columns carry the
// same before(b) result per Java's assertPropsAllValuesSame.
func TestExprDTIntervalBeforeInSelectClauseParity(t *testing.T) {
	env := newDTIntervalEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	pred := Interval(Before,
		IntervalBounds{Start: JoinField[int64](0, "st"), End: JoinField[int64](0, "en")},
		IntervalBounds{Start: JoinField[int64](1, "st"), End: JoinField[int64](1, "en")},
	)
	plan, err := env.Build(
		JoinMany(
			JoinSource(From[dtIntervalEvent](env, "A")).Window(LengthWindow(1)),
			JoinSource(From[dtIntervalEvent](env, "B")).Window(LengthWindow(1)),
		).Select(
			SelectFrom(0, "c0", pred),
			SelectFrom(0, "c1", pred),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	var last map[string]any
	statement, ok := deployment.Statement("s0")
	if !ok {
		t.Fatal("statement s0 not found")
	}
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			row, _ := batch.New[len(batch.New)-1].Row()
			c0 := row.Get("c0")
			c1 := row.Get("c1")
			last = map[string]any{"c0": c0.Any(), "c1": c1.Any()}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.Send(context.Background(), "B", dtIntervalEvent{dtThresholdSeed, dtThresholdSeed}); err != nil {
		t.Fatal(err)
	}
	// Java: A1=08:59:59.000 -> both true; A2=08:59:59.950 -> both true;
	// identical instant -> both false.
	for _, tc := range []struct {
		a    dtIntervalEvent
		want bool
	}{
		{dtIntervalEvent{dtThresholdSeed - 1000, dtThresholdSeed - 1000}, true},
		{dtIntervalEvent{dtThresholdSeed - 50, dtThresholdSeed - 50}, true},
		{dtIntervalEvent{dtThresholdSeed, dtThresholdSeed}, false},
	} {
		if err := engine.Send(context.Background(), "A", tc.a); err != nil {
			t.Fatal(err)
		}
		for _, col := range []string{"c0", "c1"} {
			if got := last[col] == true; got != tc.want {
				t.Fatalf("a=(%d,%d) %s=%v want %v", tc.a.Start, tc.a.End, col, last[col], tc.want)
			}
		}
	}
}

// TestExprDTIntervalTimePeriodUnitsParity covers
// ExprDTIntervalTimePeriodWYearNonConst: before(b, 1 <unit>) at identical
// instants. Years/months/weeks/days/hours/minutes/seconds/milliseconds
// thresholds (all >= 1ms) yield false because delta 0 is below the minimum
// threshold; the microseconds form resolves to 0.001ms in Esper and admits
// delta 0, yielding true.
func TestExprDTIntervalTimePeriodUnitsParity(t *testing.T) {
	env := newDTIntervalEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	units := []struct {
		name string
		op   IntervalComputer
		want bool
	}{
		// 1 year..1 millisecond: threshold [1, MAX]; delta 0 < 1 -> false.
		{"years", BeforeThreshold(1, math.MaxInt64), false},
		{"month", BeforeThreshold(1, math.MaxInt64), false},
		{"weeks", BeforeThreshold(1, math.MaxInt64), false},
		{"days", BeforeThreshold(1, math.MaxInt64), false},
		{"hours", BeforeThreshold(1, math.MaxInt64), false},
		{"minutes", BeforeThreshold(1, math.MaxInt64), false},
		{"seconds", BeforeThreshold(1, math.MaxInt64), false},
		{"milliseconds", BeforeThreshold(1, math.MaxInt64), false},
		// 1 microsecond resolves to 0.001ms; the long-ms delta 0 satisfies
		// [0, 0] semantics only when the threshold collapses to 0.
		{"microseconds", BeforeThreshold(0, 0), true},
	}

	for _, unit := range units {
		t.Run(unit.name, func(t *testing.T) {
			pred := Interval(unit.op,
				IntervalBounds{Start: JoinField[int64](0, "st"), End: JoinField[int64](0, "en")},
				IntervalBounds{Start: JoinField[int64](1, "st"), End: JoinField[int64](1, "en")},
			)
			plan, err := env.Build(
				JoinMany(
					JoinSource(From[dtIntervalEvent](env, "A")).Window(LengthWindow(1)),
					JoinSource(From[dtIntervalEvent](env, "B")).Window(LengthWindow(1)),
				).Select(SelectFrom(0, "c", pred)).Query(StatementName("s0")),
			)
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = deployment.Undeploy(context.Background()) }()
			subStatement, ok := deployment.Statement("s0")
			if !ok {
				t.Fatal("statement s0 not found")
			}
			var last bool
			if _, err := subStatement.Subscribe(func(_ context.Context, batch ResultBatch) error {
				if len(batch.New) > 0 {
					row, _ := batch.New[len(batch.New)-1].Row()
					last = row.Get("c") == Present(true)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := engine.Send(context.Background(), "B", dtIntervalEvent{dtThresholdSeed, dtThresholdSeed}); err != nil {
				t.Fatal(err)
			}
			if err := engine.Send(context.Background(), "A", dtIntervalEvent{dtThresholdSeed, dtThresholdSeed}); err != nil {
				t.Fatal(err)
			}
			if last != unit.want {
				t.Fatalf("%s: got %v want %v", unit.name, last, unit.want)
			}
		})
	}
}
