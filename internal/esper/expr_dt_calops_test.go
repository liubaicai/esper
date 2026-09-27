package esper

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Unit coverage for the calendar-transform and point-in-time-before
// expressions in expr_dt_calops.go (ExprDTIntervalOps executions 0 and 17).
// Expected instants are computed in UTC, matching the pinned harness
// convention (-Duser.timezone=UTC on the oracle side).

// TestDateTimeCalOpsTransforms checks the set/withDate/withTime transforms
// on both representations: int64 epoch-millis stays int64, time.Time stays
// time.Time, and the month argument is 1-based (LDT convention).
func TestDateTimeCalOpsTransforms(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "DT", []FieldSpec{
		{Name: "l", Type: reflect.TypeOf(int64(0))},
		{Name: "t", Type: reflect.TypeOf(time.Time{})},
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "DT").Select(
		Alias("set", DateTimeSet[int64](Field[map[string]any, int64]("l"), "month", 1)),
		Alias("withDate", DateTimeWithDate[time.Time](Field[map[string]any, time.Time]("t"), 2001, 1, 1)),
		Alias("withTime", DateTimeWithTime[int64](Field[map[string]any, int64]("l"), 8, 59, 59, 0)),
		Alias("setDay", DateTimeSet[time.Time](Field[map[string]any, time.Time]("t"), "dayofmonth", 15)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	rows := subscribeRows(t, engine, plan)

	base := time.Date(2002, 5, 30, 9, 0, 0, 0, time.UTC)
	if err := engine.SendRecord(context.Background(), "DT", map[string]any{"l": base.UnixMilli(), "t": base}); err != nil {
		t.Fatal(err)
	}
	got := rows()
	if len(got) != 1 {
		t.Fatalf("rows = %d, want 1", len(got))
	}
	row := got[0]
	if v, ok := row.Get("set").Any().(int64); !ok || v != time.Date(2002, 1, 30, 9, 0, 0, 0, time.UTC).UnixMilli() {
		t.Fatalf("set('month',1) = %#v, want int64 %d", row.Get("set").Any(), time.Date(2002, 1, 30, 9, 0, 0, 0, time.UTC).UnixMilli())
	}
	if v, ok := row.Get("withDate").Any().(time.Time); !ok || v.UnixMilli() != time.Date(2001, 1, 1, 9, 0, 0, 0, time.UTC).UnixMilli() {
		t.Fatalf("withDate(2001,1,1) = %#v, want time.Time 2001-01-01T09:00Z", row.Get("withDate").Any())
	}
	if v, ok := row.Get("withTime").Any().(int64); !ok || v != time.Date(2002, 5, 30, 8, 59, 59, 0, time.UTC).UnixMilli() {
		t.Fatalf("withTime(8,59,59,0) = %#v, want int64 %d", row.Get("withTime").Any(), time.Date(2002, 5, 30, 8, 59, 59, 0, time.UTC).UnixMilli())
	}
	if v, ok := row.Get("setDay").Any().(time.Time); !ok || v.UnixMilli() != time.Date(2002, 5, 15, 9, 0, 0, 0, time.UTC).UnixMilli() {
		t.Fatalf("set('dayofmonth',15) = %#v, want time.Time 2002-05-15T09:00Z", row.Get("setDay").Any())
	}

	// Null and missing inputs produce Null cells.
	if err := engine.SendRecord(context.Background(), "DT", map[string]any{"l": nil, "t": nil}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendRecord(context.Background(), "DT", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	got = rows()
	if len(got) != 3 {
		t.Fatalf("rows = %d, want 3", len(got))
	}
	for index := 1; index <= 2; index++ {
		for _, column := range []string{"set", "withDate", "withTime", "setDay"} {
			if got[index].Get(column).IsPresent() {
				t.Fatalf("row %d %s = %#v, want null", index, column, got[index].Get(column).Any())
			}
		}
	}
}

// TestDateTimeWithFieldsExpr covers the nullable expression-arg forms:
// present fields replace, null fields keep the input's current value
// (Java actionSetYMD/actionSetHMS skip-null semantics).
func TestDateTimeWithFieldsExpr(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "DT", []FieldSpec{
		{Name: "l", Type: reflect.TypeOf(int64(0))},
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "DT").Select(
		Alias("wd", DateTimeWithDateExpr[int64](Field[map[string]any, int64]("l"),
			Literal(int64(2004)), NullLiteral[int64](), Literal(int64(3)))),
		Alias("wt", DateTimeWithTimeExpr[int64](Field[map[string]any, int64]("l"),
			Literal(int64(0)), NullLiteral[int64](), NullLiteral[int64](), Literal(int64(6)))),
		Alias("wtAll", DateTimeWithTimeExpr[int64](Field[map[string]any, int64]("l"),
			Literal(int64(1)), Literal(int64(2)), Literal(int64(3)), Literal(int64(4)))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	rows := subscribeRows(t, engine, plan)

	// 2002-05-30T09:01:02.003Z: wd keeps month(5)/time, sets 2004-?-03 with
	// null month keeping May -> 2004-05-03T09:01:02.003; wt keeps
	// minute/second, sets hour=0,millis=6 -> 2002-05-30T00:01:02.006.
	base := time.Date(2002, 5, 30, 9, 1, 2, 3*int(time.Millisecond), time.UTC)
	if err := engine.SendRecord(context.Background(), "DT", map[string]any{"l": base.UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	got := rows()
	if len(got) != 1 {
		t.Fatalf("rows = %d, want 1", len(got))
	}
	wantWD := time.Date(2004, 5, 3, 9, 1, 2, 3*int(time.Millisecond), time.UTC).UnixMilli()
	wantWT := time.Date(2002, 5, 30, 0, 1, 2, 6*int(time.Millisecond), time.UTC).UnixMilli()
	wantAll := time.Date(2002, 5, 30, 1, 2, 3, 4*int(time.Millisecond), time.UTC).UnixMilli()
	if v := got[0].Get("wd").Any(); v != wantWD {
		t.Fatalf("wd = %#v, want %d", v, wantWD)
	}
	if v := got[0].Get("wt").Any(); v != wantWT {
		t.Fatalf("wt = %#v, want %d", v, wantWT)
	}
	if v := got[0].Get("wtAll").Any(); v != wantAll {
		t.Fatalf("wtAll = %#v, want %d", v, wantAll)
	}
}

// TestDateTimeShiftExpr covers the expression-arg plus/minus: the
// millisecond count evaluates per event (VariableRef) and a null count
// produces Null.
func TestDateTimeShiftExpr(t *testing.T) {
	env := NewEnvironment()
	if err := env.RegisterVariable("varmsec", int64(0)); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "DT", []FieldSpec{
		{Name: "l", Type: reflect.TypeOf(int64(0))},
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "DT").Select(
		Alias("plus", DateTimePlusExpr[int64](Field[map[string]any, int64]("l"), VariableRef[int64]("varmsec"))),
		Alias("minus", DateTimeMinusExpr[int64](Field[map[string]any, int64]("l"), VariableRef[int64]("varmsec"))),
		Alias("nullMs", DateTimePlusExpr[int64](Field[map[string]any, int64]("l"), NullLiteral[int64]())),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	rows := subscribeRows(t, engine, plan)
	if err := engine.SetVariable(context.Background(), "varmsec", int64(1000)); err != nil {
		t.Fatal(err)
	}
	base := int64(1022749200000)
	if err := engine.SendRecord(context.Background(), "DT", map[string]any{"l": base}); err != nil {
		t.Fatal(err)
	}
	got := rows()
	if len(got) != 1 {
		t.Fatalf("rows = %d, want 1", len(got))
	}
	if v := got[0].Get("plus").Any(); v != base+1000 {
		t.Fatalf("plus = %#v, want %d", v, base+1000)
	}
	if v := got[0].Get("minus").Any(); v != base-1000 {
		t.Fatalf("minus = %#v, want %d", v, base-1000)
	}
	if got[0].Get("nullMs").IsPresent() {
		t.Fatalf("nullMs = %#v, want null", got[0].Get("nullMs").Any())
	}
}

// TestDateTimeWithMinMaxTransforms checks the withMax/withMin clamps on both
// representations against the pinned ExprDTWithMax/ExprDTWithMin oracle
// values: 'week' keeps the day-of-week under Java WEEK_OF_YEAR semantics,
// 'year' hits GregorianCalendar's actual bounds, and null/missing inputs
// produce Null cells.
func TestDateTimeWithMinMaxTransforms(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "DT", []FieldSpec{
		{Name: "l", Type: reflect.TypeOf(int64(0))},
		{Name: "t", Type: reflect.TypeOf(time.Time{})},
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "DT").Select(
		Alias("maxMonth", DateTimeWithMax[int64](Field[map[string]any, int64]("l"), "month")),
		Alias("minMonth", DateTimeWithMin[time.Time](Field[map[string]any, time.Time]("t"), "month")),
		Alias("maxWeek", DateTimeWithMax[int64](Field[map[string]any, int64]("l"), "week")),
		Alias("minWeek", DateTimeWithMin[int64](Field[map[string]any, int64]("l"), "week")),
		Alias("maxYear", DateTimeWithMax[int64](Field[map[string]any, int64]("l"), "year")),
		Alias("minYear", DateTimeWithMin[int64](Field[map[string]any, int64]("l"), "year")),
		Alias("maxDay", DateTimeWithMax[time.Time](Field[map[string]any, time.Time]("t"), "day")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	rows := subscribeRows(t, engine, plan)

	base := time.Date(2002, 5, 30, 9, 0, 0, 0, time.UTC)
	if err := engine.SendRecord(context.Background(), "DT", map[string]any{"l": base.UnixMilli(), "t": base}); err != nil {
		t.Fatal(err)
	}
	got := rows()
	if len(got) != 1 {
		t.Fatalf("rows = %d, want 1", len(got))
	}
	want := map[string]int64{
		"maxMonth": time.Date(2002, 12, 30, 9, 0, 0, 0, time.UTC).UnixMilli(),
		"minMonth": time.Date(2002, 1, 30, 9, 0, 0, 0, time.UTC).UnixMilli(),
		"maxWeek":  time.Date(2002, 12, 26, 9, 0, 0, 0, time.UTC).UnixMilli(),
		"minWeek":  time.Date(2002, 1, 3, 9, 0, 0, 0, time.UTC).UnixMilli(),
		// Java GregorianCalendar resolves year-1 dates on the Julian
		// calendar (pre-1582-10-15 cutover), not Go's proleptic Gregorian:
		// 0001-05-30T09:00 Julian = epoch-millis -62122863600000.
		"minYear": -62122863600000,
		"maxDay":  time.Date(2002, 5, 31, 9, 0, 0, 0, time.UTC).UnixMilli(),
	}
	row := got[0]
	for column, millis := range want {
		var v int64
		switch value := row.Get(column).Any().(type) {
		case int64:
			v = value
		case time.Time:
			v = value.UnixMilli()
		default:
			t.Fatalf("%s = %#v, want date-time cell", column, row.Get(column).Any())
		}
		if v != millis {
			t.Fatalf("%s = %d, want %d", column, v, millis)
		}
	}
	// withMax('year') reaches GregorianCalendar's actual-maximum year
	// 292278994; the epoch-millis result is the int64-wrapped value the
	// oracle's getTime() yields for the same instant.
	if v, ok := row.Get("maxYear").Any().(int64); !ok || v != 9223372030035600000 {
		t.Fatalf("maxYear = %#v, want wrapped int64 9223372030035600000", row.Get("maxYear").Any())
	}
	// Null and missing inputs produce Null cells.
	if err := engine.SendRecord(context.Background(), "DT", map[string]any{"l": nil, "t": nil}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendRecord(context.Background(), "DT", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	got = rows()
	if len(got) != 3 {
		t.Fatalf("rows = %d, want 3", len(got))
	}
	for index := 1; index <= 2; index++ {
		for _, column := range []string{"maxMonth", "minMonth", "maxWeek", "minWeek", "maxYear", "minYear", "maxDay"} {
			if got[index].Get(column).IsPresent() {
				t.Fatalf("row %d %s = %#v, want null", index, column, got[index].Get(column).Any())
			}
		}
	}

	// Pre-cutover instants decompose on the Julian calendar (JDK-verified
	// oracle values): Julian 1500-06-15T12:00 -> withMin('week') /
	// withMax('week') under WEEK_OF_YEAR semantics.
	pre := int64(-14816606400000)
	if err := engine.SendRecord(context.Background(), "DT", map[string]any{"l": pre, "t": nil}); err != nil {
		t.Fatal(err)
	}
	got = rows()
	if len(got) != 4 {
		t.Fatalf("rows = %d, want 4", len(got))
	}
	if v, ok := got[3].Get("minWeek").Any().(int64); !ok || v != -14831121600000 {
		t.Fatalf("pre-cutover minWeek = %#v, want -14831121600000", got[3].Get("minWeek").Any())
	}
	if v, ok := got[3].Get("maxWeek").Any().(int64); !ok || v != -14800276800000 {
		t.Fatalf("pre-cutover maxWeek = %#v, want -14800276800000", got[3].Get("maxWeek").Any())
	}
}

// TestDateTimeBeforePoint checks the strict point-in-time before: true iff
// threshold - value >= 1ms; null and missing operands produce Null.
func TestDateTimeBeforePoint(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "DT", []FieldSpec{
		{Name: "t", Type: reflect.TypeOf(int64(0))},
		{Name: "p", Type: reflect.TypeOf(int64(0))},
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "DT").Select(
		Alias("c0", DateTimeBefore(Field[map[string]any, int64]("t"), Field[map[string]any, int64]("p"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	rows := subscribeRows(t, engine, plan)

	steps := []struct {
		event map[string]any
		want  any
	}{
		{map[string]any{"t": int64(1000), "p": int64(1000)}, false}, // delta 0: strict
		{map[string]any{"t": int64(1000), "p": int64(1001)}, true},  // delta 1
		{map[string]any{"t": int64(1000), "p": int64(999)}, false},
		{map[string]any{"t": nil, "p": int64(1000)}, nil},
		{map[string]any{"p": int64(1000)}, nil}, // missing t
	}
	for index, step := range steps {
		if err := engine.SendRecord(context.Background(), "DT", step.event); err != nil {
			t.Fatal(err)
		}
		got := rows()
		if len(got) != index+1 {
			t.Fatalf("step %d rows = %d, want %d", index, len(got), index+1)
		}
		cell := got[index].Get("c0")
		if step.want == nil {
			if cell.IsPresent() {
				t.Fatalf("step %d c0 = %#v, want null", index, cell.Any())
			}
			continue
		}
		if v, ok := cell.Any().(bool); !ok || v != step.want.(bool) {
			t.Fatalf("step %d c0 = %#v, want %v", index, cell.Any(), step.want)
		}
	}
}

// TestDateTimeCalOpsBoundsParity replays the ExprDTIntervalCalendarOps
// where-clause variants (seedTime 2002-05-30T09:00:00.000Z): calendar ops on
// an interval target transform the start and preserve the duration
// (end = newStart + (end - start)).
func TestDateTimeCalOpsBoundsParity(t *testing.T) {
	seed := time.Date(2002, 5, 30, 9, 0, 0, 0, time.UTC).UnixMilli()
	at := func(iso string, dur int64) dtIntervalEvent {
		parsed, err := time.Parse(time.RFC3339, iso)
		if err != nil {
			t.Fatalf("parse %s: %v", iso, err)
		}
		return dtIntervalEvent{Start: parsed.UnixMilli(), End: parsed.UnixMilli() + dur}
	}
	bounds := func() (IntervalBounds, IntervalBounds) {
		return IntervalBounds{Start: JoinField[int64](0, "st"), End: JoinField[int64](0, "en")},
			IntervalBounds{Start: JoinField[int64](1, "st"), End: JoinField[int64](1, "en")}
	}

	cases := []struct {
		name    string
		seedDur int64
		pred    func(a, b IntervalBounds) Expression[bool]
		send    []dtIntervalEvent
		want    []bool
	}{
		{"withDate-before", 0,
			func(a, b IntervalBounds) Expression[bool] {
				return Interval(Before, WithDateBounds(a, 2001, 1, 1), b)
			},
			[]dtIntervalEvent{at("2999-01-01T09:00:00.001Z", 0)},
			[]bool{true}},
		{"withDate-before-withDate", 0,
			func(a, b IntervalBounds) Expression[bool] {
				return Interval(Before, WithDateBounds(a, 2001, 1, 1), WithDateBounds(b, 2001, 1, 1))
			},
			[]dtIntervalEvent{at("2999-01-01T10:00:00.001Z", 0), at("2999-01-01T08:00:00.001Z", 0)},
			[]bool{false, true}},
		// Duration preservation: a becomes 08:59:59.000 -> 09:00:01.000, so
		// a.end is not before b.start (09:00:00.000). A point-collapse would
		// wrongly report before=true.
		{"withTime-before-duration", 2000,
			func(a, b IntervalBounds) Expression[bool] {
				return Interval(Before, WithTimeBounds(a, 8, 59, 59, 0), b)
			},
			[]dtIntervalEvent{at("2002-05-30T08:59:59.000Z", 2000)},
			[]bool{false}},
		{"after", 1000,
			func(a, b IntervalBounds) Expression[bool] {
				return Interval(After, a, b)
			},
			[]dtIntervalEvent{at("2002-05-30T09:00:01.000Z", 0), at("2002-05-30T09:00:01.001Z", 0)},
			[]bool{false, true}},
		// PointBounds collapses a non-zero-duration bound to (start,start):
		// b (09:00:00.000 + 2000ms) becomes the point 09:00:00.000. `after`
		// compares a.start against b.end, so the collapse is observable:
		// a starting 09:00:01.500 is after the collapsed point (delta 1500)
		// but NOT after the whole interval (b.end 09:00:02.000, delta -500).
		{"pointbounds-collapse", 2000,
			func(a, b IntervalBounds) Expression[bool] {
				return Interval(After, a, PointBounds(b))
			},
			[]dtIntervalEvent{at("2002-05-30T09:00:01.500Z", 0), at("2002-05-30T08:59:59.000Z", 0)},
			[]bool{true, false}},
	}

	env := newDTIntervalEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			left, right := bounds()
			plan, err := env.Build(
				JoinMany(
					JoinSource(From[dtIntervalEvent](env, "A")).Window(LengthWindow(1)),
					JoinSource(From[dtIntervalEvent](env, "B")).Window(LengthWindow(1)),
				).Select(
					SelectFrom(0, "a_st", JoinField[int64](0, "st")),
				).Where(tc.pred(left, right)).Query(StatementName("s0")),
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

			if err := engine.Send(context.Background(), "B", dtIntervalEvent{Start: seed, End: seed + tc.seedDur}); err != nil {
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

// TestDateTimeCalOpsInvalidField checks that an unknown calendar field is a
// build-time configuration error on both the point transform and the
// interval-bounds helper.
func TestDateTimeCalOpsInvalidField(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "DT", []FieldSpec{
		{Name: "l", Type: reflect.TypeOf(int64(0))},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Build(FromAny(env, "DT").Select(
		Alias("c0", DateTimeSet[int64](Field[map[string]any, int64]("l"), "nope", 1)),
	).Query(StatementName("s0"))); err == nil || !errors.Is(err, ErrorInvalidRule) ||
		!strings.Contains(err.Error(), "unknown date-time calendar field") {
		t.Fatalf("DateTimeSet unknown-field build error = %v", err)
	}

	bounds := IntervalBounds{
		Start: Field[map[string]any, int64]("l"),
		End:   Field[map[string]any, int64]("l"),
	}
	if _, err := env.Build(FromAny(env, "DT").Filter(
		Interval(Before, SetBounds(bounds, "nope", 1), bounds),
	).Query(StatementName("s1"))); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("SetBounds unknown-field build error = %v", err)
	}
}
