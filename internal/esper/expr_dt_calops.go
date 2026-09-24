package esper

import (
	"fmt"
	"strings"
	"time"
)

// This file ports the Esper date-time calendar operations (set(field,n),
// withDate(y,m,d), withTime(h,mi,s,ms)) and the point-in-time before
// comparator used by ExprDTIntervalOps executions 0 and 17.
//
// Java references:
//   - calop/CalendarSetForgeOp, CalendarWithDateForgeOp,
//     CalendarWithTimeForgeOp (field application)
//   - calop/CalendarFieldEnum (field-name aliases)
//   - dtlocal/DTLocalCalOpsIntervalEval, DTLocalLongOpsIntervalEval (interval
//     targets preserve duration: end = newStart + (end - start); point
//     targets collapse to (t,t))
//   - interval/IntervalComputerBeforeNoParam (strict before: delta >= 1ms)
//
// Month convention: Java is asymmetric — Calendar-backed reps use the
// 0-based lenient Calendar.MONTH while LocalDateTime/ZonedDateTime use the
// 1-based ChronoField.MONTH_OF_YEAR. Go collapses all reps to int64
// epoch-millis and time.Time, so a single convention is required; the frozen
// contract pins the 1-based LDT convention. Field application uses Go's
// time.Date normalization (lenient overflow, e.g. day 31 of a 30-day month
// rolls forward), which is observably identical to the oracle for every
// scenario in this unit.

// dateTimeCalField enumerates the calendar fields accepted by set(field,n).
type dateTimeCalField int

const (
	dateTimeCalMillisecond dateTimeCalField = iota
	dateTimeCalSecond
	dateTimeCalMinute
	dateTimeCalHour
	dateTimeCalDayOfMonth
	dateTimeCalMonth
	dateTimeCalYear
)

// dateTimeCalendarField resolves a set(field,n) name to a calendar field.
// Aliases follow Java's CalendarFieldEnum (case-insensitive, trimmed):
// msec/millisecond(s), sec/second(s), min/minute(s), hour(s), day(s),
// month(s), year(s). The contract field name "dayofmonth" maps to Java's
// Calendar.DATE ("day"). Java's "week" is rejected: WEEK_OF_YEAR is
// locale-dependent (first-day-of-week/minimal-days-in-first-week) and not
// part of the frozen contract.
func dateTimeCalendarField(field string) (dateTimeCalField, error) {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "msec", "millisecond", "milliseconds":
		return dateTimeCalMillisecond, nil
	case "sec", "second", "seconds":
		return dateTimeCalSecond, nil
	case "min", "minute", "minutes":
		return dateTimeCalMinute, nil
	case "hour", "hours":
		return dateTimeCalHour, nil
	case "day", "days", "dayofmonth":
		return dateTimeCalDayOfMonth, nil
	case "month", "months":
		return dateTimeCalMonth, nil
	case "year", "years":
		return dateTimeCalYear, nil
	default:
		return -1, fmt.Errorf("unknown date-time calendar field %q (valid: year,month,dayofmonth,hour,minute,second,millisecond)", field)
	}
}

// dateTimeCalOpApply transforms an epoch-millis instant in the given zone.
type dateTimeCalOpApply func(millis int64, location *time.Location) int64

// dateTimeCalOpSet returns the transform for set(field,n). The month value
// is 1-based (LDT convention); time.Date normalizes out-of-range values.
func dateTimeCalOpSet(field dateTimeCalField, n int) dateTimeCalOpApply {
	return func(millis int64, location *time.Location) int64 {
		t := time.UnixMilli(millis).In(location)
		y, mo, d := t.Date()
		h, mi, s := t.Clock()
		ns := t.Nanosecond()
		switch field {
		case dateTimeCalMillisecond:
			ns = n * int(time.Millisecond)
		case dateTimeCalSecond:
			s = n
		case dateTimeCalMinute:
			mi = n
		case dateTimeCalHour:
			h = n
		case dateTimeCalDayOfMonth:
			d = n
		case dateTimeCalMonth:
			mo = time.Month(n)
		case dateTimeCalYear:
			y = n
		}
		return time.Date(y, mo, d, h, mi, s, ns, location).UnixMilli()
	}
}

