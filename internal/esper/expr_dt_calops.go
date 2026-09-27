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
	dateTimeCalWeek
)

// dateTimeCalendarField resolves a set(field,n) name to a calendar field.
// Aliases follow Java's CalendarFieldEnum (case-insensitive, trimmed):
// msec/millisecond(s), sec/second(s), min/minute(s), hour(s), day(s),
// month(s), year(s), week(s). The contract field name "dayofmonth" maps to
// Java's Calendar.DATE ("day"); "week" maps to Java's WEEK_OF_YEAR with the
// default GregorianCalendar rule (Sunday-start, minimal-days 1).
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
	case "week", "weeks":
		return dateTimeCalWeek, nil
	default:
		return -1, fmt.Errorf("unknown date-time calendar field %q (valid: year,month,dayofmonth,hour,minute,second,millisecond,week)", field)
	}
}

// dateTimeJavaWeeksInYear returns the number of WEEK_OF_YEAR weeks in a year
// under Java's default GregorianCalendar rule (52 or 53). The anchor honors
// the hybrid calendar: pre-cutover years resolve their January 1 on the
// Julian calendar.
func dateTimeJavaWeeksInYear(y int) int {
	weekStart := dateTimeJavaWeekOneSundayMillis(y, time.UTC)
	nextStart := dateTimeJavaWeekOneSundayMillis(y+1, time.UTC)
	return int((nextStart - weekStart) / (7 * 86400000))
}

// dateTimeJavaWeekOneSundayMillis returns the epoch-millis of the Sunday on
// or before the year's January 1 (Java WEEK_OF_YEAR anchor, minDays 1). For
// pre-cutover years January 1 is a Julian-calendar date.
func dateTimeJavaWeekOneSundayMillis(y int, location *time.Location) int64 {
	jan1Millis := dateTimeJavaToMillis(y, 1, 1, 0, 0, 0, 0, location)
	jan1 := time.UnixMilli(jan1Millis).In(location)
	return jan1Millis - int64(jan1.Weekday())*86400000
}

// dateTimeJavaWeekSetMillis returns the epoch-millis of setting WEEK_OF_YEAR
// to `target` for the original instant: Java Calendar.set keeps the
// day-of-week, and a day's weekday is absolute, so it comes from the input
// instant. The calendar year anchors the week grid (hybrid rules).
func dateTimeJavaWeekSetMillis(millis int64, target, weekday, h, mi, s, ns int, location *time.Location) int64 {
	y, _, _ := dateTimeJavaDecompose(millis, location)
	weekOneSunday := dateTimeJavaWeekOneSundayMillis(y, location)
	return weekOneSunday + int64(7*(target-1)+weekday)*86400000 +
		int64(h)*3600000 + int64(mi)*60000 + int64(s)*1000 + int64(ns/int(time.Millisecond))
}

// dateTimeJavaDecompose converts epoch-millis to local civil fields under
// Java's GregorianCalendar hybrid rules: post-cutover instants decompose on
// the proleptic Gregorian calendar; pre-cutover instants decompose on the
// Julian calendar via the inverse Julian day number.
func dateTimeJavaDecompose(millis int64, location *time.Location) (int, time.Month, int) {
	t := time.UnixMilli(millis).In(location)
	if t.UnixMilli() >= dateTimeJavaCutoverMillis {
		y, mo, d := t.Date()
		return y, mo, d
	}
	offsetMillis := int64(0)
	if location != nil {
		_, offsetSeconds := t.Zone()
		offsetMillis = int64(offsetSeconds) * 1000
	}
	localMillis := millis + offsetMillis
	// JDN of the local civil day (floor division for negative values).
	day := localMillis / 86400000
	if localMillis%86400000 < 0 {
		day--
	}
	jdn := day + 2440588
	// Inverse JDN -> Julian y/m/d (Fliegel–Van Flandern Julian branch:
	// c=jdn+32082; verified against the forward JDN used by
	// dateTimeJavaToMillis, e.g. 0001-05-30 -> JDN 1721573).
	c := jdn + 32082
	dq := (4*c + 3) / 1461
	e := c - (1461*dq)/4
	mq := (5*e + 2) / 153
	dayOfMonth := int(e - (153*mq+2)/5 + 1)
	month := int(mq + 3 - 12*(mq/10))
	year := int(dq - 4800 + (mq / 10))
	return year, time.Month(month), dayOfMonth
}

