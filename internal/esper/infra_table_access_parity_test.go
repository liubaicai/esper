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