// dateTimeCalOpWithDate returns the withDate(y,m,d) transform: the calendar
// date is replaced while the time-of-day is kept (month is 1-based).
func dateTimeCalOpWithDate(year, month, day int) dateTimeCalOpApply {
	return func(millis int64, location *time.Location) int64 {
		t := time.UnixMilli(millis).In(location)
		h, mi, s := t.Clock()
		return time.Date(year, time.Month(month), day, h, mi, s, t.Nanosecond(), location).UnixMilli()
	}
}

// dateTimeCalOpWithTime returns the withTime(h,mi,s,ms) transform: the
func dateTimeCalOpWithTime(hour, minute, second, millis int) dateTimeCalOpApply {
	return func(epoch int64, location *time.Location) int64 {
		t := time.UnixMilli(epoch).In(location)
		y, mo, d := t.Date()
		return time.Date(y, mo, d, hour, minute, second, millis*int(time.Millisecond), location).UnixMilli()
	}
}

// DateTimeSet applies Esper's set(field,n) calendar operation to a date-time
// expression. The input representation is preserved: int64 epoch-millis
// stays int64, time.Time stays time.Time. Field names follow Java's
// CalendarFieldEnum aliases (year,month,dayofmonth,hour,minute,second,
// millisecond plus plural/short forms); the month value is 1-based. An
// unknown field is a build-time configuration error; null or missing input
// produces Null.
func DateTimeSet[V int64 | time.Time](value Expression[V], field string, n int) Expression[V] {
	resolved, err := dateTimeCalendarField(field)
	var apply dateTimeCalOpApply
	if err == nil {
		apply = dateTimeCalOpSet(resolved, n)
	}
	return dateTimeCalOpExpression[V]("date-time-set", value, apply,
		fmt.Sprintf("'%s',%d", field, n), errorMessage(err))
}

// DateTimeWithDate applies Esper's withDate(year,month,day) calendar
// operation, replacing the calendar date and keeping the time-of-day. The
// month argument is 1-based. The input representation is preserved; null or
// missing input produces Null.
func DateTimeWithDate[V int64 | time.Time](value Expression[V], year, month, day int) Expression[V] {
	return dateTimeCalOpExpression[V]("date-time-with-date", value,
		dateTimeCalOpWithDate(year, month, day),
		fmt.Sprintf("%d,%d,%d", year, month, day), "")
}

// DateTimeWithTime applies Esper's withTime(hour,minute,second,millis)
// calendar operation, replacing the time-of-day and keeping the calendar
// date. The input representation is preserved; null or missing input
// produces Null.
func DateTimeWithTime[V int64 | time.Time](value Expression[V], hour, minute, second, millis int) Expression[V] {
	return dateTimeCalOpExpression[V]("date-time-with-time", value,
		dateTimeCalOpWithTime(hour, minute, second, millis),
		fmt.Sprintf("%d,%d,%d,%d", hour, minute, second, millis), "")
}

// dateTimeCalOpExpression builds a rep-preserving calendar transform node.
// Calendar fields derive in the value's own zone for time.Time inputs;
// epoch-millis inputs carry no zone and evaluate in UTC (the pinned harness
// convention matching -Duser.timezone=UTC).
func dateTimeCalOpExpression[V int64 | time.Time](kind string, value Expression[V], apply dateTimeCalOpApply, detail, buildErr string) Expression[V] {
	var children []*exprNode
	if value != nil {
		children = []*exprNode{value.node()}
	}
	node := &exprNode{
		kind:               kind,
		typ:                typeOf[V](),
		description:        fmt.Sprintf("%s(%s,%s)", kind, expressionDescription(value), detail),
		children:           children,
		configurationError: buildErr,
	}
	if node.configurationError == "" {
		if value == nil || value.node() == nil {
			node.configurationError = fmt.Sprintf("%s requires a date-time operand", kind)
		} else {
			validateDateTimeOperand(node, "value", value)
		}
	}
	return typedExpr[V]{n: node, fn: func(ctx EvalContext) Value {
		if value == nil || apply == nil {
			return Null()
		}
		current := value.eval(ctx)
		millis, ok := dateTimeEpochMillis(current)
		if !ok {
			return Null()
		}
		location := time.UTC
		if located, ok2 := dateTimeCalOpLocation(current); ok2 {
			location = located
		}
		return dateTimeCalOpResult(current, apply(millis, location))
	}}
}