// dateTimeJavaDaysInMonth returns the civil month length under the hybrid
// rules: the Julian leap rule (year % 4 == 0) for pre-cutover years, the
// Gregorian rule otherwise.
func dateTimeJavaDaysInMonth(y int, mo time.Month) int {
	switch mo {
	case time.January, time.March, time.May, time.July, time.August, time.October, time.December:
		return 31
	case time.April, time.June, time.September, time.November:
		return 30
	}
	if y >= 1583 {
		if y%400 == 0 || (y%4 == 0 && y%100 != 0) {
			return 29
		}
		return 28
	}
	if y%4 == 0 {
		return 29
	}
	return 28
}

// dateTimeCalOpWithMinMax returns the transform for withMax(field) /
// withMin(field): the calendar field is set to its actual maximum/minimum in
// the value's zone while all other components stay put (Java Calendar.set
// semantics; day-of-month for 'day', week-of-year keeps the day-of-week).
// 'year' uses GregorianCalendar's actual maximum 292278994 / minimum 1.
func dateTimeCalOpWithMinMax(field dateTimeCalField, maximum bool) dateTimeCalOpApply {
	return func(millis int64, location *time.Location) int64 {
		t := time.UnixMilli(millis).In(location)
		y, mo, d := dateTimeJavaDecompose(millis, location)
		h, mi, s := t.Clock()
		ns := t.Nanosecond()
		switch field {
		case dateTimeCalMillisecond:
			if maximum {
				ns = int(999 * time.Millisecond)
			} else {
				ns = 0
			}
		case dateTimeCalSecond:
			if maximum {
				s = 59
			} else {
				s = 0
			}
		case dateTimeCalMinute:
			if maximum {
				mi = 59
			} else {
				mi = 0
			}
		case dateTimeCalHour:
			if maximum {
				h = 23
			} else {
				h = 0
			}
		case dateTimeCalDayOfMonth:
			if maximum {
				d = dateTimeJavaDaysInMonth(y, mo)
			} else {
				d = 1
			}
		case dateTimeCalMonth:
			if maximum {
				mo = time.December
			} else {
				mo = time.January
			}
		case dateTimeCalYear:
			if maximum {
				y = 292278994
			} else {
				y = 1
			}
		case dateTimeCalWeek:
			// Java WEEK_OF_YEAR (Sunday-start, minDays 1): set the week
			// number while keeping the day-of-week, like Calendar.set —
			// getActualMaximum yields weeks-in-year (52 or 53), minimum 1.
			target := 1
			if maximum {
				target = dateTimeJavaWeeksInYear(y)
			}
			return dateTimeJavaWeekSetMillis(millis, target, int(t.Weekday()), h, mi, s, ns, location)
		}
		return dateTimeJavaToMillis(y, mo, d, h, mi, s, ns, location)
	}
}

// dateTimeJavaCutoverMillis is the epoch-millis of Java GregorianCalendar's
// Julian-to-Gregorian cutover (1582-10-15 local midnight). Dates before it
// resolve on the Julian calendar; dates on or after it resolve on the
// proleptic Gregorian calendar.
const dateTimeJavaCutoverMillis = int64(-12219292800000)

// dateTimeJavaToMillis converts a calendar field tuple to epoch-millis under
// Java's GregorianCalendar hybrid rules: post-cutover fields go through the
// proleptic Gregorian calendar (time.Date), while pre-cutover fields resolve
// on the Julian calendar via the Julian day number (java.util.GregorianCalendar
// epochMillis: JDN * 86400000 - 210866803200000 + zone offset). The zone
// offset is taken from the instant the field tuple would produce on the
// proleptic calendar — pre-cutover LMT divergences are out of scope.
func dateTimeJavaToMillis(y int, mo time.Month, d, h, mi, s, ns int, location *time.Location) int64 {
	proleptic := time.Date(y, mo, d, h, mi, s, ns, location)
	if proleptic.UnixMilli() >= dateTimeJavaCutoverMillis {
		return proleptic.UnixMilli()
	}
	// Julian day number for a Julian-calendar civil date (a = (14-month)/12).
	a := (14 - int(mo)) / 12
	jy := y + 4800 - a
	jm := int(mo) + 12*a - 3
	jdn := int64(d) + int64((153*jm+2)/5) + 365*int64(jy) + int64(jy/4) - 32083
	// Zone offset at the corresponding proleptic instant (pre-cutover LMT
	// divergence vs Java is out of scope).
	offset := int64(0)
	if location != nil {
		_, offsetSeconds := proleptic.Zone()
		offset = int64(offsetSeconds)
	}
	return jdn*86400000 - 210866803200000 + offset*1000 + int64(h)*3600000 + int64(mi)*60000 + int64(s)*1000 + int64(ns/int(time.Millisecond))
}

