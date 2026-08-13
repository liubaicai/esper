package esper

import (
	"context"
	"reflect"
	"testing"
)

type joinLengthWindowOutputOrder struct {
	OrderID string  `esper:"orderId"`
	Price   float64 `esper:"price"`
}

type joinLengthWindowOutputPayment struct {
	OrderID string  `esper:"orderId"`
	Amount  float64 `esper:"amount"`
}

// TestJoinLengthWindowOutputListenerAndIteratorMatchJava mirrors the shared
// join-length-window parity scenario. The Java oracle (Esper 9.0.0 commit
// 9e1b9f1cc9117fea4bf33ab043762c045d73839c) keeps a non-aggregate join
// iterator live even for ` + "`output every 2 events`" + `: pending deltas are
// visible to statement.iterator(), while listener output is buffered until
// two accepted rows have accumulated and then delivered in ORDER BY order.
func TestJoinLengthWindowOutputListenerAndIteratorMatchJava(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinLengthWindowOutputOrder](env, "OrderEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinLengthWindowOutputPayment](env, "PaymentEvent"); err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	defer func() { _ = engine.Close(context.Background()) }()

	build := func(t *testing.T, withOutput bool) *Statement {
		t.Helper()
		orders := From[joinLengthWindowOutputOrder](env, "OrderEvent").Window(LengthWindow(3))
		payments := From[joinLengthWindowOutputPayment](env, "PaymentEvent").Window(LengthWindow(3))
		query := Join(
			orders,
			payments,
			OnEqual(
				Field[joinLengthWindowOutputOrder, string]("orderId"),
				Field[joinLengthWindowOutputPayment, string]("orderId"),
			),
		).Select(
			SelectLeft("orderId", Field[joinLengthWindowOutputOrder, string]("orderId")),
			SelectRight("amount", Field[joinLengthWindowOutputPayment, float64]("amount")),
		)
		options := []QueryOption{
			StatementName("s0"),
			OrderBy(
				Ascending(JoinField[string](0, "orderId")),
				Ascending(JoinField[float64](1, "amount")),
			),
		}
		if withOutput {
			options = append(options, WithOutput(OutputEvery(2)))
		}
		plan, err := env.Build(query.Query(options...))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = deployment.Undeploy(context.Background()) })
		return deployment.Statements()[0]
	}

	collect := func(t *testing.T, statement *Statement) *[][]map[string]any {
		t.Helper()
		batches := &[][]map[string]any{}
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			rows := make([]map[string]any, 0, len(batch.New))
			for _, result := range batch.New {
				row, ok := result.Row()
				if !ok {
					t.Fatalf("join output is not a row: %#v", result)
				}
				rows = append(rows, map[string]any{
					"orderId": row.Get("orderId").Any(),
					"amount":  row.Get("amount").Any(),
				})
			}
			*batches = append(*batches, rows)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return batches
	}

	snapshotRows := func(t *testing.T, statement *Statement) []map[string]any {
		t.Helper()
		snapshot, err := statement.SnapshotWithSelector(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		rows := make([]map[string]any, 0, len(snapshot.Batch.New))
		for _, result := range snapshot.Batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("join snapshot is not a row: %#v", result)
			}
			rows = append(rows, map[string]any{
				"orderId": row.Get("orderId").Any(),
				"amount":  row.Get("amount").Any(),
			})
		}
		return rows
	}

	assertRows := func(t *testing.T, got []map[string]any, want ...map[string]any) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("rows = %#v, want %#v", got, want)
		}
		for index := range want {
			for key, expected := range want[index] {
				actual, exists := got[index][key]
				if !exists || !reflect.DeepEqual(actual, expected) {
					t.Fatalf("rows[%d][%s] = %#v, want %#v (rows %#v)", index, key, actual, expected, got)
				}
			}
		}
	}

	row := func(orderID string, amount float64) map[string]any {
		return map[string]any{"orderId": orderID, "amount": amount}
	}

	sendOrder := func(t *testing.T, orderID string, price float64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), joinLengthWindowOutputOrder{OrderID: orderID, Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	sendPayment := func(t *testing.T, orderID string, amount float64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), joinLengthWindowOutputPayment{OrderID: orderID, Amount: amount}); err != nil {
			t.Fatal(err)
		}
	}

	plain := build(t, false)
	plainBatches := collect(t, plain)
	sendOrder(t, "O1", 10)
	sendPayment(t, "O1", 10)
	sendOrder(t, "O2", 20)
	sendPayment(t, "O1", 20)
	assertRows(t, snapshotRows(t, plain), row("O1", 10), row("O1", 20))
	sendPayment(t, "O1", 40)
	sendOrder(t, "O3", 30)
	sendPayment(t, "O2", 5)
	assertRows(t, snapshotRows(t, plain), row("O1", 20), row("O1", 40), row("O2", 5))
	if len(*plainBatches) != 4 {
		t.Fatalf("plain batches = %#v", *plainBatches)
	}
	assertRows(t, (*plainBatches)[0], row("O1", 10))
	assertRows(t, (*plainBatches)[1], row("O1", 20))
	assertRows(t, (*plainBatches)[2], row("O1", 40))
	assertRows(t, (*plainBatches)[3], row("O2", 5))

	every2 := build(t, true)
	every2Batches := collect(t, every2)
	sendOrder(t, "O1", 10)
	assertRows(t, snapshotRows(t, every2))
	sendPayment(t, "O1", 10)
	assertRows(t, snapshotRows(t, every2), row("O1", 10))
	sendOrder(t, "O2", 20)
	assertRows(t, snapshotRows(t, every2), row("O1", 10))
	sendPayment(t, "O1", 20)
	assertRows(t, snapshotRows(t, every2), row("O1", 10), row("O1", 20))
	sendPayment(t, "O2", 5)
	assertRows(t, snapshotRows(t, every2), row("O1", 10), row("O1", 20), row("O2", 5))
	if len(*every2Batches) != 1 {
		t.Fatalf("every-2 batches = %#v", *every2Batches)
	}
	assertRows(t, (*every2Batches)[0], row("O1", 10), row("O1", 20))
}
