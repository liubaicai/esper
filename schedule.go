package esper

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// CronField describes one calendar field of a CronSchedule.  It is built by
// the constructors below rather than by parsing a cron string, keeping the
// rule definition type-safe and analyzable.
type CronField struct {
	kind       cronFieldKind
	values     []int
	step       int
	start      int
	end        int
	stepExpr   Expr
	startExpr  Expr
	endExpr    Expr
	valueExprs []Expr
}

type cronFieldKind uint8

const (
	cronFieldWildcard cronFieldKind = iota
	cronFieldValues
	cronFieldStep
	cronFieldRange
	cronFieldStepExpr
	cronFieldRangeExpr
	cronFieldValuesExpr
)

// CronWildcard matches every value in the field.
func CronWildcard() CronField { return CronField{kind: cronFieldWildcard} }

// CronEvery matches the field minimum and every step after it.  For example,
// CronEvery(15) in the minute field matches 0, 15, 30 and 45.
func CronEvery(step int) CronField {
	return CronField{kind: cronFieldStep, step: step}
}

// CronRange matches an inclusive range.
func CronRange(start, end int) CronField {
	return CronField{kind: cronFieldRange, start: start, end: end}
}

// CronValues matches the supplied explicit values.
func CronValues(values ...int) CronField {
	return CronField{kind: cronFieldValues, values: append([]int(nil), values...)}
}

// CronEveryExpr is the variable/parameter form of CronEvery.  The expression
// is evaluated once when the statement is instantiated, matching Esper's
// schedule construction contract.
func CronEveryExpr(step Expr) CronField {
	return CronField{kind: cronFieldStepExpr, stepExpr: step}
}

// CronRangeExpr is the variable/parameter form of CronRange.
func CronRangeExpr(start, end Expr) CronField {
	return CronField{kind: cronFieldRangeExpr, startExpr: start, endExpr: end}
}

// CronValuesExpr is the expression form of CronValues.  Each expression must
// evaluate to one integral value when the statement is instantiated.
func CronValuesExpr(values ...Expr) CronField {
	return CronField{kind: cronFieldValuesExpr, valueExprs: append([]Expr(nil), values...)}
}

// CronSchedule is a calendar schedule in the same order as Esper's
// output-at form: minute, hour, day-of-month, month, day-of-week. Second,
// millisecond and microsecond precision are opt-in so existing five-field
// schedules keep their historical minute-boundary behavior. A zero-valued
// CronField is a wildcard, so a schedule can also be assembled with a struct
// literal when all unspecified fields should match.
type CronSchedule struct {
	Minute         CronField
	Hour           CronField
	DayOfMonth     CronField
	Month          CronField
	Weekday        CronField
	Second         CronField
	Millisecond    CronField
	Microsecond    CronField
	secondSet      bool
	millisecondSet bool
	microsecondSet bool
}

// NewCronSchedule is a named constructor for callers that prefer not to use a
// struct literal in a rule definition.
func NewCronSchedule(minute, hour, dayOfMonth, month, weekday CronField) CronSchedule {
	return CronSchedule{
		Minute:     minute,
		Hour:       hour,
		DayOfMonth: dayOfMonth,
		Month:      month,
		Weekday:    weekday,
	}
}

// NewCronScheduleWithSeconds enables the optional seconds field. The default
// millisecond value remains zero, matching a calendar clock with millisecond
// precision but no explicit millisecond selector.
func NewCronScheduleWithSeconds(second, minute, hour, dayOfMonth, month, weekday CronField) CronSchedule {
	return CronSchedule{
		Minute: minute, Hour: hour, DayOfMonth: dayOfMonth, Month: month, Weekday: weekday,
		Second: second, secondSet: true,
	}
}

// NewCronScheduleWithMilliseconds enables both seconds and milliseconds.
// Supplying CronWildcard for either field is valid and means every value in
// that field.
func NewCronScheduleWithMilliseconds(milliseconds, seconds, minute, hour, dayOfMonth, month, weekday CronField) CronSchedule {
	return CronSchedule{
		Minute: minute, Hour: hour, DayOfMonth: dayOfMonth, Month: month, Weekday: weekday,
		Second: seconds, Millisecond: milliseconds, secondSet: true, millisecondSet: true,
	}
}