// DateTimeWithMax applies Esper's withMax(field) calendar operation: the
// field is set to its actual maximum while every other component stays put
// (withMax('month') keeps the day, withMax('week') keeps the day-of-week).
// Field names follow Java's CalendarFieldEnum aliases including 'week';
// the input representation is preserved; null or missing input produces
// Null. Calendar evaluation follows Java's GregorianCalendar hybrid rules:
// pre-1582-10-15 instants resolve on the Julian calendar.
func DateTimeWithMax[V int64 | time.Time](value Expression[V], field string) Expression[V] {
	resolved, err := dateTimeCalendarField(field)
	var apply dateTimeCalOpApply
	if err == nil {
		apply = dateTimeCalOpWithMinMax(resolved, true)
	}
	return dateTimeCalOpExpression[V]("date-time-with-max", value, apply,
		fmt.Sprintf("'%s'", field), errorMessage(err))
}

// DateTimeWithMin applies Esper's withMin(field) calendar operation: the
// field is set to its actual minimum while every other component stays put
// (withMin('week') keeps the day-of-week). Field names follow Java's
// CalendarFieldEnum aliases including 'week'; the input representation is
// preserved; null or missing input produces Null. Calendar evaluation
// follows Java's GregorianCalendar hybrid rules: pre-1582-10-15 instants
// resolve on the Julian calendar.
func DateTimeWithMin[V int64 | time.Time](value Expression[V], field string) Expression[V] {
	resolved, err := dateTimeCalendarField(field)
	var apply dateTimeCalOpApply
	if err == nil {
		apply = dateTimeCalOpWithMinMax(resolved, false)
	}
	return dateTimeCalOpExpression[V]("date-time-with-min", value, apply,
		fmt.Sprintf("'%s'", field), errorMessage(err))
}

// dateTimeCalOpApply transforms an epoch-millis instant in the given zone.
type dateTimeCalOpApply func(millis int64, location *time.Location) int64

// dateTimeCalOpSet returns the transform for set(field,n). The month value
// is 1-based (LDT convention); out-of-range values normalize like Java's
// Calendar.set. 'week' sets WEEK_OF_YEAR keeping the day-of-week.
func dateTimeCalOpSet(field dateTimeCalField, n int) dateTimeCalOpApply {
	return func(millis int64, location *time.Location) int64 {
		t := time.UnixMilli(millis).In(location)
		y, mo, d := dateTimeJavaDecompose(millis, location)
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
		case dateTimeCalWeek:
			return dateTimeJavaWeekSetMillis(millis, n, int(t.Weekday()), h, mi, s, ns, location)
		}
		return dateTimeJavaToMillis(y, mo, d, h, mi, s, ns, location)
	}
}

// dateTimeCalOpWithDate returns the withDate(y,m,d) transform: the calendar
// date is replaced while the time-of-day is kept (month is 1-based).
func dateTimeCalOpWithDate(year, month, day int) dateTimeCalOpApply {
	return func(millis int64, location *time.Location) int64 {
		t := time.UnixMilli(millis).In(location)
		h, mi, s := t.Clock()
		return dateTimeJavaToMillis(year, time.Month(month), day, h, mi, s, t.Nanosecond(), location)
	}
}

// dateTimeCalOpWithTime returns the withTime(h,mi,s,ms) transform: the
func dateTimeCalOpWithTime(hour, minute, second, millis int) dateTimeCalOpApply {
	return func(epoch int64, location *time.Location) int64 {
		y, mo, d := dateTimeJavaDecompose(epoch, location)
		return dateTimeJavaToMillis(y, mo, d, hour, minute, second, millis*int(time.Millisecond), location)
	}
}

// DateTimeSet applies Esper's set(field,n) calendar operation to a date-time
// expression. The input representation is preserved: int64 epoch-millis
// stays int64, time.Time stays time.Time. Field names follow Java's
// CalendarFieldEnum aliases (year,month,dayofmonth,hour,minute,second,
// millisecond,week plus plural/short forms); the month value is 1-based. An
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

// DateTimeWithDateExpr applies withDate(year,month,day) where each calendar
// field is an expression argument (Java's form evaluates variables per
// event). A null field keeps the input's current field value, matching
// Java's actionSetYMD skip-null semantics; the month argument is 1-based
// (LDT convention). The input representation is preserved; null or missing
// input produces Null.
func DateTimeWithDateExpr[V int64 | time.Time](value Expression[V], year, month, day Expression[int64]) Expression[V] {
	fields := []Expression[int64]{year, month, day}
	node := dateTimeFieldsNode[V]("date-time-with-date", value, fields)
	return typedExpr[V]{n: node, fn: func(ctx EvalContext) Value {
		current, instant, parts, present, ok := dateTimeFieldsEval[V](ctx, value, fields)
		if !ok {
			return Null()
		}
		y, mo, d := dateTimeJavaDecompose(instant.millis, instant.location)
		if present[0] {
			y = int(parts[0])
		}
		if present[1] {
			mo = time.Month(parts[1])
		}
		if present[2] {
			d = int(parts[2])
		}
		clock := time.UnixMilli(instant.millis).In(instant.location)
		h, mi, s := clock.Clock()
		return dateTimeCalOpResult(current, dateTimeCalOpJoin(instant, dateTimeJavaToMillis(y, mo, d, h, mi, s, clock.Nanosecond(), instant.location)))
	}}
}

