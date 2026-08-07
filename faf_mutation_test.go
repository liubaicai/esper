package esper

import (
	"context"
	"strconv"
	"strings"
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

func TestOnDemandContextPartitionMutationMatchesInfraDeleteContextPartitioned(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(map[bool]string{true: "named-window", false: "table"}[namedWindow], func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[infraContextFAFEvent](env, "InfraContextFAFEvent"); err != nil {
				t.Fatal(err)
			}
			if _, err := CreateHashContext(env, "infra-hash", Field[infraContextFAFEvent, string]("theString"), 4); err != nil {
				t.Fatal(err)
			}
			if namedWindow {
				schema, ok := env.Schema("InfraContextFAFEvent")
				if !ok {
					t.Fatal("InfraContextFAFEvent schema is missing")
				}
				if _, err := CreateNamedWindow(env, "CtxInfra", schema, NamedWindowRetention(KeepAll()), NamedWindowContext("infra-hash")); err != nil {
					t.Fatal(err)
				}
			} else if _, err := CreateTable(env, "CtxInfra", []TableColumn{
				PrimaryKeyColumn[string]("theString"),
				PrimaryKeyColumn[int64]("intPrimitive"),
			}); err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			ctx := context.Background()
			for index, event := range []infraContextFAFEvent{
				{TheString: "E0", IntPrimitive: 0},
				{TheString: "E1", IntPrimitive: 1},
				{TheString: "E2", IntPrimitive: 2},
				{TheString: "E3", IntPrimitive: 3},
			} {
				if namedWindow {
					if err := engine.InsertNamedWindow(ctx, "CtxInfra", event); err != nil {
						t.Fatal(err)
					}
				} else {
					table, ok := engine.Table("CtxInfra")
					if !ok {
						t.Fatal("CtxInfra table is missing")
					}
					if _, err := table.Insert(ctx, map[string]any{
						"theString": event.TheString, "intPrimitive": event.IntPrimitive,
					}); err != nil {
						t.Fatalf("insert row %d: %v", index, err)
					}
				}
			}

			definition, ok := env.Context("infra-hash")
			if !ok {
				t.Fatal("infra-hash context is missing")
			}
			window, _ := engine.NamedWindow("CtxInfra")
			var e0, e1 Event
			if namedWindow {
				events, err := window.Snapshot(ctx)
				if err != nil || len(events) != 4 {
					t.Fatalf("context window snapshot = %#v, err=%v", events, err)
				}
				for _, event := range events {
					switch event.Get("theString").Any() {
					case "E0":
						e0 = event
					case "E1":
						e1 = event
					}
				}
			} else {
				table, ok := engine.Table("CtxInfra")
				if !ok {
					t.Fatal("CtxInfra table is missing")
				}
				rows, err := table.Snapshot(ctx)
				if err != nil || len(rows) != 4 {
					t.Fatalf("context table snapshot = %#v, err=%v", rows, err)
				}
				for _, row := range rows {
					target, eventErr := tableRowEvent(table, "CtxInfra", row, engine.Now())
					if eventErr != nil {
						t.Fatal(eventErr)
					}
					switch target.Get("theString").Any() {
					case "E0":
						e0 = target
					case "E1":
						e1 = target
					}
				}
			}
			keyE0, active, err := definition.partition(e0, engine.Now(), nil)
			if err != nil || !active {
				t.Fatalf("E0 context partition = %q, active=%v, err=%v", keyE0, active, err)
			}
			keyE1, active, err := definition.partition(e1, engine.Now(), nil)
			if err != nil || !active {
				t.Fatalf("E1 context partition = %q, active=%v, err=%v", keyE1, active, err)
			}
			if keyE0 == keyE1 {
				t.Fatalf("test data did not produce distinct hash partitions: E0=%q E1=%q", keyE0, keyE1)
			}
			bucketE0, err := strconv.ParseInt(strings.TrimPrefix(keyE0, "hash:"), 10, 64)
			if err != nil {
				t.Fatalf("E0 hash bucket = %q: %v", keyE0, err)
			}
			bucketE1, err := strconv.ParseInt(strings.TrimPrefix(keyE1, "hash:"), 10, 64)
			if err != nil {
				t.Fatalf("E1 hash bucket = %q: %v", keyE1, err)
			}

			var deletePlan Plan
			if namedWindow {
				deletePlan, err = env.Build(FromNamedWindow(env, "CtxInfra").OnDemand().WithContext("infra-hash").DeleteWhere(
					Equal[string](NamedWindowField[string]("theString"), Literal("E0")),
				))
			} else {
				deletePlan, err = env.Build(FromTable(env, "CtxInfra").OnDemand().WithContext("infra-hash").DeleteWhere(
					Equal[string](TableField[string]("theString"), Literal("E0")),
				))
			}
			if err != nil {
				t.Fatal(err)
			}
			wrong, err := engine.ExecuteFireAndForgetWithSelector(ctx, deletePlan, SelectContextPartitionHashes(bucketE1))
			if err != nil {
				t.Fatal(err)
			}
			if namedWindow && len(wrong.Results()) != 0 {
				t.Fatalf("wrong hash partition returned rows = %#v", wrong.Results())
			}
			if namedWindow {
				events, _ := window.Snapshot(ctx)
				if len(events) != 4 {
					t.Fatalf("wrong hash partition changed named window: %#v", events)
				}
			} else {
				table, _ := engine.Table("CtxInfra")
				rows, _ := table.Snapshot(ctx)
				if len(rows) != 4 {
					t.Fatalf("wrong hash partition changed table: %#v", rows)
				}
			}

			selected, err := engine.ExecuteFireAndForgetWithSelector(ctx, deletePlan, SelectContextPartitionHashes(bucketE0))
			if err != nil {
				t.Fatal(err)
			}
			if namedWindow {
				if len(selected.Results()) != 1 || selected.Results()[0].Get("theString").Any() != "E0" {
					t.Fatalf("selected hash partition delete result = %#v", selected.Results())
				}
			} else if len(selected.Results()) != 0 {
				t.Fatalf("table context mutation result = %#v, want empty", selected.Results())
			}
		})
	}
}

