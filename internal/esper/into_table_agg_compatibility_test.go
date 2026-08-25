package esper

import (
	"context"
	"strings"
	"testing"
)

type intoTableCompatBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func declareIntoTableCompatTable(t *testing.T, env *Environment) {
	t.Helper()
	if _, err := RegisterStruct[intoTableCompatBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	columns := []TableColumn{
		OptionalTableColumnOf[int]("maxb", WithTableAgg("max", "max(int)", true)),
		OptionalTableColumnOf[int]("maxu", WithTableAgg("maxever", "maxever(int)", false)),
		OptionalTableColumnOf[int]("minb", WithTableAgg("min", "min(int)", true)),
		OptionalTableColumnOf[int]("minu", WithTableAgg("minever", "minever(int)", false)),
		OptionalTableColumnOf[any]("lasteveru", WithTableAgg("lastever", "lastever(*)", false)),
		OptionalTableColumnOf[any]("firsteveru", WithTableAgg("firstever", "firstever(*)", false)),
		OptionalTableColumnOf[any]("windowb", WithTableAgg("window", "window(*)", false)),
		OptionalTableColumnOf[any]("maxbyeveru", WithTableAgg("maxbyever", "maxbyever(intPrimitive)", false)),
		OptionalTableColumnOf[any]("minbyeveru", WithTableAgg("minbyever", "minbyever(intPrimitive)", false)),
		OptionalTableColumnOf[any]("sortedb", WithTableAgg("sorted", "sorted(intPrimitive)", false)),
	}
	if _, err := CreateTable(env, "varagg", columns); err != nil {
		t.Fatal(err)
	}
}

// TestIntoTableCompatibilityRejections pins the exact Java diagnostics that
// InfraBoundUnbound asserts via tryInvalidCompile; every message must stay
// byte-compatible with the pinned oracle trace.
func TestIntoTableCompatibilityRejections(t *testing.T) {
	tests := []struct {
		name  string
		build func(env *Environment) Query
		want  string
	}{
		{
			name: "unbound max into bound column",
			build: func(env *Environment) Query {
				return From[intoTableCompatBean](env, "SupportBean").Aggregate(
					Alias("maxb", Max[int](Field[intoTableCompatBean, int]("intPrimitive"))),
				).IntoTable("varagg")
			},
			want: "Incompatible aggregation function for table 'varagg' column 'maxb', expecting 'max(int)' and received 'max(intPrimitive)': The table declares use with data windows and provided is unbound",
		},
		{
			name: "last into table",
			build: func(env *Environment) Query {
				return From[intoTableCompatBean](env, "SupportBean").Window(LengthWindow(2)).Aggregate(
					Alias("lasteveru", Last[Event](EventValue[Event]())),
				).IntoTable("varagg")
			},
			want: "Failed to validate select-clause expression 'last(*)': For into-table use 'window(*)' or 'window(stream.*)' instead",
		},
		{
			name: "lastever into window column",
			build: func(env *Environment) Query {
				return From[intoTableCompatBean](env, "SupportBean").Window(LengthWindow(2)).Aggregate(
					Alias("windowb", LastEver[Event](EventValue[Event]())),
				).IntoTable("varagg")
			},
			want: "Incompatible aggregation function for table 'varagg' column 'windowb', expecting 'window(*)' and received 'lastever(*)': The table declares 'window(*)' and provided is 'lastever(*)'",
		},
		{
			name: "lastever null argument",
			build: func(env *Environment) Query {
				return From[intoTableCompatBean](env, "SupportBean").Window(LengthWindow(2)).Aggregate(
					Alias("lasteveru", LastEver[any](Literal[any](nil))),
				).IntoTable("varagg")
			},
			want: "Failed to validate select-clause expression 'lastever(null)': Null-type is not allowed",
		},
		{
			name: "maxby sort expression",
			build: func(env *Environment) Query {
				return From[intoTableCompatBean](env, "SupportBean").Window(LengthWindow(2)).Aggregate(
					Alias("maxbyeveru", MaxBy[Event, int](EventValue[Event](), Field[intoTableCompatBean, int]("intPrimitive"))),
				).IntoTable("varagg")
			},
			want: "Failed to validate select-clause expression 'maxby(intPrimitive)': When specifying into-table a sort expression cannot be provided",
		},
		{
			name: "maxbyever into sorted column",
			build: func(env *Environment) Query {
				return From[intoTableCompatBean](env, "SupportBean").Window(LengthWindow(2)).Aggregate(
					Alias("sortedb", MaxByEver[Event, int](EventValue[Event]())),
				).IntoTable("varagg")
			},
			want: "Incompatible aggregation function for table 'varagg' column 'sortedb', expecting 'sorted(intPrimitive)' and received 'maxbyever()': The required aggregation function name is 'sorted' and provided is 'maxbyever'",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := NewEnvironment()
			declareIntoTableCompatTable(t, env)
			_, err := env.Build(test.build(env))
			if err == nil {
				t.Fatalf("expected rejection containing %q", test.want)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), test.want)
			}
		})
	}
}

