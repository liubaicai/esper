package esper

import (
	"fmt"
	"strings"
	"time"
)

// DateTime rounding modes mirroring Java's CalendarForgeRound: ceiling,
// floor, and the Apache Commons DateUtils.modify(MODIFY_ROUND) half-up
// semantics whose month carry is month-length dependent.
type dateTimeRoundMode int

const (
	dateTimeRoundCeiling dateTimeRoundMode = iota
	dateTimeRoundFloor
	dateTimeRoundHalf
)

// DateTimeRoundCeiling advances a date-time expression to the next unit
// boundary (Commons MODIFY_CEILING adds one target unit unconditionally, so
// an on-boundary input advances). The input representation is preserved:
// int64 epoch-millis stays int64, time.Time stays time.Time. Accepted units
// follow CalendarFieldEnum aliases: msec, sec, minutes, min, hour, day,
// month, year; msec is the identity and week errors at evaluation like
// Java's unsupported-field path.
func DateTimeRoundCeiling[T int64 | time.Time](value Expression[T], unit string) Expression[T] {
	return dateTimeRoundExpression[T]("date-time-round-ceiling", value, unit, dateTimeRoundCeiling)
}

// DateTimeRoundFloor truncates a date-time expression down to the unit
// boundary, preserving the input representation.
func DateTimeRoundFloor[T int64 | time.Time](value Expression[T], unit string) Expression[T] {
	return dateTimeRoundExpression[T]("date-time-round-floor", value, unit, dateTimeRoundFloor)
}

// DateTimeRoundHalf rounds a date-time expression to the nearest unit
// boundary with exact ties rounding up. Semantics mirror Apache Commons
// DateUtils.modify(MODIFY_ROUND): msec is the identity, sub-second
// pre-passes keep millis >= 500, seconds < 30 and minutes < 30 drop, and
// the month carry compares the day offset against the actual month length
// (31d: day >= 17, 30d: >= 16, Feb28: >= 15, Feb29: >= 16).
func DateTimeRoundHalf[T int64 | time.Time](value Expression[T], unit string) Expression[T] {
	return dateTimeRoundExpression[T]("date-time-round-half", value, unit, dateTimeRoundHalf)
}

func dateTimeRoundExpression[T int64 | time.Time](kind string, value Expression[T], unit string, mode dateTimeRoundMode) Expression[T] {
	var children []*exprNode
	if value != nil {
		children = []*exprNode{value.node()}
	}
	node := &exprNode{
		kind:        kind,
		typ:         typeOf[T](),
		description: fmt.Sprintf("%s(%s,'%s')", kind, expressionDescription(value), unit),
		children:    children,
	}
	if value == nil || value.node() == nil {
		node.configurationError = fmt.Sprintf("%s requires a date-time operand", kind)
		return typedExpr[T]{n: node, fn: func(EvalContext) Value { return Null() }}
	}
	if _, err := dateTimeRoundUnit(unit); err != nil {
		node.configurationError = err.Error()
		return typedExpr[T]{n: node, fn: func(EvalContext) Value { return Null() }}
	}
	return typedExpr[T]{n: node, fn: func(ctx EvalContext) Value {
		current := value.eval(ctx)
		millis, ok := dateTimeEpochMillis(current)
		if !ok {
			return Null()
		}
		// Calendar semantics derive fields in the value's own zone; epoch
		// millis carry no zone, so they round in UTC (the pinned harness
		// convention matching -Duser.timezone=UTC).
		location := time.UTC
		if _, isTime := current.Any().(time.Time); isTime {
			if located, ok2 := dateTimeValueLocation(current); ok2 {
				location = located
			}
		}

		rounded, err := roundDateTimeMillis(millis, unit, mode, location)
		if err != nil {
			node.configurationError = err.Error()
			return Null()
		}
		switch current.Any().(type) {
		case int64:
			return Present(rounded)
		default:
			return Present(time.UnixMilli(rounded))
		}
	}}
}

