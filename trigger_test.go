package esper

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"
)

type triggerTestReset struct {
	ID string `esper:"id"`
}

func TestTableTriggerBuilderAndLifecycle(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.RegisterTable("positions", []TableColumn{
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[float64]("price"),
	}); err != nil {
		t.Fatal(err)
	}
	source := From[runtimeTestTrade](env, "Trade")
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")

	insertPlan, err := env.Build(OnEvent(source).InsertIntoTable("positions",
		SetColumn("symbol", symbol),
		SetColumn("price", price),
	).Query(StatementName("on-insert")))
	if err != nil {
		t.Fatal(err)
	}
	upsertPlan, err := env.Build(OnEvent(source).MergeIntoTable("positions",
		SetColumn("symbol", symbol),
		SetColumn("price", price),
	).Query(StatementName("on-merge")))
	if err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(OnEvent(source).UpdateTable("positions", []Expr{symbol}, SetColumn("price", price)).Query(StatementName("on-update")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(source).DeleteFromTable("positions", symbol).Query(StatementName("on-delete")))
	if err != nil {
		t.Fatal(err)
	}
	deleteAllPlan, err := env.Build(OnEvent(source).DeleteAllFromTable("positions").Query(StatementName("on-delete-all")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deploy := func(plan Plan) *Deployment {
		deployment, deployErr := engine.Deploy(context.Background(), plan)
		if deployErr != nil {
			t.Fatal(deployErr)
		}
		return deployment
	}

	insertDeployment := deploy(insertPlan)
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	table, ok := engine.Table("positions")
	if !ok {
		t.Fatal("positions table is missing")
	}
	row, found, err := table.Get(context.Background(), "A")
	if err != nil || !found || row.Get("price").Any() != float64(1) {
		t.Fatalf("inserted table row = %#v, found=%v, err=%v", row.Values(), found, err)
	}
	if err := engine.Undeploy(context.Background(), insertDeployment.ID()); err != nil {
		t.Fatal(err)
	}
	upsertDeployment := deploy(upsertPlan)
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B", Price: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B", Price: 3}); err != nil {
		t.Fatal(err)
	}
	row, found, err = table.Get(context.Background(), "B")
	if err != nil || !found || row.Get("price").Any() != float64(3) {
		t.Fatalf("upserted table row = %#v, found=%v, err=%v", row.Values(), found, err)
	}
	if err := engine.Undeploy(context.Background(), upsertDeployment.ID()); err != nil {
		t.Fatal(err)
	}
	updateDeployment := deploy(updatePlan)
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 4}); err != nil {
		t.Fatal(err)
	}
	row, found, err = table.Get(context.Background(), "A")
	if err != nil || !found || row.Get("price").Any() != float64(4) {
		t.Fatalf("updated table row = %#v, found=%v, err=%v", row.Values(), found, err)
	}
	if err := engine.Undeploy(context.Background(), updateDeployment.ID()); err != nil {
		t.Fatal(err)
	}
	deleteDeployment := deploy(deletePlan)
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 0}); err != nil {
		t.Fatal(err)
	}
	if _, found, err := table.Get(context.Background(), "A"); err != nil || found {
		t.Fatalf("deleted table row found=%v, err=%v", found, err)
	}
	if err := engine.Undeploy(context.Background(), deleteDeployment.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := table.Insert(context.Background(), map[string]any{"symbol": "A", "price": 5.0}); err != nil {
		t.Fatal(err)
	}
	if _, err := table.Insert(context.Background(), map[string]any{"symbol": "C", "price": 6.0}); err != nil {
		t.Fatal(err)
	}
	deleteAllDeployment := deploy(deleteAllPlan)
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "ignored"}); err != nil {
		t.Fatal(err)
	}
	if rows, err := table.Snapshot(context.Background()); err != nil {
		t.Fatal(err)
	} else if len(rows) != 0 {
		t.Fatalf("delete-all table rows = %#v", rows)
	}
	if err := engine.Undeploy(context.Background(), deleteAllDeployment.ID()); err != nil {
		t.Fatal(err)
	}
}

func TestTableTriggerEmitsMutationNewAndOldStreams(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.RegisterTable("positions", []TableColumn{
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[float64]("price"),
	}); err != nil {
		t.Fatal(err)
	}
	source := From[runtimeTestTrade](env, "Trade")
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	insertPlan, err := env.Build(OnEvent(source).InsertIntoTable("positions",
		SetColumn("symbol", symbol),
		SetColumn("price", price),
	).Query(StatementName("mutation-insert")))
	if err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(OnEvent(source).UpdateTable("positions", []Expr{symbol}, SetColumn("price", price)).Query(StatementName("mutation-update")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(source).DeleteFromTable("positions", symbol).Query(StatementName("mutation-delete")))
	if err != nil {
		t.Fatal(err)
	}
	deleteAllPlan, err := env.Build(OnEvent(source).DeleteAllFromTable("positions").Query(StatementName("mutation-delete-all")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deploy := func(plan Plan) *Deployment {
		deployment, deployErr := engine.Deploy(context.Background(), plan)
		if deployErr != nil {
			t.Fatal(deployErr)
		}
		return deployment
	}
	collect := func(deployment *Deployment) *[]ResultBatch {
		batches := new([]ResultBatch)
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			*batches = append(*batches, batch)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return batches
	}
	assertPrice := func(result Result, expected float64) {
		t.Helper()
		event, ok := result.Event()
		if !ok || event.Get("price").Any() != expected {
			t.Fatalf("table mutation result = %#v, expected price %v", result.Underlying(), expected)
		}
	}

	insertDeployment := deploy(insertPlan)
	insertBatches := collect(insertDeployment)
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if len(*insertBatches) != 1 || len((*insertBatches)[0].New) != 1 || len((*insertBatches)[0].Old) != 0 {
		t.Fatalf("insert mutation batch = %#v", *insertBatches)
	}
	assertPrice((*insertBatches)[0].New[0], 1)
	if err := engine.Undeploy(context.Background(), insertDeployment.ID()); err != nil {
		t.Fatal(err)
	}

	updateDeployment := deploy(updatePlan)
	updateBatches := collect(updateDeployment)
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 2}); err != nil {
		t.Fatal(err)
	}
	if len(*updateBatches) != 1 || len((*updateBatches)[0].New) != 1 || len((*updateBatches)[0].Old) != 1 {
		t.Fatalf("update mutation batch = %#v", *updateBatches)
	}
	assertPrice((*updateBatches)[0].Old[0], 1)
	assertPrice((*updateBatches)[0].New[0], 2)
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "missing", Price: 9}); err != nil {
		t.Fatal(err)
	}
	if len(*updateBatches) != 1 {
		t.Fatalf("update trigger emitted for a missing row: %#v", *updateBatches)
	}
	if err := engine.Undeploy(context.Background(), updateDeployment.ID()); err != nil {
		t.Fatal(err)
	}

	deleteDeployment := deploy(deletePlan)
	deleteBatches := collect(deleteDeployment)
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if len(*deleteBatches) != 1 || len((*deleteBatches)[0].New) != 0 || len((*deleteBatches)[0].Old) != 1 {
		t.Fatalf("delete mutation batch = %#v", *deleteBatches)
	}
	assertPrice((*deleteBatches)[0].Old[0], 2)
	if err := engine.Undeploy(context.Background(), deleteDeployment.ID()); err != nil {
		t.Fatal(err)
	}

	table, ok := engine.Table("positions")
	if !ok {
		t.Fatal("positions table is missing")
	}
	if _, err := table.Insert(context.Background(), map[string]any{"symbol": "A", "price": 3.0}); err != nil {
		t.Fatal(err)
	}
	if _, err := table.Insert(context.Background(), map[string]any{"symbol": "B", "price": 4.0}); err != nil {
		t.Fatal(err)
	}
	deleteAllDeployment := deploy(deleteAllPlan)
	deleteAllBatches := collect(deleteAllDeployment)
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "ignored"}); err != nil {
		t.Fatal(err)
	}
	if len(*deleteAllBatches) != 1 || len((*deleteAllBatches)[0].New) != 0 || len((*deleteAllBatches)[0].Old) != 2 {
		t.Fatalf("delete-all mutation batch = %#v", *deleteAllBatches)
	}
	assertPrice((*deleteAllBatches)[0].Old[0], 3)
	assertPrice((*deleteAllBatches)[0].Old[1], 4)
}

