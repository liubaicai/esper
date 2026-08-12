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

// TestContextTableLiveDuplicatePrimaryKeysRemainPartitionLocal closes the
// remaining physical-storage gap from ContextKeyedSegmentedTable: the same
// primary key is valid once in every Context partition. Reads, updates and
// deletes must resolve the current partition rather than accidentally using
// a global Table primary-key map.
func TestContextTableLiveDuplicatePrimaryKeysRemainPartitionLocal(t *testing.T) {
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
			if _, err := RegisterStruct[contextTableLiveEvent](env, "ContextTableDuplicateEvent"); err != nil {
				t.Fatal(err)
			}
			if _, err := CreateKeyContext(env, "context-table-duplicate", Field[any, string]("group")); err != nil {
				t.Fatal(err)
			}
			tableName := "ContextTableDuplicate_" + variant.name
			if _, err := CreateTable(env, tableName, []TableColumn{
				PrimaryKeyColumn[int64]("id"),
				TableColumnOf[string]("group"),
				TableColumnOf[int64]("value"),
			}); err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			ctx := context.Background()
			source := From[contextTableLiveEvent](env, "ContextTableDuplicateEvent").AsRecord()

			insertPlan, err := env.Build(variant.build(source, tableName).Query(
				StatementName("context-table-duplicate-"+variant.name+"-insert"),
				WithContext("context-table-duplicate"),
			))
			if err != nil {
				t.Fatal(err)
			}
			insertDeployment, err := engine.Deploy(ctx, insertPlan)
			if err != nil {
				t.Fatal(err)
			}
			for _, event := range []contextTableLiveEvent{
				{Group: "A", ID: 1, Value: 10},
				{Group: "B", ID: 1, Value: 20},
			} {
				if err := engine.SendEvent(ctx, event); err != nil {
					t.Fatal(err)
				}
			}
			if err := insertDeployment.Undeploy(ctx); err != nil {
				t.Fatal(err)
			}

			table, ok := engine.Table(tableName)
			if !ok {
				t.Fatal("duplicate-key context table is missing")
			}
			rows, err := table.Snapshot(ctx)
			if err != nil || len(rows) != 2 {
				t.Fatalf("duplicate-key context rows = %#v, err=%v; want two rows", rows, err)
			}

			selectPlan, err := env.Build(OnRecord(source).SelectFromTable(tableName, []Expr{Field[any, int64]("id")},
				Alias("group", TableField[string]("group")),
				Alias("value", TableField[int64]("value")),
			).Query(StatementName("context-table-duplicate-"+variant.name+"-select"), WithContext("context-table-duplicate")))
			if err != nil {
				t.Fatal(err)
			}
			selectDeployment, err := engine.Deploy(ctx, selectPlan)
			if err != nil {
				t.Fatal(err)
			}
			var selected []Row
			if _, err := selectDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
				for _, result := range batch.New {
					row, ok := result.Row()
					if !ok {
						return NewError(ErrorState, "context table select returned a non-row result")
					}
					selected = append(selected, row)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(ctx, contextTableLiveEvent{Group: "A", ID: 1}); err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(ctx, contextTableLiveEvent{Group: "B", ID: 1}); err != nil {
				t.Fatal(err)
			}
			if len(selected) != 2 || selected[0].Get("group").Any() != "A" || selected[0].Get("value").Any() != int64(10) || selected[1].Get("group").Any() != "B" || selected[1].Get("value").Any() != int64(20) {
				t.Fatalf("partition-local select rows = %#v", selected)
			}
			if err := selectDeployment.Undeploy(ctx); err != nil {
				t.Fatal(err)
			}

			updatePlan, err := env.Build(OnRecord(source).UpdateTable(tableName,
				[]Expr{Field[any, int64]("id")},
				SetColumn("value", Field[any, int64]("value")),
			).Query(StatementName("context-table-duplicate-"+variant.name+"-update"), WithContext("context-table-duplicate")))
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
			if err := engine.SendEvent(ctx, contextTableLiveEvent{Group: "B", ID: 1, Value: 21}); err != nil {
				t.Fatal(err)
			}
			if err := updateDeployment.Undeploy(ctx); err != nil {
				t.Fatal(err)
			}
			rows, err = table.Snapshot(ctx)
			if err != nil || len(rows) != 2 || rows[0].Get("value").Any() != int64(11) || rows[1].Get("value").Any() != int64(21) {
				t.Fatalf("partition-local update rows = %#v, err=%v", rows, err)
			}

			keyA := encodeKey([]any{ValuePresent, "A"})
			deletePlan, err := env.Build(FromTable(env, tableName).OnDemand().WithContext("context-table-duplicate").DeleteWhere(
				Equal[int64](TableField[int64]("id"), Literal[int64](1)),
			))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.ExecuteFireAndForgetWithSelector(ctx, deletePlan, SelectContextPartitions(keyA)); err != nil {
				t.Fatal(err)
			}
			rows, err = table.Snapshot(ctx)
			if err != nil || len(rows) != 1 || rows[0].Get("group").Any() != "B" || rows[0].Get("value").Any() != int64(21) {
				t.Fatalf("partition-local FAF delete rows = %#v, err=%v", rows, err)
			}

			deleteDeployment, err := env.Build(OnRecord(source).DeleteFromTable(tableName, Field[any, int64]("id")).Query(
				StatementName("context-table-duplicate-"+variant.name+"-delete"), WithContext("context-table-duplicate")))
			if err != nil {
				t.Fatal(err)
			}
			deleteLiveDeployment, err := engine.Deploy(ctx, deleteDeployment)
			if err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(ctx, contextTableLiveEvent{Group: "B", ID: 1}); err != nil {
				t.Fatal(err)
			}
			if err := deleteLiveDeployment.Undeploy(ctx); err != nil {
				t.Fatal(err)
			}
			rows, err = table.Snapshot(ctx)
			if err != nil || len(rows) != 0 {
				t.Fatalf("partition-local final rows = %#v, err=%v", rows, err)
			}
		})
	}
}