// NewCronScheduleWithMicroseconds enables seconds, milliseconds and
// microseconds. The microsecond field is the 0..999 remainder within a
// millisecond, matching Esper's timer:at microsecond form and preserving
// time.Time nanosecond precision in the Go runtime.
func NewCronScheduleWithMicroseconds(microseconds, milliseconds, seconds, minute, hour, dayOfMonth, month, weekday CronField) CronSchedule {
	return CronSchedule{
		Minute: minute, Hour: hour, DayOfMonth: dayOfMonth, Month: month, Weekday: weekday,
		Second: seconds, Millisecond: milliseconds, Microsecond: microseconds,
		secondSet: true, millisecondSet: true, microsecondSet: true,
	}
}

// WithSeconds returns a copy with second-level scheduling enabled. It is
// useful when callers prefer a fluent rule definition over a constructor.
func (schedule CronSchedule) WithSeconds(field CronField) CronSchedule {
	schedule.Second = field
	schedule.secondSet = true
	return schedule
}

// WithMilliseconds returns a copy with millisecond-level scheduling enabled.
// If seconds were not selected explicitly, the schedule is anchored at
// second zero.
func (schedule CronSchedule) WithMilliseconds(field CronField) CronSchedule {
	if !schedule.secondSet && schedule.Second.isWildcard() {
		schedule.Second = CronValues(0)
		schedule.secondSet = true
	}
	schedule.Millisecond = field
	schedule.millisecondSet = true
	return schedule
}

// WithMicroseconds returns a copy with microsecond-level scheduling enabled.
// Milliseconds are anchored at zero unless already selected explicitly.
func (schedule CronSchedule) WithMicroseconds(field CronField) CronSchedule {
	if !schedule.millisecondSet && schedule.Millisecond.isWildcard() {
		schedule.Millisecond = CronValues(0)
		schedule.millisecondSet = true
	}
	if !schedule.secondSet && schedule.Second.isWildcard() {
		schedule.Second = CronValues(0)
		schedule.secondSet = true
	}
	schedule.Microsecond = field
	schedule.microsecondSet = true
	return schedule
}

type resolvedCronField struct {
	wildcard bool
	values   []int
}

type resolvedCronSchedule struct {
	minute      resolvedCronField
	hour        resolvedCronField
	dayOfMonth  resolvedCronField
	month       resolvedCronField
	weekday     resolvedCronField
	second      resolvedCronField
	millisecond resolvedCronField
	microsecond resolvedCronField
}

func (schedule CronSchedule) hasSeconds() bool {
	return schedule.secondSet || !schedule.Second.isWildcard()
}

func (schedule CronSchedule) hasMilliseconds() bool {
	return schedule.millisecondSet || !schedule.Millisecond.isWildcard()
}

func (schedule CronSchedule) hasMicroseconds() bool {
	return schedule.microsecondSet || !schedule.Microsecond.isWildcard()
}

// OutputAt emits the accumulated result at each matching calendar instant.
// The default result policy is OutputAll; a base policy can be supplied when
// the caller needs last/snapshot/first behavior at each schedule tick.
func OutputAt(schedule CronSchedule, base ...OutputPolicy) OutputPolicy {
	policy := outputBasePolicy(base)
	copySchedule := schedule
	policy.Cron = &copySchedule
	return policy
}

func (field CronField) isWildcard() bool {
	return field.kind == cronFieldWildcard
}

func (field CronField) expressions() []Expr {
	switch field.kind {
	case cronFieldStepExpr:
		return []Expr{field.stepExpr}
	case cronFieldRangeExpr:
		return []Expr{field.startExpr, field.endExpr}
	case cronFieldValuesExpr:
		return append([]Expr(nil), field.valueExprs...)
	default:
		return nil
	}
}