func TestTablePredicateTriggerMutatesEveryMatchingRow(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.RegisterTable("positions", []TableColumn{
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[float64]("price"),
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	table, ok := engine.Table("positions")
	if !ok {
		t.Fatal("positions table is missing")
	}
	for _, values := range []map[string]any{
		{"symbol": "A", "price": 1.0},
		{"symbol": "B", "price": 4.0},
		{"symbol": "C", "price": 8.0},
	} {
		if _, err := table.Insert(context.Background(), values); err != nil {
			t.Fatal(err)
		}
	}
	source := From[runtimeTestTrade](env, "Trade")
	triggerPrice := Field[runtimeTestTrade, float64]("price")
	deletePlan, err := env.Build(OnEvent(source).DeleteFromTableWhere("positions",
		Less[float64](TableField[float64]("price"), triggerPrice),
	).Query(StatementName("delete-table-where")))
	if err != nil {
		t.Fatal(err)
	}
	deleteDeployment, err := engine.Deploy(context.Background(), deletePlan)
	if err != nil {
		t.Fatal(err)
	}
	var deleteBatches []ResultBatch
	if _, err := deleteDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		deleteBatches = append(deleteBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 5}); err != nil {
		t.Fatal(err)
	}
	if len(deleteBatches) != 1 || len(deleteBatches[0].Old) != 2 || len(deleteBatches[0].New) != 0 {
		t.Fatalf("predicate delete batch = %#v", deleteBatches)
	}
	if _, found, err := table.Get(context.Background(), "A"); err != nil || found {
		t.Fatalf("deleted row A found=%v, err=%v", found, err)
	}
	if _, found, err := table.Get(context.Background(), "B"); err != nil || found {
		t.Fatalf("deleted row B found=%v, err=%v", found, err)
	}
	if _, found, err := table.Get(context.Background(), "C"); err != nil || !found {
		t.Fatalf("non-matching row C found=%v, err=%v", found, err)
	}
	if err := engine.Undeploy(context.Background(), deleteDeployment.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := table.Insert(context.Background(), map[string]any{"symbol": "A", "price": 2.0}); err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(OnEvent(source).UpdateTableWhere("positions",
		GreaterOrEqual[float64](TableField[float64]("price"), Literal[float64](4)),
		SetColumn("price", Add[float64](TableField[float64]("price"), triggerPrice)),
	).Query(StatementName("update-table-where")))
	if err != nil {
		t.Fatal(err)
	}
	updateDeployment, err := engine.Deploy(context.Background(), updatePlan)
	if err != nil {
		t.Fatal(err)
	}
	var updateBatches []ResultBatch
	if _, err := updateDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		updateBatches = append(updateBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Price: 2}); err != nil {
		t.Fatal(err)
	}
	if len(updateBatches) != 1 || len(updateBatches[0].Old) != 1 || len(updateBatches[0].New) != 1 {
		t.Fatalf("predicate update batch = %#v", updateBatches)
	}
	row, found, err := table.Get(context.Background(), "C")
	if err != nil || !found || row.Get("price").Any() != float64(10) {
		t.Fatalf("predicate updated row C = %#v, found=%v, err=%v", row.Values(), found, err)
	}
}

func TestNamedWindowOnTriggerInsertUpdateSelectDelete(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "trigger-trades", schema); err != nil {
		t.Fatal(err)
	}
	source := From[runtimeTestTrade](env, "Trade")
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	insertPlan, err := env.Build(OnEvent(source).InsertIntoNamedWindow("trigger-trades",
		SetColumn("symbol", symbol),
		SetColumn("price", price),
	).Query(StatementName("named-insert")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	consumerPlan, err := env.Build(FromNamedWindow(env, "trigger-trades").Query(StatementName("named-consumer"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var consumerBatches []ResultBatch
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		consumerBatches = append(consumerBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	insertDeployment, err := engine.Deploy(context.Background(), insertPlan)
	if err != nil {
		t.Fatal(err)
	}
	var insertBatches []ResultBatch
	if _, err := insertDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		insertBatches = append(insertBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, trade := range []runtimeTestTrade{{Symbol: "A", Price: 1}, {Symbol: "B", Price: 2}} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(insertBatches) != 2 || len(insertBatches[0].New) != 1 || len(insertBatches[0].Old) != 0 {
		t.Fatalf("named-window insert batches = %#v", insertBatches)
	}
	if len(consumerBatches) != 2 || len(consumerBatches[0].New) != 1 || len(consumerBatches[0].Old) != 0 {
		t.Fatalf("named-window consumer insert batches = %#v", consumerBatches)
	}
	if err := engine.Undeploy(context.Background(), insertDeployment.ID()); err != nil {
		t.Fatal(err)
	}

	matchSymbol := Equal[string](NamedWindowField[string]("symbol"), symbol)
	updatePlan, err := env.Build(OnEvent(source).UpdateNamedWindow("trigger-trades", matchSymbol,
		SetColumn("price", price),
	).Query(StatementName("named-update")))
	if err != nil {
		t.Fatal(err)
	}
	updateDeployment, err := engine.Deploy(context.Background(), updatePlan)
	if err != nil {
		t.Fatal(err)
	}
	var updateBatches []ResultBatch
	if _, err := updateDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		updateBatches = append(updateBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 5}); err != nil {
		t.Fatal(err)
	}
	if len(updateBatches) != 1 || len(updateBatches[0].Old) != 1 || len(updateBatches[0].New) != 1 {
		t.Fatalf("named-window update batch = %#v", updateBatches)
	}
	oldEvent, oldOK := updateBatches[0].Old[0].Event()
	newEvent, newOK := updateBatches[0].New[0].Event()
	if !oldOK || !newOK || oldEvent.Get("price").Any() != float64(1) || newEvent.Get("price").Any() != float64(5) {
		t.Fatalf("named-window update old/new = %#v / %#v", updateBatches[0].Old, updateBatches[0].New)
	}
	if len(consumerBatches) != 3 || len(consumerBatches[2].Old) != 1 || len(consumerBatches[2].New) != 1 {
		t.Fatalf("named-window consumer update batches = %#v", consumerBatches)
	}
	if err := engine.Undeploy(context.Background(), updateDeployment.ID()); err != nil {
		t.Fatal(err)
	}

	selectPlan, err := env.Build(OnEvent(source).SelectFromNamedWindow("trigger-trades",
		LessOrEqual[float64](NamedWindowField[float64]("price"), Literal[float64](2)),
		Alias("stored-symbol", NamedWindowField[string]("symbol")),
		Alias("trigger-symbol", symbol),
	).Query(StatementName("named-select")))
	if err != nil {
		t.Fatal(err)
	}
	selectDeployment, err := engine.Deploy(context.Background(), selectPlan)
	if err != nil {
		t.Fatal(err)
	}
	var selected []Row
	if _, err := selectDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("named-window select result is not a row")
			}
			selected = append(selected, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "X"}); err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0].Get("stored-symbol").Any() != "B" || selected[0].Get("trigger-symbol").Any() != "X" {
		t.Fatalf("named-window select rows = %#v", selected)
	}
	if err := engine.Undeploy(context.Background(), selectDeployment.ID()); err != nil {
		t.Fatal(err)
	}

	deletePlan, err := env.Build(OnEvent(source).DeleteFromNamedWindow("trigger-trades", matchSymbol).Query(StatementName("named-delete")))
	if err != nil {
		t.Fatal(err)
	}
	deleteDeployment, err := engine.Deploy(context.Background(), deletePlan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if len(consumerBatches) != 4 || len(consumerBatches[3].Old) != 1 || len(consumerBatches[3].New) != 0 {
		t.Fatalf("named-window consumer delete batches = %#v", consumerBatches)
	}
	if err := engine.Undeploy(context.Background(), deleteDeployment.ID()); err != nil {
		t.Fatal(err)
	}
	deleteAllPlan, err := env.Build(OnEvent(source).DeleteAllFromNamedWindow("trigger-trades").Query(StatementName("named-delete-all")))
	if err != nil {
		t.Fatal(err)
	}
	deleteAllDeployment, err := engine.Deploy(context.Background(), deleteAllPlan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "ignored"}); err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow("trigger-trades")
	if !ok {
		t.Fatal("trigger-trades named window is missing")
	}
	if events, err := window.Snapshot(context.Background()); err != nil || len(events) != 0 {
		t.Fatalf("named-window delete-all snapshot = %#v, err=%v", events, err)
	}
	if len(consumerBatches) != 5 || len(consumerBatches[4].Old) != 1 || len(consumerBatches[4].New) != 0 {
		t.Fatalf("named-window consumer delete-all batches = %#v", consumerBatches)
	}
	if err := engine.Undeploy(context.Background(), deleteAllDeployment.ID()); err != nil {
		t.Fatal(err)
	}
	if err := engine.Undeploy(context.Background(), consumerDeployment.ID()); err != nil {
		t.Fatal(err)
	}
}

func TestTableTriggerValidation(t *testing.T) {
	env, _ := newRuntimeTest(t)
	if _, err := env.RegisterTable("positions", []TableColumn{PrimaryKeyColumn[string]("symbol")}); err != nil {
		t.Fatal(err)
	}
	source := From[runtimeTestTrade](env, "Trade")
	if _, err := env.Build(OnEvent(source).InsertIntoTable("missing", SetColumn("symbol", Field[runtimeTestTrade, string]("symbol"))).Query()); err == nil {
		t.Fatal("unknown trigger table was accepted")
	}
	if _, err := env.Build(OnEvent(source).InsertIntoTable("positions").Query()); err == nil {
		t.Fatal("empty table trigger assignment was accepted")
	}
	if _, err := env.Build(OnEvent(source).DeleteFromTable("positions").Query()); err == nil {
		t.Fatal("delete trigger without primary-key expressions was accepted")
	}
	if _, err := env.Build(OnEvent(source).DeleteAllFromTable("positions").Query()); err != nil {
		t.Fatalf("delete-all trigger was rejected: %v", err)
	}
	if _, err := env.Build(OnEvent(source).DeleteFromTableWhere("positions", Field[runtimeTestTrade, string]("symbol")).Query()); err == nil {
		t.Fatal("non-boolean table predicate was accepted")
	}
	if _, err := env.Build(OnEvent(source).DeleteFromTableWhere("positions", Equal[string](TableField[string]("missing"), Field[runtimeTestTrade, string]("symbol"))).Query()); err == nil {
		t.Fatal("table predicate with unknown target field was accepted")
	}
	if _, err := env.Build(OnEvent(source).MergeIntoTableWhen("positions", nil,
		WhenMatched(Literal(true), SetColumn("symbol", Field[runtimeTestTrade, string]("symbol"))),
		WhenNotMatched(Literal(true), SetColumn("symbol", Field[runtimeTestTrade, string]("symbol"))),
	).Query()); err == nil {
		t.Fatal("table merge without primary-key expressions was accepted")
	}
	if _, err := env.Build(OnEvent(source).MergeIntoTableWhen("positions", []Expr{Field[runtimeTestTrade, string]("symbol")},
		WhenMatched(Literal(true), SetColumn("symbol", Field[runtimeTestTrade, string]("symbol"))),
	).Query()); err != nil {
		t.Fatal("table merge with a matched clause was rejected")
	}
	if _, err := env.Build(OnEvent(source).MergeIntoTableWhen("positions", []Expr{Field[runtimeTestTrade, string]("symbol")},
		WhenNotMatched(Literal(true), SetColumn("symbol", Field[runtimeTestTrade, string]("symbol"))),
	).Query()); err != nil {
		t.Fatalf("table merge with a not-matched clause was rejected: %v", err)
	}
	if _, err := env.Build(OnEvent(source).MergeIntoTableWhen("positions", []Expr{Field[runtimeTestTrade, string]("symbol")}).Query()); err == nil {
		t.Fatal("table merge without any clause was accepted")
	}
	if _, err := env.Build(OnEvent(source).MergeIntoTableWhen("positions", []Expr{Field[runtimeTestTrade, string]("symbol")},
		TableMergeClause{Delete: true, Condition: Literal(true)},
		WhenNotMatched(Literal(true), SetColumn("symbol", Field[runtimeTestTrade, string]("symbol"))),
	).Query()); err == nil {
		t.Fatal("not-matched table merge delete clause was accepted")
	}
}

func TestNamedWindowConditionalMergeBranchesAndDispatch(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := env.RegisterNamedWindow("merge-trades", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := env.RegisterNamedWindow("append-trades", schema); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	source := From[runtimeTestTrade](env, "Trade")
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	match := Equal[string](NamedWindowField[string]("symbol"), symbol)
	plan, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen("merge-trades", match,
		WhenMatched(Greater[float64](price, Literal[float64](10)), SetColumn("price", price)),
		WhenMatchedDelete(LessOrEqual[float64](price, Literal[float64](0))),
		WhenNotMatched(Literal(true), SetColumn("symbol", symbol), SetColumn("price", price)),
	).Query(StatementName("named-merge")))
	if err != nil {
		t.Fatal(err)
	}
	otherPlan, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen("merge-trades", match,
		WhenMatched(Greater[float64](price, Literal[float64](20)), SetColumn("price", price)),
		WhenNotMatched(Literal(true), SetColumn("symbol", symbol), SetColumn("price", price)),
	).Query(StatementName("named-merge")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() == otherPlan.Hash() {
		t.Fatal("named-window merge rules with different matched conditions share a plan hash")
	}
	appendPlan, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen("append-trades", Literal(false),
		WhenNotMatched(Literal(true)),
	).Query(StatementName("named-merge-append")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromNamedWindow(env, "merge-trades").Query(StatementName("named-merge-consumer"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	mergeDeployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	appendDeployment, err := engine.Deploy(context.Background(), appendPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = mergeDeployment.Undeploy(context.Background())
		_ = appendDeployment.Undeploy(context.Background())
		_ = consumerDeployment.Undeploy(context.Background())
	}()
	var mergeBatches []ResultBatch
	if _, err := mergeDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		mergeBatches = append(mergeBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var consumerBatches []ResultBatch
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		consumerBatches = append(consumerBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(symbol string, price float64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol, Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	send("A", 1)
	if len(mergeBatches) != 1 || len(mergeBatches[0].New) != 1 || len(mergeBatches[0].Old) != 0 {
		t.Fatalf("named merge insert batch = %#v", mergeBatches)
	}
	send("A", 5)
	if len(mergeBatches) != 1 {
		t.Fatalf("named merge no-op emitted a batch = %#v", mergeBatches)
	}
	send("A", 11)
	if len(mergeBatches) != 2 || len(mergeBatches[1].Old) != 1 || len(mergeBatches[1].New) != 1 {
		t.Fatalf("named merge update batch = %#v", mergeBatches)
	}
	oldEvent, oldOK := mergeBatches[1].Old[0].Event()
	newEvent, newOK := mergeBatches[1].New[0].Event()
	if !oldOK || !newOK || oldEvent.Get("price").Any() != float64(1) || newEvent.Get("price").Any() != float64(11) {
		t.Fatalf("named merge update old/new = %#v / %#v", mergeBatches[1].Old, mergeBatches[1].New)
	}
	send("B", 2)
	send("A", 0)
	if len(mergeBatches) != 4 || len(mergeBatches[3].Old) != 1 || len(mergeBatches[3].New) != 0 {
		t.Fatalf("named merge delete batch = %#v", mergeBatches)
	}
	window, ok := engine.NamedWindow("merge-trades")
	if !ok {
		t.Fatal("merge-trades named window is missing")
	}
	events, err := window.Snapshot(context.Background())
	if err != nil || len(events) != 1 || events[0].Get("symbol").Any() != "B" {
		t.Fatalf("named merge final snapshot = %#v, err=%v", events, err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "C", Price: 3}); err != nil {
		t.Fatal(err)
	}
	appendWindow, ok := engine.NamedWindow("append-trades")
	if !ok {
		t.Fatal("append-trades named window is missing")
	}
	appendEvents, err := appendWindow.Snapshot(context.Background())
	if err != nil || len(appendEvents) != 6 || appendEvents[len(appendEvents)-1].Get("symbol").Any() != "C" {
		t.Fatalf("named merge select-star insert snapshot = %#v, err=%v", appendEvents, err)
	}
	if len(consumerBatches) != 5 || len(consumerBatches[1].Old) != 1 || len(consumerBatches[1].New) != 1 || len(consumerBatches[3].Old) != 1 || len(consumerBatches[4].New) != 1 {
		t.Fatalf("named merge consumer batches = %#v", consumerBatches)
	}
}

func TestNamedWindowMergeWithoutWhereUsesMatchedBranch(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := env.RegisterNamedWindow("no-where-merge", schema); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	source := From[runtimeTestTrade](env, "Trade")
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen("no-where-merge", nil,
		WhenMatched(Literal(true), SetColumn("price", price)),
		WhenNotMatched(Literal(true)),
	).Query(StatementName("named-merge-no-where")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	for _, trade := range []runtimeTestTrade{{Symbol: "A", Price: 1}, {Symbol: "A", Price: 9}} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	window, ok := engine.NamedWindow("no-where-merge")
	if !ok {
		t.Fatal("no-where-merge named window is missing")
	}
	events, err := window.Snapshot(context.Background())
	if err != nil || len(events) != 1 || events[0].Get("price").Any() != float64(9) {
		t.Fatalf("named merge without where snapshot = %#v, err=%v", events, err)
	}
}

func TestNamedWindowTriggerValidation(t *testing.T) {
	env, _ := newRuntimeTest(t)
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := env.RegisterNamedWindow("validation-window", schema); err != nil {
		t.Fatal(err)
	}
	source := From[runtimeTestTrade](env, "Trade")
	if _, err := env.Build(OnEvent(source).InsertIntoNamedWindow("validation-window").Query()); err == nil {
		t.Fatal("empty named-window insert was accepted")
	}
	if _, err := env.Build(OnEvent(source).UpdateNamedWindow("validation-window", nil, SetColumn("price", Field[runtimeTestTrade, float64]("price"))).Query()); err == nil {
		t.Fatal("named-window update without predicate was accepted")
	}
	if _, err := env.Build(OnEvent(source).DeleteFromNamedWindow("validation-window", Equal[string](NamedWindowField[string]("missing"), Field[runtimeTestTrade, string]("symbol"))).Query()); err == nil {
		t.Fatal("named-window predicate with unknown target field was accepted")
	}
	if _, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen("validation-window", Equal[string](NamedWindowField[string]("missing"), Field[runtimeTestTrade, string]("symbol")), WhenNotMatched(Literal(true))).Query()); err == nil {
		t.Fatal("named-window merge with unknown target field was accepted")
	}
	if _, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen("validation-window", nil, WhenNotMatched(Equal[string](NamedWindowField[string]("symbol"), Field[runtimeTestTrade, string]("symbol")))).Query()); err == nil {
		t.Fatal("not-matched named-window merge condition referenced a target field")
	}
	if _, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen("validation-window", nil).Query()); err == nil {
		t.Fatal("named-window merge without a clause was accepted")
	}
	if _, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen("validation-window", nil, TableMergeClause{Delete: true, Condition: Literal(true)}).Query()); err == nil {
		t.Fatal("not-matched named-window merge delete was accepted")
	}
	if _, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen("validation-window", nil, WhenMatched(Literal(true))).Query()); err == nil {
		t.Fatal("matched named-window merge without assignments was accepted")
	}
}

func TestConditionalTableMergeBranches(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.RegisterTable("positions", []TableColumn{
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[float64]("price"),
	}); err != nil {
		t.Fatal(err)
	}
	source := From[runtimeTestTrade](env, "Trade")
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(OnEvent(source).MergeIntoTableWhen("positions", []Expr{symbol},
		WhenNotMatched(Literal(true),
			SetColumn("symbol", symbol),
			SetColumn("price", price),
		),
		WhenMatched(Greater[float64](price, Literal[float64](5.0)),
			SetColumn("price", price),
		),
		WhenMatchedDelete(LessOrEqual[float64](price, Literal[float64](0.0))),
	).Query(StatementName("conditional-merge")))
	if err != nil {
		t.Fatal(err)
	}
	otherPlan, err := env.Build(OnEvent(source).MergeIntoTableWhen("positions", []Expr{symbol},
		WhenNotMatched(Literal(true),
			SetColumn("symbol", symbol),
			SetColumn("price", price),
		),
		WhenMatched(Greater[float64](price, Literal[float64](10.0)),
			SetColumn("price", price),
		),
		WhenMatchedDelete(LessOrEqual[float64](price, Literal[float64](0.0))),
	).Query(StatementName("conditional-merge-other")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() == otherPlan.Hash() {
		t.Fatal("conditional table merge rules with different conditions share a plan hash")
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	send := func(price float64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	send(2)
	table, ok := engine.Table("positions")
	if !ok {
		t.Fatal("positions table is missing")
	}
	row, found, err := table.Get(context.Background(), "A")
	if err != nil || !found || row.Get("price").Any() != float64(2) {
		t.Fatalf("not-matched merge row = %#v, found=%v, err=%v", row.Values(), found, err)
	}
	send(8)
	row, found, err = table.Get(context.Background(), "A")
	if err != nil || !found || row.Get("price").Any() != float64(8) {
		t.Fatalf("matched merge row = %#v, found=%v, err=%v", row.Values(), found, err)
	}
	send(0)
	if _, found, err := table.Get(context.Background(), "A"); err != nil || found {
		t.Fatalf("matched delete merge row found=%v, err=%v", found, err)
	}
}

func TestTableMergeWithoutPrimaryKeyUsesExistingRowAsMatch(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.RegisterTable("no-key-table", []TableColumn{
		TableColumnOf[string]("symbol"),
		TableColumnOf[float64]("price"),
	}); err != nil {
		t.Fatal(err)
	}
	source := From[runtimeTestTrade](env, "Trade")
	if _, err := RegisterStruct[triggerTestReset](env, "TriggerReset"); err != nil {
		t.Fatal(err)
	}
	resetSource := From[triggerTestReset](env, "TriggerReset")
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	mergePlan, err := env.Build(OnEvent(source).MergeIntoTableWhen("no-key-table", nil,
		WhenNotMatched(LikeOf(symbol, Literal("A%")),
			SetColumn("symbol", symbol),
			SetColumn("price", price),
		),
		WhenNotMatched(LikeOf(symbol, Literal("B%")),
			SetColumn("symbol", symbol),
			SetColumn("price", price),
		),
		WhenMatched(LikeOf(symbol, Literal("C%")),
			SetColumn("symbol", Literal("Z")),
			SetColumn("price", Literal(-1.0)),
		),
		WhenNotMatchedAny(
			SetColumn("symbol", ConcatOf(Literal("x"), symbol, Literal("x"))),
			SetColumn("price", Negate[float64](price)),
		),
	).Query(StatementName("no-key-merge")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.Build(OnEvent(source).MergeIntoTableWhen("no-key-table", []Expr{symbol},
		WhenNotMatchedAny(SetColumn("symbol", symbol), SetColumn("price", price)),
	).Query()); err == nil {
		t.Fatal("table merge with a key expression against a no-key table was accepted")
	}
	deletePlan, err := env.Build(OnEvent(resetSource).DeleteAllFromTable("no-key-table").Query(StatementName("no-key-delete-all")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	mergeDeployment, err := engine.Deploy(context.Background(), mergePlan)
	if err != nil {
		t.Fatal(err)
	}
	defer mergeDeployment.Undeploy(context.Background())
	var mergeBatches []ResultBatch
	if _, err := mergeDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		mergeBatches = append(mergeBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(symbol string, price float64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol, Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	send("E1", 2)
	if len(mergeBatches) != 1 || len(mergeBatches[0].New) != 1 || len(mergeBatches[0].Old) != 0 ||
		mergeBatches[0].New[0].Get("symbol").Any() != "xE1x" || mergeBatches[0].New[0].Get("price").Any() != float64(-2) {
		t.Fatalf("no-key fallback insert batch = %#v", mergeBatches)
	}
	send("A1", 3)
	if len(mergeBatches) != 1 {
		t.Fatalf("no-key matched row allowed not-matched branch = %#v", mergeBatches)
	}

	deleteDeployment, err := engine.Deploy(context.Background(), deletePlan)
	if err != nil {
		t.Fatal(err)
	}
	var deleteBatches []ResultBatch
	if _, err := deleteDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		deleteBatches = append(deleteBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), triggerTestReset{ID: "clear"}); err != nil {
		t.Fatal(err)
	}
	if len(deleteBatches) != 1 || len(deleteBatches[0].Old) != 1 || len(deleteBatches[0].New) != 0 ||
		deleteBatches[0].Old[0].Get("symbol").Any() != "xE1x" {
		t.Fatalf("no-key delete-all batch = %#v", deleteBatches)
	}
	if err := deleteDeployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}

	send("A1", 4)
	send("B1", 5)
	if len(mergeBatches) != 2 || len(mergeBatches[1].New) != 1 || mergeBatches[1].New[0].Get("symbol").Any() != "A1" {
		t.Fatalf("no-key A branch lifecycle = %#v", mergeBatches)
	}
	if table, ok := engine.Table("no-key-table"); !ok {
		t.Fatal("no-key table is missing")
	} else if rows, err := table.Snapshot(context.Background()); err != nil || len(rows) != 1 || rows[0].Get("symbol").Any() != "A1" {
		t.Fatalf("no-key A branch snapshot = %#v, err=%v", rows, err)
	}
	if table, ok := engine.Table("no-key-table"); ok {
		if _, err := table.Clear(context.Background()); err != nil {
			t.Fatal(err)
		}
	} else {
		t.Fatal("no-key table disappeared before B branch")
	}
	send("B1", 5)
	send("C", 6)
	if len(mergeBatches) != 4 || len(mergeBatches[2].New) != 1 || len(mergeBatches[3].Old) != 1 || len(mergeBatches[3].New) != 1 ||
		mergeBatches[2].New[0].Get("symbol").Any() != "B1" || mergeBatches[3].Old[0].Get("price").Any() != float64(5) ||
		mergeBatches[3].New[0].Get("symbol").Any() != "Z" || mergeBatches[3].New[0].Get("price").Any() != float64(-1) {
		t.Fatalf("no-key B/C branch lifecycle = %#v", mergeBatches)
	}
}

func TestMergeMatchedBranchReadsTargetRow(t *testing.T) {
	collect := func(t *testing.T, deployment *Deployment) *[]ResultBatch {
		t.Helper()
		batches := new([]ResultBatch)
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			*batches = append(*batches, batch)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return batches
	}

	t.Run("table", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
			t.Fatal(err)
		}
		if _, err := env.RegisterTable("merge-target-table", []TableColumn{
			PrimaryKeyColumn[string]("symbol"),
			TableColumnOf[float64]("price"),
		}); err != nil {
			t.Fatal(err)
		}
		source := From[runtimeTestTrade](env, "Trade")
		symbol := Field[runtimeTestTrade, string]("symbol")
		price := Field[runtimeTestTrade, float64]("price")
		oldPrice := TableField[float64]("price")
		plan, err := env.Build(OnEvent(source).MergeIntoTableWhen("merge-target-table", []Expr{symbol},
			WhenNotMatchedAny(SetColumn("symbol", symbol), SetColumn("price", price)),
			WhenMatchedDelete(Less[float64](price, Literal[float64](0))),
			WhenMatched(Greater[float64](price, Literal[float64](0)), SetColumn("price", Add[float64](price, oldPrice))),
		).Query(StatementName("merge-target-table-rule")))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := env.Build(OnEvent(source).MergeIntoTableWhen("merge-target-table", []Expr{symbol},
			WhenNotMatchedAny(SetColumn("symbol", symbol), SetColumn("price", oldPrice)),
		).Query()); err == nil {
			t.Fatal("not-matched table merge assignment could reference target row")
		}
		engine := NewEngine(env)
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer deployment.Undeploy(context.Background())
		batches := collect(t, deployment)
		send := func(symbol string, price float64) {
			t.Helper()
			if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol, Price: price}); err != nil {
				t.Fatal(err)
			}
		}
		send("E2", 2)
		send("E2", 10)
		if len(*batches) != 2 || len((*batches)[1].Old) != 1 || len((*batches)[1].New) != 1 ||
			(*batches)[1].Old[0].Get("price").Any() != float64(2) || (*batches)[1].New[0].Get("price").Any() != float64(12) {
			t.Fatalf("table matched target old/new = %#v", *batches)
		}
		send("E2", -1)
		if len(*batches) != 3 || len((*batches)[2].Old) != 1 || len((*batches)[2].New) != 0 || (*batches)[2].Old[0].Get("price").Any() != float64(12) {
			t.Fatalf("table matched target delete = %#v", *batches)
		}
		send("E3", 3)
		if len(*batches) != 4 || len((*batches)[3].New) != 1 || (*batches)[3].New[0].Get("price").Any() != float64(3) {
			t.Fatalf("table matched target second insert = %#v", *batches)
		}
	})

	t.Run("named-window", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
			t.Fatal(err)
		}
		schema, ok := env.Schema("Trade")
		if !ok {
			t.Fatal("Trade schema is missing")
		}
		if _, err := env.RegisterNamedWindow("merge-target-window", schema); err != nil {
			t.Fatal(err)
		}
		source := From[runtimeTestTrade](env, "Trade")
		symbol := Field[runtimeTestTrade, string]("symbol")
		price := Field[runtimeTestTrade, float64]("price")
		oldPrice := NamedWindowField[float64]("price")
		match := Equal[string](NamedWindowField[string]("symbol"), symbol)
		plan, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen("merge-target-window", match,
			WhenNotMatchedAny(SetColumn("symbol", symbol), SetColumn("price", price)),
			WhenMatchedDelete(Less[float64](price, Literal[float64](0))),
			WhenMatched(Greater[float64](price, Literal[float64](0)), SetColumn("price", Add[float64](price, oldPrice))),
		).Query(StatementName("merge-target-window-rule")))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer deployment.Undeploy(context.Background())
		batches := collect(t, deployment)
		send := func(symbol string, price float64) {
			t.Helper()
			if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol, Price: price}); err != nil {
				t.Fatal(err)
			}
		}
		send("E2", 2)
		send("E2", 10)
		if len(*batches) != 2 || len((*batches)[1].Old) != 1 || len((*batches)[1].New) != 1 ||
			(*batches)[1].Old[0].Get("price").Any() != float64(2) || (*batches)[1].New[0].Get("price").Any() != float64(12) {
			t.Fatalf("named-window matched target old/new = %#v", *batches)
		}
		send("E2", -1)
		if len(*batches) != 3 || len((*batches)[2].Old) != 1 || len((*batches)[2].New) != 0 {
			t.Fatalf("named-window matched target delete = %#v", *batches)
		}
	})
}

func TestSingleSidedMergeBranchesAndConvenienceConstructors(t *testing.T) {
	newEngine := func(t *testing.T, namedWindow bool) (*Environment, *Engine, Stream[runtimeTestTrade], Expression[string], Expression[float64]) {
		t.Helper()
		env := NewEnvironment()
		if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
			t.Fatal(err)
		}
		schema, ok := env.Schema("Trade")
		if !ok {
			t.Fatal("Trade schema is missing")
		}
		if namedWindow {
			if _, err := env.RegisterNamedWindow("branch-window", schema); err != nil {
				t.Fatal(err)
			}
		} else if _, err := env.RegisterTable("branch-table", []TableColumn{
			PrimaryKeyColumn[string]("symbol"),
			TableColumnOf[float64]("price"),
		}); err != nil {
			t.Fatal(err)
		}
		return env, NewEngine(env), From[runtimeTestTrade](env, "Trade"), Field[runtimeTestTrade, string]("symbol"), Field[runtimeTestTrade, float64]("price")
	}

	collect := func(t *testing.T, deployment *Deployment) *[]ResultBatch {
		t.Helper()
		batches := new([]ResultBatch)
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			*batches = append(*batches, batch)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return batches
	}

	t.Run("table", func(t *testing.T) {
		env, engine, source, symbol, price := newEngine(t, false)
		insertOnlyPlan, err := env.Build(OnEvent(source).MergeIntoTableWhen("branch-table", []Expr{symbol},
			WhenNotMatchedAny(SetColumn("symbol", symbol), SetColumn("price", price)),
		).Query(StatementName("table-insert-only")))
		if err != nil {
			t.Fatal(err)
		}
		insertOnlyDeployment, err := engine.Deploy(context.Background(), insertOnlyPlan)
		if err != nil {
			t.Fatal(err)
		}
		insertBatches := collect(t, insertOnlyDeployment)
		send := func(event runtimeTestTrade) {
			t.Helper()
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
		send(runtimeTestTrade{Symbol: "A", Price: 1})
		send(runtimeTestTrade{Symbol: "A", Price: 2})
		if len(*insertBatches) != 1 || len((*insertBatches)[0].New) != 1 || len((*insertBatches)[0].Old) != 0 {
			t.Fatalf("table insert-only batches = %#v", *insertBatches)
		}
		if err := engine.Undeploy(context.Background(), insertOnlyDeployment.ID()); err != nil {
			t.Fatal(err)
		}

		matchedPlan, err := env.Build(OnEvent(source).MergeIntoTableWhen("branch-table", []Expr{symbol},
			WhenMatchedAny(SetColumn("price", price)),
		).Query(StatementName("table-matched-only")))
		if err != nil {
			t.Fatal(err)
		}
		matchedDeployment, err := engine.Deploy(context.Background(), matchedPlan)
		if err != nil {
			t.Fatal(err)
		}
		matchedBatches := collect(t, matchedDeployment)
		send(runtimeTestTrade{Symbol: "A", Price: 3})
		send(runtimeTestTrade{Symbol: "missing", Price: 9})
		if len(*matchedBatches) != 1 || len((*matchedBatches)[0].Old) != 1 || len((*matchedBatches)[0].New) != 1 {
			t.Fatalf("table matched-only batches = %#v", *matchedBatches)
		}
		if (*matchedBatches)[0].Old[0].Get("price").Any() != float64(1) || (*matchedBatches)[0].New[0].Get("price").Any() != float64(3) {
			t.Fatalf("table matched-only old/new = %#v", *matchedBatches)
		}
		if err := engine.Undeploy(context.Background(), matchedDeployment.ID()); err != nil {
			t.Fatal(err)
		}

		deletePlan, err := env.Build(OnEvent(source).MergeIntoTableWhen("branch-table", []Expr{symbol},
			WhenMatchedDeleteAny(),
		).Query(StatementName("table-delete-only")))
		if err != nil {
			t.Fatal(err)
		}
		deleteDeployment, err := engine.Deploy(context.Background(), deletePlan)
		if err != nil {
			t.Fatal(err)
		}
		deleteBatches := collect(t, deleteDeployment)
		send(runtimeTestTrade{Symbol: "A"})
		send(runtimeTestTrade{Symbol: "missing"})
		if len(*deleteBatches) != 1 || len((*deleteBatches)[0].Old) != 1 || len((*deleteBatches)[0].New) != 0 {
			t.Fatalf("table delete-only batches = %#v", *deleteBatches)
		}
	})

	t.Run("named-window", func(t *testing.T) {
		env, engine, source, symbol, price := newEngine(t, true)
		match := Equal[string](NamedWindowField[string]("symbol"), symbol)
		insertOnlyPlan, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen("branch-window", match,
			WhenNotMatchedAny(SetColumn("symbol", symbol), SetColumn("price", price)),
		).Query(StatementName("window-insert-only")))
		if err != nil {
			t.Fatal(err)
		}
		insertOnlyDeployment, err := engine.Deploy(context.Background(), insertOnlyPlan)
		if err != nil {
			t.Fatal(err)
		}
		insertBatches := collect(t, insertOnlyDeployment)
		send := func(event runtimeTestTrade) {
			t.Helper()
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
		send(runtimeTestTrade{Symbol: "A", Price: 1})
		send(runtimeTestTrade{Symbol: "A", Price: 2})
		send(runtimeTestTrade{Symbol: "B", Price: 3})
		if len(*insertBatches) != 2 || len((*insertBatches)[0].New) != 1 || len((*insertBatches)[1].New) != 1 {
			t.Fatalf("named-window insert-only batches = %#v", *insertBatches)
		}
		if err := engine.Undeploy(context.Background(), insertOnlyDeployment.ID()); err != nil {
			t.Fatal(err)
		}

		matchedPlan, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen("branch-window", match,
			WhenMatchedAny(SetColumn("price", price)),
		).Query(StatementName("window-matched-only")))
		if err != nil {
			t.Fatal(err)
		}
		matchedDeployment, err := engine.Deploy(context.Background(), matchedPlan)
		if err != nil {
			t.Fatal(err)
		}
		matchedBatches := collect(t, matchedDeployment)
		send(runtimeTestTrade{Symbol: "A", Price: 4})
		send(runtimeTestTrade{Symbol: "missing", Price: 9})
		if len(*matchedBatches) != 1 || len((*matchedBatches)[0].Old) != 1 || len((*matchedBatches)[0].New) != 1 {
			t.Fatalf("named-window matched-only batches = %#v", *matchedBatches)
		}
		if err := engine.Undeploy(context.Background(), matchedDeployment.ID()); err != nil {
			t.Fatal(err)
		}

		deletePlan, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen("branch-window", match,
			WhenMatchedDeleteAny(),
		).Query(StatementName("window-delete-only")))
		if err != nil {
			t.Fatal(err)
		}
		deleteDeployment, err := engine.Deploy(context.Background(), deletePlan)
		if err != nil {
			t.Fatal(err)
		}
		deleteBatches := collect(t, deleteDeployment)
		send(runtimeTestTrade{Symbol: "A"})
		send(runtimeTestTrade{Symbol: "missing"})
		if len(*deleteBatches) != 1 || len((*deleteBatches)[0].Old) != 1 || len((*deleteBatches)[0].New) != 0 {
			t.Fatalf("named-window delete-only batches = %#v", *deleteBatches)
		}
	})
}

func TestVariableTriggerSetUsesEventSnapshotAndOrderedDispatch(t *testing.T) {
	env, _ := newRuntimeTest(t)
	if err := env.RegisterVariable("threshold", 0.0); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("last-symbol", ""); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("double-threshold", 0.0); err != nil {
		t.Fatal(err)
	}
	source := From[runtimeTestTrade](env, "Trade")
	symbol := Field[runtimeTestTrade, string]("symbol")
	price := Field[runtimeTestTrade, float64]("price")
	setPlan, err := env.Build(OnEvent(source).SetVariables(
		SetVariableExpr("threshold", price),
		SetVariableExpr("double-threshold", Add[float64](VariableRef[float64]("threshold"), price)),
		SetVariableExpr("last-symbol", symbol),
	).Query(StatementName("00-set-variables")))
	if err != nil {
		t.Fatal(err)
	}
	filterPlan, err := env.Build(source.Filter(
		Equal[float64](price, VariableRef[float64]("threshold")),
	).Query(StatementName("01-observe-variable")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), setPlan); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), filterPlan)
	if err != nil {
		t.Fatal(err)
	}
	var observed []runtimeTestTrade
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				return fmt.Errorf("variable trigger result is not an event")
			}
			observed = append(observed, event.Underlying().(runtimeTestTrade))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 12}); err != nil {
		t.Fatal(err)
	}
	if len(observed) != 1 || observed[0].Symbol != "A" {
		t.Fatalf("ordered variable-trigger observation = %#v", observed)
	}
	threshold, ok := engine.GetVariable("threshold")
	if !ok || !threshold.Equal(Present(12.0)) {
		t.Fatalf("threshold after trigger = %#v, ok=%v", threshold, ok)
	}
	lastSymbol, ok := engine.GetVariable("last-symbol")
	if !ok || !lastSymbol.Equal(Present("A")) {
		t.Fatalf("last-symbol after trigger = %#v, ok=%v", lastSymbol, ok)
	}
	doubleThreshold, ok := engine.GetVariable("double-threshold")
	if !ok || !doubleThreshold.Equal(Present(24.0)) {
		t.Fatalf("ordered double-threshold after trigger = %#v, ok=%v", doubleThreshold, ok)
	}
}

