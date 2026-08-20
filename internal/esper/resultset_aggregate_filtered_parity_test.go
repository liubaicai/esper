package esper

import (
	"context"
	"math"
	"testing"
)

type resultsetAggregateFilteredTestBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	IntBoxed      *int   `esper:"intBoxed"`
	BoolPrimitive bool   `esper:"boolPrimitive"`
}

func newResultSetAggregateFilteredTestRuntime(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[resultsetAggregateFilteredTestBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env)
}

func collectResultSetAggregateFilteredRows(t *testing.T, statement *Statement) *[]Row {
	t.Helper()
	rows := make([]Row, 0)
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("aggregate result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &rows
}

func TestResultSetAggregateBlackWhitePercentParity(t *testing.T) {
	env, engine := newResultSetAggregateFilteredTestRuntime(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[resultsetAggregateFilteredTestBean](env, "SupportBean").Window(LengthWindow(3))
	flag := Field[resultsetAggregateFilteredTestBean, bool]("boolPrimitive")
	trueCount := CountIf(flag)
	plan, err := env.Build(source.Aggregate(
		Alias("cb", trueCount),
		Alias("cnb", CountIf(Not(flag))),
		Alias("c", CountAll()),
		Alias("pct", Divide[float64](
			Cast[int64, float64](trueCount),
			Cast[int64, float64](CountAll()),
		)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectResultSetAggregateFilteredRows(t, deployment.Statements()[0])

	for _, event := range []resultsetAggregateFilteredTestBean{
		{TheString: "E", BoolPrimitive: true},
		{TheString: "E", BoolPrimitive: false},
		{TheString: "E", BoolPrimitive: false},
		{TheString: "E", BoolPrimitive: false},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 4 {
		t.Fatalf("black/white rows = %d, want 4", len(*rows))
	}
	expected := [][4]float64{{1, 0, 1, 1}, {1, 1, 2, 0.5}, {1, 2, 3, 1.0 / 3.0}, {0, 3, 3, 0}}
	for index, values := range expected {
		row := (*rows)[index]
		if row.Get("cb").Any() != int64(values[0]) || row.Get("cnb").Any() != int64(values[1]) || row.Get("c").Any() != int64(values[2]) {
			t.Fatalf("black/white row %d = %#v", index, row.AsMap())
		}
		pct, ok := row.Get("pct").Any().(float64)
		if !ok || math.Abs(pct-values[3]) > 1e-12 {
			t.Fatalf("black/white pct %d = %#v, want %v", index, row.Get("pct").Any(), values[3])
		}
	}
}

func TestResultSetAggregateCountVariationsParity(t *testing.T) {
	env, engine := newResultSetAggregateFilteredTestRuntime(t)
	defer func() { _ = engine.Close(context.Background()) }()

	source := From[resultsetAggregateFilteredTestBean](env, "SupportBean").Window(LengthWindow(3))
	flag := Field[resultsetAggregateFilteredTestBean, bool]("boolPrimitive")
	intBoxed := Field[resultsetAggregateFilteredTestBean, *int]("intBoxed")
	plan, err := env.Build(source.Aggregate(
		Alias("c1", FilterAggregate[int64](Count[*int](intBoxed), flag)),
		Alias("c2", FilterAggregate[int64](CountDistinct[*int](intBoxed), flag)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectResultSetAggregateFilteredRows(t, deployment.Statements()[0])

	boxed := func(value int) *int { return &value }
	inputs := []struct {
		value int
		flag  bool
	}{
		{100, true}, {100, true}, {101, false}, {102, true}, {103, false}, {104, false}, {105, false},
	}
	for _, input := range inputs {
		if err := engine.SendEvent(context.Background(), resultsetAggregateFilteredTestBean{IntBoxed: boxed(input.value), BoolPrimitive: input.flag}); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != len(inputs) {
		t.Fatalf("count variations rows = %d, want %d", len(*rows), len(inputs))
	}
	expected := [][2]int64{{1, 1}, {2, 1}, {2, 1}, {2, 2}, {1, 1}, {1, 1}, {0, 0}}
	for index, values := range expected {
		row := (*rows)[index]
		if row.Get("c1").Any() != values[0] || row.Get("c2").Any() != values[1] {
			t.Fatalf("count variations row %d = %#v, want [%d %d]", index, row.AsMap(), values[0], values[1])
		}
	}
}
