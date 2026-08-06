package esper

import (
	"context"
	"fmt"
	"testing"
)

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