func TestOnDemandCategoryPartitionMutationMatchesInfraDeleteContextPartitioned(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(map[bool]string{true: "named-window", false: "table"}[namedWindow], func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[infraContextFAFEvent](env, "InfraCategoryFAFEvent"); err != nil {
				t.Fatal(err)
			}
			if _, err := CreateCategoryContext(env, "infra-category",
				Category("negative", Less[int64](Field[infraContextFAFEvent, int64]("intPrimitive"), Literal[int64](0))),
				Category("positive", Greater[int64](Field[infraContextFAFEvent, int64]("intPrimitive"), Literal[int64](0))),
			); err != nil {
				t.Fatal(err)
			}
			if namedWindow {
				schema, _ := env.Schema("InfraCategoryFAFEvent")
				if _, err := CreateNamedWindow(env, "CtxInfraCat", schema, NamedWindowRetention(KeepAll()), NamedWindowContext("infra-category")); err != nil {
					t.Fatal(err)
				}
			} else if _, err := CreateTable(env, "CtxInfraCat", []TableColumn{
				PrimaryKeyColumn[string]("theString"),
				PrimaryKeyColumn[int64]("intPrimitive"),
			}); err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			ctx := context.Background()
			for _, event := range []infraContextFAFEvent{
				{TheString: "E1", IntPrimitive: -2},
				{TheString: "E2", IntPrimitive: 1},
				{TheString: "E3", IntPrimitive: -3},
				{TheString: "E4", IntPrimitive: 2},
			} {
				if namedWindow {
					if err := engine.InsertNamedWindow(ctx, "CtxInfraCat", event); err != nil {
						t.Fatal(err)
					}
				} else {
					table, _ := engine.Table("CtxInfraCat")
					if _, err := table.Insert(ctx, map[string]any{"theString": event.TheString, "intPrimitive": event.IntPrimitive}); err != nil {
						t.Fatal(err)
					}
				}
			}

			var deletePlan Plan
			var err error
			if namedWindow {
				deletePlan, err = env.Build(FromNamedWindow(env, "CtxInfraCat").OnDemand().WithContext("infra-category").DeleteWhere(
					Equal[string](ContextLabel(), Literal("negative")),
				))
			} else {
				deletePlan, err = env.Build(FromTable(env, "CtxInfraCat").OnDemand().WithContext("infra-category").DeleteWhere(
					Equal[string](ContextLabel(), Literal("negative")),
				))
			}
			if err != nil {
				t.Fatal(err)
			}
			wrong, err := engine.ExecuteFireAndForgetWithSelector(ctx, deletePlan, SelectContextPartitionCategories("positive"))
			if err != nil {
				t.Fatal(err)
			}
			if namedWindow && len(wrong.Results()) != 0 {
				t.Fatalf("wrong category partition returned rows = %#v", wrong.Results())
			}
			selected, err := engine.ExecuteFireAndForgetWithSelector(ctx, deletePlan, SelectContextPartitionCategories("negative"))
			if err != nil {
				t.Fatal(err)
			}
			if namedWindow {
				if len(selected.Results()) != 2 {
					t.Fatalf("negative named-window result = %#v", selected.Results())
				}
			} else if len(selected.Results()) != 0 {
				t.Fatalf("category table mutation result = %#v, want empty", selected.Results())
			}
		})
	}
}