func TestContextTableAggregateIntoTableUsesPartitionLocalReplacement(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextTableLiveEvent](env, "ContextTableAggregateEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "context-table-aggregate", Field[any, string]("group")); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "ContextTableAggregate", []TableColumn{
		PrimaryKeyColumn[string]("bucket"),
		TableColumnOf[int64]("count"),
	}); err != nil {
		t.Fatal(err)
	}

	constantBucket := Literal("all")
	plan, err := env.Build(From[contextTableLiveEvent](env, "ContextTableAggregateEvent").GroupBy(constantBucket).Select(
		Alias("bucket", constantBucket),
		Alias("count", CountAll()),
	).IntoTable("ContextTableAggregate", StatementName("context-table-aggregate"), WithContext("context-table-aggregate")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()

	for _, event := range []contextTableLiveEvent{
		{Group: "A", ID: 1},
		{Group: "A", ID: 2},
		{Group: "B", ID: 3},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := engineTableSnapshot(t, engine, "ContextTableAggregate")
	if err != nil || len(rows) != 2 {
		t.Fatalf("context aggregate table rows = %#v, err=%v; want one row per partition", rows, err)
	}
	if rows[0].Get("bucket").Any() != "all" || rows[0].Get("count").Any() != int64(2) || rows[1].Get("bucket").Any() != "all" || rows[1].Get("count").Any() != int64(1) {
		t.Fatalf("context aggregate table rows = %#v", rows)
	}
}

func TestContextTableLifecycleReleasesPartitionState(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextTableLiveEvent](env, "ContextTableLifecycleEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateInitiatedTerminatedContext(env, "context-table-lifecycle",
		Field[any, string]("group"),
		Equal[int64](Field[any, int64]("value"), Literal[int64](1)),
		Equal[int64](Field[any, int64]("value"), Literal[int64](9)),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "ContextTableLifecycle", []TableColumn{
		PrimaryKeyColumn[int64]("id"),
		TableColumnOf[string]("group"),
		TableColumnOf[int64]("value"),
	}); err != nil {
		t.Fatal(err)
	}
	source := From[contextTableLiveEvent](env, "ContextTableLifecycleEvent")
	plan, err := env.Build(OnRecord(source.AsRecord()).InsertIntoTable("ContextTableLifecycle",
		SetColumn("group", Field[any, string]("group")),
		SetColumn("id", Field[any, int64]("id")),
		SetColumn("value", Field[any, int64]("value")),
	).Query(StatementName("context-table-lifecycle"), WithContext("context-table-lifecycle")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()

	for _, event := range []contextTableLiveEvent{
		{Group: "A", ID: 1, Value: 1},
		{Group: "A", ID: 2, Value: 2},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := engineTableSnapshot(t, engine, "ContextTableLifecycle")
	if err != nil || len(rows) != 2 {
		t.Fatalf("active lifecycle table rows = %#v, err=%v", rows, err)
	}
	if err := engine.SendEvent(context.Background(), contextTableLiveEvent{Group: "A", ID: 3, Value: 9}); err != nil {
		t.Fatal(err)
	}
	rows, err = engineTableSnapshot(t, engine, "ContextTableLifecycle")
	if err != nil || len(rows) != 0 {
		t.Fatalf("terminated lifecycle table rows = %#v, err=%v", rows, err)
	}
	if table, ok := engine.Table("ContextTableLifecycle"); !ok || table.hasScopedState() {
		t.Fatalf("terminated lifecycle table retained scoped state: ok=%v", ok)
	}
}

func engineTableSnapshot(t *testing.T, engine *Engine, name string) ([]TableRow, error) {
	t.Helper()
	table, ok := engine.Table(name)
	if !ok {
		t.Fatalf("table %q is missing", name)
	}
	return table.Snapshot(context.Background())
}
