package esper

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// patternTimerScheduleSpec is the normalized representation shared by the
// typed schedule builder and the ISO-8601 migration adapter.
type patternTimerScheduleSpec struct {
	start        time.Time
	hasStart     bool
	period       PatternTimerPeriod
	hasPeriod    bool
	repetitions  int64
	includeStart bool
}

// patternTimerScheduleRuntime is statement-local schedule state. The
// definition may be shared by deployments and by overlapping pattern matches,
// so the next occurrence and emitted count must never live on patternNode.
type patternTimerScheduleRuntime struct {
	next        time.Time
	period      PatternTimerPeriod
	repetitions int64
	emitted     int64
	active      bool
}

var patternISO8601PeriodPattern = regexp.MustCompile(`^P(?:(\d+)Y)?(?:(\d+)M)?(?:(\d+)W)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$`)

func patternTimerPeriodDescription(period PatternTimerPeriod) string {
	parts := make([]string, 0, 4)
	if period.Years != 0 {
		parts = append(parts, fmt.Sprintf("%dY", period.Years))
	}
	if period.Months != 0 {
		parts = append(parts, fmt.Sprintf("%dM", period.Months))
	}
	if period.Days != 0 {
		parts = append(parts, fmt.Sprintf("%dD", period.Days))
	}
	if period.FixedDuration != 0 {
		parts = append(parts, period.FixedDuration.String())
	}
	if len(parts) == 0 {
		return "<empty-period>"
	}
	return strings.Join(parts, "+")
}

func validatePatternTimerPeriod(period PatternTimerPeriod) error {
	if period.Years < 0 || period.Months < 0 || period.Days < 0 {
		return NewError(ErrorInvalidRule, "timer schedule calendar period cannot be negative")
	}
	if period.FixedDuration < 0 {
		return NewError(ErrorInvalidRule, "timer schedule fixed duration cannot be negative")
	}
	if period.Years == 0 && period.Months == 0 && period.Days == 0 && period.FixedDuration == 0 {
		return NewError(ErrorInvalidRule, "timer schedule period must be positive")
	}
	return nil
}

func patternTimerPeriodDeadline(at time.Time, period PatternTimerPeriod) (time.Time, bool) {
	if err := validatePatternTimerPeriod(period); err != nil {
		return time.Time{}, false
	}
	deadline := at.AddDate(period.Years, period.Months, period.Days)
	if period.FixedDuration != 0 {
		deadline = deadline.Add(period.FixedDuration)
	}
	if !deadline.After(at) {
		return time.Time{}, false
	}
	return deadline, true
}

func patternTimerScheduleSpecFromNode(node *patternNode, at time.Time, tags map[string]Event, tagValues map[string][]Event, variables map[string]Value) (patternTimerScheduleSpec, bool) {
	if node == nil {
		return patternTimerScheduleSpec{}, false
	}
	if node.scheduleExpr != nil {
		value := node.scheduleExpr.eval(EvalContext{
			Tags:       tags,
			TagValues:  tagValues,
			Now:        at,
			Variables:  variables,
			Parameters: parameterValuesFromVariables(variables),
		})
		if !value.IsPresent() {
			return patternTimerScheduleSpec{}, false
		}
		iso, ok := value.Any().(string)
		if !ok {
			return patternTimerScheduleSpec{}, false
		}
		spec, err := parsePatternTimerScheduleISO(iso)
		return spec, err == nil
	}
	if node.schedulePeriod == nil {
		return patternTimerScheduleSpec{}, false
	}
	repetitions := node.scheduleRepetitions
	if !node.scheduleRepetitionsSet {
		repetitions = 1
	}
	period := *node.schedulePeriod
	// A zero period with an anchor is the one-shot date form (Esper
	// timer:schedule(date: X)), matching the ISO date-only parse.
	hasPeriod := period.Years != 0 || period.Months != 0 || period.Days != 0 || period.FixedDuration != 0
	return patternTimerScheduleSpec{
		start:        node.scheduleAnchor,
		hasStart:     node.scheduleAnchorSet,
		period:       period,
		hasPeriod:    hasPeriod,
		repetitions:  repetitions,
		includeStart: node.scheduleIncludeAnchor,
	}, true
}