func TestOnDemandContextPartitionUpdateAndDeleteAllRespectSelector(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(map[bool]string{true: "named-window", false: "table"}[namedWindow], func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[infraContextFAFEvent](env, "InfraContextMutationEvent"); err != nil {
				t.Fatal(err)
			}
			if _, err := CreateCategoryContext(env, "context-mutation",
				Category("negative", Less[int64](Field[infraContextFAFEvent, int64]("intPrimitive"), Literal[int64](0))),
				Category("positive", Greater[int64](Field[infraContextFAFEvent, int64]("intPrimitive"), Literal[int64](0))),
			); err != nil {
				t.Fatal(err)
			}
			if namedWindow {
				schema, _ := env.Schema("InfraContextMutationEvent")
				if _, err := CreateNamedWindow(env, "CtxMutation", schema, NamedWindowRetention(KeepAll()), NamedWindowContext("context-mutation")); err != nil {
					t.Fatal(err)
				}
			} else if _, err := CreateTable(env, "CtxMutation", []TableColumn{
				PrimaryKeyColumn[string]("theString"),
				TableColumnOf[int64]("intPrimitive"),
			}); err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			ctx := context.Background()
			for _, event := range []infraContextFAFEvent{
				{TheString: "N1", IntPrimitive: -2},
				{TheString: "N2", IntPrimitive: -3},
				{TheString: "P1", IntPrimitive: 1},
				{TheString: "P2", IntPrimitive: 2},
			} {
				if namedWindow {
					if err := engine.InsertNamedWindow(ctx, "CtxMutation", event); err != nil {
						t.Fatal(err)
					}
				} else {
					table, _ := engine.Table("CtxMutation")
					if _, err := table.Insert(ctx, map[string]any{"theString": event.TheString, "intPrimitive": event.IntPrimitive}); err != nil {
						t.Fatal(err)
					}
				}
			}

			var updatePlan, deleteAllPlan Plan
			var err error
			if namedWindow {
				field := NamedWindowField[int64]("intPrimitive")
				target := FromNamedWindow(env, "CtxMutation").OnDemand().WithContext("context-mutation")
				updatePlan, err = env.Build(target.UpdateWhere(Equal[string](ContextLabel(), Literal("negative")), SetColumn("intPrimitive", Add[int64](field, Literal[int64](10)))))
				if err == nil {
					deleteAllPlan, err = env.Build(target.DeleteAll())
				}
			} else {
				field := TableField[int64]("intPrimitive")
				target := FromTable(env, "CtxMutation").OnDemand().WithContext("context-mutation")
				updatePlan, err = env.Build(target.UpdateWhere(Equal[string](ContextLabel(), Literal("negative")), SetColumn("intPrimitive", Add[int64](field, Literal[int64](10)))))
				if err == nil {
					deleteAllPlan, err = env.Build(target.DeleteAll())
				}
			}
			if err != nil {
				t.Fatal(err)
			}

			wrong, err := engine.ExecuteFireAndForgetWithSelector(ctx, updatePlan, SelectContextPartitionCategories("positive"))
			if err != nil {
				t.Fatal(err)
			}
			if namedWindow && len(wrong.Results()) != 0 {
				t.Fatalf("wrong category update returned rows = %#v", wrong.Results())
			}
			updated, err := engine.ExecuteFireAndForgetWithSelector(ctx, updatePlan, SelectContextPartitionCategories("negative"))
			if err != nil {
				t.Fatal(err)
			}
			if namedWindow && len(updated.Results()) != 2 {
				t.Fatalf("selected category update result = %#v", updated.Results())
			}

			wrong, err = engine.ExecuteFireAndForgetWithSelector(ctx, deleteAllPlan, SelectContextPartitionCategories("negative"))
			if err != nil {
				t.Fatal(err)
			}
			if namedWindow && len(wrong.Results()) != 2 {
				t.Fatalf("selected negative delete-all result = %#v", wrong.Results())
			}
			remaining, err := engine.ExecuteFireAndForgetWithSelector(ctx, deleteAllPlan, SelectContextPartitionCategories("positive"))
			if err != nil {
				t.Fatal(err)
			}
			if namedWindow && len(remaining.Results()) != 2 {
				t.Fatalf("selected positive delete-all result = %#v", remaining.Results())
			}
			if namedWindow {
				window, _ := engine.NamedWindow("CtxMutation")
				events, snapshotErr := window.Snapshot(ctx)
				if snapshotErr != nil || len(events) != 0 {
					t.Fatalf("context named-window remaining events = %#v, err=%v", events, snapshotErr)
				}
			} else {
				table, _ := engine.Table("CtxMutation")
				rows, snapshotErr := table.Snapshot(ctx)
				if snapshotErr != nil || len(rows) != 0 {
					t.Fatalf("context table remaining rows = %#v, err=%v", rows, snapshotErr)
				}
			}
		})
	}
}

type infraContextFAFEvent struct {
	TheString    string `esper:"theString"`
	IntPrimitive int64  `esper:"intPrimitive"`
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
