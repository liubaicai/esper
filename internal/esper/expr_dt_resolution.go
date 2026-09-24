package esper

import (
	"fmt"
	"strings"
	"time"
)

// This file ports the resolution-aware date-time semantics exercised by
// ExprDTResolution (executions(isMicrosecond)): the EventTime execution
// (a.withDate(2002,4,30).before(b) over object-array events) and the
// LongProperty execution (withTime/set/get/getMinuteOfHour/toDate/toCalendar/
// minus over a long property and current_timestamp).
//
// Java references:
//   - TimeAbacus / TimeAbacusMicroseconds: long date-time values are epoch in
//     engine units. calendarSet splits v into millis = v/1000 plus remainder
//     v%1000; calendarGet returns cal.getTimeInMillis()*1000 + remainder.
//     toDate/toCalendar truncate to milliseconds; plus/minus add MILLISECONDS
//     regardless of resolution.
//   - dtlocal/DTLocalCalOpsLongEval, DTLocalLongOpsIntervalEval: calendar ops
//     on a long target run on the millisecond part and re-add the remainder;
//     interval targets preserve the duration measured in engine units.
//   - reformatop/CalendarEvalStatics (get(field), getMinuteOfHour, toDate,
//     toCalendar) and calop/CalendarPlusMinusForgeOp (plus/minus).
//
// Resolution model: int64 date-time values are epoch in engine units
// (milliseconds or microseconds per WithTimeUnit). time.Time values carry
// Calendar precision: they coerce to UnixMilli regardless of resolution,
// matching Java's Date/Calendar representations which are always
// millisecond-based.

// timeUnit resolves the date-time resolution visible to this evaluation
// context. An explicit TimeUnit field wins (direct expression evaluation);
// otherwise the engine's WithTimeUnit setting applies. The zero value is
// Milliseconds.
func (ctx EvalContext) timeUnit() TimeUnit {
	if ctx.TimeUnit == Microseconds {
		return Microseconds
	}
	if ctx.Engine != nil && ctx.Engine.timeUnit == Microseconds {
		return Microseconds
	}
	// Deployed paths that build an EvalContext without Engine still carry the
	// engine in Variables (variablesWithEngineLockState); read it so
	// resolution-aware date-time ops see the configured unit everywhere.
	if engine := aggregateEngineFromVariables(ctx.Variables); engine != nil && engine.timeUnit == Microseconds {
		return Microseconds
	}
	return Milliseconds
}

// dateTimeEngineUnits coerces a date-time value to engine units for
// comparisons (before/after/between): time.Time is Calendar-precision and
// always yields UnixMilli; int64 is already expressed in engine units and is
// returned as-is. The coercion is resolution-independent — the resolution
// only decides how an int64 is interpreted downstream.
func dateTimeEngineUnits(value Value) (int64, bool) {
	return dateTimeEpochMillis(value)
}

// dateTimeCalOpInstant is one date-time value split for a calendar
// operation: units is the raw engine-unit value, millis the millisecond part
// the calendar transform applies to, remainder the sub-millisecond part
// re-added to long results (TimeAbacusMicroseconds semantics), and scale the
// units-per-millisecond factor used to reassemble long results. timeRep
// marks time.Time/*time.Time inputs, which are Calendar-precision: scale 1,
// remainder 0, and calendar fields resolve in the value's own zone.
type dateTimeCalOpInstant struct {
	units     int64
	millis    int64
	remainder int64
	scale     int64
	location  *time.Location
	timeRep   bool
}

// dateTimeCalOpSplit decomposes a date-time value for a calendar operation.
// Under Microseconds an int64 input splits into millis = v/1000 and
// remainder = v%1000 (Go truncates toward zero exactly like Java's / and %).
// time.Time inputs keep millisecond precision in their own zone.
func dateTimeCalOpSplit(ctx EvalContext, value Value) (dateTimeCalOpInstant, bool) {
	location, timeRep := dateTimeCalOpLocation(value)
	units, ok := dateTimeEpochMillis(value)
	if !ok {
		return dateTimeCalOpInstant{}, false
	}
	instant := dateTimeCalOpInstant{
		units:    units,
		millis:   units,
		scale:    1,
		location: location,
		timeRep:  timeRep,
	}
	if !timeRep && ctx.timeUnit() == Microseconds {
		instant.millis = units / 1000
		instant.remainder = units % 1000
		instant.scale = 1000
	}
	return instant, true
}

// dateTimeCalOpJoin reassembles a transformed millisecond instant into
// engine units, re-adding the sub-millisecond remainder for long inputs
// under Microseconds (calendarGet: cal.getTimeInMillis()*1000 + remainder).
// time.Time inputs re-wrap at scale 1 with no remainder.
func dateTimeCalOpJoin(instant dateTimeCalOpInstant, millis int64) int64 {
	return millis*instant.scale + instant.remainder
}

// dateTimeFieldExtract reads one calendar field from an instant already
// resolved in the value's zone.
type dateTimeFieldExtract func(t time.Time) int64

