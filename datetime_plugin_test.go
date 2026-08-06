package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

type dateTimePluginEvent struct {
	CalDate   time.Time `esper:"caldate"`
	LongDate  int64     `esper:"longdate"`
	UtilDate  time.Time `esper:"utildate"`
	LocalDate time.Time `esper:"localdate"`
	ZonedDate time.Time `esper:"zoneddate"`
}

func dateTimePluginFootprints() []DateTimeMethodFootprint {
	return []DateTimeMethodFootprint{{
		Input: DateTimeInputAny,
		Parameters: []DateTimeMethodParameter{
			{Description: "calendar field", Expected: DateTimeParameterString},
			{Description: "roll direction", Expected: DateTimeParameterBoolean},
		},
	}}
}

func dateTimePluginReformatFootprints() []DateTimeMethodFootprint {
	return []DateTimeMethodFootprint{{Input: DateTimeInputAny}}
}

func dateTimePluginRollFactory(context DateTimePluginFactoryContext) DateTimePluginOps {
	switch dateTimePluginInputKind(context.InputType) {
	case DateTimeInputTime, DateTimeInputEpochMillis:
		return DateTimePluginOpsFunc(dateTimePluginRoll)
	default:
		return nil
	}
}

func dateTimePluginReformatFactory(context DateTimePluginFactoryContext) DateTimePluginOps {
	switch dateTimePluginInputKind(context.InputType) {
	case DateTimeInputTime, DateTimeInputEpochMillis:
		return DateTimePluginOpsFunc(dateTimePluginReformat)
	default:
		return nil
	}
}

func dateTimePluginTime(value Value) (time.Time, bool) {
	if !value.IsPresent() {
		return time.Time{}, false
	}
	switch raw := value.Any().(type) {
	case time.Time:
		return raw, true
	case *time.Time:
		if raw == nil {
			return time.Time{}, false
		}
		return *raw, true
	}
	numeric, ok := numericValue(value)
	if !ok {
		return time.Time{}, false
	}
	return time.UnixMilli(int64(numeric)).UTC(), true
}

func dateTimePluginRoll(input Value, arguments []Value) (Value, error) {
	if len(arguments) != 2 {
		return Null(), nil
	}
	field, err := As[string](arguments[0])
	if err != nil || field != "date" {
		return Null(), nil
	}
	forward, err := As[bool](arguments[1])
	if err != nil {
		return Null(), nil
	}
	value, ok := dateTimePluginTime(input)
	if !ok {
		return Null(), nil
	}
	delta := 1
	if !forward {
		delta = -1
	}
	rolled := value.AddDate(0, 0, delta)
	if dateTimePluginInputKind(reflect.TypeOf(input.Any())) == DateTimeInputEpochMillis {
		return Present(rolled.UnixMilli()), nil
	}
	return Present(rolled), nil
}

func dateTimePluginReformat(input Value, _ []Value) (Value, error) {
	value, ok := dateTimePluginTime(input)
	if !ok {
		return Null(), nil
	}
	return Present([]string{
		itoa(int64(value.Day())),
		itoa(int64(value.Month())),
		itoa(int64(value.Year())),
	}), nil
}

func itoa(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		index--
		digits[index] = '-'
	}
	return string(digits[index:])
}

func buildDateTimePluginExpression(t *testing.T, env *Environment, expression Expr) {
	t.Helper()
	if _, err := env.Build(SelectOnce(env, Alias("value", expression))); err != nil {
		t.Fatal(err)
	}
}