func patternTimerScheduleRuntimeFor(node *patternNode, at time.Time, tags map[string]Event, tagValues map[string][]Event, variables map[string]Value) (*patternTimerScheduleRuntime, bool) {
	spec, ok := patternTimerScheduleSpecFromNode(node, at, tags, tagValues, variables)
	if !ok {
		return nil, false
	}
	if !spec.hasPeriod {
		if !spec.hasStart || spec.start.IsZero() || !spec.start.After(at) {
			return &patternTimerScheduleRuntime{}, true
		}
		return &patternTimerScheduleRuntime{next: spec.start, repetitions: 1, active: true}, true
	}
	if err := validatePatternTimerPeriod(spec.period); err != nil || spec.repetitions < -1 {
		return nil, false
	}
	anchor := at
	if spec.hasStart {
		anchor = spec.start
	}
	next, emitted, ok := patternTimerScheduleFirstDue(anchor, spec.hasStart, spec.includeStart, spec.period, spec.repetitions, at)
	if !ok {
		return &patternTimerScheduleRuntime{period: spec.period, repetitions: spec.repetitions, emitted: emitted}, true
	}
	return &patternTimerScheduleRuntime{
		next:        next,
		period:      spec.period,
		repetitions: spec.repetitions,
		emitted:     emitted,
		active:      true,
	}, true
}

// patternTimerScheduleFirstDue returns the first occurrence strictly after
// the current virtual time, together with the number of occurrences skipped
// while anchoring a past-dated recurring schedule. A future explicit start is
// preserved exactly, including nanosecond precision.
func patternTimerScheduleFirstDue(anchor time.Time, hasStart, includeStart bool, period PatternTimerPeriod, repetitions int64, now time.Time) (time.Time, int64, bool) {
	if repetitions == 0 {
		return time.Time{}, 0, false
	}
	if period.Years == 0 && period.Months == 0 && period.Days == 0 && period.FixedDuration > 0 {
		step := period.FixedDuration
		index := int64(1)
		if includeStart {
			index = 0
		}
		if now.After(anchor) || now.Equal(anchor) {
			elapsed := now.Sub(anchor)
			index = elapsed.Nanoseconds()/step.Nanoseconds() + 1
			if includeStart && index < 1 {
				index = 1
			}
		}
		if !hasStart && !includeStart && index < 1 {
			index = 1
		}
		if !includeStart && index < 1 {
			index = 1
		}
		if index > math.MaxInt64/step.Nanoseconds() {
			return time.Time{}, 0, false
		}
		delta := time.Duration(index * step.Nanoseconds())
		next := anchor.Add(delta)
		if next.IsZero() || !next.After(now) {
			return time.Time{}, 0, false
		}
		emitted := index
		if includeStart {
			emitted = index
		} else if emitted > 0 {
			emitted--
		}
		if repetitions > 0 && emitted >= repetitions {
			return time.Time{}, emitted, false
		}
		return next, emitted, true
	}

	index := int64(0)
	if !includeStart {
		index = 1
	}
	next := anchor
	var emitted int64
	if !includeStart {
		var ok bool
		next, ok = patternTimerPeriodDeadline(anchor, period)
		if !ok {
			return time.Time{}, 0, false
		}
		emitted = 0
	}
	for !next.After(now) {
		if includeStart {
			emitted = index + 1
		} else {
			emitted = index
		}
		if repetitions > 0 && emitted >= repetitions {
			return time.Time{}, emitted, false
		}
		var ok bool
		next, ok = patternTimerPeriodDeadline(next, period)
		if !ok {
			return time.Time{}, emitted, false
		}
		index++
		if index > 1000000 {
			return time.Time{}, emitted, false
		}
	}
	if includeStart {
		emitted = index
	} else if index > 0 {
		emitted = index - 1
	}
	if repetitions > 0 && emitted >= repetitions {
		return time.Time{}, emitted, false
	}
	return next, emitted, true
}

func advancePatternTimerScheduleRuntime(state *patternTimerScheduleRuntime) bool {
	if state == nil || !state.active {
		return false
	}
	state.emitted++
	if state.repetitions > 0 && state.emitted >= state.repetitions {
		state.active = false
		state.next = time.Time{}
		return true
	}
	next, ok := patternTimerPeriodDeadline(state.next, state.period)
	if !ok {
		state.active = false
		state.next = time.Time{}
		return true
	}
	state.next = next
	return true
}

func clonePatternTimerScheduleRuntime(state *patternTimerScheduleRuntime) *patternTimerScheduleRuntime {
	if state == nil {
		return nil
	}
	copyState := *state
	return &copyState
}

