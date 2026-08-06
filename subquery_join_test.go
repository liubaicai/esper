package esper

import (
	"context"
	"testing"
)

type joinSubqueryReference struct {
	OrderID string `esper:"orderID"`
	Value   string `esper:"value"`
}

func TestJoinSelectionCorrelatedSubqueryUsesCurrentOuterSide(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "JoinSubqueryOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "JoinSubqueryPayment"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinSubqueryReference](env, "JoinSubqueryReference"); err != nil {
		t.Fatal(err)
	}
	inner := Select(From[joinSubqueryReference](env, "JoinSubqueryReference")).Window(LastEvent())
	correlated := SubqueryValue[string](
		inner,
		Field[joinSubqueryReference, string]("value"),
		Equal[string](
			Field[joinSubqueryReference, string]("orderID"),
			OuterField[string]("orderID"),
		),
	)
	plan, err := env.Build(Join(
		From[joinOrder](env, "JoinSubqueryOrder"),
		From[joinPayment](env, "JoinSubqueryPayment"),
		OnEqual(Field[joinOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
	).Select(
		SelectLeft("orderID", Field[joinOrder, string]("orderID")),
		SelectLeft("reference", correlated),
	).Query(StatementName("join-correlated-subquery")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("join correlated subquery result = %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "JoinSubqueryReference", joinSubqueryReference{OrderID: "O1", Value: "v1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "JoinSubqueryPayment", joinPayment{OrderID: "O1", Amount: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "JoinSubqueryOrder", joinOrder{OrderID: "O1"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("orderID").Any() != "O1" || rows[0].Get("reference").Any() != "v1" {
		t.Fatalf("join correlated subquery rows = %#v", rows)
	}
}

func TestJoinConditionCorrelatedSubqueryUsesLeftOuterSide(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "JoinConditionSubqueryOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "JoinConditionSubqueryPayment"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinSubqueryReference](env, "JoinConditionSubqueryReference"); err != nil {
		t.Fatal(err)
	}
	inner := Select(From[joinSubqueryReference](env, "JoinConditionSubqueryReference")).Window(KeepAll())
	correlatedExists := SubqueryExists(inner, Equal[string](
		Field[joinSubqueryReference, string]("orderID"),
		OuterField[string]("orderID"),
	))
	condition := AllJoin(
		OnEqual(Field[joinOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
		OnEqual(correlatedExists, Literal(true)),
	)
	plan, err := env.Build(Join(
		From[joinOrder](env, "JoinConditionSubqueryOrder"),
		From[joinPayment](env, "JoinConditionSubqueryPayment"),
		condition,
	).Select(
		SelectLeft("orderID", Field[joinOrder, string]("orderID")),
	).Query(StatementName("join-condition-correlated-subquery")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "JoinConditionSubqueryReference", joinSubqueryReference{OrderID: "O1", Value: "present"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "JoinConditionSubqueryPayment", joinPayment{OrderID: "O2", Amount: 20}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "JoinConditionSubqueryOrder", joinOrder{OrderID: "O2"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("correlated join condition matched wrong key = %#v", rows)
	}
	if err := engine.Send(context.Background(), "JoinConditionSubqueryPayment", joinPayment{OrderID: "O1", Amount: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "JoinConditionSubqueryOrder", joinOrder{OrderID: "O1"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("orderID").Any() != "O1" {
		t.Fatalf("correlated join condition rows = %#v", rows)
	}
}

func TestMultiJoinSelectionCorrelatesSubqueryToAnyOuterSource(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "MultiJoinSubqueryOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "MultiJoinSubqueryPayment"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinShipment](env, "MultiJoinSubqueryShipment"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinSubqueryReference](env, "MultiJoinSubqueryReference"); err != nil {
		t.Fatal(err)
	}
	inner := Select(From[joinSubqueryReference](env, "MultiJoinSubqueryReference")).Window(LastEvent())
	correlated := SubqueryValue[string](
		inner,
		Field[joinSubqueryReference, string]("value"),
		Equal[string](
			Field[joinSubqueryReference, string]("orderID"),
			JoinField[string](1, "orderID"),
		),
	)
	plan, err := env.Build(JoinMany(
		JoinSource(From[joinOrder](env, "MultiJoinSubqueryOrder")),
		JoinSource(From[joinPayment](env, "MultiJoinSubqueryPayment")),
		JoinSource(From[joinShipment](env, "MultiJoinSubqueryShipment")),
	).On(
		OnSourcesEqual(0, Field[joinOrder, string]("orderID"), 1, Field[joinPayment, string]("orderID")),
		OnSourcesEqual(0, Field[joinOrder, string]("orderID"), 2, Field[joinShipment, string]("orderID")),
	).Select(
		SelectFrom(0, "orderID", Field[joinOrder, string]("orderID")),
		SelectFrom(0, "reference", correlated),
	).Query(StatementName("multi-join-correlated-subquery")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "MultiJoinSubqueryReference", joinSubqueryReference{OrderID: "O1", Value: "v1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "MultiJoinSubqueryPayment", joinPayment{OrderID: "O1", Amount: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "MultiJoinSubqueryShipment", joinShipment{OrderID: "O1", Carrier: "carrier"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "MultiJoinSubqueryOrder", joinOrder{OrderID: "O1"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("orderID").Any() != "O1" || rows[0].Get("reference").Any() != "v1" {
		t.Fatalf("multi-join correlated subquery rows = %#v", rows)
	}
}

func TestContextJoinSelectionCorrelatedSubqueryKeepsPartitionLocalState(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "ContextJoinSubqueryOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "ContextJoinSubqueryPayment"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinSubqueryReference](env, "ContextJoinSubqueryReference"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "context-join-subquery", Field[any, string]("orderID")); err != nil {
		t.Fatal(err)
	}
	inner := Select(From[joinSubqueryReference](env, "ContextJoinSubqueryReference")).Window(LastEvent())
	correlated := SubqueryValue[string](
		inner,
		Field[joinSubqueryReference, string]("value"),
		Equal[string](
			Field[joinSubqueryReference, string]("orderID"),
			OuterField[string]("orderID"),
		),
	)
	plan, err := env.Build(Join(
		From[joinOrder](env, "ContextJoinSubqueryOrder"),
		From[joinPayment](env, "ContextJoinSubqueryPayment"),
		OnEqual(
			Field[joinOrder, string]("orderID"),
			Field[joinPayment, string]("orderID"),
		),
	).Select(
		SelectLeft("orderID", Field[joinOrder, string]("orderID")),
		SelectLeft("reference", correlated),
	).Query(StatementName("context-join-correlated-subquery"), WithContext("context-join-subquery")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(source string, value any) {
		t.Helper()
		if err := engine.Send(context.Background(), source, value); err != nil {
			t.Fatal(err)
		}
	}
	send("ContextJoinSubqueryOrder", joinOrder{OrderID: "O1"})
	send("ContextJoinSubqueryReference", joinSubqueryReference{OrderID: "O1", Value: "v1"})
	send("ContextJoinSubqueryPayment", joinPayment{OrderID: "O1", Amount: 1})
	send("ContextJoinSubqueryOrder", joinOrder{OrderID: "O2"})
	send("ContextJoinSubqueryPayment", joinPayment{OrderID: "O2", Amount: 2})
	send("ContextJoinSubqueryReference", joinSubqueryReference{OrderID: "O2", Value: "v2"})
	send("ContextJoinSubqueryPayment", joinPayment{OrderID: "O1", Amount: 3})
	send("ContextJoinSubqueryPayment", joinPayment{OrderID: "O2", Amount: 4})
	if len(rows) != 4 {
		t.Fatalf("context join correlated subquery rows = %#v", rows)
	}
	want := []struct {
		orderID   string
		reference any
	}{
		{orderID: "O1", reference: "v1"},
		{orderID: "O2", reference: nil},
		{orderID: "O1", reference: nil},
		{orderID: "O2", reference: "v2"},
	}
	for index, expected := range want {
		value := rows[index].Get("reference")
		if rows[index].Get("orderID").Any() != expected.orderID || (expected.reference == nil && !value.IsNull()) || (expected.reference != nil && value.Any() != expected.reference) {
			t.Fatalf("context join correlated subquery partition leakage = %#v", rows)
		}
	}
}