func TestVariableTriggerValidation(t *testing.T) {
	env, _ := newRuntimeTest(t)
	if err := env.RegisterVariable("threshold", 0.0); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("constant", 1.0, ConstantVariable()); err != nil {
		t.Fatal(err)
	}
	source := From[runtimeTestTrade](env, "Trade")
	price := Field[runtimeTestTrade, float64]("price")
	if _, err := env.Build(OnEvent(source).SetVariables().Query()); err == nil {
		t.Fatal("empty variable trigger was accepted")
	}
	if _, err := env.Build(OnEvent(source).SetVariable("missing", price).Query()); err == nil {
		t.Fatal("unknown variable trigger was accepted")
	}
	if _, err := env.Build(OnEvent(source).SetVariable("constant", price).Query()); err == nil {
		t.Fatal("constant variable trigger was accepted")
	}
	if _, err := env.Build(OnEvent(source).SetVariable("threshold", Field[runtimeTestTrade, string]("symbol")).Query()); err == nil {
		t.Fatal("variable trigger type mismatch was accepted")
	}
}

func TestTableSelectTriggerEmitsProjectedRow(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.RegisterTable("positions", []TableColumn{
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[float64]("price"),
	}); err != nil {
		t.Fatal(err)
	}
	source := From[runtimeTestTrade](env, "Trade")
	symbol := Field[runtimeTestTrade, string]("symbol")
	plan, err := env.Build(OnEvent(source).SelectFromTable("positions", []Expr{symbol},
		Alias("price", Field[any, float64]("price")),
	).Query(StatementName("on-select")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	table, ok := engine.Table("positions")
	if !ok {
		t.Fatal("positions table is missing")
	}
	if _, err := table.Insert(context.Background(), map[string]any{"symbol": "A", "price": 7.5}); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("on-select result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Get("price").State() != ValueNull || rows[1].Get("price").Any() != float64(7.5) {
		t.Fatalf("on-select rows = %#v", rows)
	}
}

func TestTablePredicateSelectProjectsMatchingSnapshotRows(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.RegisterTable("positions", []TableColumn{
		PrimaryKeyColumn[string]("symbol"),
		TableColumnOf[float64]("price"),
	}); err != nil {
		t.Fatal(err)
	}
	source := From[runtimeTestTrade](env, "Trade")
	triggerSymbol := Field[runtimeTestTrade, string]("symbol")
	triggerPrice := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(OnEvent(source).SelectFromTableWhere("positions",
		Greater[float64](TableField[float64]("price"), triggerPrice),
		Alias("stored-symbol", TableField[string]("symbol")),
		Alias("stored-price", TableField[float64]("price")),
		Alias("trigger-symbol", triggerSymbol),
	).Query(StatementName("table-select-where")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	table, ok := engine.Table("positions")
	if !ok {
		t.Fatal("positions table is missing")
	}
	for _, values := range []map[string]any{{"symbol": "A", "price": 2.0}, {"symbol": "B", "price": 8.0}} {
		if _, err := table.Insert(context.Background(), values); err != nil {
			t.Fatal(err)
		}
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("table predicate select result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "T", Price: 5}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("stored-symbol").Any() != "B" || rows[0].Get("stored-price").Any() != float64(8) || rows[0].Get("trigger-symbol").Any() != "T" {
		t.Fatalf("table predicate select rows = %#v", rows)
	}
}

type triggerArrayAssignmentEvent struct {
	Index int     `esper:"index"`
	Value float64 `esper:"value"`
}

type triggerArrayAssignmentRow struct {
	Count int       `esper:"cnt"`
	Array []float64 `esper:"thearray"`
}

type triggerOrderedScalarAssignmentEvent struct {
	Key string `esper:"key"`
	ID  int    `esper:"id"`
}

func TestTriggerArrayAssignmentsPreserveOrderedWorkingAndInitialValues(t *testing.T) {
	setups := []struct {
		name        string
		namedWindow bool
	}{
		{name: "table", namedWindow: false},
		{name: "named-window", namedWindow: true},
	}
	assignments := []struct {
		name string
		list func() []TableAssignment
		want []float64
		cnt  int
	}{
		{
			name: "direct-indexes",
			list: func() []TableAssignment {
				return []TableAssignment{
					SetArrayElement("thearray", TableField[int]("cnt"), Literal[float64](1)),
					SetArrayElement("thearray", Field[triggerArrayAssignmentEvent, int]("index"), Literal[float64](2)),
				}
			},
			want: []float64{1, 2, 0}, cnt: 0,
		},
		{
			name: "working-count",
			list: func() []TableAssignment {
				return []TableAssignment{
					SetColumn("cnt", Add[int](TableField[int]("cnt"), Literal[int](1))),
					SetArrayElement("thearray", TableField[int]("cnt"), Literal[float64](1)),
				}
			},
			want: []float64{0, 1, 0}, cnt: 1,
		},
		{
			name: "working-count-twice",
			list: func() []TableAssignment {
				return []TableAssignment{
					SetColumn("cnt", Add[int](TableField[int]("cnt"), Literal[int](1))),
					SetArrayElement("thearray", TableField[int]("cnt"), Literal[float64](3)),
					SetColumn("cnt", Add[int](TableField[int]("cnt"), Literal[int](1))),
					SetArrayElement("thearray", TableField[int]("cnt"), Literal[float64](4)),
				}
			},
			want: []float64{0, 3, 4}, cnt: 2,
		},
		{
			name: "initial-count",
			list: func() []TableAssignment {
				return []TableAssignment{
					SetColumn("cnt", Add[int](TableField[int]("cnt"), Literal[int](1))),
					SetArrayElement("thearray", InitialTableField[int]("cnt"), Literal[float64](3)),
				}
			},
			want: []float64{3, 0, 0}, cnt: 1,
		},
	}

	for _, setup := range setups {
		for _, assignment := range assignments {
			t.Run(setup.name+"/"+assignment.name, func(t *testing.T) {
				env := NewEnvironment()
				_, err := RegisterStruct[triggerArrayAssignmentEvent](env, "TriggerArrayAssignmentEvent")
				if err != nil {
					t.Fatal(err)
				}
				var targetName string
				if setup.namedWindow {
					targetName = "trigger-array-window"
					targetSchema, schemaErr := RegisterStruct[triggerArrayAssignmentRow](env, "TriggerArrayAssignmentRow")
					if schemaErr != nil {
						t.Fatal(schemaErr)
					}
					if _, err := CreateNamedWindow(env, targetName, targetSchema, NamedWindowRetention(KeepAll())); err != nil {
						t.Fatal(err)
					}
				} else {
					targetName = "trigger-array-table"
					if _, err := CreateTable(env, targetName, []TableColumn{
						TableColumnOf[int]("cnt"),
						TableColumnOf[[]float64]("thearray"),
					}); err != nil {
						t.Fatal(err)
					}
				}
				source := From[triggerArrayAssignmentEvent](env, "TriggerArrayAssignmentEvent")
				var createPlan, updatePlan Plan
				if setup.namedWindow {
					createPlan, err = env.Build(OnEvent(source).MergeIntoNamedWindowWhen(targetName, Literal(false),
						WhenNotMatchedAny(
							SetColumn("cnt", Literal[int](0)),
							SetColumn("thearray", Literal[[]float64]([]float64{0, 0, 0})),
						),
					).Query(StatementName("trigger-array-create")))
					if err != nil {
						t.Fatal(err)
					}
					updatePlan, err = env.Build(OnEvent(source).UpdateNamedWindow(targetName, Literal(true), assignment.list()...).Query(StatementName("trigger-array-update")))
				} else {
					createPlan, err = env.Build(OnEvent(source).MergeIntoTableWhen(targetName, nil,
						WhenNotMatchedAny(
							SetColumn("cnt", Literal[int](0)),
							SetColumn("thearray", Literal[[]float64]([]float64{0, 0, 0})),
						),
					).Query(StatementName("trigger-array-create")))
					if err != nil {
						t.Fatal(err)
					}
					updatePlan, err = env.Build(OnEvent(source).UpdateTableWhere(targetName, Literal(true), assignment.list()...).Query(StatementName("trigger-array-update")))
				}
				if err != nil {
					t.Fatal(err)
				}
				engine := NewEngine(env)
				if _, err := engine.Deploy(context.Background(), createPlan); err != nil {
					t.Fatal(err)
				}
				if _, err := engine.Deploy(context.Background(), updatePlan); err != nil {
					t.Fatal(err)
				}
				if err := engine.SendEvent(context.Background(), triggerArrayAssignmentEvent{Index: 1, Value: 2}); err != nil {
					t.Fatal(err)
				}
				if setup.namedWindow {
					window, ok := engine.NamedWindow(targetName)
					if !ok {
						t.Fatal("named window is missing")
					}
					// The public snapshot is sufficient for the mutation contract.
					events, snapshotErr := window.Snapshot(context.Background())
					if snapshotErr != nil {
						t.Fatal(snapshotErr)
					}
					if len(events) != 1 {
						t.Fatalf("named-window rows = %d", len(events))
					}
					if got := events[0].Get("cnt").Any(); got != assignment.cnt {
						t.Fatalf("named-window cnt = %#v, want %d", got, assignment.cnt)
					}
					if got := events[0].Get("thearray").Any(); !reflect.DeepEqual(got, assignment.want) {
						t.Fatalf("named-window array = %#v, want %#v", got, assignment.want)
					}
				} else {
					table, ok := engine.Table(targetName)
					if !ok {
						t.Fatal("table is missing")
					}
					rows, snapshotErr := table.Snapshot(context.Background())
					if snapshotErr != nil {
						t.Fatal(snapshotErr)
					}
					if len(rows) != 1 {
						t.Fatalf("table rows = %d", len(rows))
					}
					if got := rows[0].Get("cnt").Any(); got != assignment.cnt {
						t.Fatalf("table cnt = %#v, want %d", got, assignment.cnt)
					}
					if got := rows[0].Get("thearray").Any(); !reflect.DeepEqual(got, assignment.want) {
						t.Fatalf("table array = %#v, want %#v", got, assignment.want)
					}
				}
			})
		}
	}
}

func TestTriggerArrayAssignmentsRejectInvalidBuildersAndRuntimeBounds(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[triggerArrayAssignmentEvent](env, "TriggerArrayAssignmentEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "trigger-array-invalid", []TableColumn{
		PrimaryKeyColumn[string]("id"),
		OptionalTableColumnOf[[]int]("numbers"),
		TableColumnOf[int]("scalar"),
	}); err != nil {
		t.Fatal(err)
	}
	source := From[triggerArrayAssignmentEvent](env, "TriggerArrayAssignmentEvent")
	invalid := []struct {
		name string
		set  TableAssignment
	}{
		{name: "unknown-column", set: SetArrayElement("missing", Literal[int](0), Literal[int](1))},
		{name: "non-array-column", set: SetArrayElement("scalar", Literal[int](0), Literal[int](1))},
		{name: "non-integer-index", set: SetArrayElement("numbers", Literal("bad"), Literal[int](1))},
		{name: "incompatible-element", set: SetArrayElement("numbers", Literal[int](0), Literal[string]("bad"))},
	}
	for _, testCase := range invalid {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(OnEvent(source).UpdateTableWhere("trigger-array-invalid", Literal(true), testCase.set).
				Query(StatementName("invalid-array-" + testCase.name)))
			if err == nil {
				t.Fatalf("invalid array assignment %q was accepted", testCase.name)
			}
		})
	}

	engine := NewEngine(env)
	table, ok := engine.Table("trigger-array-invalid")
	if !ok {
		t.Fatal("invalid-assignment table is missing")
	}
	if _, err := table.Insert(context.Background(), map[string]any{"id": "present", "numbers": []int{1, 2}, "scalar": 0}); err != nil {
		t.Fatal(err)
	}
	if _, err := table.Insert(context.Background(), map[string]any{"id": "null", "numbers": nil, "scalar": 0}); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(OnEvent(source).UpdateTableWhere("trigger-array-invalid", Literal(true),
		SetArrayElement("numbers", Literal[int](0), Literal[int](9)),
	).Query(StatementName("array-bounds")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), triggerArrayAssignmentEvent{}); err != nil {
		t.Fatal(err)
	}
	rows, err := table.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !reflect.DeepEqual(rows[0].Get("numbers").Any(), []int{9, 2}) || !rows[1].Get("numbers").IsNull() {
		t.Fatalf("null-array indexed assignment rows = %#v", rows)
	}
	if err := engine.Undeploy(context.Background(), deployment.ID()); err != nil {
		t.Fatal(err)
	}

	plan, err = env.Build(OnEvent(source).UpdateTableWhere("trigger-array-invalid", Literal(true),
		SetArrayElement("numbers", Literal[int](2), Literal[int](9)),
	).Query(StatementName("array-out-of-range")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err = engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), triggerArrayAssignmentEvent{}); err == nil {
		t.Fatal("out-of-range array assignment did not fail")
	}
}

func TestTriggerNestedAssignmentsPreserveMapAndObjectArrayValues(t *testing.T) {
	cases := []struct {
		name        string
		objectArray bool
		namedWindow bool
	}{
		{name: "map-table"},
		{name: "map-named-window", namedWindow: true},
		{name: "object-array-table", objectArray: true},
		{name: "object-array-named-window", objectArray: true, namedWindow: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env := NewEnvironment()
			var sourceName, targetName string
			var source Schema
			var target Schema
			var nested Schema
			if testCase.objectArray {
				inner, err := RegisterObjectArray(env, "NestedAssignmentInner", []FieldSpec{
					FieldDef("c0", reflect.TypeOf(int(0))),
				})
				if err != nil {
					t.Fatal(err)
				}
				nested = inner
				target, err = RegisterObjectArray(env, "NestedAssignmentTarget", []FieldSpec{
					FieldDef("k", reflect.TypeOf("")),
					FieldDef("cflat", reflect.TypeOf([]any{})),
					FieldDef("carr", reflect.TypeOf([][]any{})),
				}, WithNestedPropertySchema("cflat", inner), WithNestedPropertySchema("carr", inner))
				if err != nil {
					t.Fatal(err)
				}
				source, err = RegisterObjectArray(env, "NestedAssignmentSource", []FieldSpec{
					FieldDef("cf", reflect.TypeOf([]any{})),
					FieldDef("ca", reflect.TypeOf([][]any{})),
				})
				if err != nil {
					t.Fatal(err)
				}
				sourceName, targetName = "NestedAssignmentSource", "nested-assignment-object-array"
			} else {
				inner, err := RegisterMap(env, "NestedAssignmentInner", []FieldSpec{
					FieldDef("c0", reflect.TypeOf(int(0))),
				})
				if err != nil {
					t.Fatal(err)
				}
				target, err = RegisterMap(env, "NestedAssignmentTarget", []FieldSpec{
					FieldDef("k", reflect.TypeOf("")),
					FieldDef("cflat", reflect.TypeOf(map[string]any{})),
					FieldDef("carr", reflect.TypeOf([]map[string]any{})),
				}, WithNestedPropertySchema("cflat", inner), WithNestedPropertySchema("carr", inner))
				if err != nil {
					t.Fatal(err)
				}
				nested = inner
				source, err = RegisterMap(env, "NestedAssignmentSource", []FieldSpec{
					FieldDef("cf", reflect.TypeOf(map[string]any{})),
					FieldDef("ca", reflect.TypeOf([]map[string]any{})),
				})
				if err != nil {
					t.Fatal(err)
				}
				sourceName, targetName = "NestedAssignmentSource", "nested-assignment-map"
			}
			_ = source

			var table *Table
			var window *NamedWindow
			if testCase.namedWindow {
				if _, err := CreateNamedWindow(env, targetName, target, NamedWindowRetention(LastEvent())); err != nil {
					t.Fatal(err)
				}
			} else {
				columns := []TableColumn{PrimaryKeyColumn[string]("k")}
				if testCase.objectArray {
					columns = append(columns,
						OptionalTableColumnOf[[]any]("cflat", WithTableColumnNestedSchema(nested)),
						OptionalTableColumnOf[[][]any]("carr", WithTableColumnNestedSchema(nested)),
					)
				} else {
					columns = append(columns,
						OptionalTableColumnOf[map[string]any]("cflat", WithTableColumnNestedSchema(nested)),
						OptionalTableColumnOf[[]map[string]any]("carr", WithTableColumnNestedSchema(nested)),
					)
				}
				if _, err := CreateTable(env, targetName, columns); err != nil {
					t.Fatal(err)
				}
			}

			sourceStream := FromAny(env, sourceName)
			var plan Plan
			var err error
			if testCase.objectArray {
				cf := Field[any, []any]("cf")
				ca := Field[any, [][]any]("ca")
				if testCase.namedWindow {
					plan, err = env.Build(OnRecord(sourceStream).UpdateNamedWindow(targetName, Literal(true),
						SetColumn("cflat", cf), SetColumn("carr", ca),
					).Query(StatementName("nested-assignment-update")))
				} else {
					plan, err = env.Build(OnRecord(sourceStream).UpdateTableWhere(targetName, Literal(true),
						SetColumn("cflat", cf), SetColumn("carr", ca),
					).Query(StatementName("nested-assignment-update")))
				}
			} else {
				cf := Field[any, map[string]any]("cf")
				ca := Field[any, []map[string]any]("ca")
				if testCase.namedWindow {
					plan, err = env.Build(OnRecord(sourceStream).UpdateNamedWindow(targetName, Literal(true),
						SetColumn("cflat", cf), SetColumn("carr", ca),
					).Query(StatementName("nested-assignment-update")))
				} else {
					plan, err = env.Build(OnRecord(sourceStream).UpdateTableWhere(targetName, Literal(true),
						SetColumn("cflat", cf), SetColumn("carr", ca),
					).Query(StatementName("nested-assignment-update")))
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			if testCase.namedWindow {
				window, _ = engine.NamedWindow(targetName)
				if testCase.objectArray {
					if err := engine.InsertNamedWindow(context.Background(), targetName, []any{"E1", nil, nil}); err != nil {
						t.Fatal(err)
					}
				} else if err := engine.InsertNamedWindow(context.Background(), targetName, map[string]any{"k": "E1", "cflat": nil, "carr": nil}); err != nil {
					t.Fatal(err)
				}
			} else {
				table, _ = engine.Table(targetName)
				if _, err := table.Insert(context.Background(), map[string]any{"k": "E1", "cflat": nil, "carr": nil}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := engine.Deploy(context.Background(), plan); err != nil {
				t.Fatal(err)
			}
			if testCase.objectArray {
				if err := engine.SendObjectArray(context.Background(), sourceName, []any{[]any{1}, [][]any{{1}, {2}}}); err != nil {
					t.Fatal(err)
				}
			} else if err := engine.Send(context.Background(), sourceName, map[string]any{
				"cf": map[string]any{"c0": 1},
				"ca": []map[string]any{{"c0": 1}, {"c0": 2}},
			}); err != nil {
				t.Fatal(err)
			}

			var resultEvent Event
			if testCase.namedWindow {
				events, snapshotErr := window.Snapshot(context.Background())
				if snapshotErr != nil || len(events) != 1 {
					t.Fatalf("nested named-window snapshot = %#v, err=%v", events, snapshotErr)
				}
				resultEvent = events[0]
			} else {
				rows, snapshotErr := table.Snapshot(context.Background())
				if snapshotErr != nil || len(rows) != 1 {
					t.Fatalf("nested table snapshot = %#v, err=%v", rows, snapshotErr)
				}
				resultEvent, err = tableRowEvent(table, targetName, rows[0], engine.Now())
				if err != nil {
					t.Fatal(err)
				}
			}
			if testCase.objectArray {
				if !reflect.DeepEqual(resultEvent.Get("cflat").Any(), []any{1}) ||
					!reflect.DeepEqual(resultEvent.Get("carr").Any(), [][]any{{1}, {2}}) {
					t.Fatalf("object-array nested assignment values = cflat=%#v carr=%#v", resultEvent.Get("cflat"), resultEvent.Get("carr"))
				}
			}
			if resultEvent.Get("cflat.c0").Any() != int(1) || resultEvent.Get("carr[0].c0").Any() != int(1) || resultEvent.Get("carr[1].c0").Any() != int(2) {
				t.Fatalf("map nested assignment values = cflat=%#v carr0=%#v carr1=%#v", resultEvent.Get("cflat.c0"), resultEvent.Get("carr[0].c0"), resultEvent.Get("carr[1].c0"))
			}
		})
	}
}

func TestTriggerOrderedScalarAssignmentsMatchInfraUpdateOrderOfFields(t *testing.T) {
	cases := []struct {
		name        string
		namedWindow bool
		merge       bool
	}{
		{name: "table-merge", merge: true},
		{name: "named-window-merge", namedWindow: true, merge: true},
		{name: "table-update"},
		{name: "named-window-update", namedWindow: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[triggerOrderedScalarAssignmentEvent](env, "TriggerOrderedScalarAssignmentEvent"); err != nil {
				t.Fatal(err)
			}
			const targetName = "trigger-ordered-scalar"
			fields := []FieldSpec{
				FieldDef("theString", reflect.TypeOf("")),
				FieldDef("intPrimitive", reflect.TypeOf(int(0))),
				FieldDef("intBoxed", reflect.TypeOf(int(0))),
				FieldDef("doublePrimitive", reflect.TypeOf(float64(0))),
			}
			var table *Table
			if testCase.namedWindow {
				targetSchema, err := RegisterMap(env, "TriggerOrderedScalarTarget", fields)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := CreateNamedWindow(env, targetName, targetSchema, NamedWindowRetention(KeepAll())); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := CreateTable(env, targetName, []TableColumn{
					PrimaryKeyColumn[string]("theString"),
					TableColumnOf[int]("intPrimitive"),
					TableColumnOf[int]("intBoxed"),
					TableColumnOf[float64]("doublePrimitive"),
				}); err != nil {
					t.Fatal(err)
				}
			}

			engine := NewEngine(env)
			initial := map[string]any{
				"theString":       "E1",
				"intPrimitive":    1,
				"intBoxed":        2,
				"doublePrimitive": 2.0,
			}
			if testCase.namedWindow {
				if err := engine.InsertNamedWindow(context.Background(), targetName, initial); err != nil {
					t.Fatal(err)
				}
			} else {
				table, _ = engine.Table(targetName)
				if _, err := table.Insert(context.Background(), initial); err != nil {
					t.Fatal(err)
				}
			}

			source := From[triggerOrderedScalarAssignmentEvent](env, "TriggerOrderedScalarAssignmentEvent")
			key := Field[triggerOrderedScalarAssignmentEvent, string]("key")
			id := Field[triggerOrderedScalarAssignmentEvent, int]("id")
			var targetInt Expression[int]
			var initialInt Expression[int]
			if testCase.namedWindow {
				targetInt = NamedWindowField[int]("intPrimitive")
				initialInt = InitialNamedWindowField[int]("intPrimitive")
			} else {
				targetInt = TableField[int]("intPrimitive")
				initialInt = InitialTableField[int]("intPrimitive")
			}
			assignments := []TableAssignment{
				SetColumn("intPrimitive", id),
				SetColumn("intBoxed", targetInt),
				SetColumn("doublePrimitive", Cast[int, float64](initialInt)),
			}
			var plan Plan
			var err error
			if testCase.merge {
				if testCase.namedWindow {
					match := Equal[string](NamedWindowField[string]("theString"), key)
					plan, err = env.Build(OnEvent(source).MergeIntoNamedWindowWhen(targetName, match,
						WhenMatchedAny(assignments...),
					).Query(StatementName("trigger-ordered-scalar")))
				} else {
					plan, err = env.Build(OnEvent(source).MergeIntoTableWhen(targetName, []Expr{key},
						WhenMatchedAny(assignments...),
					).Query(StatementName("trigger-ordered-scalar")))
				}
			} else if testCase.namedWindow {
				match := Equal[string](NamedWindowField[string]("theString"), key)
				plan, err = env.Build(OnEvent(source).UpdateNamedWindow(targetName, match, assignments...).Query(StatementName("trigger-ordered-scalar")))
			} else {
				match := Equal[string](TableField[string]("theString"), key)
				plan, err = env.Build(OnEvent(source).UpdateTableWhere(targetName, match, assignments...).Query(StatementName("trigger-ordered-scalar")))
			}
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			var batches []ResultBatch
			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
				batches = append(batches, batch)
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			sendAndAssert := func(key string, id int, wantBoxed, wantDouble float64) {
				t.Helper()
				if err := engine.SendEvent(context.Background(), triggerOrderedScalarAssignmentEvent{Key: key, ID: id}); err != nil {
					t.Fatal(err)
				}
				if len(batches) == 0 || len(batches[len(batches)-1].New) != 1 {
					t.Fatalf("ordered scalar mutation batches = %#v", batches)
				}
				event, ok := batches[len(batches)-1].New[0].Event()
				if !ok {
					t.Fatalf("ordered scalar result is not an event: %#v", batches[len(batches)-1].New[0])
				}
				if event.Get("intPrimitive").Any() != id || event.Get("intBoxed").Any() != int(wantBoxed) || event.Get("doublePrimitive").Any() != wantDouble {
					t.Fatalf("ordered scalar result = %v/%v/%v, want %d/%d/%v", event.Get("intPrimitive").Any(), event.Get("intBoxed").Any(), event.Get("doublePrimitive").Any(), id, int(wantBoxed), wantDouble)
				}
			}
			sendAndAssert("E1", 5, 5, 1.0)
			sendAndAssert("E1", 7, 7, 5.0)
		})
	}
}

func TestTriggerMergeInsertOnlyConvenienceMatchesInfraOnMergeSimpleInsert(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		namedWindow bool
	}{
		{name: "table"},
		{name: "named-window", namedWindow: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[runtimeTestTrade](env, "MergeInsertOnlyTrade"); err != nil {
				t.Fatal(err)
			}
			const targetName = "merge-insert-only"
			if testCase.namedWindow {
				schema, ok := env.Schema("MergeInsertOnlyTrade")
				if !ok {
					t.Fatal("merge insert-only schema is missing")
				}
				if _, err := CreateNamedWindow(env, targetName, schema, NamedWindowRetention(KeepAll())); err != nil {
					t.Fatal(err)
				}
			} else if _, err := CreateTable(env, targetName, []TableColumn{
				PrimaryKeyColumn[string]("symbol"),
				TableColumnOf[float64]("price"),
			}); err != nil {
				t.Fatal(err)
			}

			source := From[runtimeTestTrade](env, "MergeInsertOnlyTrade")
			symbol := Field[runtimeTestTrade, string]("symbol")
			price := Field[runtimeTestTrade, float64]("price")
			var plan Plan
			var err error
			if testCase.namedWindow {
				match := Equal[string](NamedWindowField[string]("symbol"), symbol)
				plan, err = env.Build(OnEvent(source).MergeInsertIntoNamedWindow(targetName, match,
					SetColumn("symbol", symbol), SetColumn("price", price),
				).Query(StatementName("merge-insert-only")))
			} else {
				plan, err = env.Build(OnEvent(source).MergeInsertIntoTable(targetName, []Expr{symbol},
					SetColumn("symbol", symbol), SetColumn("price", price),
				).Query(StatementName("merge-insert-only")))
			}
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			var batches []ResultBatch
			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
				batches = append(batches, batch)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			send := func(event runtimeTestTrade) {
				t.Helper()
				if err := engine.SendEvent(context.Background(), event); err != nil {
					t.Fatal(err)
				}
			}
			send(runtimeTestTrade{Symbol: "A", Price: 1})
			send(runtimeTestTrade{Symbol: "B", Price: 2})
			send(runtimeTestTrade{Symbol: "A", Price: 9})
			if len(batches) != 2 || len(batches[0].New) != 1 || len(batches[1].New) != 1 {
				t.Fatalf("insert-only merge batches = %#v", batches)
			}
			for index, expected := range []struct {
				symbol string
				price  float64
			}{{symbol: "A", price: 1}, {symbol: "B", price: 2}} {
				event, ok := batches[index].New[0].Event()
				if !ok || event.Get("symbol").Any() != expected.symbol || event.Get("price").Any() != expected.price {
					t.Fatalf("insert-only merge result[%d] = %#v", index, batches[index].New[0])
				}
			}
			if testCase.namedWindow {
				window, ok := engine.NamedWindow(targetName)
				if !ok {
					t.Fatal("merge insert-only named window is missing")
				}
				events, snapshotErr := window.Snapshot(context.Background())
				if snapshotErr != nil || len(events) != 2 || events[0].Get("price").Any() != float64(1) || events[1].Get("price").Any() != float64(2) {
					t.Fatalf("insert-only named-window snapshot = %#v, err=%v", events, snapshotErr)
				}
			} else {
				table, ok := engine.Table(targetName)
				if !ok {
					t.Fatal("merge insert-only table is missing")
				}
				rows, snapshotErr := table.Snapshot(context.Background())
				if snapshotErr != nil || len(rows) != 2 || rows[0].Get("price").Any() != float64(1) || rows[1].Get("price").Any() != float64(2) {
					t.Fatalf("insert-only table snapshot = %#v, err=%v", rows, snapshotErr)
				}
			}
		})
	}
}
