package esper

import (
	"context"
	"testing"
)

type enumTableTrigger struct {
	ID string `esper:"id"`
}

// TestEnumerableTableWindowSourceMatchesJava covers the Java
// ExprEnumDataSources.ExprEnumTableRow footprint: a table-resident window
// access value is unwrapped through its typed Values method and then consumed
// by an analyzable enumeration predicate.
func TestEnumerableTableWindowSourceMatchesJava(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "EnumTableTrade"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[enumTableTrigger](env, "EnumTableTrigger"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "EnumTableWindow", []TableColumn{
		TableColumnOf[WindowAccessValue[runtimeTestTrade]]("theWindow"),
	}); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	window := WindowAccessBy[runtimeTestTrade](EventValue[runtimeTestTrade]())
	aggregatePlan, err := env.Build(From[runtimeTestTrade](env, "EnumTableTrade").
		Window(LengthWindow(2)).
		Aggregate(Alias("theWindow", window)).
		IntoTable("EnumTableWindow", StatementName("enum-table-window")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}

	values := Method[[]runtimeTestTrade](
		TableField[WindowAccessValue[runtimeTestTrade]]("theWindow"),
		"Values",
	)
	hasTen := EnumAnyOf[runtimeTestTrade](values, Equal[float64](
		EnumField[runtimeTestTrade, float64]("price"),
		Literal(10.0),
	))
	triggerPlan, err := env.Build(OnEvent(From[enumTableTrigger](env, "EnumTableTrigger")).
		SelectFromTableWhere("EnumTableWindow", Literal(true), Alias("hasTen", hasTen)).
		Query(StatementName("enum-table-enum")))
	if err != nil {
		t.Fatal(err)
	}
	triggerDeployment, err := engine.Deploy(context.Background(), triggerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var results []Row
	if _, err := triggerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				results = append(results, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, event := range []runtimeTestTrade{{Symbol: "E1", Price: 10}, {Symbol: "E2", Price: 20}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.SendEvent(context.Background(), enumTableTrigger{ID: "T1"}); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Get("hasTen").Any() != true {
		t.Fatalf("table enumerable first result = %#v", results)
	}

	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E3", Price: 30}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), enumTableTrigger{ID: "T2"}); err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[1].Get("hasTen").Any() != false {
		t.Fatalf("table enumerable eviction result = %#v", results)
	}
}