// dateTimeCalOpLocation extracts the zone of a time.Time (or *time.Time)
// valued datetime so calendar fields resolve in the value's own zone.
func dateTimeCalOpLocation(value Value) (*time.Location, bool) {
	switch located := value.Any().(type) {
	case time.Time:
		if located.Location() != nil {
			return located.Location(), true
		}
		return time.UTC, true
	case *time.Time:
		if located != nil && located.Location() != nil {
			return located.Location(), true
		}
		return time.UTC, true
	}
	return time.UTC, false
}

// dateTimeCalOpResult re-wraps a transformed instant in the input's
// representation: int64 stays int64, time.Time stays time.Time (UTC instant,
// matching the DateTimeRound* precedent), *time.Time stays *time.Time, and
// any other numeric rep collapses to int64 epoch-millis.
func dateTimeCalOpResult(input Value, millis int64) Value {
	switch input.Any().(type) {
	case time.Time:
		return Present(time.UnixMilli(millis))
	case *time.Time:
		out := time.UnixMilli(millis)
		return Present(&out)
	default:
		return Present(millis)
	}
}

// DateTimeBefore reports whether a point-in-time value is strictly before a
// threshold: true iff threshold - value >= 1 millisecond (Java's default
// before range [1, Long.MAX_VALUE] applied to the point interval (t,t)).
// Null and missing operands produce Null, matching Esper's boxed Boolean
// result. It mirrors DateTimeAfter with the comparison inverted.
func DateTimeBefore(value, threshold Expr) Expression[bool] {
	children := []*exprNode{nil, nil}
	if value != nil {
		children[0] = value.node()
	}
	if threshold != nil {
		children[1] = threshold.node()
	}
	node := &exprNode{
		kind:        "date-time-before",
		typ:         typeOf[bool](),
		description: fmt.Sprintf("(%s before %s)", expressionDescription(value), expressionDescription(threshold)),
		children:    children,
	}
	validateDateTimeOperand(node, "value", value)
	if node.configurationError == "" {
		validateDateTimeOperand(node, "threshold", threshold)
	}
	return typedExpr[bool]{n: node, fn: func(ctx EvalContext) Value {
		if value == nil || threshold == nil {
			return Null()
		}
		left, ok := dateTimeEpochMillis(value.eval(ctx))
		if !ok {
			return Null()
		}
		right, ok := dateTimeEpochMillis(threshold.eval(ctx))
		if !ok {
			return Null()
		}
		return Present(right > left)
	}}
}

// WithDateBounds applies withDate(year,month,day) to an interval bound,
// preserving duration: Start = transform(start) and
// End = transform(start) + (end - start), matching Java's
// DTLocalCalOpsIntervalEval.evaluate(start,end). The month argument is
// 1-based. Both sides emit int64 epoch-millis expressions so the result
// feeds Interval(Before/After/...) directly.
//
// Java collapses calendar ops on the parameter-side event to a point; this
// unit only exercises that form with duration 0, so the duration-preserving
// transform is observably identical on both sides (see PointBounds for the
// explicit point-collapse variant).
func WithDateBounds(bounds IntervalBounds, year, month, day int) IntervalBounds {
	return dateTimeCalOpBounds("date-time-with-date", bounds,
		dateTimeCalOpWithDate(year, month, day),
		fmt.Sprintf("%d,%d,%d", year, month, day), "")
}

