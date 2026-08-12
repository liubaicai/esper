package esper

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestOnDemandMultirowInsertMatchesEsper(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(map[bool]string{true: "named-window", false: "table"}[namedWindow], func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterMap(env, "MultirowTargetSchema", []FieldSpec{
				FieldDef("k", typeOf[string]()),
				FieldDef("v", typeOf[int64]()),
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterMap(env, "MultirowLastSchema", []FieldSpec{
				FieldDef("theString", typeOf[string]()),
				FieldDef("intPrimitive", typeOf[int64]()),
			}); err != nil {
				t.Fatal(err)
			}
			if namedWindow {
				schema := mustSchema(env, "MultirowTargetSchema")
				if _, err := CreateNamedWindow(env, "MultirowTarget", schema, NamedWindowRetention(KeepAll())); err != nil {
					t.Fatal(err)
				}
			} else if _, err := CreateTable(env, "MultirowTarget", []TableColumn{
				PrimaryKeyColumn[string]("k"),
				TableColumnOf[int64]("v"),
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := CreateNamedWindow(env, "MultirowLast", mustSchema(env, "MultirowLastSchema"), NamedWindowRetention(LastEvent())); err != nil {
				t.Fatal(err)
			}

			engine := NewEngine(env)
			ctx := context.Background()
			var target RecordStream
			if namedWindow {
				target = FromNamedWindow(env, "MultirowTarget")
			} else {
				target = FromTable(env, "MultirowTarget")
			}
			plan, err := env.Build(target.OnDemand().InsertRows(
				InsertValues(Literal("a"), Literal[int64](1)),
				InsertValues(Literal("b"), Literal[int64](2)),
			))
			if err != nil {
				t.Fatal(err)
			}
			result, err := engine.ExecuteFireAndForget(ctx, plan)
			if err != nil {
				t.Fatal(err)
			}
			if namedWindow {
				if got := result.Results(); len(got) != 2 || got[0].Get("k").Any() != "a" || got[1].Get("v").Any() != int64(2) {
					t.Fatalf("multi-row named-window result = %#v", got)
				}
			} else if got := result.Results(); len(got) != 0 {
				t.Fatalf("multi-row table result = %#v, want empty", got)
			}

			last := map[string]any{"theString": "x", "intPrimitive": int64(50)}
			if err := engine.InsertNamedWindow(ctx, "MultirowLast", last); err != nil {
				t.Fatal(err)
			}
			clearPlan, err := env.Build(target.OnDemand().DeleteAll())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.ExecuteFireAndForget(ctx, clearPlan); err != nil {
				t.Fatal(err)
			}
			subquery := SubqueryValue[string](
				FromNamedWindow(env, "MultirowLast"),
				Field[any, string]("theString"),
			)
			subqueryValue := SubqueryValue[int64](
				FromNamedWindow(env, "MultirowLast"),
				Field[any, int64]("intPrimitive"),
			)
			plan, err = env.Build(target.OnDemand().InsertRows(
				InsertValues(Literal("a"), Literal[int64](1)),
				InsertValues(subquery, subqueryValue),
			))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.ExecuteFireAndForget(ctx, plan); err != nil {
				t.Fatal(err)
			}
			if namedWindow {
				window, _ := engine.NamedWindow("MultirowTarget")
				rows, snapshotErr := window.Snapshot(ctx)
				if snapshotErr != nil || len(rows) != 2 || rows[1].Get("k").Any() != "x" || rows[1].Get("v").Any() != int64(50) {
					t.Fatalf("subquery multi-row named-window snapshot = %#v, err=%v", rows, snapshotErr)
				}
			} else {
				table, _ := engine.Table("MultirowTarget")
				rows, snapshotErr := table.Snapshot(ctx)
				if snapshotErr != nil || len(rows) != 2 {
					t.Fatalf("subquery multi-row table snapshot = %#v, err=%v", rows, snapshotErr)
				}
				if rows[1].Get("k").Any() != "x" || rows[1].Get("v").Any() != int64(50) {
					t.Fatalf("subquery multi-row table row = %#v", rows[1].Values())
				}
			}

			if _, err := engine.ExecuteFireAndForget(ctx, clearPlan); err != nil {
				t.Fatal(err)
			}
			rows := make([]OnDemandInsertRow, 1000)
			for index := range rows {
				rows[index] = InsertValues(Literal(fmt.Sprintf("E%d", index)), Literal[int64](int64(index)))
			}
			plan, err = env.Build(target.OnDemand().InsertRows(rows...))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.ExecuteFireAndForget(ctx, plan); err != nil {
				t.Fatal(err)
			}
			if namedWindow {
				window, _ := engine.NamedWindow("MultirowTarget")
				stored, snapshotErr := window.Snapshot(ctx)
				if snapshotErr != nil || len(stored) != 1000 {
					t.Fatalf("1000-row named-window snapshot length = %d, err=%v", len(stored), snapshotErr)
				}
			} else {
				table, _ := engine.Table("MultirowTarget")
				stored, snapshotErr := table.Snapshot(ctx)
				if snapshotErr != nil || len(stored) != 1000 {
					t.Fatalf("1000-row table snapshot length = %d, err=%v", len(stored), snapshotErr)
				}
			}
		})
	}
}

func TestOnDemandMultirowInsertValidationMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "MultirowInvalidSchema", []FieldSpec{
		FieldDef("k", typeOf[string]()),
		FieldDef("v", typeOf[int64]()),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MultirowInvalid", mustSchema(env, "MultirowInvalidSchema")); err != nil {
		t.Fatal(err)
	}
	target := FromNamedWindow(env, "MultirowInvalid")
	_, err := env.Build(target.OnDemand().InsertRows(
		InsertValues(Literal("a"), Literal[int64](1)),
		InsertValues(Literal("b")),
	))
	if err == nil || !strings.Contains(err.Error(), "row 2 of 2") {
		t.Fatalf("row-count validation error = %v", err)
	}
	rows := make([]OnDemandInsertRow, 1001)
	for index := range rows {
		rows[index] = InsertValues(Literal(fmt.Sprintf("E%d", index)), Literal[int64](int64(index)))
	}
	_, err = env.Build(target.OnDemand().InsertRows(rows...))
	if err == nil || !strings.Contains(err.Error(), "maximum of 1000 rows") {
		t.Fatalf("maximum-row validation error = %v", err)
	}
}

func TestOnDemandMultirowInsertRollsBackNamedWindowAndTable(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(map[bool]string{true: "named-window", false: "table"}[namedWindow], func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterMap(env, "MultirowRollbackSchema", []FieldSpec{
				FieldDef("k", typeOf[string]()),
				FieldDef("v", typeOf[int64]()),
			}); err != nil {
				t.Fatal(err)
			}
			if namedWindow {
				if _, err := CreateNamedWindow(env, "MultirowRollback", mustSchema(env, "MultirowRollbackSchema"), NamedWindowUniqueIndex("Idx", "k")); err != nil {
					t.Fatal(err)
				}
			} else if _, err := CreateTable(env, "MultirowRollback", []TableColumn{
				PrimaryKeyColumn[string]("k"),
				TableColumnOf[int64]("v"),
			}); err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			ctx := context.Background()
			var target RecordStream
			if namedWindow {
				target = FromNamedWindow(env, "MultirowRollback")
			} else {
				target = FromTable(env, "MultirowRollback")
			}
			plan, err := env.Build(target.OnDemand().InsertRows(
				InsertValues(Literal("a"), Literal[int64](0)),
				InsertValues(Literal("b"), Literal[int64](10)),
				InsertValues(Literal("b"), Literal[int64](11)),
				InsertValues(Literal("c"), Literal[int64](20)),
			))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.ExecuteFireAndForget(ctx, plan); err == nil {
				t.Fatal("duplicate multi-row insert unexpectedly succeeded")
			}
			if namedWindow {
				window, _ := engine.NamedWindow("MultirowRollback")
				rows, snapshotErr := window.Snapshot(ctx)
				if snapshotErr != nil || len(rows) != 0 {
					t.Fatalf("named-window rollback snapshot = %#v, err=%v", rows, snapshotErr)
				}
			} else {
				table, _ := engine.Table("MultirowRollback")
				rows, snapshotErr := table.Snapshot(ctx)
				if snapshotErr != nil || len(rows) != 0 {
					t.Fatalf("table rollback snapshot = %#v, err=%v", rows, snapshotErr)
				}
			}
		})
	}
}
