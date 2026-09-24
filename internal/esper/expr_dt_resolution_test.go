package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Unit coverage for the resolution-aware date-time surface in
// expr_dt_resolution.go (ExprDTResolution executions(isMicrosecond)).
// Expected instants are computed in UTC, matching the pinned harness
// convention (-Duser.timezone=UTC on the oracle side). The Java scenario
// values are reused: t = 2002-05-30T09:05:06.007Z for the long-property
// execution and 2002-05-30T09:00:00.000Z for the event-time execution.

var dtResolutionLongPropertyTime = time.Date(2002, 5, 30, 9, 5, 6, 7*int(time.Millisecond), time.UTC)

func TestTimeUnitDefaultsToMillisecondsAndEngineOptionSelectsMicroseconds(t *testing.T) {
	if got := (EvalContext{}).timeUnit(); got != Milliseconds {
		t.Fatalf("zero EvalContext timeUnit = %v, want Milliseconds", got)
	}
	if got := (EvalContext{TimeUnit: Microseconds}).timeUnit(); got != Microseconds {
		t.Fatalf("explicit EvalContext timeUnit = %v, want Microseconds", got)
	}
	engine := NewEngine(NewEnvironment(), WithTimeUnit(Microseconds))
	defer func() { _ = engine.Close(context.Background()) }()
	if got := (EvalContext{Engine: engine}).timeUnit(); got != Microseconds {
		t.Fatalf("engine-derived timeUnit = %v, want Microseconds", got)
	}
	defaultEngine := NewEngine(NewEnvironment())
	defer func() { _ = defaultEngine.Close(context.Background()) }()
	if got := (EvalContext{Engine: defaultEngine}).timeUnit(); got != Milliseconds {
		t.Fatalf("default engine timeUnit = %v, want Milliseconds", got)
	}
}

func TestCurrentTimestampUsesEngineUnits(t *testing.T) {
	now := time.Date(2002, 5, 30, 9, 5, 6, 7*int(time.Millisecond), time.UTC)
	if got := CurrentTimestamp().eval(EvalContext{Now: now}); !got.Equal(Present(now.UnixNano() / int64(time.Millisecond))) {
		t.Fatalf("millisecond current_timestamp = %v, want %d", got, now.UnixNano()/int64(time.Millisecond))
	}
	if got := CurrentTimestamp().eval(EvalContext{Now: now, TimeUnit: Microseconds}); !got.Equal(Present(now.UnixNano() / int64(time.Microsecond))) {
		t.Fatalf("microsecond current_timestamp = %v, want %d", got, now.UnixNano()/int64(time.Microsecond))
	}
}

