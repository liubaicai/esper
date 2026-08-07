package esper

import (
	"context"
	"testing"
)

type contextTableLiveEvent struct {
	Group string `esper:"group"`
	ID    int64  `esper:"id"`
	Value int64  `esper:"value"`
}

// TestContextTableLiveInsertOwnershipMatchesEsper covers the ContextKeyed
// table behavior from ContextKeySegmentedInfra with all three Go-native live
// table insertion forms. The update changes the field used by the context
// key; the row must still belong to its original partition when a later FAF
// selector addresses that partition.
func TestContextTableLiveInsertOwnershipMatchesEsper(t *testing.T) {
	variants := []struct {
		name  string
		build func(RecordStream, string) TriggerQuery
	}{
		{
			name: "insert",
			build: func(source RecordStream, table string) TriggerQuery {
				return OnRecord(source).InsertIntoTable(table,
					SetColumn("group", Field[any, string]("group")),
					SetColumn("id", Field[any, int64]("id")),
					SetColumn("value", Field[any, int64]("value")),
				)
			},
		},
		{
			name: "upsert",
			build: func(source RecordStream, table string) TriggerQuery {
				return OnRecord(source).UpsertIntoTable(table,
					SetColumn("group", Field[any, string]("group")),
					SetColumn("id", Field[any, int64]("id")),
					SetColumn("value", Field[any, int64]("value")),
				)
			},
		},
		{
			name: "merge-not-matched",
			build: func(source RecordStream, table string) TriggerQuery {
				return OnRecord(source).MergeInsertIntoTable(table,
					[]Expr{Field[any, int64]("id")},
					SetColumn("group", Field[any, string]("group")),
					SetColumn("id", Field[any, int64]("id")),
					SetColumn("value", Field[any, int64]("value")),
				)
			},
		},
	}

	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[contextTableLiveEvent](env, "ContextTableLiveEvent"); err != nil {
				t.Fatal(err)
			}
			if _, err := CreateKeyContext(env, "context-table-live", Field[any, string]("group")); err != nil {
				t.Fatal(err)
			}
			tableName := "ContextTableLive_" + variant.name
			if _, err := CreateTable(env, tableName, []TableColumn{
				PrimaryKeyColumn[int64]("id"),
				TableColumnOf[string]("group"),
				TableColumnOf[int64]("value"),
			}); err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			ctx := context.Background()
			source := From[contextTableLiveEvent](env, "ContextTableLiveEvent")

			recordSource := source.AsRecord()
			insertPlan, err := env.Build(variant.build(recordSource, tableName).Query(
				StatementName("context-table-live-"+variant.name+"-insert"),
				WithContext("context-table-live"),
			))
			if err != nil {
				t.Fatal(err)
			}
			insertDeployment, err := engine.Deploy(ctx, insertPlan)
			if err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(ctx, contextTableLiveEvent{Group: "A", ID: 1, Value: 10}); err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(ctx, contextTableLiveEvent{Group: "B", ID: 2, Value: 20}); err != nil {
				t.Fatal(err)
			}
			if err := insertDeployment.Undeploy(ctx); err != nil {
				t.Fatal(err)
			}

			if got := liveContextTableOwnershipCount(engine, tableName, "context-table-live"); got != 2 {
				t.Fatalf("live %s ownership count = %d, want 2", variant.name, got)
			}

			// Re-key the context field through a live update. The stable row
			// identity must keep the original A ownership instead of moving the
			// row to a newly computed "moved" partition.
			updatePlan, err := env.Build(OnRecord(recordSource).UpdateTable(tableName,
				[]Expr{Field[any, int64]("id")},
				SetColumn("group", Literal("moved")),
			).Query(
				StatementName("context-table-live-"+variant.name+"-update"),
				WithContext("context-table-live"),
			))
			if err != nil {
				t.Fatal(err)
			}
			updateDeployment, err := engine.Deploy(ctx, updatePlan)
			if err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(ctx, contextTableLiveEvent{Group: "A", ID: 1, Value: 11}); err != nil {
				t.Fatal(err)
			}
			if err := updateDeployment.Undeploy(ctx); err != nil {
				t.Fatal(err)
			}

			keyA := encodeKey([]any{ValuePresent, "A"})
			deleteAPlan, err := env.Build(FromTable(env, tableName).OnDemand().WithContext("context-table-live").DeleteWhere(
				Equal[int64](TableField[int64]("id"), Literal[int64](1)),
			))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.ExecuteFireAndForgetWithSelector(ctx, deleteAPlan, SelectContextPartitions(keyA)); err != nil {
				t.Fatal(err)
			}
			if got := liveContextTableOwnershipCount(engine, tableName, "context-table-live"); got != 1 {
				t.Fatalf("post-FAF A delete ownership count = %d, want 1", got)
			}
			table, ok := engine.Table(tableName)
			if !ok {
				t.Fatal("live context table is missing")
			}
			rows, err := table.Snapshot(ctx)
			if err != nil || len(rows) != 1 || rows[0].Get("id").Any() != int64(2) {
				t.Fatalf("selected A delete changed B row = %#v, err=%v", rows, err)
			}

			// A live delete must also remove the row-identity ownership entry;
			// otherwise later inserts leak stale selector state.
			deletePlan, err := env.Build(OnRecord(recordSource).DeleteFromTable(tableName, Field[any, int64]("id")).Query(
				StatementName("context-table-live-"+variant.name+"-delete"),
				WithContext("context-table-live"),
			))
			if err != nil {
				t.Fatal(err)
			}
			deleteDeployment, err := engine.Deploy(ctx, deletePlan)
			if err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(ctx, contextTableLiveEvent{Group: "B", ID: 2, Value: 20}); err != nil {
				t.Fatal(err)
			}
			if err := deleteDeployment.Undeploy(ctx); err != nil {
				t.Fatal(err)
			}
			if got := liveContextTableOwnershipCount(engine, tableName, "context-table-live"); got != 0 {
				t.Fatalf("live delete ownership count = %d, want 0", got)
			}
		})
	}
}

func liveContextTableOwnershipCount(engine *Engine, tableName, contextName string) int {
	if engine == nil {
		return 0
	}
	byTable := engine.contextTableOwnership[catalogKey("", tableName)]
	return len(byTable[contextName])
}