func (field CronField) validate(minimum, maximum int, label string) error {
	switch field.kind {
	case cronFieldWildcard:
		return nil
	case cronFieldValues:
		if len(field.values) == 0 {
			return NewError(ErrorInvalidRule, fmt.Sprintf("cron %s values cannot be empty", label))
		}
		for _, value := range field.values {
			if value < minimum || value > maximum {
				return NewError(ErrorInvalidRule, fmt.Sprintf("cron %s value %d is outside %d..%d", label, value, minimum, maximum))
			}
		}
	case cronFieldStep:
		if field.step <= 0 {
			return NewError(ErrorInvalidRule, fmt.Sprintf("cron %s step must be positive", label))
		}
	case cronFieldRange:
		if field.start < minimum || field.start > maximum || field.end < minimum || field.end > maximum || field.start > field.end {
			return NewError(ErrorInvalidRule, fmt.Sprintf("cron %s range must be an inclusive range within %d..%d", label, minimum, maximum))
		}
	case cronFieldStepExpr:
		if field.stepExpr == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("cron %s step expression is required", label))
		}
	case cronFieldRangeExpr:
		if field.startExpr == nil || field.endExpr == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("cron %s range expressions are required", label))
		}
	case cronFieldValuesExpr:
		if len(field.valueExprs) == 0 {
			return NewError(ErrorInvalidRule, fmt.Sprintf("cron %s value expressions cannot be empty", label))
		}
		for _, expression := range field.valueExprs {
			if expression == nil {
				return NewError(ErrorInvalidRule, fmt.Sprintf("cron %s contains a nil value expression", label))
			}
		}
	default:
		return NewError(ErrorInvalidRule, fmt.Sprintf("unknown cron %s field form", label))
	}
	return nil
}

func (field CronField) resolve(ctx EvalContext, minimum, maximum int, label string) (resolvedCronField, error) {
	if field.kind == cronFieldWildcard {
		return resolvedCronField{wildcard: true}, nil
	}
	values := make([]int, 0)
	switch field.kind {
	case cronFieldValues:
		values = append(values, field.values...)
	case cronFieldStep:
		for value := minimum; value <= maximum; value += field.step {
			values = append(values, value)
		}
	case cronFieldRange:
		for value := field.start; value <= field.end; value++ {
			values = append(values, value)
		}
	case cronFieldStepExpr:
		step, err := evalCronInt(field.stepExpr, ctx, label+" step")
		if err != nil {
			return resolvedCronField{}, err
		}
		if step <= 0 {
			return resolvedCronField{}, NewError(ErrorInvalidRule, fmt.Sprintf("cron %s step must be positive", label))
		}
		for value := minimum; value <= maximum; value += step {
			values = append(values, value)
		}
	case cronFieldRangeExpr:
		start, err := evalCronInt(field.startExpr, ctx, label+" start")
		if err != nil {
			return resolvedCronField{}, err
		}
		end, err := evalCronInt(field.endExpr, ctx, label+" end")
		if err != nil {
			return resolvedCronField{}, err
		}
		if start < minimum || start > maximum || end < minimum || end > maximum || start > end {
			return resolvedCronField{}, NewError(ErrorInvalidRule, fmt.Sprintf("cron %s range must be an inclusive range within %d..%d", label, minimum, maximum))
		}
		for value := start; value <= end; value++ {
			values = append(values, value)
		}
	case cronFieldValuesExpr:
		for index, expression := range field.valueExprs {
			value, err := evalCronInt(expression, ctx, fmt.Sprintf("%s value %d", label, index))
			if err != nil {
				return resolvedCronField{}, err
			}
			values = append(values, value)
		}
	default:
		return resolvedCronField{}, NewError(ErrorInvalidRule, fmt.Sprintf("unknown cron %s field form", label))
	}
	for _, value := range values {
		if value < minimum || value > maximum {
			return resolvedCronField{}, NewError(ErrorInvalidRule, fmt.Sprintf("cron %s value %d is outside %d..%d", label, value, minimum, maximum))
		}
	}
	if label == "weekday" {
		for index, value := range values {
			// Esper accepts both 0 and 7 for Sunday.  Internally time.Weekday
			// uses 0, so normalize the alias before scheduling.
			if value == 7 {
				values[index] = 0
			}
		}
	}
	sort.Ints(values)
	unique := values[:0]
	for _, value := range values {
		if len(unique) == 0 || unique[len(unique)-1] != value {
			unique = append(unique, value)
		}
	}
	return resolvedCronField{values: append([]int(nil), unique...)}, nil
}

func evalCronInt(expression Expr, ctx EvalContext, label string) (int, error) {
	if expression == nil {
		return 0, NewError(ErrorInvalidRule, fmt.Sprintf("cron %s expression is required", label))
	}
	number, ok := numericValue(expression.eval(ctx))
	if !ok || math.IsNaN(number) || math.IsInf(number, 0) || math.Trunc(number) != number || number < math.MinInt32 || number > math.MaxInt32 {
		return 0, NewError(ErrorTypeMismatch, fmt.Sprintf("cron %s expression must evaluate to an integer", label))
	}
	return int(number), nil
}