// WithTimeBounds applies withTime(hour,minute,second,millis) to an interval
// bound, preserving duration exactly like WithDateBounds.
func WithTimeBounds(bounds IntervalBounds, hour, minute, second, millis int) IntervalBounds {
	return dateTimeCalOpBounds("date-time-with-time", bounds,
		dateTimeCalOpWithTime(hour, minute, second, millis),
		fmt.Sprintf("%d,%d,%d,%d", hour, minute, second, millis), "")
}

// SetBounds applies set(field,n) to an interval bound, preserving duration
// exactly like WithDateBounds. Field names and the unknown-field build error
// match DateTimeSet.
func SetBounds(bounds IntervalBounds, field string, n int) IntervalBounds {
	resolved, err := dateTimeCalendarField(field)
	var apply dateTimeCalOpApply
	if err == nil {
		apply = dateTimeCalOpSet(resolved, n)
	}
	return dateTimeCalOpBounds("date-time-set", bounds, apply,
		fmt.Sprintf("'%s',%d", field, n), errorMessage(err))
}

// PointBounds collapses an interval bound to the point (start,start),
// matching Java's point-target calendar-op evaluation
// (intervalOp.evaluate(time,time)) and the parameter-side collapse of
// calendar ops on timestamped events. The start is coerced to int64
// epoch-millis so the result feeds Interval(Before/After/...) directly.
func PointBounds(bounds IntervalBounds) IntervalBounds {
	point := dateTimeCalOpBoundsExpr("date-time-point", bounds.Start, bounds.Start,
		func(millis int64, _ *time.Location) int64 { return millis }, "", "")
	return IntervalBounds{Start: point, End: point}
}

// dateTimeCalOpBounds builds the duration-preserving interval transform:
// Start = apply(start), End = apply(start) + (end - start).
func dateTimeCalOpBounds(kind string, bounds IntervalBounds, apply dateTimeCalOpApply, detail, buildErr string) IntervalBounds {
	return IntervalBounds{
		Start: dateTimeCalOpBoundsExpr(kind, bounds.Start, bounds.End, apply, detail, buildErr),
		End:   dateTimeCalOpBoundsExpr(kind+"-end", bounds.Start, bounds.End, apply, detail, buildErr),
	}
}

// dateTimeCalOpBoundsExpr emits an int64 epoch-millis expression. For the
// "-end" kind the value is apply(start) + (end - start); otherwise it is
// apply(start). Null or missing operands produce Null.
func dateTimeCalOpBoundsExpr(kind string, start, end Expr, apply dateTimeCalOpApply, detail, buildErr string) Expression[int64] {
	isEnd := strings.HasSuffix(kind, "-end")
	children := []*exprNode{nil, nil}
	if start != nil {
		children[0] = start.node()
	}
	if end != nil {
		children[1] = end.node()
	}
	node := &exprNode{
		kind:               kind,
		typ:                typeOf[int64](),
		description:        fmt.Sprintf("%s(%s,%s)", kind, expressionDescription(start), detail),
		children:           children,
		configurationError: buildErr,
	}
	if node.configurationError == "" {
		if start == nil || start.node() == nil {
			node.configurationError = fmt.Sprintf("%s requires an interval start operand", kind)
		} else {
			validateDateTimeOperand(node, "interval start", start)
		}
	}
	if node.configurationError == "" && isEnd {
		if end == nil || end.node() == nil {
			node.configurationError = fmt.Sprintf("%s requires an interval end operand", kind)
		} else {
			validateDateTimeOperand(node, "interval end", end)
		}
	}
	return typedExpr[int64]{n: node, fn: func(ctx EvalContext) Value {
		if start == nil || apply == nil {
			return Null()
		}
		startValue := start.eval(ctx)
		startMillis, ok := dateTimeEpochMillis(startValue)
		if !ok {
			return Null()
		}
		location := time.UTC
		if located, ok2 := dateTimeCalOpLocation(startValue); ok2 {
			location = located
		}
		newStart := apply(startMillis, location)
		if !isEnd {
			return Present(newStart)
		}
		if end == nil {
			return Null()
		}
		endMillis, ok := dateTimeEpochMillis(end.eval(ctx))
		if !ok {
			return Null()
		}
		return Present(newStart + (endMillis - startMillis))
	}}
}
