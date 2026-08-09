package esper

import (
	"context"
	"reflect"
	"testing"
)

type clientMultitenancySupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func newClientMultitenancyEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientMultitenancySupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

func subscribeClientMultitenancyRows(t *testing.T, statement *Statement) *[]Row {
	t.Helper()
	rows := &[]Row{}
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("multitenancy result = %#v, want row", result)
			}
			*rows = append(*rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestClientMultitenancyInsertIntoSingleModuleTwoStatementsParity(t *testing.T) {
	env := newClientMultitenancyEnvironment(t)
	if _, err := RegisterMap(env, "SomeStream", []FieldSpec{
		FieldDef("theString", reflect.TypeOf("")),
		FieldDef("intPrimitive", reflect.TypeOf(int(0))),
	}); err != nil {
		t.Fatal(err)
	}
	producer, err := env.Build(Select(
		From[clientMultitenancySupportBean](env, "SupportBean"),
		Alias("theString", Field[clientMultitenancySupportBean, string]("theString")),
		Alias("intPrimitive", Field[clientMultitenancySupportBean, int]("intPrimitive")),
	).InsertInto("SomeStream", StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := env.Build(FromAny(env, "SomeStream").
		Filter(Equal[int](Field[Event, int]("intPrimitive"), Literal(0))).
		Select(
			Alias("theString", Field[Event, string]("theString")),
			Alias("intPrimitive", Field[Event, int]("intPrimitive")),
		).
		Query(StatementName("s1")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.DeployPlans(context.Background(), []Plan{producer, consumer})
	if err != nil {
		t.Fatal(err)
	}
	statement, ok := deployment.Statement("s1")
	if !ok {
		t.Fatal("single-module consumer s1 is missing")
	}
	rows := subscribeClientMultitenancyRows(t, statement)

	send := func(name string, value int, want bool) {
		t.Helper()
		before := len(*rows)
		if err := engine.SendEvent(context.Background(), clientMultitenancySupportBean{TheString: name, IntPrimitive: value}); err != nil {
			t.Fatal(err)
		}
		if got := len(*rows) - before; got != boolCount(want) {
			t.Fatalf("%s delivered = %d, want %d", name, got, boolCount(want))
		}
		if want {
			row := (*rows)[len(*rows)-1]
			if row.Get("theString").Any() != name || row.Get("intPrimitive").Any() != value {
				t.Fatalf("%s row = %#v", name, row)
			}
		}
	}
	send("E1", 0, true)
	send("E2", 1, false)
	send("E3", 1, false)
	send("E4", 0, true)
}

func TestClientMultitenancyInsertIntoTwoModuleParity(t *testing.T) {
	env := newClientMultitenancyEnvironment(t)
	if _, err := RegisterMap(env, "SomePublicStream", []FieldSpec{
		FieldDef("a", reflect.TypeOf("")),
		FieldDef("b", reflect.TypeOf(int(0))),
	}); err != nil {
		t.Fatal(err)
	}
	producer, err := env.Build(Select(
		From[clientMultitenancySupportBean](env, "SupportBean"),
		Alias("a", Field[clientMultitenancySupportBean, string]("theString")),
		Alias("b", Field[clientMultitenancySupportBean, int]("intPrimitive")),
	).InsertInto("SomePublicStream", StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := env.Build(FromAny(env, "SomePublicStream").Select(
		Alias("a", Field[Event, string]("a")),
		Alias("b", Field[Event, int]("b")),
	).Query(StatementName("s1")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), producer); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientMultitenancySupportBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	consumerDeployment, err := engine.Deploy(context.Background(), consumer)
	if err != nil {
		t.Fatal(err)
	}
	rows := subscribeClientMultitenancyRows(t, consumerDeployment.Statements()[0])
	for index, name := range []string{"E2", "E3", "E4"} {
		value := index + 2
		if err := engine.SendEvent(context.Background(), clientMultitenancySupportBean{TheString: name, IntPrimitive: value}); err != nil {
			t.Fatal(err)
		}
		row := (*rows)[index]
		if row.Get("a").Any() != name || row.Get("b").Any() != value {
			t.Fatalf("late consumer row %d = %#v", index, row)
		}
	}
	if len(*rows) != 3 {
		t.Fatalf("late consumer rows = %d, want 3", len(*rows))
	}
}

func TestClientMultitenancyIndexTableParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "VWAPrice", []FieldSpec{FieldDef("symbol", reflect.TypeOf(""))}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "Basket", []TableColumn{
		PrimaryKeyColumn[string]("basket_id"),
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[float64]("weight"),
	}, SecondaryIndex("BasketIndex", "symbol")); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	table, ok := engine.Table("Basket")
	if !ok {
		t.Fatal("Basket table is missing")
	}
	for _, row := range []map[string]any{
		{"basket_id": "1", "symbol": "A", "weight": 1.0},
		{"basket_id": "2", "symbol": "B", "weight": 2.0},
	} {
		if _, err := table.Insert(context.Background(), row); err != nil {
			t.Fatal(err)
		}
	}
	join := JoinMany(
		JoinRecordSource(FromTable(env, "Basket")),
		JoinRecordSource(FromAny(env, "VWAPrice")).Unidirectional(),
	).On(OnSourcesEqual(
		0, Field[any, string]("symbol"),
		1, Field[any, string]("symbol"),
	))
	plan, err := env.Build(join.Select(
		SelectFrom(0, "weight", JoinField[float64](0, "weight")),
	).Query(StatementName("s0"), UseIndexOn(0, "BasketIndex")))
	if err != nil {
		t.Fatal(err)
	}
	selection, ok := plan.IndexPlan().ForSource(0)
	if !ok || selection.IndexName != "BasketIndex" || selection.Access != IndexAccessEquality || !selection.Hinted {
		t.Fatalf("Basket index plan = %#v", selection)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := subscribeClientMultitenancyRows(t, deployment.Statements()[0])
	for index, symbol := range []string{"A", "B"} {
		if err := engine.Send(context.Background(), "VWAPrice", map[string]any{"symbol": symbol}); err != nil {
			t.Fatal(err)
		}
		if got := (*rows)[index].Get("weight").Any(); got != float64(index+1) {
			t.Fatalf("%s weight = %v, want %d", symbol, got, index+1)
		}
	}
}
