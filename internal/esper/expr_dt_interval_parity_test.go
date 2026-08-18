package esper

import (
	"context"
	"testing"
)

// Parity coverage for the date-time interval operators, ported from the
// where-clause executions of ExprDTIntervalOps:
//
// - ExprDTIntervalBeforeWhereClause / ExprDTIntervalAfterWhereClause /
//   ExprDTIntervalCoincidesWhereClause / ExprDTIntervalDuringWhereClause /
//   ExprDTIntervalFinishesWhereClause / ExprDTIntervalFinishedByWhereClause /
//   ExprDTIntervalIncludesByWhereClause / ExprDTIntervalMeetsWhereClause /
//   ExprDTIntervalMetByWhereClause / ExprDTIntervalOverlapsWhereClause /
//   ExprDTIntervalOverlappedByWhereClause / ExprDTIntervalStartsWhereClause /
//   ExprDTIntervalStartedByWhereClause.
//
// The Java executions iterate SupportDateTimeFieldType variants (long, util
// date, calendar, LocalDateTime, ZonedDateTime); all reduce to the same
// epoch-millis comparison in IntervalComputerForgeFactory, so the Go chain
// binds int64 start/end fields directly. Expected values below were
// cross-checked against the Java oracle trace
// testdata/parity/dt-interval-ops.trace.json (43 records, zero mismatch).
//
// Java's BeforeValidator/AfterValidator/etc. classes restate the same
// IntervalComputer semantics and are covered by the same assertions.

type dtIntervalEvent struct {
	Start int64 `esper:"st"`
	End   int64 `esper:"en"`
}

func newDTIntervalEnv(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[dtIntervalEvent](env, "A"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[dtIntervalEvent](env, "B"); err != nil {
		t.Fatal(err)
	}
	return env
}

// TestExprDTIntervalOpsParity exercises all thirteen no-parameter interval
// operators against a fixed B interval [0, 1000) using the same A sequences
// as the Java oracle scenario. Expected flags come from the Java trace.
func TestExprDTIntervalOpsParity(t *testing.T) {
	bounds := func() (IntervalBounds, IntervalBounds) {
		left := IntervalBounds{
			Start: JoinField[int64](0, "st"),
			End:   JoinField[int64](0, "en"),
		}
		right := IntervalBounds{
			Start: JoinField[int64](1, "st"),
			End:   JoinField[int64](1, "en"),
		}
		return left, right
	}

	cases := []struct {
		name string
		op   IntervalComputer
		send []dtIntervalEvent
		want []bool
	}{
		{"before", Before, []dtIntervalEvent{{0, 200}, {-500, -100}, {-100, 0}, {900, 1200}},
			[]bool{false, true, false, false}},
		{"after", After, []dtIntervalEvent{{1200, 1500}, {1000, 1200}, {500, 900}},
			[]bool{true, false, false}},
		{"coincides", Coincides, []dtIntervalEvent{{0, 1000}, {0, 1001}, {1, 1000}},
			[]bool{true, false, false}},
		{"during", During, []dtIntervalEvent{{100, 900}, {0, 900}, {100, 1000}, {-100, 1100}},
			[]bool{true, false, false, false}},
		{"finishes", Finishes, []dtIntervalEvent{{100, 1000}, {0, 1000}, {100, 999}},
			[]bool{true, false, false}},
		{"finishedBy", FinishedBy, []dtIntervalEvent{{-100, 1000}, {0, 1000}, {-100, 999}},
			[]bool{true, false, false}},
		{"includes", Includes, []dtIntervalEvent{{-100, 1100}, {0, 1100}, {-100, 1000}},
			[]bool{true, false, false}},
		{"meets", Meets, []dtIntervalEvent{{-200, 0}, {-200, -1}, {100, 300}},
			[]bool{true, false, false}},
		{"metBy", MetBy, []dtIntervalEvent{{1000, 1200}, {1001, 1200}, {500, 700}},
			[]bool{true, false, false}},
		{"overlaps", Overlaps, []dtIntervalEvent{{-100, 500}, {-100, 0}, {0, 500}, {500, 1200}},
			[]bool{true, false, false, false}},
		{"overlappedBy", OverlappedBy, []dtIntervalEvent{{500, 1200}, {1000, 1200}, {500, 1000}, {-100, 500}},
			[]bool{true, false, false, false}},
		{"starts", Starts, []dtIntervalEvent{{0, 500}, {0, 1000}, {1, 500}},
			[]bool{true, false, false}},
		{"startedBy", StartedBy, []dtIntervalEvent{{0, 1200}, {0, 1000}, {0, 900}},
			[]bool{true, false, false}},
	}

	env := newDTIntervalEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			left, right := bounds()
			pred := Interval(tc.op, left, right)
			plan, err := env.Build(
				JoinMany(
					JoinSource(From[dtIntervalEvent](env, "A")).Window(LengthWindow(1)),
					JoinSource(From[dtIntervalEvent](env, "B")).Window(LengthWindow(1)),
				).Select(
					SelectFrom(0, "a_st", JoinField[int64](0, "st")),
					SelectFrom(0, "a_en", JoinField[int64](0, "en")),
					SelectFrom(1, "b_st", JoinField[int64](1, "st")),
					SelectFrom(1, "b_en", JoinField[int64](1, "en")),
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

			if err := engine.Send(context.Background(), "B", dtIntervalEvent{Start: 0, End: 1000}); err != nil {
				t.Fatal(err)
			}
			_ = fired() // drain B-only join result (no A yet -> no fire)

			for i, ev := range tc.send {
				if err := engine.Send(context.Background(), "A", ev); err != nil {
					t.Fatal(err)
				}
				if got := fired(); got != tc.want[i] {
					t.Fatalf("%s step %d (%d,%d): fired=%v want %v", tc.name, i, ev.Start, ev.End, got, tc.want[i])
				}
			}
		})
	}
}