// TestDateTimeCalOpsPreserveMicrosecondRemainder mirrors the LongProperty
// execution's c0/c1 columns: under Microseconds a long input splits into
// millis = v/1000 plus remainder v%1000, the calendar op runs on the
// millisecond part, and long results re-add the remainder. toCalendar()
// (DateTimeToTime) truncates to the millisecond part.
func TestDateTimeCalOpsPreserveMicrosecondRemainder(t *testing.T) {
	micros := dtResolutionLongPropertyTime.UnixMilli()*1000 + 123
	calMod := time.Date(2002, 5, 30, 1, 2, 3, 4*int(time.Millisecond), time.UTC)
	microCtx := EvalContext{TimeUnit: Microseconds}

	if got := DateTimeWithTime[int64](Literal(micros), 1, 2, 3, 4).eval(microCtx); !got.Equal(Present(calMod.UnixMilli()*1000 + 123)) {
		t.Fatalf("microsecond withTime = %v, want %d", got, calMod.UnixMilli()*1000+123)
	}
	set := DateTimeSet[int64](
		DateTimeSet[int64](
			DateTimeSet[int64](
				DateTimeSet[int64](Literal(micros), "hour", 1),
				"minute", 2),
			"second", 3),
		"millisecond", 4)
	if got := set.eval(microCtx); !got.Equal(Present(calMod.UnixMilli()*1000 + 123)) {
		t.Fatalf("microsecond chained set = %v, want %d", got, calMod.UnixMilli()*1000+123)
	}
	if got := DateTimeToTime[int64](set).eval(microCtx); !got.Equal(Present(calMod)) {
		t.Fatalf("microsecond set().toCalendar = %v, want %v", got, calMod)
	}

	// Millisecond resolution is unchanged: the same expressions treat the
	// value as epoch milliseconds with no remainder split.
	millis := dtResolutionLongPropertyTime.UnixMilli()
	if got := DateTimeWithTime[int64](Literal(millis), 1, 2, 3, 4).eval(EvalContext{}); !got.Equal(Present(calMod.UnixMilli())) {
		t.Fatalf("millisecond withTime = %v, want %d", got, calMod.UnixMilli())
	}
	setMillis := DateTimeSet[int64](
		DateTimeSet[int64](
			DateTimeSet[int64](
				DateTimeSet[int64](Literal(millis), "hour", 1),
				"minute", 2),
			"second", 3),
		"millisecond", 4)
	if got := setMillis.eval(EvalContext{TimeUnit: Milliseconds}); !got.Equal(Present(calMod.UnixMilli())) {
		t.Fatalf("millisecond chained set = %v, want %d", got, calMod.UnixMilli())
	}

	// time.Time inputs are Calendar-precision in both resolutions: no
	// remainder split and fields resolve in the value's own zone. The result
	// re-wraps through dateTimeCalOpResult, so compare the instant.
	instant := dtResolutionLongPropertyTime
	if got := DateTimeWithTime[time.Time](Literal(instant), 1, 2, 3, 4).eval(microCtx); !got.IsPresent() || got.Any().(time.Time).UnixMilli() != calMod.UnixMilli() {
		t.Fatalf("time.Time withTime under microseconds = %v, want instant %v", got, calMod)
	}
}

// TestDateTimeGetExtractsCalendarFields mirrors the LongProperty execution's
// c2/c3/c4 columns plus the ExprDTDataSources AllCombinations getters:
// get('month') is the 0-based Calendar.MONTH, getMinuteOfHour() maps to
// "minute_of_hour", getDayOfYear() to "day_of_year" (Calendar.DAY_OF_YEAR),
// getEra() to "era" (Calendar.ERA = 1 for CE dates), getmillisOfSecond() to
// "millis_of_second", and getweekyear() to "weekyear" (Calendar.WEEK_OF_YEAR,
// the ISO week number). Under Microseconds the field reads from the
// millisecond part.
func TestDateTimeGetExtractsCalendarFields(t *testing.T) {
	micros := dtResolutionLongPropertyTime.UnixMilli()*1000 + 123
	microCtx := EvalContext{TimeUnit: Microseconds}
	for _, testCase := range []struct {
		field string
		want  int64
	}{
		{"month", 4},
		{"minute_of_hour", 5},
		{"hour", 9},
		{"second", 6},
		{"millisecond", 7},
		{"millis_of_second", 7},
		{"day_of_month", 30},
		{"day_of_year", 150}, // 2002-05-30 is day 150; Calendar.DAY_OF_YEAR
		{"doy", 150},
		{"day_of_week", 5}, // 2002-05-30 was a Thursday; Calendar.THURSDAY = 5
		{"era", 1},         // Calendar.ERA = AD for CE dates
		{"weekyear", 22},   // Calendar.WEEK_OF_YEAR = ISO week 22
		{"week_of_year", 22},
		{"year", 2002},
	} {
		if got := DateTimeGet[int64](Literal(micros), testCase.field).eval(microCtx); !got.Equal(Present(testCase.want)) {
			t.Fatalf("microsecond get(%q) = %v, want %d", testCase.field, got, testCase.want)
		}
		if got := DateTimeGet[int64](Literal(dtResolutionLongPropertyTime.UnixMilli()), testCase.field).eval(EvalContext{}); !got.Equal(Present(testCase.want)) {
			t.Fatalf("millisecond get(%q) = %v, want %d", testCase.field, got, testCase.want)
		}
	}
	if got := DateTimeGet[time.Time](Literal(dtResolutionLongPropertyTime), "month").eval(microCtx); !got.Equal(Present(int64(4))) {
		t.Fatalf("time.Time get(month) = %v, want 4", got)
	}
	if got := DateTimeGet[int64](NullLiteral[int64](), "month").eval(microCtx); !got.IsNull() {
		t.Fatalf("null get(month) = %v, want null", got)
	}
}