func TestDateTimePluginTransformAndReformatMatchJava(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dateTimePluginEvent](env, "DateTimePluginEvent"); err != nil {
		t.Fatal(err)
	}
	if err := RegisterDateTimePlugin(env, "roll", dateTimePluginFootprints(), dateTimePluginRollFactory); err != nil {
		t.Fatal(err)
	}
	if err := RegisterDateTimePlugin(env, "as-array", dateTimePluginReformatFootprints(), dateTimePluginReformatFactory); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2002, time.May, 30, 9, 1, 2, 0, time.UTC)
	roll := func(input Expr) Expr {
		return DateTimePluginRef[time.Time](env, "roll", input, DateTimeArgument(Literal("date")), DateTimeArgument(Literal(true)))
	}
	rollCal := roll(Field[dateTimePluginEvent, time.Time]("caldate"))
	rollLong := DateTimePluginRef[int64](env, "roll", Field[dateTimePluginEvent, int64]("longdate"), DateTimeArgument(Literal("date")), DateTimeArgument(Literal(true)))
	rollUtil := roll(Field[dateTimePluginEvent, time.Time]("utildate"))
	rollLocal := roll(Field[dateTimePluginEvent, time.Time]("localdate"))
	rollZoned := roll(Field[dateTimePluginEvent, time.Time]("zoneddate"))
	arrayCal := DateTimePluginRef[[]string](env, "as-array", Field[dateTimePluginEvent, time.Time]("caldate"))
	arrayLong := DateTimePluginRef[[]string](env, "as-array", Field[dateTimePluginEvent, int64]("longdate"))
	metadata, ok := DateTimePluginMetadataOf(rollCal)
	if !ok || metadata.Method != "roll" || metadata.InputType != typeOf[time.Time]() || len(metadata.Footprints) != 1 {
		t.Fatalf("date-time plugin metadata = %#v", metadata)
	}
	stream := From[dateTimePluginEvent](env, "DateTimePluginEvent")
	plan, err := env.Build(Select(stream, Alias("roll", rollCal)).Query(StatementName("datetime-plugin-metadata")))
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(string(plan.Canonical()), "datetime-plugin(roll:") {
		t.Fatalf("date-time plugin missing from plan canonical: %s", plan.Canonical())
	}

	query := Select(stream,
		Alias("rollCal", rollCal),
		Alias("rollLong", rollLong),
		Alias("rollUtil", rollUtil),
		Alias("rollLocal", rollLocal),
		Alias("rollZoned", rollZoned),
		Alias("arrayCal", arrayCal),
		Alias("arrayLong", arrayLong),
	).Query(StatementName("datetime-plugin-parity"))
	plan, err = env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 1)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("date-time plugin result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), dateTimePluginEvent{
		CalDate: base, LongDate: base.UnixMilli(), UtilDate: base, LocalDate: base, ZonedDate: base,
	}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("date-time plugin row count = %d", len(rows))
	}
	wantTime := base.AddDate(0, 0, 1)
	for _, name := range []string{"rollCal", "rollUtil", "rollLocal", "rollZoned"} {
		got, ok := rows[0].Get(name).Any().(time.Time)
		if !ok || !got.Equal(wantTime) {
			t.Fatalf("date-time plugin %s = %#v, want %s", name, rows[0].Get(name).Any(), wantTime)
		}
	}
	if got, ok := rows[0].Get("rollLong").Any().(int64); !ok || got != wantTime.UnixMilli() {
		t.Fatalf("date-time plugin rollLong = %#v, want %d", rows[0].Get("rollLong").Any(), wantTime.UnixMilli())
	}
	wantArray := []string{"30", "5", "2002"}
	for _, name := range []string{"arrayCal", "arrayLong"} {
		got, ok := rows[0].Get(name).Any().([]string)
		if !ok || !reflect.DeepEqual(got, wantArray) {
			t.Fatalf("date-time plugin %s = %#v, want %#v", name, rows[0].Get(name).Any(), wantArray)
		}
	}
}

func TestDateTimePluginInvalidRulesAndInlineForm(t *testing.T) {
	env := NewEnvironment()
	if err := RegisterDateTimePlugin(env, "as-array", dateTimePluginReformatFootprints(), dateTimePluginReformatFactory); err != nil {
		t.Fatal(err)
	}
	inline := DateTimePlugin[[]string]("inline-array", Literal(time.Date(2002, time.May, 30, 0, 0, 0, 0, time.UTC)), dateTimePluginReformatFootprints(), dateTimePluginReformatFactory)
	buildDateTimePluginExpression(t, env, inline)
	if got := inline.eval(EvalContext{}); !got.Equal(Present([]string{"30", "5", "2002"})) {
		t.Fatalf("inline date-time plugin = %v", got)
	}
	footprints, ok := env.DateTimePluginFootprints("as-array")
	if !ok || len(footprints) != 1 {
		t.Fatalf("date-time footprints = %#v, %v", footprints, ok)
	}
	footprints[0].Input = DateTimeInputEpochMillis
	unchanged, ok := env.DateTimePluginFootprints("as-array")
	if !ok || unchanged[0].Input != DateTimeInputAny {
		t.Fatalf("date-time footprints were not copied: %#v", unchanged)
	}
	if err := RegisterDateTimePlugin(env, "invalid-noop", dateTimePluginReformatFootprints(), func(DateTimePluginFactoryContext) DateTimePluginOps { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Build(SelectOnce(env, Alias("bad", DateTimePluginRef[[]string](env, "invalid-noop", Literal(time.Now()))))); err == nil {
		t.Fatal("date-time no-op plugin unexpectedly built")
	}
	if err := RegisterDateTimePlugin(env, "specific-time", []DateTimeMethodFootprint{{Input: DateTimeInputTime}}, dateTimePluginReformatFactory); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Build(SelectOnce(env, Alias("bad-input", DateTimePluginRef[[]string](env, "specific-time", Literal(int64(1)))))); err == nil {
		t.Fatal("date-time input mismatch unexpectedly built")
	}
	if err := RegisterDateTimePlugin(env, "numeric-arg", []DateTimeMethodFootprint{{
		Input:      DateTimeInputAny,
		Parameters: []DateTimeMethodParameter{{Expected: DateTimeParameterNumeric}},
	}}, dateTimePluginReformatFactory); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Build(SelectOnce(env, Alias("bad-arg", DateTimePluginRef[[]string](env, "numeric-arg", Literal(time.Now()), DateTimeArgument(Literal("not-number")))))); err == nil {
		t.Fatal("date-time argument mismatch unexpectedly built")
	}
	if _, err := env.Build(SelectOnce(env, Alias("unknown", DateTimePluginRef[[]string](env, "missing", Literal(time.Now()))))); err == nil {
		t.Fatal("unknown date-time plugin unexpectedly built")
	}
	if err := RegisterDateTimePlugin(env, "as-array", dateTimePluginReformatFootprints(), dateTimePluginReformatFactory); err == nil {
		t.Fatal("duplicate date-time plugin unexpectedly registered")
	}
	if err := RegisterDateTimePlugin(env, "nil-factory", dateTimePluginReformatFootprints(), nil); err == nil {
		t.Fatal("nil date-time factory unexpectedly registered")
	}
}
