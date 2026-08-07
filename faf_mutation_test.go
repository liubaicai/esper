package esper

import (
	"context"
	"testing"
)

func TestOnDemandNamedWindowMutationMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "OnDemandWindowSchema", []FieldSpec{
		FieldDef("theString", typeOf[string]()),
		FieldDef("intPrimitive", typeOf[int64]()),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "OnDemandWindow", mustSchema(env, "OnDemandWindowSchema"), NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	ctx := context.Background()
	for _, event := range []map[string]any{
		{"theString": "E0", "intPrimitive": int64(0)},
		{"theString": "E1", "intPrimitive": int64(1)},
		{"theString": "E2", "intPrimitive": int64(2)},
	} {
		if err := engine.InsertNamedWindow(ctx, "OnDemandWindow", event); err != nil {
			t.Fatal(err)
		}
	}

	theString := NamedWindowField[string]("theString")
	window := FromNamedWindow(env, "OnDemandWindow")

	insertPlan, err := env.Build(window.OnDemand().Insert(
		SetColumn("theString", Literal("E3")),
		SetColumn("intPrimitive", Literal[int64](3)),
	))
	if err != nil {
		t.Fatal(err)
	}
	inserted, err := engine.ExecuteFireAndForget(ctx, insertPlan)
	if err != nil || len(inserted.Results()) != 1 {
		t.Fatalf("on-demand named-window insert = %#v, err=%v", inserted.Results(), err)
	}
	if inserted.Results()[0].Get("theString").Any() != "E3" {
		t.Fatalf("insert result = %#v", inserted.Results()[0])
	}

	updatePlan, err := env.Build(window.OnDemand().UpdateWhere(
		Equal[string](theString, Literal("E1")),
		SetColumn("theString", Literal("E1-updated")),
		SetColumn("intPrimitive", Add[int64](InitialNamedWindowField[int64]("intPrimitive"), Literal[int64](10))),
	))
	if err != nil {
		t.Fatal(err)
	}
	updated, err := engine.ExecuteFireAndForget(ctx, updatePlan)
	if err != nil || len(updated.Results()) != 1 {
		t.Fatalf("on-demand named-window update = %#v, err=%v", updated.Results(), err)
	}
	if got := updated.Results()[0].Get("intPrimitive").Any(); got != int64(11) {
		t.Fatalf("updated result intPrimitive = %#v, want 11", got)
	}

	deletePlan, err := env.Build(window.OnDemand().DeleteWhere(
		Equal[string](theString, Literal("E0")),
	))
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := engine.ExecuteFireAndForget(ctx, deletePlan)
	if err != nil || len(deleted.Results()) != 1 || deleted.Results()[0].Get("theString").Any() != "E0" {
		t.Fatalf("on-demand named-window delete = %#v, err=%v", deleted.Results(), err)
	}

	deleteAllPlan, err := env.Build(window.OnDemand().DeleteAll())
	if err != nil {
		t.Fatal(err)
	}
	cleared, err := engine.ExecuteFireAndForget(ctx, deleteAllPlan)
	if err != nil || len(cleared.Results()) != 3 {
		t.Fatalf("on-demand named-window delete-all = %#v, err=%v", cleared.Results(), err)
	}
	remaining, err := engine.ExecuteFireAndForget(ctx, env.mustBuild(FromNamedWindow(env, "OnDemandWindow").Query()))
	if err != nil || len(remaining.Results()) != 0 {
		t.Fatalf("on-demand named-window remaining = %#v, err=%v", remaining.Results(), err)
	}
}

func TestOnDemandTableMutationMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := CreateTable(env, "OnDemandTable", []TableColumn{
		PrimaryKeyColumn[string]("theString"),
		TableColumnOf[int64]("intPrimitive"),
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	table, ok := engine.Table("OnDemandTable")
	if !ok {
		t.Fatal("OnDemandTable is missing")
	}
	ctx := context.Background()
	for _, row := range []map[string]any{
		{"theString": "E0", "intPrimitive": int64(0)},
		{"theString": "E1", "intPrimitive": int64(1)},
	} {
		if _, err := table.Insert(ctx, row); err != nil {
			t.Fatal(err)
		}
	}

	target := FromTable(env, "OnDemandTable")
	insertPlan, err := env.Build(target.OnDemand().Insert(
		SetColumn("theString", Literal("E2")),
		SetColumn("intPrimitive", Literal[int64](2)),
	))
	if err != nil {
		t.Fatal(err)
	}
	inserted, err := engine.ExecuteFireAndForget(ctx, insertPlan)
	if err != nil || len(inserted.Results()) != 0 {
		t.Fatalf("on-demand table insert = %#v, err=%v", inserted.Results(), err)
	}

	minimum := Parameter[int64]("minimum")
	updatePlan, err := env.Build(target.OnDemand().UpdateWhere(
		GreaterOrEqual[int64](TableField[int64]("intPrimitive"), minimum),
		SetColumn("intPrimitive", Add[int64](TableField[int64]("intPrimitive"), Literal[int64](10))),
	))
	if err != nil {
		t.Fatal(err)
	}
	updated, err := engine.ExecuteFireAndForgetWithParameters(ctx, updatePlan, ParameterValues{"minimum": int64(1)})
	if err != nil || len(updated.Results()) != 0 {
		t.Fatalf("on-demand table update = %#v, err=%v", updated.Results(), err)
	}
	row, found, err := table.Get(ctx, "E1")
	if err != nil || !found || row.Get("intPrimitive").Any() != int64(11) {
		t.Fatalf("on-demand table updated row = %#v, found=%v, err=%v", row.Values(), found, err)
	}

	deletePlan, err := env.Build(target.OnDemand().DeleteWhere(
		Equal[string](TableField[string]("theString"), Literal("E0")),
	))
	if err != nil {
		t.Fatal(err)
	}
	if result, err := engine.ExecuteFireAndForget(ctx, deletePlan); err != nil || len(result.Results()) != 0 {
		t.Fatalf("on-demand table delete = %#v, err=%v", result.Results(), err)
	}
	deleteAllPlan, err := env.Build(target.OnDemand().DeleteAll())
	if err != nil {
		t.Fatal(err)
	}
	if result, err := engine.ExecuteFireAndForget(ctx, deleteAllPlan); err != nil || len(result.Results()) != 0 {
		t.Fatalf("on-demand table delete-all = %#v, err=%v", result.Results(), err)
	}
	rows, err := table.Snapshot(ctx)
	if err != nil || len(rows) != 0 {
		t.Fatalf("on-demand table remaining rows = %#v, err=%v", rows, err)
	}
}

func mustSchema(env *Environment, name string) Schema {
	schema, ok := env.Schema(name)
	if !ok {
		panic("schema " + name + " is missing")
	}
	return schema
}

func (e *Environment) mustBuild(query Query) Plan {
	plan, err := e.Build(query)
	if err != nil {
		panic(err)
	}
	return plan
}