func (schedule CronSchedule) validate() error {
	if err := schedule.Minute.validate(0, 59, "minute"); err != nil {
		return err
	}
	if err := schedule.Hour.validate(0, 23, "hour"); err != nil {
		return err
	}
	if err := schedule.DayOfMonth.validate(1, 31, "day-of-month"); err != nil {
		return err
	}
	if err := schedule.Month.validate(1, 12, "month"); err != nil {
		return err
	}
	// Java's ScheduleSpec uses 0..6 and accepts 7 as a Sunday alias.
	if err := schedule.Weekday.validate(0, 7, "weekday"); err != nil {
		return err
	}
	if schedule.hasSeconds() {
		if err := schedule.Second.validate(0, 59, "second"); err != nil {
			return err
		}
	}
	if schedule.hasMilliseconds() {
		if err := schedule.Millisecond.validate(0, 999, "millisecond"); err != nil {
			return err
		}
	}
	if schedule.hasMicroseconds() {
		if err := schedule.Microsecond.validate(0, 999, "microsecond"); err != nil {
			return err
		}
	}
	return nil
}

func (e *Environment) validateCronSchedule(schedule *CronSchedule) error {
	if schedule == nil {
		return nil
	}
	if err := schedule.validate(); err != nil {
		return err
	}
	fields := []CronField{schedule.Minute, schedule.Hour, schedule.DayOfMonth, schedule.Month, schedule.Weekday}
	labels := []string{"minute", "hour", "day-of-month", "month", "weekday"}
	if schedule.hasSeconds() {
		fields = append(fields, schedule.Second)
		labels = append(labels, "second")
	}
	if schedule.hasMilliseconds() {
		fields = append(fields, schedule.Millisecond)
		labels = append(labels, "millisecond")
	}
	if schedule.hasMicroseconds() {
		fields = append(fields, schedule.Microsecond)
		labels = append(labels, "microsecond")
	}
	for index, field := range fields {
		for _, expression := range field.expressions() {
			if expression == nil {
				return NewError(ErrorInvalidRule, fmt.Sprintf("cron %s expression is required", labels[index]))
			}
			var referencedFields []string
			expression.node().referencedFields(&referencedFields)
			if len(referencedFields) > 0 {
				return NewError(ErrorInvalidRule, fmt.Sprintf("cron %s expressions cannot reference event fields", labels[index]))
			}
			if err := e.validateExprVariables(expression); err != nil {
				return err
			}
		}
	}
	return nil
}

func (schedule CronSchedule) resolve(ctx EvalContext) (resolvedCronSchedule, error) {
	minute, err := schedule.Minute.resolve(ctx, 0, 59, "minute")
	if err != nil {
		return resolvedCronSchedule{}, err
	}
	hour, err := schedule.Hour.resolve(ctx, 0, 23, "hour")
	if err != nil {
		return resolvedCronSchedule{}, err
	}
	dayOfMonth, err := schedule.DayOfMonth.resolve(ctx, 1, 31, "day-of-month")
	if err != nil {
		return resolvedCronSchedule{}, err
	}
	month, err := schedule.Month.resolve(ctx, 1, 12, "month")
	if err != nil {
		return resolvedCronSchedule{}, err
	}
	weekday, err := schedule.Weekday.resolve(ctx, 0, 7, "weekday")
	if err != nil {
		return resolvedCronSchedule{}, err
	}
	if !weekday.wildcard {
		for index, value := range weekday.values {
			if value == 7 {
				weekday.values[index] = 0
			}
		}
		sort.Ints(weekday.values)
	}
	second := resolvedCronField{values: []int{0}}
	if schedule.hasSeconds() {
		second, err = schedule.Second.resolve(ctx, 0, 59, "second")
		if err != nil {
			return resolvedCronSchedule{}, err
		}
	}
	millisecond := resolvedCronField{values: []int{0}}
	if schedule.hasMilliseconds() {
		millisecond, err = schedule.Millisecond.resolve(ctx, 0, 999, "millisecond")
		if err != nil {
			return resolvedCronSchedule{}, err
		}
	}
	microsecond := resolvedCronField{values: []int{0}}
	if schedule.hasMicroseconds() {
		microsecond, err = schedule.Microsecond.resolve(ctx, 0, 999, "microsecond")
		if err != nil {
			return resolvedCronSchedule{}, err
		}
	}
	return resolvedCronSchedule{minute: minute, hour: hour, dayOfMonth: dayOfMonth, month: month, weekday: weekday, second: second, millisecond: millisecond, microsecond: microsecond}, nil
}