// TestDateTimeMinusPlusShiftMillisecondsRegardlessOfResolution mirrors the
// LongProperty execution's c7 column: minus(1) subtracts one millisecond,
// which is 1 engine unit under Milliseconds and 1000 under Microseconds.
func TestDateTimeMinusPlusShiftMillisecondsRegardlessOfResolution(t *testing.T) {
	millis := dtResolutionLongPropertyTime.UnixMilli()
	micros := millis*1000 + 123
	if got := DateTimeMinus[int64](Literal(millis), 1).eval(EvalContext{}); !got.Equal(Present(millis - 1)) {
		t.Fatalf("millisecond minus(1) = %v, want %d", got, millis-1)
	}
	if got := DateTimeMinus[int64](Literal(micros), 1).eval(EvalContext{TimeUnit: Microseconds}); !got.Equal(Present(micros - 1000)) {
		t.Fatalf("microsecond minus(1) = %v, want %d", got, micros-1000)
	}
	if got := DateTimePlus[int64](Literal(micros), 2).eval(EvalContext{TimeUnit: Microseconds}); !got.Equal(Present(micros + 2000)) {
		t.Fatalf("microsecond plus(2) = %v, want %d", got, micros+2000)
	}
	if got := DateTimeMinus[time.Time](Literal(dtResolutionLongPropertyTime), 1).eval(EvalContext{TimeUnit: Microseconds}); !got.Equal(Present(dtResolutionLongPropertyTime.Add(-time.Millisecond))) {
		t.Fatalf("time.Time minus(1) = %v, want %v", got, dtResolutionLongPropertyTime.Add(-time.Millisecond))
	}
	if got := DateTimeMinus[int64](NullLiteral[int64](), 1).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("null minus(1) = %v, want null", got)
	}
}

// TestDateTimeToTimeTruncatesToMilliseconds mirrors the LongProperty
// execution's c5/c6 columns: toDate()/toCalendar() truncate engine units to
// millisecond precision.
func TestDateTimeToTimeTruncatesToMilliseconds(t *testing.T) {
	micros := dtResolutionLongPropertyTime.UnixMilli()*1000 + 123
	if got := DateTimeToTime[int64](Literal(micros)).eval(EvalContext{TimeUnit: Microseconds}); !got.Equal(Present(dtResolutionLongPropertyTime)) {
		t.Fatalf("microsecond toTime = %v, want %v", got, dtResolutionLongPropertyTime)
	}
	if got := DateTimeToTime[int64](Literal(dtResolutionLongPropertyTime.UnixMilli())).eval(EvalContext{}); !got.Equal(Present(dtResolutionLongPropertyTime)) {
		t.Fatalf("millisecond toTime = %v, want %v", got, dtResolutionLongPropertyTime)
	}
	if got := DateTimeToTime[time.Time](Literal(dtResolutionLongPropertyTime)).eval(EvalContext{TimeUnit: Microseconds}); !got.Equal(Present(dtResolutionLongPropertyTime)) {
		t.Fatalf("time.Time toTime = %v, want %v", got, dtResolutionLongPropertyTime)
	}
	if got := DateTimeToTime[int64](NullLiteral[int64]()).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("null toTime = %v, want null", got)
	}
}