// TestIntoTableCompatibilityAccepts covers the positive half of the
// bound/unbound matrix: bound streams may feed ever-columns and window
// columns accept length-bounded streams.
func TestIntoTableCompatibilityAccepts(t *testing.T) {
	build := func(f func(env *Environment) Query) error {
		env := NewEnvironment()
		declareIntoTableCompatTable(t, env)
		_, err := env.Build(f(env))
		return err
	}
	if err := build(func(env *Environment) Query {
		src := From[intoTableCompatBean](env, "SupportBean").Window(LengthWindow(2))
		ip := Field[intoTableCompatBean, int]("intPrimitive")
		return src.Aggregate(
			Alias("maxb", Max[int](ip)),
			Alias("maxu", MaxEver[int](ip)),
			Alias("minb", Min[int](ip)),
			Alias("minu", MinEver[int](ip)),
		).IntoTable("varagg")
	}); err != nil {
		t.Fatalf("bound min/max + ever: %v", err)
	}
	if err := build(func(env *Environment) Query {
		src := From[intoTableCompatBean](env, "SupportBean").Window(LengthWindow(2))
		return src.Aggregate(
			Alias("windowb", WindowEvents()),
			Alias("lasteveru", LastEver[Event](EventValue[Event]())),
			Alias("firsteveru", FirstEver[Event](EventValue[Event]())),
		).IntoTable("varagg")
	}); err != nil {
		t.Fatalf("window + ever from bound stream: %v", err)
	}
	if err := build(func(env *Environment) Query {
		src := From[intoTableCompatBean](env, "SupportBean").Window(LengthWindow(2))
		ip := Field[intoTableCompatBean, int]("intPrimitive")
		return src.Aggregate(
			Alias("maxbyeveru", MaxByEver[Event, int](EventValue[Event](), ip)),
			Alias("sortedb", SortedEventsBy[Event, int](EventValue[Event](), ip, false)),
		).IntoTable("varagg")
	}); err != nil {
		t.Fatalf("maxbyever + sorted: %v", err)
	}
}

// TestMaxMinEverScalarAggregations pins the bound-vs-ever discriminator of
// the InfraBoundUnbound contract: length-window max/min slide with the
// window while max-ever/min-ever retain full history.
func TestMaxMinEverScalarAggregations(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[intoTableCompatBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	theString := Field[intoTableCompatBean, string]("theString")
	intPrimitive := Field[intoTableCompatBean, int]("intPrimitive")
	plan, err := env.Build(From[intoTableCompatBean](env, "SupportBean").Window(LengthWindow(2)).GroupBy(theString).Select(
		Alias("maxb", Max[int](intPrimitive)),
		Alias("maxu", MaxEver[int](intPrimitive)),
		Alias("minb", Min[int](intPrimitive)),
		Alias("minu", MinEver[int](intPrimitive)),
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
	var last map[string]int
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, r := range batch.New {
			last = map[string]int{
				"maxb": r.Get("maxb").Any().(int),
				"maxu": r.Get("maxu").Any().(int),
				"minb": r.Get("minb").Any().(int),
				"minu": r.Get("minu").Any().(int),
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []intoTableCompatBean{{"A", 20}, {"A", 15}, {"A", 10}} {
		if err := engine.Send(context.Background(), "SupportBean", event); err != nil {
			t.Fatal(err)
		}
	}
	want := map[string]int{"maxb": 15, "maxu": 20, "minb": 10, "minu": 10}
	for name, value := range want {
		if last[name] != value {
			t.Fatalf("%s = %d, want %d (row %v)", name, last[name], value, last)
		}
	}
}

// TestUnkeyedIntoTableRowLifecycle pins the Go into-table lifecycle for an
// unkeyed table: the logical row is materialized at deployment and its
// count advances with contributions. Java defers the observable row to the
// first contribution; the differential runner registers that representation
// difference by normalizing pre-contribution snapshots to the empty set.
func TestUnkeyedIntoTableRowLifecycle(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[intoTableCompatBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "MyTable", []TableColumn{TableColumnOf[int64]("mycnt")}); err != nil {
		t.Fatal(err)
	}
	snapPlan, err := env.Build(FromTable(env, "MyTable").Query(StatementName("snapshot")))
	if err != nil {
		t.Fatal(err)
	}
	intoPlan, err := env.Build(From[intoTableCompatBean](env, "SupportBean").Aggregate(
		Alias("mycnt", CountAll()),
	).IntoTable("MyTable"))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer engine.Close(context.Background())
	if _, err := engine.Deploy(context.Background(), intoPlan); err != nil {
		t.Fatal(err)
	}
	read := func() int {
		res, err := engine.ExecuteFireAndForget(context.Background(), snapPlan)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, result := range res.Results() {
			event, ok := result.Event()
			if !ok {
				continue
			}
			for _, field := range event.Schema().Fields() {
				if event.Get(field.Name).IsPresent() {
					count++
					break
				}
			}
		}
		return count
	}
	if got := read(); got != 1 {
		t.Fatalf("pre-event rows = %d, want 1 (registered representation difference)", got)
	}
	if err := engine.Send(context.Background(), "SupportBean", intoTableCompatBean{IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if got := read(); got != 1 {
		t.Fatalf("post-event rows = %d, want 1", got)
	}
	table, ok := engine.Table("MyTable")
	if !ok {
		t.Fatal("table missing")
	}
	rows, err := table.Snapshot(context.Background())
	if err != nil || len(rows) != 1 {
		t.Fatalf("table snapshot = %d rows, err=%v", len(rows), err)
	}
	if got := rows[0].Get("mycnt").Any(); got != int64(1) {
		t.Fatalf("mycnt = %#v, want 1", got)
	}
}
