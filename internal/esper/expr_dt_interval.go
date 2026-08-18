package esper

import (
	"strings"
)

// This file ports the Esper date-time interval algebra (the DT methods
// before/after/coincedes/during/includes/finishes/finishedBy/meets/metBy/
// overlaps/overlappedBy/starts/startedBy) as typed, analyzable expression
// combinators. The interval bounds are epoch-millisecond int64 expressions,
// mirroring how Esper derives interval start/end from date property pairs
// (IntervalForge / ExprDTIntervalOps A/B event pairs).
//
// Java reference: IntervalComputerForgeFactory in
// common/src/main/java/com/espertech/esper/common/internal/epl/datetime/interval/.

// IntervalBounds identifies one side of an interval comparison: a start and
// an end epoch-millisecond expression evaluated against the same stream row.
// Building blocks are plain field expressions (Field[T,"startField"]) so the
// planner retains full schema knowledge.
type IntervalBounds struct {
	Start Expr
	End   Expr
}

// Interval computes a boolean relationship between two interval bounds.
// The comparison is an explicit expression node (not an opaque callback) so
// it composes with And/Or/Not and stays describable in plans.
func Interval(computer intervalComputer, left, right IntervalBounds) Expression[bool] {
	children := []*exprNode{left.Start.node(), left.End.node(), right.Start.node(), right.End.node()}
	name := computer.name
	return makeExpr[bool]("dt-interval-"+name,
		"("+strings.Join([]string{
			describe(left.Start), describe(left.End), describe(right.Start), describe(right.End),
		}, " "+name+" ")+")",
		children, func(ctx EvalContext) Value {
			leftStart := left.Start.eval(ctx)
			leftEnd := left.End.eval(ctx)
			rightStart := right.Start.eval(ctx)
			rightEnd := right.End.eval(ctx)
			for _, v := range []Value{leftStart, leftEnd, rightStart, rightEnd} {
				if !v.IsPresent() {
					return Missing()
				}
			}
			ls, ok1 := toInt64Value(leftStart)
			le, ok2 := toInt64Value(leftEnd)
			rs, ok3 := toInt64Value(rightStart)
			re, ok4 := toInt64Value(rightEnd)
			if !ok1 || !ok2 || !ok3 || !ok4 {
				return Missing()
			}
			return Present(computer.compute(ls, le, rs, re))
		})
}

type intervalComputer struct {
	name    string
	compute func(leftStart, leftEnd, rightStart, rightEnd int64) bool
}

// Before reports whether the left interval ends before the right interval
// starts (strict): leftEnd < rightStart. Java: IntervalComputerBeforeNoParam.
var Before = intervalComputer{"before", func(ls, le, rs, re int64) bool { return le < rs }}

// After reports whether the left interval starts after the right interval
// ends (strict): leftStart > rightEnd. Java: IntervalComputerAfterNoParam.
var After = intervalComputer{"after", func(ls, le, rs, re int64) bool { return ls > re }}

// Coincides reports identical start and end points.
// Java: IntervalComputerCoincidesNoParam.
var Coincides = intervalComputer{"coincides", func(ls, le, rs, re int64) bool { return ls == rs && le == re }}

// During reports the left interval fully inside the right interval
// (strict): rightStart < leftStart && leftEnd < rightEnd.
// Java: IntervalComputerDuringNoParam.
var During = intervalComputer{"during", func(ls, le, rs, re int64) bool { return rs < ls && le < re }}

// Includes reports the right interval fully inside the left interval
// (strict). Java: IntervalComputerIncludesNoParam.
var Includes = intervalComputer{"includes", func(ls, le, rs, re int64) bool { return ls < rs && re < le }}

// Finishes reports that both intervals share an end while the right starts
// earlier: rightStart < leftStart && leftEnd == rightEnd.
// Java: IntervalComputerFinishesNoParam.
var Finishes = intervalComputer{"finishes", func(ls, le, rs, re int64) bool { return rs < ls && le == re }}

// FinishedBy is the inverse of Finishes: same end, left starts earlier.
// Java: IntervalComputerFinishedByNoParam.
var FinishedBy = intervalComputer{"finishedBy", func(ls, le, rs, re int64) bool { return ls < rs && le == re }}

// Meets reports that the left end equals the right start.
// Java: IntervalComputerMeetsNoParam.
var Meets = intervalComputer{"meets", func(ls, le, rs, re int64) bool { return le == rs }}

// MetBy is the inverse of Meets: the right end equals the left start.
// Java: IntervalComputerMetByNoParam.
var MetBy = intervalComputer{"metBy", func(ls, le, rs, re int64) bool { return re == ls }}

// Overlaps reports a partial overlap where the left starts first:
// leftStart < rightStart && rightStart < leftEnd && leftEnd < rightEnd.
// Java: IntervalComputerOverlapsNoParam.
var Overlaps = intervalComputer{"overlaps", func(ls, le, rs, re int64) bool { return ls < rs && rs < le && le < re }}

// OverlappedBy is the inverse of Overlaps: the right starts first.
// Java: IntervalComputerOverlappedByNoParam.
var OverlappedBy = intervalComputer{"overlappedBy", func(ls, le, rs, re int64) bool { return rs < ls && ls < re && re < le }}

// Starts reports the same start with the left ending earlier.
// Java: IntervalComputerStartsNoParam.
var Starts = intervalComputer{"starts", func(ls, le, rs, re int64) bool { return ls == rs && le < re }}

// StartedBy is the inverse of Starts: same start, left ends later.
// Java: IntervalComputerStartedByNoParam.
var StartedBy = intervalComputer{"startedBy", func(ls, le, rs, re int64) bool { return ls == rs && le > re }}

// BeforeThreshold reports whether the left interval ends before the right
// interval starts by a delta within [startDelta, endDelta] milliseconds.
// Java: IntervalComputerConstantBefore.computeIntervalBefore with the
// range swapped when startDelta exceeds endDelta (WithDeltaExpr semantics).
func BeforeThreshold(startDelta, endDelta int64) intervalComputer {
	return intervalComputer{"before", func(ls, le, rs, re int64) bool {
		lo, hi := startDelta, endDelta
		if lo > hi {
			lo, hi = hi, lo
		}
		delta := rs - le
		return lo <= delta && delta <= hi
	}}
}

// AfterThreshold reports whether the left interval starts after the right
// interval ends by a delta within [startDelta, endDelta] milliseconds.
// Java: IntervalComputerConstantAfter.computeIntervalAfter with the range
// swapped when startDelta exceeds endDelta.
func AfterThreshold(startDelta, endDelta int64) intervalComputer {
	return intervalComputer{"after", func(ls, le, rs, re int64) bool {
		lo, hi := startDelta, endDelta
		if lo > hi {
			lo, hi = hi, lo
		}
		delta := ls - re
		return lo <= delta && delta <= hi
	}}
}

// toInt64Value coerces a present Value to int64 without allocating.
func toInt64Value(v Value) (int64, bool) {
	switch n := v.Any().(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	}
	return 0, false
}

func describe(e Expr) string {
	if e == nil {
		return "<nil>"
	}
	return e.Description()
}