// TestEventIntervalBoundsReadsSchemaTimestampFields builds the object-array
// MyEvent(id,sts,ets) shape from the EventTime execution: sts is flagged
// starttimestamp and ets endtimestamp, so EventIntervalBounds resolves the
// bounds from the schema rather than fixed field names.
func TestEventIntervalBoundsReadsSchemaTimestampFields(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterObjectArray(env, "MyEvent", []FieldSpec{
		{Name: "id", Type: reflect.TypeOf("")},
		{Name: "sts", Type: reflect.TypeOf(int64(0)), StartTimestamp: true},
		{Name: "ets", Type: reflect.TypeOf(int64(0)), EndTimestamp: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	eventA, err := ParseObjectArray(schema, []any{"A", int64(1000), int64(2000)}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	eventB, err := ParseObjectArray(schema, []any{"B", int64(3000), int64(4000)}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := EvalContext{JoinEvents: []Event{eventA, eventB}}
	bounds := EventIntervalBounds(0)
	if got := bounds.Start.eval(ctx); !got.Equal(Present(int64(1000))) {
		t.Fatalf("source 0 start = %v, want 1000", got)
	}
	if got := bounds.End.eval(ctx); !got.Equal(Present(int64(2000))) {
		t.Fatalf("source 0 end = %v, want 2000", got)
	}
	if got := EventIntervalBounds(1).Start.eval(ctx); !got.Equal(Present(int64(3000))) {
		t.Fatalf("source 1 start = %v, want 3000", got)
	}
	if got := EventIntervalBounds(2).Start.eval(ctx); !got.IsMissing() {
		t.Fatalf("out-of-range source = %v, want missing", got)
	}
	if got := EventIntervalBounds(0).Start.eval(EvalContext{}); !got.IsMissing() {
		t.Fatalf("non-join context = %v, want missing", got)
	}

	// A schema without flagged fields yields Missing bounds.
	plain, err := RegisterObjectArray(env, "PlainEvent", []FieldSpec{
		{Name: "v", Type: reflect.TypeOf(int64(0))},
	})
	if err != nil {
		t.Fatal(err)
	}
	plainEvent, err := ParseObjectArray(plain, []any{int64(1)}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if got := EventIntervalBounds(0).Start.eval(EvalContext{JoinEvents: []Event{plainEvent}}); !got.IsMissing() {
		t.Fatalf("unflagged schema start = %v, want missing", got)
	}
}

// TestEventIntervalBoundsSingleStreamEvent covers the ExprDTDataSources
// StartEndTS shape: outside a join, source 0 resolves to the context event
// itself so an event reference can serve as its own interval. The flagged
// starttimestamp/endtimestamp fields still supply the bounds; an unflagged
// schema or a nonzero source stays Missing.
func TestEventIntervalBoundsSingleStreamEvent(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterObjectArray(env, "StartEndEvent", []FieldSpec{
		{Name: "id", Type: reflect.TypeOf("")},
		{Name: "sts", Type: reflect.TypeOf(int64(0)), StartTimestamp: true},
		{Name: "ets", Type: reflect.TypeOf(int64(0)), EndTimestamp: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	event, err := ParseObjectArray(schema, []any{"A", int64(1000), int64(2000)}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := EvalContext{Event: event}
	bounds := EventIntervalBounds(0)
	if got := bounds.Start.eval(ctx); !got.Equal(Present(int64(1000))) {
		t.Fatalf("single-stream start = %v, want 1000", got)
	}
	if got := bounds.End.eval(ctx); !got.Equal(Present(int64(2000))) {
		t.Fatalf("single-stream end = %v, want 2000", got)
	}
	if got := EventIntervalBounds(1).Start.eval(ctx); !got.IsMissing() {
		t.Fatalf("single-stream source 1 = %v, want missing", got)
	}

	// An unflagged schema still yields Missing bounds in the fallback.
	plain, err := RegisterObjectArray(env, "PlainStartEndEvent", []FieldSpec{
		{Name: "v", Type: reflect.TypeOf(int64(0))},
	})
	if err != nil {
		t.Fatal(err)
	}
	plainEvent, err := ParseObjectArray(plain, []any{int64(1)}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if got := EventIntervalBounds(0).Start.eval(EvalContext{Event: plainEvent}); !got.IsMissing() {
		t.Fatalf("unflagged single-stream start = %v, want missing", got)
	}
}

// TestIntervalBeforeBoundaryAtOneEngineUnit mirrors the EventTime
// execution's flip: a.withDate(2002,4,30).before(b) fires when the
// transformed left end is one engine unit below the right start and does not
// fire at equality. Java's withDate month is the 0-based Calendar.MONTH, so
// the Go 1-based WithDateBounds takes month 5 for the same 2002-05-30 date.
func TestIntervalBeforeBoundaryAtOneEngineUnit(t *testing.T) {
	flip := time.Date(2002, 5, 30, 9, 0, 0, 0, time.UTC).UnixMilli()
	for _, unit := range []TimeUnit{Milliseconds, Microseconds} {
		scale := int64(1)
		if unit == Microseconds {
			scale = 1000
		}
		ctx := EvalContext{TimeUnit: unit}
		b := IntervalBounds{Start: Literal(flip * scale), End: Literal(flip * scale)}
		for _, testCase := range []struct {
			name string
			a    int64
			want bool
		}{
			{"one-unit-before", flip*scale - 1, true},
			{"equal", flip * scale, false},
			{"one-unit-after", flip*scale + 1, false},
		} {
			a := IntervalBounds{Start: Literal(testCase.a), End: Literal(testCase.a)}
			if got := IntervalBefore(WithDateBounds(a, 2002, 5, 30), b).eval(ctx); !got.Equal(Present(testCase.want)) {
				t.Fatalf("unit %d %s: withDate before = %v, want %v", unit, testCase.name, got, testCase.want)
			}
		}
	}
}

// TestDateTimeComparisonsUseEngineUnits checks that before/after/between
// compare int64 operands as engine units: the strict boundary is one engine
// unit in both resolutions, while time.Time operands stay
// Calendar-precision (UnixMilli) regardless of resolution.
func TestDateTimeComparisonsUseEngineUnits(t *testing.T) {
	microCtx := EvalContext{TimeUnit: Microseconds}
	if got := DateTimeBefore(Literal(int64(999)), Literal(int64(1000))).eval(microCtx); !got.Equal(Present(true)) {
		t.Fatalf("microsecond before delta 1 = %v, want true", got)
	}
	if got := DateTimeAfter(Literal(int64(1001)), Literal(int64(1000))).eval(microCtx); !got.Equal(Present(true)) {
		t.Fatalf("microsecond after delta 1 = %v, want true", got)
	}
	if got := DateTimeBetween(Literal(int64(1000)), Literal(int64(999)), Literal(int64(1001))).eval(microCtx); !got.Equal(Present(true)) {
		t.Fatalf("microsecond between = %v, want true", got)
	}
	// time.Time operands coerce to UnixMilli even under Microseconds.
	instant := time.UnixMilli(1000).UTC()
	if got := DateTimeBefore(Literal(instant), Literal(int64(1001))).eval(microCtx); !got.Equal(Present(true)) {
		t.Fatalf("time.Time before under microseconds = %v, want true", got)
	}
}

func TestDateTimeResolutionRejectsInvalidBuilders(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "DT", []FieldSpec{
		{Name: "l", Type: reflect.TypeOf(int64(0))},
	}); err != nil {
		t.Fatal(err)
	}
	input := FromAny(env, "DT")
	for _, testCase := range []struct {
		name string
		expr Expr
		want string
	}{
		{"get-unknown-field", DateTimeGet[int64](Field[map[string]any, int64]("l"), "fortnight"), "unknown date-time calendar field"},
		{"get-nil-operand", DateTimeGet[int64](nil, "month"), "requires a date-time operand"},
		{"minus-nil-operand", DateTimeMinus[int64](nil, 1), "requires a date-time operand"},
		{"to-time-nil-operand", DateTimeToTime[int64](nil), "requires a date-time operand"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(input.Select(Alias("c0", testCase.expr)).Query(StatementName("invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("invalid builder error = %v, want %q", err, testCase.want)
			}
		})
	}
}
