package esper

import (
	"context"
	"strings"
	"testing"
	"time"
)

type whenValidMD struct {
	Symbol string
	ID     string
	Price  float64
	Volume *int64
	Feed   *string
}

func TestOutputWhenRejectsAggregate(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[whenValidMD](env, "MD"); err != nil {
		t.Fatal(err)
	}
	env.RegisterVariable("myvar", 0)
	_, err := env.Build(From[whenValidMD](env, "MD").Query(StatementName("s"),
		WithOutput(OutputWhen(Greater[int64](Sum[int64](OutputCountInsert()), Literal(int64(0)))))))
	if err == nil || !strings.Contains(err.Error(), "aggregate function may not appear") {
		t.Fatalf("err = %v", err)
	}
}

func TestOutputWhenRejectsPrev(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[whenValidMD](env, "MD"); err != nil {
		t.Fatal(err)
	}
	env.RegisterVariable("myvar", 0)
	_, err := env.Build(From[whenValidMD](env, "MD").Query(StatementName("s"),
		WithOutput(OutputWhen(Equal[int64](Prev[int64](1, OutputCountInsert()), Literal(int64(0)))))))
	if err == nil || !strings.Contains(err.Error(), "Previous function cannot be used") {
		t.Fatalf("err = %v", err)
	}
}

func TestOutputThenRejectsAggregate(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[whenValidMD](env, "MD"); err != nil {
		t.Fatal(err)
	}
	env.RegisterVariable("myvar", 0)
	env.RegisterVariable("dummy", 0)
	_, err := env.Build(From[whenValidMD](env, "MD").Query(StatementName("s"),
		WithOutput(OutputWhen(Equal[int](VariableRef[int]("myvar"), Literal(1)),
			SetOutputVariable("dummy", Sum[int](VariableRef[int]("myvar")))))))
	if err == nil || !strings.Contains(err.Error(), "Aggregation functions may not be used within update-set") {
		t.Fatalf("err = %v", err)
	}
}

func TestOutputLastWhenEmitsLastRowOnly(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[whenValidMD](env, "MD"); err != nil {
		t.Fatal(err)
	}
	env.RegisterVariable("myvar", 0)
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	defer engine.Close(context.Background())
	plan, err := env.Build(From[whenValidMD](env, "MD").Query(StatementName("s1"),
		WithOutput(OutputWhenWith(OutputLast(), Equal[int](VariableRef[int]("myvar"), Literal(100))))))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := dep.Statements()[0].Subscribe(func(_ context.Context, b ResultBatch) error {
		batches = append(batches, b)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), whenValidMD{Symbol: "ABC", ID: "E1", Price: 100}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), whenValidMD{Symbol: "ABC", ID: "E2", Price: 100}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 0 {
		t.Fatalf("batches before trigger = %d", len(batches))
	}
	if err := engine.SetVariable(context.Background(), "myvar", 100); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("batches after trigger = %#v, want 1 batch with 1 row (E2)", batches)
	}
	if got := batches[0].New[0].Get("ID").Any(); got != "E2" {
		t.Fatalf("row ID = %v, want E2", got)
	}
}

func TestOutputLastWhenGroupedEmitsLastRowPerKey(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[whenValidMD](env, "MD"); err != nil {
		t.Fatal(err)
	}
	env.RegisterVariable("myvar", 0)
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	defer engine.Close(context.Background())
	plan, err := env.Build(From[whenValidMD](env, "MD").
		GroupBy(Field[whenValidMD, string]("Symbol")).
		Select(
			Alias("Symbol", Field[whenValidMD, string]("Symbol")),
			Alias("cnt", CountAll()),
		).
		Query(StatementName("s1"),
			WithOutput(OutputWhenWith(OutputLast(), Equal[int](VariableRef[int]("myvar"), Literal(100))))))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := dep.Statements()[0].Subscribe(func(_ context.Context, b ResultBatch) error {
		batches = append(batches, b)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []whenValidMD{
		{Symbol: "A", ID: "E1", Price: 1},
		{Symbol: "B", ID: "E2", Price: 2},
		{Symbol: "A", ID: "E3", Price: 3},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 0 {
		t.Fatalf("batches before trigger = %d", len(batches))
	}
	if err := engine.SetVariable(context.Background(), "myvar", 100); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	// Java's row-per-group output-last helper keeps only the last row per
	// group key: A's final row (cnt=2) and B's only row (cnt=1).
	if len(batches) != 1 || len(batches[0].New) != 2 {
		t.Fatalf("batches after trigger = %#v, want 1 batch with 2 rows", batches)
	}
	got := map[string]int64{}
	for _, result := range batches[0].New {
		row, ok := result.Row()
		if !ok {
			t.Fatal("result is not a row")
		}
		got[row.Get("Symbol").Any().(string)] = row.Get("cnt").Any().(int64)
	}
	if got["A"] != 2 || got["B"] != 1 {
		t.Fatalf("rows = %v, want A=2 B=1", got)
	}
}