// dateTimeGetFieldExtractor resolves a get(field) name to a calendar field
// extractor. Names follow Java's CalendarFieldEnum aliases (get('month')
// reads the 0-based Calendar.MONTH) plus the underscored reformat names used
// by the frozen contract (minute_of_hour, day_of_month, day_of_week,
// hour_of_day). day_of_week returns Java's Calendar.DAY_OF_WEEK numbering
// (1 = Sunday). Java's "week" is rejected: WEEK_OF_YEAR is locale-dependent
// and not part of the frozen contract.
func dateTimeGetFieldExtractor(field string) (dateTimeFieldExtract, error) {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "msec", "millisecond", "milliseconds", "milli_of_second":
		return func(t time.Time) int64 { return int64(t.Nanosecond()) / int64(time.Millisecond) }, nil
	case "sec", "second", "seconds", "second_of_minute":
		return func(t time.Time) int64 { return int64(t.Second()) }, nil
	case "min", "minute", "minutes", "minute_of_hour":
		return func(t time.Time) int64 { return int64(t.Minute()) }, nil
	case "hour", "hours", "hour_of_day":
		return func(t time.Time) int64 { return int64(t.Hour()) }, nil
	case "day", "days", "dayofmonth", "day_of_month":
		return func(t time.Time) int64 { return int64(t.Day()) }, nil
	case "dayofweek", "day_of_week":
		return func(t time.Time) int64 { return int64(t.Weekday()) + 1 }, nil
	case "month", "months", "month_of_year":
		return func(t time.Time) int64 { return int64(t.Month()) - 1 }, nil
	case "year", "years":
		return func(t time.Time) int64 { return int64(t.Year()) }, nil
	default:
		return nil, fmt.Errorf("unknown date-time calendar field %q (valid: year,month,dayofmonth,day_of_week,hour,minute,second,millisecond)", field)
	}
}

// DateTimeGet applies Esper's get(field) calendar-field extraction to a
// date-time expression: get('month') returns the 0-based Calendar.MONTH,
// getMinuteOfHour() maps to field "minute_of_hour" (Calendar.MINUTE).
// int64 inputs are engine units; under Microseconds the field is read from
// the millisecond part and the sub-millisecond remainder is ignored.
// time.Time inputs resolve fields in the value's own zone. An unknown field
// is a build-time configuration error; null or missing input produces Null.
func DateTimeGet[V int64 | time.Time](value Expression[V], field string) Expression[int64] {
	extract, err := dateTimeGetFieldExtractor(field)
	node := &exprNode{
		kind:               "date-time-get",
		typ:                typeOf[int64](),
		description:        fmt.Sprintf("date-time-get(%s,'%s')", expressionDescription(value), field),
		configurationError: errorMessage(err),
	}
	if value != nil {
		node.children = []*exprNode{value.node()}
	}
	if node.configurationError == "" {
		if value == nil || value.node() == nil {
			node.configurationError = "date-time-get requires a date-time operand"
		} else {
			validateDateTimeOperand(node, "value", value)
		}
	}
	return typedExpr[int64]{n: node, fn: func(ctx EvalContext) Value {
		if value == nil || extract == nil {
			return Null()
		}
		instant, ok := dateTimeCalOpSplit(ctx, value.eval(ctx))
		if !ok {
			return Null()
		}
		return Present(extract(time.UnixMilli(instant.millis).In(instant.location)))
	}}
}

// DateTimeMinus applies Esper's minus(ms) date-time operation: the argument
// is always a count of MILLISECONDS regardless of resolution, so under
// Microseconds an int64 result shifts by ms*1000 engine units
// (CalendarPlusMinusForgeOp on the millisecond part, remainder re-added).
// The input representation is preserved: int64 stays int64 engine units,
// time.Time stays time.Time shifted by ms milliseconds. Null or missing
// input produces Null.
func DateTimeMinus[V int64 | time.Time](value Expression[V], ms int64) Expression[V] {
	return dateTimeShift[V]("date-time-minus", value, ms, -1)
}

// DateTimePlus applies Esper's plus(ms) date-time operation, the additive
// counterpart of DateTimeMinus with identical resolution semantics.
func DateTimePlus[V int64 | time.Time](value Expression[V], ms int64) Expression[V] {
	return dateTimeShift[V]("date-time-plus", value, ms, 1)
}

func dateTimeShift[V int64 | time.Time](kind string, value Expression[V], ms int64, sign int64) Expression[V] {
	var children []*exprNode
	if value != nil {
		children = []*exprNode{value.node()}
	}
	node := &exprNode{
		kind:        kind,
		typ:         typeOf[V](),
		description: fmt.Sprintf("%s(%s,%d)", kind, expressionDescription(value), ms),
		children:    children,
	}
	if value == nil || value.node() == nil {
		node.configurationError = fmt.Sprintf("%s requires a date-time operand", kind)
	} else {
		validateDateTimeOperand(node, "value", value)
	}
	return typedExpr[V]{n: node, fn: func(ctx EvalContext) Value {
		if value == nil {
			return Null()
		}
		current := value.eval(ctx)
		if !current.IsPresent() {
			return Null()
		}
		switch instant := current.Any().(type) {
		case time.Time:
			return Present(instant.Add(time.Duration(sign*ms) * time.Millisecond))
		case *time.Time:
			if instant == nil {
				return Null()
			}
			shifted := instant.Add(time.Duration(sign*ms) * time.Millisecond)
			return Present(&shifted)
		}
		units, ok := dateTimeEngineUnits(current)
		if !ok {
			return Null()
		}
		scale := int64(1)
		if ctx.timeUnit() == Microseconds {
			scale = 1000
		}
		return dateTimeCalOpResult(current, units+sign*ms*scale)
	}}
}