func dateTimeRoundUnit(unit string) (int, error) {
	switch normalizeDateTimeUnit(unit) {
	case "msec":
		return 0, nil
	case "sec":
		return 1, nil
	case "min":
		return 2, nil
	case "hour":
		return 3, nil
	case "day":
		return 4, nil
	case "week":
		return 5, nil
	case "month":
		return 6, nil
	case "year":
		return 7, nil
	default:
		return -1, fmt.Errorf("unknown date-time rounding unit %q", unit)
	}
}

func normalizeDateTimeUnit(unit string) string {
	alias := strings.ToLower(strings.TrimSpace(unit))
	switch alias {
	case "milliseconds", "millisecond":
		return "msec"
	case "seconds", "second":
		return "sec"
	case "minutes", "minute":
		return "min"
	case "hours":
		return "hour"
	case "days":
		return "day"
	case "weeks":
		return "week"
	case "months":
		return "month"
	case "years":
		return "year"
	}
	return alias
}

// roundDateTimeMillis mirrors Apache Commons DateUtils.modify (the pinned
// CalendarForgeRound delegate) field-by-field: the LANG-59 pre-pass
// subtracts sub-threshold millis/seconds/minutes from the absolute time
// (CEILING keeps millis >= 500, TRUNCATE always drops, ROUND drops < 500),
// then rows walk bottom-up zeroing each field past its half-way offset and
// finally add one target unit for CEILING always and for ROUND when the
// last row before the target rounded up. WEEK is not in the Commons matrix
// and errors at evaluation exactly like Java's "The field 3 is not
// supported".
func roundDateTimeMillis(millis int64, unit string, mode dateTimeRoundMode, location *time.Location) (int64, error) {
	unitIndex, err := dateTimeRoundUnit(unit)
	if err != nil {
		return 0, err
	}
	if unitIndex == 5 {
		// Calendar.WEEK_OF_YEAR is absent from the Commons FIELDS matrix.
		return 0, fmt.Errorf("The field 3 is not supported")
	}
	if unitIndex == 0 {
		// modify() early-returns for MILLISECOND in every mode; values are
		// already at millisecond precision so all three modes are identity.
		return millis, nil
	}
	loc := time.UTC
	if location != nil {
		loc = location
	}
	t := time.UnixMilli(millis).In(loc)
	msec := t.Nanosecond() / 1e6
	second := t.Second()
	minute := t.Minute()

	// LANG-59 pre-pass on the absolute time.
	adjusted := millis
	if mode == dateTimeRoundFloor || msec < 500 {
		adjusted -= int64(msec)
	}
	done := unitIndex == 1
	if !done && (mode == dateTimeRoundFloor || second < 30) {
		adjusted -= int64(second) * 1000
	}
	done = done || unitIndex == 2
	if !done && (mode == dateTimeRoundFloor || minute < 30) {
		adjusted -= int64(minute) * 60000
	}
	cur := time.UnixMilli(adjusted).In(loc)

	// Row walk bottom-up; each row carries (read field, minimum, maximum,
	// add-one step). getActualMaximum for DATE is month-length aware.
	type row struct {
		field    int
		get      func(time.Time) int
		min      func(time.Time) int
		max      func(time.Time) int
		zero     func(time.Time, int) time.Time
		addOne   func(time.Time) time.Time
		isTarget bool
	}
	lastDay := func(of time.Time) int {
		return time.Date(of.Year(), of.Month()+1, 0, 0, 0, 0, 0, loc).Day()
	}
	rows := []row{
		{
			field: 0,
			get:   func(v time.Time) int { return v.Nanosecond() / 1e6 },
			min:   func(time.Time) int { return 0 },
			max:   func(time.Time) int { return 999 },
			zero: func(v time.Time, value int) time.Time {
				return time.Date(v.Year(), v.Month(), v.Day(), v.Hour(), v.Minute(), v.Second(), value*1e6, loc)
			},
			addOne: func(v time.Time) time.Time { return v.Add(time.Millisecond) },
		},
		{
			field: 1,
			get:   func(v time.Time) int { return v.Second() },
			min:   func(time.Time) int { return 0 },
			max:   func(time.Time) int { return 59 },
			zero: func(v time.Time, value int) time.Time {
				y, m, d := v.Date()
				h, i, _ := v.Clock()
				return time.Date(y, m, d, h, i, value, 0, loc)
			},
			addOne: func(v time.Time) time.Time { return v.Add(time.Second) },
		},
		{
			field: 2,
			get:   func(v time.Time) int { return v.Minute() },
			min:   func(time.Time) int { return 0 },
			max:   func(time.Time) int { return 59 },
			zero: func(v time.Time, value int) time.Time {
				y, m, d := v.Date()
				h, _, _ := v.Clock()
				return time.Date(y, m, d, h, value, 0, 0, loc)
			},
			addOne: func(v time.Time) time.Time { return v.Add(time.Minute) },
		},
		{
			field: 3,
			get:   func(v time.Time) int { return v.Hour() },
			min:   func(time.Time) int { return 0 },
			max:   func(time.Time) int { return 23 },
			zero: func(v time.Time, value int) time.Time {
				y, m, d := v.Date()
				return time.Date(y, m, d, value, 0, 0, 0, loc)
			},
			addOne: func(v time.Time) time.Time { return v.Add(time.Hour) },
		},
		{
			field: 4,
			get:   func(v time.Time) int { return v.Day() },
			min:   func(time.Time) int { return 1 },
			max:   func(v time.Time) int { return lastDay(v) },
			zero: func(v time.Time, value int) time.Time {
				y, m, _ := v.Date()
				h, i, sec := v.Clock()
				return time.Date(y, m, value, h, i, sec, 0, loc)
			},
			addOne: func(v time.Time) time.Time { return v.AddDate(0, 0, 1) },
		},
		{
			field: 6,
			get:   func(v time.Time) int { return int(v.Month()) - 1 },
			min:   func(time.Time) int { return 0 },
			max:   func(time.Time) int { return 11 },
			zero: func(v time.Time, value int) time.Time {
				y, _, d := v.Date()
				h, i, sec := v.Clock()
				return time.Date(y, time.Month(value+1), d, h, i, sec, 0, loc)
			},
			addOne: func(v time.Time) time.Time { return v.AddDate(0, 1, 0) },
		},
		{
			field: 7,
			get:   func(v time.Time) int { return v.Year() },
			min:   func(v time.Time) int { return v.Year() },
			max:   func(v time.Time) int { return v.Year() },
			zero: func(v time.Time, value int) time.Time {
				m, d := v.Month(), v.Day()
				h, i, sec := v.Clock()
				return time.Date(value, m, d, h, i, sec, 0, loc)
			},
			addOne: func(v time.Time) time.Time { return v.AddDate(1, 0, 0) },
		},
	}

	roundUp := false
	for _, current := range rows {
		if current.field == unitIndex {
			if mode == dateTimeRoundCeiling || (mode == dateTimeRoundHalf && roundUp) {
				cur = current.addOne(cur)
			}
			return cur.UnixMilli(), nil
		}
		minimum := current.min(cur)
		maximum := current.max(cur)
		offset := current.get(cur) - minimum
		roundUp = offset > (maximum-minimum)/2
		if offset != 0 {
			cur = current.zero(cur, current.get(cur)-offset)
		}
	}
	return cur.UnixMilli(), nil
}

// dateTimeValueLocation extracts the zone of a time.Time-valued datetime.
func dateTimeValueLocation(value Value) (*time.Location, bool) {
	if located, ok := value.Any().(time.Time); ok {
		if located.Location() != nil {
			return located.Location(), true
		}
		return time.UTC, true
	}
	return time.UTC, false
}