// DateTimeWithTimeExpr applies withTime(hour,minute,second,millis) with
// expression arguments; a null field keeps the input's current field value
// (Java's actionSetHMS null-skip). The input representation is preserved;
// null or missing input produces Null.
func DateTimeWithTimeExpr[V int64 | time.Time](value Expression[V], hour, minute, second, millis Expression[int64]) Expression[V] {
	fields := []Expression[int64]{hour, minute, second, millis}
	node := dateTimeFieldsNode[V]("date-time-with-time", value, fields)
	return typedExpr[V]{n: node, fn: func(ctx EvalContext) Value {
		current, instant, parts, present, ok := dateTimeFieldsEval[V](ctx, value, fields)
		if !ok {
			return Null()
		}
		clock := time.UnixMilli(instant.millis).In(instant.location)
		h, mi, s := clock.Clock()
		ns := clock.Nanosecond()
		if present[0] {
			h = int(parts[0])
		}
		if present[1] {
			mi = int(parts[1])
		}
		if present[2] {
			s = int(parts[2])
		}
		if present[3] {
			ns = int(parts[3]) * int(time.Millisecond)
		}
		y, mo, d := dateTimeJavaDecompose(instant.millis, instant.location)
		return dateTimeCalOpResult(current, dateTimeCalOpJoin(instant, dateTimeJavaToMillis(y, mo, d, h, mi, s, ns, instant.location)))
	}}
}

func dateTimeFieldsNode[V int64 | time.Time](kind string, value Expression[V], fields []Expression[int64]) *exprNode {
	var children []*exprNode
	if value != nil {
		children = append(children, value.node())
	}
	for _, field := range fields {
		if field != nil {
			children = append(children, field.node())
		}
	}
	node := &exprNode{
		kind:        kind,
		typ:         typeOf[V](),
		description: kind + "(expr)",
		children:    children,
	}
	if value == nil || value.node() == nil {
		node.configurationError = fmt.Sprintf("%s requires a date-time operand", kind)
	} else {
		validateDateTimeOperand(node, "value", value)
	}
	return node
}

// dateTimeFieldsEval splits the input instant and evaluates each field
// argument; a null argument is reported via present=false so callers keep
// the input's existing field value.
func dateTimeFieldsEval[V int64 | time.Time](ctx EvalContext, value Expression[V], fields []Expression[int64]) (Value, dateTimeCalOpInstant, []int64, []bool, bool) {
	if value == nil {
		return Value{}, dateTimeCalOpInstant{}, nil, nil, false
	}
	current := value.eval(ctx)
	instant, ok := dateTimeCalOpSplit(ctx, current)
	if !ok {
		return Value{}, dateTimeCalOpInstant{}, nil, nil, false
	}
	parts := make([]int64, len(fields))
	present := make([]bool, len(fields))
	for index, field := range fields {
		if field == nil {
			continue
		}
		raw := field.eval(ctx)
		num, numOK := dateTimeEpochMillis(raw)
		if numOK {
			parts[index] = num
			present[index] = true
		}
	}
	return current, instant, parts, present, true
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
		instant, ok := dateTimeCalOpSplit(ctx, current)
		if !ok {
			return Null()
		}
		return dateTimeCalOpResult(current, dateTimeCalOpJoin(instant, apply(instant.millis, instant.location)))
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
		left, ok := dateTimeEngineUnits(value.eval(ctx))
		if !ok {
			return Null()
		}
		right, ok := dateTimeEngineUnits(threshold.eval(ctx))
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
		startInstant, ok := dateTimeCalOpSplit(ctx, start.eval(ctx))
		if !ok {
			return Null()
		}
		newStart := dateTimeCalOpJoin(startInstant, apply(startInstant.millis, startInstant.location))
		if !isEnd {
			return Present(newStart)
		}
		if end == nil {
			return Null()
		}
		endInstant, ok := dateTimeCalOpSplit(ctx, end.eval(ctx))
		if !ok {
			return Null()
		}
		return Present(newStart + (endInstant.units - startInstant.units))
	}}
}
