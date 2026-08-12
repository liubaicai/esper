package esper

import (
	"context"
	"testing"
)

type infraTableFilterS0 struct {
	ID int64 `esper:"id"`
}

type infraTableFilterBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int64  `esper:"intPrimitive"`
}

type infraTableWindowEvent struct {
	C0 int64 `esper:"c0"`
}

type infraTableWindowTrigger struct {
	ID int64 `esper:"id"`
}

// TestInfraTableAccessFilterBehaviorParity mirrors Java
// InfraTableAccessCore.InfraFilterBehavior:
//
//	create table varaggFB (total count(*))
//	into table varaggFB select count(*) from SupportBean_S0
//	select * from SupportBean(varaggFB.total = intPrimitive)
//
// The Go form keeps the same dataflow explicit: an aggregate statement
// materializes the table, and a typed scalar subquery reads its current row in
// the ordinary event filter.
func TestInfraTableAccessFilterBehaviorParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableFilterS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableFilterBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "varaggFB", []TableColumn{
		TableColumnOf[int64]("total"),
	}); err != nil {
		t.Fatal(err)
	}

	countPlan, err := env.Build(
		From[infraTableFilterS0](env, "SupportBean_S0").
			Aggregate(Alias("total", CountAll())).
			IntoTable("varaggFB", StatementName("varaggFB-count")),
	)
	if err != nil {
		t.Fatal(err)
	}
	countDeployment, err := NewEngine(env).Deploy(context.Background(), countPlan)
	if err != nil {
		t.Fatal(err)
	}
	engine := countDeployment.engine
	if engine == nil {
		t.Fatal("count deployment has no engine")
	}

	total := SubqueryValue[int64](
		FromTable(env, "varaggFB"),
		Field[any, int64]("total"),
	)
	filterPlan, err := env.Build(
		From[infraTableFilterBean](env, "SupportBean").
			Filter(Equal[int64](total, Field[infraTableFilterBean, int64]("intPrimitive"))).
			Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	filterDeployment, err := engine.Deploy(context.Background(), filterPlan)
	if err != nil {
		t.Fatal(err)
	}

	var matched []int64
	if _, err := filterDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("filter result is not an event: %#v", result)
			}
			matched = append(matched, event.Get("intPrimitive").Any().(int64))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := engine.SendEvent(ctx, infraTableFilterS0{ID: 0}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, infraTableFilterBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, infraTableFilterS0{ID: 0}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, infraTableFilterBean{TheString: "E1", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, infraTableFilterBean{TheString: "E1", IntPrimitive: 3}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, infraTableFilterBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}

	want := []int64{1, 2}
	if len(matched) != len(want) {
		t.Fatalf("table filter matches = %v, want %v", matched, want)
	}
	for index, value := range want {
		if matched[index] != value {
			t.Fatalf("table filter match[%d] = %d, want %d (all=%v)", index, matched[index], value, matched)
		}
	}

	table, ok := engine.Table("varaggFB")
	if !ok {
		t.Fatal("varaggFB table is missing")
	}
	rows, err := table.Snapshot(ctx)
	if err != nil || len(rows) != 1 || rows[0].Get("total").Any() != int64(2) {
		t.Fatalf("varaggFB snapshot = %#v, err=%v", rows, err)
	}
}

// TestInfraTableAccessUngroupedWindowAndSumParity mirrors Java
// InfraTableAccessCoreUnGroupedWindowAndSum. The table has one unkeyed row
// containing both a length-window access value and a running sum; a trigger
// stream reads that row after each insertion.
func TestInfraTableAccessUngroupedWindowAndSumParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableWindowEvent](env, "MyEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[infraTableWindowTrigger](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "windowAndTotal", []TableColumn{
		TableColumnOf[WindowAccessValue[infraTableWindowEvent]]("thewindow"),
		TableColumnOf[int64]("thetotal"),
	}); err != nil {
		t.Fatal(err)
	}

	c0 := Field[infraTableWindowEvent, int64]("c0")
	aggregatePlan, err := env.Build(
		From[infraTableWindowEvent](env, "MyEvent").
			Window(LengthWindow(2)).
			Aggregate(
				Alias("thewindow", WindowAccessBy[infraTableWindowEvent](EventValue[infraTableWindowEvent]())),
				Alias("thetotal", Sum[int64](c0)),
			).
			IntoTable("windowAndTotal", StatementName("window-and-total")),
	)
	if err != nil {
		t.Fatal(err)
	}

	triggerPlan, err := env.Build(
		OnEvent(From[infraTableWindowTrigger](env, "SupportBean_S0")).
			SelectFromTableWhere("windowAndTotal", Literal(true),
				Alias("thewindow", TableField[WindowAccessValue[infraTableWindowEvent]]("thewindow")),
				Alias("thetotal", TableField[int64]("thetotal")),
			).
			Query(StatementName("read-window-and-total")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), triggerPlan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 3)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("table access result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendAndRead := func(value, triggerID int64, wantValues []int64, wantTotal int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), infraTableWindowEvent{C0: value}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), infraTableWindowTrigger{ID: triggerID}); err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Fatal("table access trigger produced no row")
		}
		row := rows[len(rows)-1]
		if row.Get("thetotal").Any() != wantTotal {
			t.Fatalf("table total after %d = %v, want %d", value, row.Get("thetotal"), wantTotal)
		}
		access, ok := row.Get("thewindow").Any().(WindowAccessValue[infraTableWindowEvent])
		if !ok {
			t.Fatalf("table window type = %T", row.Get("thewindow").Any())
		}
		values := access.Values()
		if len(values) != len(wantValues) {
			t.Fatalf("table window after %d = %#v, want %#v", value, values, wantValues)
		}
		for index, want := range wantValues {
			if values[index].C0 != want {
				t.Fatalf("table window after %d index %d = %d, want %d", value, index, values[index].C0, want)
			}
		}
	}

	sendAndRead(10, 0, []int64{10}, 10)
	sendAndRead(20, 1, []int64{10, 20}, 30)
	sendAndRead(30, 2, []int64{20, 30}, 50)
}