func parsePatternTimerScheduleISO(value string) (patternTimerScheduleSpec, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return patternTimerScheduleSpec{}, fmt.Errorf("received an empty string")
	}
	parts := strings.Split(value, "/")
	if len(parts) == 0 || len(parts) > 3 || value == "/" || strings.HasSuffix(value, "/") {
		return patternTimerScheduleSpec{}, fmt.Errorf("invalid number of parts")
	}
	var spec patternTimerScheduleSpec
	switch len(parts) {
	case 1:
		if strings.HasPrefix(parts[0], "P") {
			period, err := parsePatternTimerPeriodISO(parts[0])
			if err != nil {
				return patternTimerScheduleSpec{}, err
			}
			spec.period, spec.hasPeriod, spec.repetitions = period, true, 1
			return spec, nil
		}
		date, err := parsePatternTimerScheduleDate(parts[0])
		if err != nil {
			return patternTimerScheduleSpec{}, err
		}
		spec.start, spec.hasStart, spec.repetitions, spec.includeStart = date, true, 1, true
		return spec, nil
	case 2:
		if strings.HasPrefix(parts[0], "R") {
			repetitions, err := parsePatternTimerScheduleRepeat(parts[0])
			if err != nil {
				return patternTimerScheduleSpec{}, err
			}
			period, err := parsePatternTimerPeriodISO(parts[1])
			if err != nil {
				return patternTimerScheduleSpec{}, err
			}
			spec.period, spec.hasPeriod, spec.repetitions = period, true, repetitions
			return spec, nil
		}
		date, err := parsePatternTimerScheduleDate(parts[0])
		if err != nil {
			return patternTimerScheduleSpec{}, err
		}
		period, err := parsePatternTimerPeriodISO(parts[1])
		if err != nil {
			return patternTimerScheduleSpec{}, err
		}
		spec.start, spec.hasStart = date, true
		spec.period, spec.hasPeriod, spec.repetitions = period, true, 1
		return spec, nil
	case 3:
		repetitions, err := parsePatternTimerScheduleRepeat(parts[0])
		if err != nil {
			return patternTimerScheduleSpec{}, err
		}
		date, err := parsePatternTimerScheduleDate(parts[1])
		if err != nil {
			return patternTimerScheduleSpec{}, err
		}
		period, err := parsePatternTimerPeriodISO(parts[2])
		if err != nil {
			return patternTimerScheduleSpec{}, err
		}
		spec.start, spec.hasStart, spec.period, spec.hasPeriod = date, true, period, true
		spec.repetitions, spec.includeStart = repetitions, true
		return spec, nil
	}
	return patternTimerScheduleSpec{}, fmt.Errorf("invalid number of parts")
}

func parsePatternTimerScheduleRepeat(value string) (int64, error) {
	if value == "R" {
		return -1, nil
	}
	if !strings.HasPrefix(value, "R") || len(value) == 1 {
		return 0, fmt.Errorf("invalid repeat %q", value)
	}
	repetitions, err := strconv.ParseInt(value[1:], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid repeat %q: %w", value, err)
	}
	if repetitions < 0 {
		return 0, fmt.Errorf("invalid repeat %q", value)
	}
	return repetitions, nil
}

func parsePatternTimerScheduleDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	layoutsWithZone := []string{time.RFC3339Nano, time.RFC3339}
	for _, layout := range layoutsWithZone {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	for _, layout := range []string{"2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("exception parsing date %q, the date is not a supported ISO 8601 date", value)
}

func parsePatternTimerPeriodISO(value string) (PatternTimerPeriod, error) {
	matches := patternISO8601PeriodPattern.FindStringSubmatch(strings.TrimSpace(value))
	if matches == nil {
		return PatternTimerPeriod{}, fmt.Errorf("invalid period %q", value)
	}
	period := PatternTimerPeriod{}
	values := make([]int64, len(matches))
	for index := 1; index < len(matches); index++ {
		if matches[index] == "" {
			continue
		}
		parsed, err := strconv.ParseInt(matches[index], 10, 64)
		if err != nil || parsed > int64(math.MaxInt) {
			return PatternTimerPeriod{}, fmt.Errorf("invalid period %q", value)
		}
		values[index] = parsed
	}
	period.Years = int(values[1])
	period.Months = int(values[2])
	period.Days = int(values[4])
	if values[3] > 0 {
		if values[3] > int64(math.MaxInt-period.Days)/7 {
			return PatternTimerPeriod{}, fmt.Errorf("invalid period %q", value)
		}
		period.Days += int(values[3]) * 7
	}
	units := []int64{values[5], values[6], values[7]}
	multipliers := []int64{int64(time.Hour), int64(time.Minute), int64(time.Second)}
	var duration int64
	for index, unit := range units {
		if unit != 0 && (unit > math.MaxInt64/multipliers[index] || duration > math.MaxInt64-unit*multipliers[index]) {
			return PatternTimerPeriod{}, fmt.Errorf("invalid period %q", value)
		}
		duration += unit * multipliers[index]
	}
	period.FixedDuration = time.Duration(duration)
	if err := validatePatternTimerPeriod(period); err != nil {
		return PatternTimerPeriod{}, fmt.Errorf("invalid period %q", value)
	}
	return period, nil
}