// DateTimeToTime applies Esper's toDate()/toCalendar() conversion: engine
// units become a time.Time truncated to millisecond precision (under
// Microseconds, ts/1000). time.Time inputs pass through unchanged, matching
// Java's Date/Calendar representations which are already millisecond-based.
// Null or missing input produces Null.
func DateTimeToTime[V int64 | time.Time](value Expression[V]) Expression[time.Time] {
	var children []*exprNode
	if value != nil {
		children = []*exprNode{value.node()}
	}
	node := &exprNode{
		kind:        "date-time-to-time",
		typ:         typeOf[time.Time](),
		description: fmt.Sprintf("date-time-to-time(%s)", expressionDescription(value)),
		children:    children,
	}
	if value == nil || value.node() == nil {
		node.configurationError = "date-time-to-time requires a date-time operand"
	} else {
		validateDateTimeOperand(node, "value", value)
	}
	return typedExpr[time.Time]{n: node, fn: func(ctx EvalContext) Value {
		if value == nil {
			return Null()
		}
		current := value.eval(ctx)
		if !current.IsPresent() {
			return Null()
		}
		switch instant := current.Any().(type) {
		case time.Time:
			return Present(instant)
		case *time.Time:
			if instant == nil {
				return Null()
			}
			return Present(*instant)
		}
		units, ok := dateTimeEngineUnits(current)
		if !ok {
			return Null()
		}
		if ctx.timeUnit() == Microseconds {
			units /= 1000
		}
		return Present(time.UnixMilli(units).UTC())
	}}
}

// EventIntervalBounds derives the interval bounds of one join source from
// its event schema: the field flagged StartTimestamp supplies Start and the
// field flagged EndTimestamp supplies End (Esper's starttimestamp/
// endtimestamp event-type configuration, used by the ExprDTResolution
// EventTime execution's object-array MyEvent(id,sts,ets)). Bound values are
// coerced to engine units through dateTimeEngineUnits, so the result feeds
// WithDateBounds/WithTimeBounds/SetBounds/PointBounds and Interval directly.
// A source without the flagged fields, an out-of-range source index, or a
// non-coercible bound value produces Missing/Null at evaluation time.
func EventIntervalBounds(source int) IntervalBounds {
	return IntervalBounds{
		Start: eventIntervalBoundExpr(source, false),
		End:   eventIntervalBoundExpr(source, true),
	}
}

// eventIntervalBoundExpr builds one bound of EventIntervalBounds. The
// timestamp field name is resolved at evaluation time from the join event's
// schema so the same expression works across event representations.
func eventIntervalBoundExpr(source int, end bool) Expression[int64] {
	bound := "startTimestamp"
	if end {
		bound = "endTimestamp"
	}
	node := &exprNode{
		kind:        "event-interval-bound",
		typ:         typeOf[int64](),
		description: fmt.Sprintf("join[%d].%s", source, bound),
		joinSource:  source,
	}
	return typedExpr[int64]{n: node, fn: func(ctx EvalContext) Value {
		event, ok := joinSourceEvent(ctx, source)
		if !ok {
			return Missing()
		}
		for _, field := range event.Schema().fields {
			if (end && field.EndTimestamp) || (!end && field.StartTimestamp) {
				units, ok := dateTimeEngineUnits(event.Get(field.Name))
				if !ok {
					return Null()
				}
				return Present(units)
			}
		}
		return Missing()
	}}
}

// joinSourceEvent resolves the event occupying one join-tuple slot, the same
// scope JoinField and JoinEventValue read.
func joinSourceEvent(ctx EvalContext, source int) (Event, bool) {
	if source < 0 {
		return Event{}, false
	}
	events := ctx.JoinEvents
	if events == nil {
		tuple, ok := ctx.Event.Underlying().(joinTuple)
		if !ok {
			return Event{}, false
		}
		events = tuple.events
	}
	if source >= len(events) {
		return Event{}, false
	}
	event := events[source]
	if !event.Schema().valid() {
		return Event{}, false
	}
	return event, true
}

// IntervalBefore reports whether the left interval ends strictly before the
// right interval starts (leftEnd < rightStart) in engine units. It is the
// descriptive single-computer form of Interval(Before, left, right), used by
// the ExprDTResolution EventTime execution's a.withDate(...).before(b).
func IntervalBefore(left, right IntervalBounds) Expression[bool] {
	return Interval(Before, left, right)
}
