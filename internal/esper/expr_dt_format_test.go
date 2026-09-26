package esper

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestDateTimeFormatRenders checks all format() surface variants against the
// pinned ExprDTFormat oracle strings (JDK17, en_US, UTC): SimpleDateFormat
// default and getDateInstance, ISO_DATE_TIME / ISO_ZONED_DATE_TIME, the
// 'yyyy.MM.dd G 'at' HH:mm:ss' pattern (incl. the 'G' era field), and
// BASIC_ISO_DATE. Null and missing inputs produce Null cells.
func TestDateTimeFormatRenders(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "DT", []FieldSpec{
		{Name: "l", Type: reflect.TypeOf(int64(0))},
		{Name: "t", Type: reflect.TypeOf(time.Time{})},
	}); err != nil {
		t.Fatal(err)
	}
	longdate := Field[map[string]any, int64]("l")
	utildate := Field[map[string]any, time.Time]("t")
	plan, err := env.Build(FromAny(env, "DT").Select(
		Alias("defLong", DateTimeFormatDefault[int64](longdate)),
		Alias("defUtil", DateTimeFormatDefault[time.Time](utildate)),
		Alias("dateInst", DateTimeFormatDateInstance[time.Time](utildate)),
		Alias("iso", DateTimeFormatISO[time.Time](utildate)),
		Alias("isoZoned", DateTimeFormatISOZoned[time.Time](utildate)),
		Alias("patLong", DateTimeFormatPattern[int64](longdate, "yyyy.MM.dd G 'at' HH:mm:ss")),
		Alias("patUtil", DateTimeFormatPattern[time.Time](utildate, "yyyy.MM.dd G 'at' HH:mm:ss")),
		Alias("basic", DateTimeFormatPattern[time.Time](utildate, "yyyyMMdd")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	rows := subscribeRows(t, engine, plan)

	// 2002-05-30T09:00:00Z
	if err := engine.SendRecord(context.Background(), "DT", map[string]any{
		"l": int64(1022749200000),
		"t": time.Date(2002, 5, 30, 9, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	got := rows()
	if len(got) != 1 {
		t.Fatalf("rows = %d, want 1", len(got))
	}
	row := got[0]
	want := map[string]string{
		"defLong":  "5/30/02, 9:00 AM",
		"defUtil":  "5/30/02, 9:00 AM",
		"dateInst": "May 30, 2002",
		"iso":      "2002-05-30T09:00:00",
		"isoZoned": "2002-05-30T09:00:00Z[UTC]",
		"patLong":  "2002.05.30 AD at 09:00:00",
		"patUtil":  "2002.05.30 AD at 09:00:00",
		"basic":    "20020530",
	}
	for column, expected := range want {
		if v, ok := row.Get(column).Any().(string); !ok || v != expected {
			t.Fatalf("%s = %#v, want %q", column, row.Get(column).Any(), expected)
		}
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
		for column := range want {
			if got[index].Get(column).IsPresent() {
				t.Fatalf("row %d %s = %#v, want null", index, column, got[index].Get(column).Any())
			}
		}
	}
	// Single-digit day: getDateInstance() pads nothing (Java 'May 5, 2002',
	// not 'May  5, 2002').
	if err := engine.SendRecord(context.Background(), "DT", map[string]any{
		"l": int64(1020618000000), // 2002-05-05T17:00:00Z
		"t": time.Date(2002, 5, 5, 9, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	got = rows()
	if len(got) != 4 {
		t.Fatalf("rows = %d, want 4", len(got))
	}
	if v := got[3].Get("dateInst").Any(); v != "May 5, 2002" {
		t.Fatalf("dateInst single-digit day = %#v, want %q", v, "May 5, 2002")
	}
}

// TestDateTimeFormatQuotedLiterals checks that quoted Java literals never
// reach the Go layout: a literal 'Jan', '05', 'pm', 'MST', or '2006' must
// render verbatim rather than being re-tokenized by time.Format.
func TestDateTimeFormatQuotedLiterals(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "DT", []FieldSpec{
		{Name: "t", Type: reflect.TypeOf(time.Time{})},
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "DT").Select(
		Alias("lit", DateTimeFormatPattern[time.Time](Field[map[string]any, time.Time]("t"), "'Jan at 05 pm'")),
		Alias("litM", DateTimeFormatPattern[time.Time](Field[map[string]any, time.Time]("t"), "'MST 2006'")),
		Alias("real", DateTimeFormatPattern[time.Time](Field[map[string]any, time.Time]("t"), "yyyy")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	rows := subscribeRows(t, engine, plan)
	if err := engine.SendRecord(context.Background(), "DT", map[string]any{
		"t": time.Date(2002, 5, 30, 9, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	got := rows()
	if len(got) != 1 {
		t.Fatalf("literal rows = %d, want 1", len(got))
	}
	if v := got[0].Get("lit").Any(); v != "Jan at 05 pm" {
		t.Fatalf("quoted literal = %#v, want %q", v, "Jan at 05 pm")
	}
	if v := got[0].Get("litM").Any(); v != "MST 2006" {
		t.Fatalf("quoted literal = %#v, want %q", v, "MST 2006")
	}
	if v := got[0].Get("real").Any(); v != "2002" {
		t.Fatalf("yyyy = %#v, want %q", v, "2002")
	}
}

// TestDateTimeFormatPatternValidation checks the constant-pattern
// build-time validation: unsupported Java pattern letters, unsupported run
// lengths, and unterminated quotes are all configuration errors.
func TestDateTimeFormatPatternValidation(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "DT", []FieldSpec{
		{Name: "l", Type: reflect.TypeOf(int64(0))},
	}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		pattern string
		want    string
	}{
		{name: "unsupported-letter", pattern: "w", want: "unsupported"},
		{name: "hour-1-24", pattern: "kk:mm", want: "unsupported"},
		{name: "unpadded-hour", pattern: "H:mm", want: "unsupported"},
		{name: "frac-width", pattern: "mm:ss.SS", want: "unsupported"},
		{name: "full-zone", pattern: "zzzz", want: "unsupported"},
		{name: "unterminated-quote", pattern: "yyyy 'at", want: "unterminated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := env.Build(FromAny(env, "DT").Select(
				Alias("c0", DateTimeFormatPattern[int64](Field[map[string]any, int64]("l"), tc.pattern)),
			).Query(StatementName("s0"))); err == nil || !errors.Is(err, ErrorInvalidRule) ||
				!strings.Contains(err.Error(), tc.want) {
				t.Fatalf("build error = %v, want %q", err, tc.want)
			}
		})
	}
}
