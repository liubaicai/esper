package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestDateTimeRoundVectorsParity(t *testing.T) {
	base := time.Date(2002, 5, 30, 9, 1, 2, int(3*time.Millisecond), time.Local)
	halfBase := time.Date(2002, 5, 30, 15, 30, 2, int(550*time.Millisecond), time.Local)
	tie := time.Date(2002, 5, 30, 15, 30, 30, 0, time.Local)
	type spec struct {
		name string
		in   time.Time
		unit string
		mode string
		want time.Time
	}
	cases := []spec{
		{"ceil-hour", base, "hour", "ceil", time.Date(2002, 5, 30, 10, 0, 0, 0, time.Local)},
		{"ceil-msec", base, "msec", "ceil", base},
		{"ceil-sec", base, "sec", "ceil", time.Date(2002, 5, 30, 9, 1, 3, 0, time.Local)},
		{"ceil-min", base, "min", "ceil", time.Date(2002, 5, 30, 9, 2, 0, 0, time.Local)},
		{"ceil-day", base, "day", "ceil", time.Date(2002, 5, 31, 0, 0, 0, 0, time.Local)},
		{"ceil-month", base, "month", "ceil", time.Date(2002, 6, 1, 0, 0, 0, 0, time.Local)},
		{"ceil-year", base, "year", "ceil", time.Date(2003, 1, 1, 0, 0, 0, 0, time.Local)},
		{"floor-sec", base, "sec", "floor", time.Date(2002, 5, 30, 9, 1, 2, 0, time.Local)},
		{"floor-min", base, "min", "floor", time.Date(2002, 5, 30, 9, 1, 0, 0, time.Local)},
		{"floor-hour", base, "hour", "floor", time.Date(2002, 5, 30, 9, 0, 0, 0, time.Local)},
		{"floor-day", base, "day", "floor", time.Date(2002, 5, 30, 0, 0, 0, 0, time.Local)},
		{"floor-month", base, "month", "floor", time.Date(2002, 5, 1, 0, 0, 0, 0, time.Local)},
		{"floor-year", base, "year", "floor", time.Date(2002, 1, 1, 0, 0, 0, 0, time.Local)},
		{"half-msec", halfBase, "msec", "half", halfBase},
		{"half-sec", halfBase, "sec", "half", time.Date(2002, 5, 30, 15, 30, 3, 0, time.Local)},
		{"half-min", halfBase, "min", "half", time.Date(2002, 5, 30, 15, 30, 0, 0, time.Local)},
		{"half-hour", halfBase, "hour", "half", time.Date(2002, 5, 30, 16, 0, 0, 0, time.Local)},
		{"half-day", halfBase, "day", "half", time.Date(2002, 5, 31, 0, 0, 0, 0, time.Local)},
		{"half-month-may31", halfBase, "month", "half", time.Date(2002, 6, 1, 0, 0, 0, 0, time.Local)},
		{"half-year", halfBase, "year", "half", time.Date(2002, 1, 1, 0, 0, 0, 0, time.Local)},
		{"half-min-tie-up", tie, "min", "half", time.Date(2002, 5, 30, 15, 31, 0, 0, time.Local)},
		// P1 repro: Commons MODIFY_CEILING adds one unit unconditionally.
		{"ceil-min-on-boundary", time.Date(2002, 5, 30, 9, 0, 0, 0, time.Local), "min", "ceil", time.Date(2002, 5, 30, 9, 1, 0, 0, time.Local)},
		{"ceil-hour-on-boundary", time.Date(2002, 5, 30, 9, 0, 0, 0, time.Local), "hour", "ceil", time.Date(2002, 5, 30, 10, 0, 0, 0, time.Local)},
		{"ceil-day-on-boundary", time.Date(2002, 5, 30, 0, 0, 0, 0, time.Local), "day", "ceil", time.Date(2002, 5, 31, 0, 0, 0, 0, time.Local)},
		// LANG-59 pre-pass subtracts the sub-threshold seconds on the
		// absolute time (00:00:05 - 5s = 00:00:00, same day), so the
		// DATE-row offset still reads 16 > 15 and carries to June.
		{"half-month-midnight-crossing", time.Date(2002, 5, 17, 0, 0, 5, 0, time.Local), "month", "half", time.Date(2002, 6, 1, 0, 0, 0, 0, time.Local)},
		{"half-month-feb28-threshold", time.Date(2002, 2, 15, 0, 0, 0, 0, time.Local), "month", "half", time.Date(2002, 3, 1, 0, 0, 0, 0, time.Local)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterMap(env, "DT", []FieldSpec{
				{Name: "d", Type: reflect.TypeOf(time.Time{})},
				{Name: "l", Type: reflect.TypeOf(int64(0))},
			}); err != nil {
				t.Fatal(err)
			}
			var expr Expression[time.Time]
			switch c.mode {
			case "ceil":
				expr = DateTimeRoundCeiling[time.Time](Field[map[string]any, time.Time]("d"), c.unit)
			case "floor":
				expr = DateTimeRoundFloor[time.Time](Field[map[string]any, time.Time]("d"), c.unit)
			default:
				expr = DateTimeRoundHalf[time.Time](Field[map[string]any, time.Time]("d"), c.unit)
			}
			plan, err := env.Build(FromAny(env, "DT").Select(
				Alias("v", expr),
			).Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			defer engine.Close(context.Background())
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			statement := deployment.Statements()[0]
			var got time.Time
			has := false
			if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
				for _, result := range batch.New {
					if row, ok := result.Row(); ok {
						if v := row.Get("v"); v.IsPresent() {
							got = v.Any().(time.Time)
							has = true
						}
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := engine.SendRecord(context.Background(), "DT", map[string]any{"d": c.in, "l": c.in.UnixMilli()}); err != nil {
				t.Fatal(err)
			}
			if !has {
				t.Fatalf("no row delivered")
			}
			if !got.Equal(c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
	// Representation preservation: int64 in, int64 out.
	env := NewEnvironment()
	if _, err := RegisterMap(env, "DT", []FieldSpec{
		{Name: "l", Type: reflect.TypeOf(int64(0))},
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "DT").Select(
		Alias("v", DateTimeRoundCeiling[time.Time](Field[map[string]any, int64]("l"), "hour")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer engine.Close(context.Background())
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var got int64
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				got = row.Get("v").Any().(int64)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2002, 5, 30, 10, 0, 0, 0, time.Local).UnixMilli()
	if err := engine.SendRecord(context.Background(), "DT", map[string]any{"l": base.UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("int64 rep: got %d want %d", got, want)
	}
}