func (field resolvedCronField) candidates(minimum, maximum int) []int {
	if field.wildcard {
		values := make([]int, 0, maximum-minimum+1)
		for value := minimum; value <= maximum; value++ {
			values = append(values, value)
		}
		return values
	}
	return field.values
}

func (field resolvedCronField) matches(value int) bool {
	if field.wildcard {
		return true
	}
	index := sort.SearchInts(field.values, value)
	return index < len(field.values) && field.values[index] == value
}

// nextAfter returns the first schedule instant strictly after after. The
// search is hierarchical (year/month/day/hour/minute/second/millisecond/microsecond)
// rather than a fixed-duration scan, which keeps sparse schedules such as
// February 29 practical while retaining sub-second precision.
func (schedule resolvedCronSchedule) nextAfter(after time.Time) (time.Time, error) {
	if after.IsZero() {
		after = time.Unix(0, 0).UTC()
	}
	location := after.Location()
	if location == nil {
		location = time.UTC
	}
	localAfter := after.In(location)
	months := schedule.month.candidates(1, 12)
	for year := localAfter.Year(); year <= localAfter.Year()+400; year++ {
		for _, month := range months {
			if year == localAfter.Year() && month < int(localAfter.Month()) {
				continue
			}
			first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, location)
			daysInMonth := first.AddDate(0, 1, -1).Day()
			for day := 1; day <= daysInMonth; day++ {
				if year == localAfter.Year() && month == int(localAfter.Month()) && day < localAfter.Day() {
					continue
				}
				candidateDate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, location)
				if !schedule.dayMatches(candidateDate) {
					continue
				}
				for _, hour := range schedule.hour.candidates(0, 23) {
					if year == localAfter.Year() && month == int(localAfter.Month()) && day == localAfter.Day() && hour < localAfter.Hour() {
						continue
					}
					for _, minute := range schedule.minute.candidates(0, 59) {
						for _, second := range schedule.second.candidates(0, 59) {
							for _, millisecond := range schedule.millisecond.candidates(0, 999) {
								for _, microsecond := range schedule.microsecond.candidates(0, 999) {
									candidate := time.Date(year, time.Month(month), day, hour, minute, second, millisecond*int(time.Millisecond)+microsecond*int(time.Microsecond), location)
									if candidate.After(localAfter) {
										return candidate, nil
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return time.Time{}, NewError(ErrorInvalidRule, "cron schedule has no occurrence within 400 years")
}

// previousOrAt returns the latest calendar occurrence at or before at. The
// search mirrors nextAfter from the most significant field down, so sparse
// schedules do not require a millisecond-by-millisecond scan.
func (schedule resolvedCronSchedule) previousOrAt(at time.Time) (time.Time, error) {
	if at.IsZero() {
		at = time.Unix(0, 0).UTC()
	}
	location := at.Location()
	if location == nil {
		location = time.UTC
	}
	localAt := at.In(location)
	months := reverseInts(schedule.month.candidates(1, 12))
	for year := localAt.Year(); year >= localAt.Year()-400; year-- {
		for _, month := range months {
			if year == localAt.Year() && month > int(localAt.Month()) {
				continue
			}
			first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, location)
			daysInMonth := first.AddDate(0, 1, -1).Day()
			for day := daysInMonth; day >= 1; day-- {
				if year == localAt.Year() && month == int(localAt.Month()) && day > localAt.Day() {
					continue
				}
				candidateDate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, location)
				if !schedule.dayMatches(candidateDate) {
					continue
				}
				for _, hour := range reverseInts(schedule.hour.candidates(0, 23)) {
					if year == localAt.Year() && month == int(localAt.Month()) && day == localAt.Day() && hour > localAt.Hour() {
						continue
					}
					for _, minute := range reverseInts(schedule.minute.candidates(0, 59)) {
						if year == localAt.Year() && month == int(localAt.Month()) && day == localAt.Day() && hour == localAt.Hour() && minute > localAt.Minute() {
							continue
						}
						for _, second := range reverseInts(schedule.second.candidates(0, 59)) {
							if year == localAt.Year() && month == int(localAt.Month()) && day == localAt.Day() && hour == localAt.Hour() && minute == localAt.Minute() && second > localAt.Second() {
								continue
							}
							for _, millisecond := range reverseInts(schedule.millisecond.candidates(0, 999)) {
								for _, microsecond := range reverseInts(schedule.microsecond.candidates(0, 999)) {
									nanosecond := millisecond*int(time.Millisecond) + microsecond*int(time.Microsecond)
									if year == localAt.Year() && month == int(localAt.Month()) && day == localAt.Day() && hour == localAt.Hour() && minute == localAt.Minute() && second == localAt.Second() && nanosecond > localAt.Nanosecond() {
										continue
									}
									candidate := time.Date(year, time.Month(month), day, hour, minute, second, nanosecond, location)
									if !candidate.After(localAt) {
										return candidate, nil
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return time.Time{}, NewError(ErrorInvalidRule, "cron schedule has no occurrence within 400 years")
}

func reverseInts(values []int) []int {
	result := append([]int(nil), values...)
	sort.Sort(sort.Reverse(sort.IntSlice(result)))
	return result
}

func (schedule resolvedCronSchedule) dayMatches(date time.Time) bool {
	dayOfMonthMatch := schedule.dayOfMonth.matches(date.Day())
	weekday := int(date.Weekday())
	weekdayMatch := schedule.weekday.matches(weekday)
	if !schedule.weekday.wildcard {
		// The resolver normalizes Sunday 7 to 0, but keep this alias here for
		// defensive compatibility with manually constructed resolved values.
		weekdayMatch = weekdayMatch || (weekday == 0 && schedule.weekday.matches(7))
	}
	if schedule.dayOfMonth.wildcard && schedule.weekday.wildcard {
		return true
	}
	if schedule.dayOfMonth.wildcard {
		return weekdayMatch
	}
	if schedule.weekday.wildcard {
		return dayOfMonthMatch
	}
	// Esper ORs explicit day-of-month and day-of-week sets.
	return dayOfMonthMatch || weekdayMatch
}

func (field CronField) description() string {
	switch field.kind {
	case cronFieldWildcard:
		return "*"
	case cronFieldValues:
		parts := make([]string, 0, len(field.values))
		for _, value := range field.values {
			parts = append(parts, fmt.Sprint(value))
		}
		return strings.Join(parts, ",")
	case cronFieldStep:
		return fmt.Sprintf("*/%d", field.step)
	case cronFieldRange:
		return fmt.Sprintf("%d:%d", field.start, field.end)
	case cronFieldStepExpr:
		return "*/" + cronExprDescription(field.stepExpr)
	case cronFieldRangeExpr:
		return cronExprDescription(field.startExpr) + ":" + cronExprDescription(field.endExpr)
	case cronFieldValuesExpr:
		parts := make([]string, 0, len(field.valueExprs))
		for _, expression := range field.valueExprs {
			parts = append(parts, cronExprDescription(expression))
		}
		return strings.Join(parts, ",")
	default:
		return "<invalid>"
	}
}

func cronExprDescription(expression Expr) string {
	if expression == nil {
		return "<nil>"
	}
	return expression.Description()
}

func (schedule CronSchedule) description() string {
	parts := []string{
		schedule.Minute.description(),
		schedule.Hour.description(),
		schedule.DayOfMonth.description(),
		schedule.Month.description(),
		schedule.Weekday.description(),
	}
	if schedule.hasSeconds() {
		parts = append(parts, schedule.Second.description())
	}
	if schedule.hasMilliseconds() {
		parts = append(parts, schedule.Millisecond.description())
	}
	if schedule.hasMicroseconds() {
		parts = append(parts, schedule.Microsecond.description())
	}
	return strings.Join(parts, ",")
}

func visitCronScheduleExpressions(schedule *CronSchedule, visit func(Expr) error) error {
	if schedule == nil || visit == nil {
		return nil
	}
	fields := []CronField{schedule.Minute, schedule.Hour, schedule.DayOfMonth, schedule.Month, schedule.Weekday}
	if schedule.hasSeconds() {
		fields = append(fields, schedule.Second)
	}
	if schedule.hasMilliseconds() {
		fields = append(fields, schedule.Millisecond)
	}
	if schedule.hasMicroseconds() {
		fields = append(fields, schedule.Microsecond)
	}
	for _, field := range fields {
		for _, expression := range field.expressions() {
			if err := visit(expression); err != nil {
				return err
			}
		}
	}
	return nil
}
