package esper

import (
	"context"
	"reflect"
	"testing"
)

type joinOuterCompositeLeft struct {
	ID              int     `esper:"id"`
	P00             string  `esper:"p00"`
	P01             string  `esper:"p01"`
	IntPrimitive    int     `esper:"intPrimitive"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
}

type joinOuterCompositeRight struct {
	ID              int     `esper:"id"`
	P10             string  `esper:"p10"`
	P11             string  `esper:"p11"`
	IntPrimitive    int64   `esper:"intPrimitive"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
}

type joinOuterArrayLeft struct {
	ID    string `esper:"id"`
	Array []int  `esper:"array"`
}

type joinOuterArrayRight struct {
	ID     string `esper:"id"`
	IntOne []int  `esper:"intOne"`
}

type joinOuterRangeValue struct {
	Key   string `esper:"key"`
	Value int    `esper:"value"`
}

type joinOuterRangeBounds struct {
	ID    string `esper:"id"`
	Key   string `esper:"key"`
	Start int    `esper:"start"`
	End   int    `esper:"end"`
}

func joinOuterRow(t *testing.T, result Result) Row {
	t.Helper()
	row, ok := result.Row()
	if !ok {
		t.Fatalf("outer join result is not a row: %#v", result)
	}
	return row
}

func TestTwoStreamOuterJoinVariantsMatchesEsper(t *testing.T) {
	for _, test := range []struct {
		name string
		kind JoinKind
	}{
		{name: "left", kind: JoinLeftOuter},
		{name: "right", kind: JoinRightOuter},
		{name: "full", kind: JoinFullOuter},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[joinOrder](env, "JoinOuterOrder"); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterStruct[joinPayment](env, "JoinOuterPayment"); err != nil {
				t.Fatal(err)
			}

			join := Join(
				From[joinOrder](env, "JoinOuterOrder"),
				From[joinPayment](env, "JoinOuterPayment"),
				OnEqual(
					Field[joinOrder, string]("orderID"),
					Field[joinPayment, string]("orderID"),
				),
			)
			switch test.kind {
			case JoinLeftOuter:
				join = join.LeftOuter()
			case JoinRightOuter:
				join = join.RightOuter()
			case JoinFullOuter:
				join = join.FullOuter()
			}
			plan, err := env.Build(join.Select(
				SelectLeft("leftKey", JoinField[string](0, "orderID")),
				SelectRight("rightAmount", JoinField[float64](1, "amount")),
			).Query(StatementName("join-two-outer-"+test.name), WithOldStream()))
			if err != nil {
				t.Fatal(err)
			}
			schema, ok := plan.ResultSchema()
			if !ok {
				t.Fatal("join result schema is missing")
			}
			leftField, leftOK := schema.Field("leftKey")
			rightField, rightOK := schema.Field("rightAmount")
			if !leftOK || !rightOK || leftField.Type != reflect.TypeOf("") || rightField.Type != reflect.TypeOf(float64(0)) {
				t.Fatalf("join result schema = %#v", schema.Fields())
			}

			engine := env.NewEngine()
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			defer deployment.Undeploy(context.Background())
			var batches []ResultBatch
			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
				batches = append(batches, batch)
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			sendOrder := func(id string) {
				t.Helper()
				if err := engine.Send(context.Background(), "JoinOuterOrder", joinOrder{OrderID: id, Symbol: "order-" + id}); err != nil {
					t.Fatal(err)
				}
			}
			sendPayment := func(id string) {
				t.Helper()
				if err := engine.Send(context.Background(), "JoinOuterPayment", joinPayment{OrderID: id, Amount: float64(len(id))}); err != nil {
					t.Fatal(err)
				}
			}
			assertBatch := func(index, oldCount, newCount int) Row {
				t.Helper()
				if index >= len(batches) || len(batches[index].Old) != oldCount || len(batches[index].New) != newCount {
					t.Fatalf("%s outer batches = %#v", test.name, batches)
				}
				if newCount == 0 {
					return Row{}
				}
				row, ok := batches[index].New[0].Row()
				if !ok {
					t.Fatalf("%s outer result is not a row", test.name)
				}
				return row
			}

			switch test.kind {
			case JoinLeftOuter:
				sendOrder("A")
				row := assertBatch(0, 0, 1)
				if row.Get("leftKey").Any() != "A" || !row.Get("rightAmount").IsNull() {
					t.Fatalf("left outer unmatched row = %#v", row.AsMap())
				}
				sendPayment("B")
				if len(batches) != 1 {
					t.Fatalf("left outer right-only event emitted = %#v", batches)
				}
				sendPayment("A")
				row = assertBatch(1, 0, 1)
				if row.Get("leftKey").Any() != "A" || row.Get("rightAmount").Any() != float64(1) {
					t.Fatalf("left outer matched row = %#v", row.AsMap())
				}
			case JoinRightOuter:
				sendPayment("A")
				row := assertBatch(0, 0, 1)
				if !row.Get("leftKey").IsNull() || row.Get("rightAmount").Any() != float64(1) {
					t.Fatalf("right outer unmatched row = %#v", row.AsMap())
				}
				sendOrder("B")
				if len(batches) != 1 {
					t.Fatalf("right outer left-only event emitted = %#v", batches)
				}
				sendOrder("A")
				row = assertBatch(1, 0, 1)
				if row.Get("leftKey").Any() != "A" || row.Get("rightAmount").Any() != float64(1) {
					t.Fatalf("right outer matched row = %#v", row.AsMap())
				}
			case JoinFullOuter:
				sendOrder("A")
				row := assertBatch(0, 0, 1)
				if row.Get("leftKey").Any() != "A" || !row.Get("rightAmount").IsNull() {
					t.Fatalf("full outer left-only row = %#v", row.AsMap())
				}
				sendPayment("B")
				row = assertBatch(1, 0, 1)
				if !row.Get("leftKey").IsNull() || row.Get("rightAmount").Any() != float64(1) {
					t.Fatalf("full outer right-only row = %#v", row.AsMap())
				}
				sendPayment("A")
				row = assertBatch(2, 0, 1)
				if row.Get("leftKey").Any() != "A" || row.Get("rightAmount").Any() != float64(1) {
					t.Fatalf("full outer matched row = %#v", row.AsMap())
				}
			}
		})
	}
}

func TestTwoStreamOuterCompositeAndCoercionMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOuterCompositeLeft](env, "JoinOuterCompositeLeft"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinOuterCompositeRight](env, "JoinOuterCompositeRight"); err != nil {
		t.Fatal(err)
	}
	left := From[joinOuterCompositeLeft](env, "JoinOuterCompositeLeft")
	right := From[joinOuterCompositeRight](env, "JoinOuterCompositeRight")
	query := Join(left, right,
		OnEqual(Field[joinOuterCompositeLeft, string]("p00"), Field[joinOuterCompositeRight, string]("p10")),
		OnEqual(Field[joinOuterCompositeLeft, string]("p01"), Field[joinOuterCompositeRight, string]("p11")),
		OnEqual(Field[joinOuterCompositeLeft, int]("intPrimitive"), Field[joinOuterCompositeRight, float64]("doublePrimitive")),
		OnEqual(Field[joinOuterCompositeLeft, float64]("doublePrimitive"), Field[joinOuterCompositeRight, int64]("intPrimitive")),
	).LeftOuter().Select(
		SelectFrom(0, "leftID", JoinField[int](0, "id")),
		SelectFrom(1, "rightID", JoinField[int](1, "id")),
	).Query(StatementName("join-two-outer-composite"), WithOldStream())
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendLeft := func(event joinOuterCompositeLeft) {
		t.Helper()
		if err := engine.Send(context.Background(), "JoinOuterCompositeLeft", event); err != nil {
			t.Fatal(err)
		}
	}
	sendRight := func(event joinOuterCompositeRight) {
		t.Helper()
		if err := engine.Send(context.Background(), "JoinOuterCompositeRight", event); err != nil {
			t.Fatal(err)
		}
	}
	sendLeft(joinOuterCompositeLeft{ID: 1, P00: "A_1", P01: "B_1", IntPrimitive: 10, DoublePrimitive: 20})
	if len(batches) != 1 || len(batches[0].New) != 1 || !joinOuterRow(t, batches[0].New[0]).Get("rightID").IsNull() {
		t.Fatalf("composite unmatched left = %#v", batches)
	}
	sendRight(joinOuterCompositeRight{ID: 2, P10: "A_2", P11: "B_1", IntPrimitive: 10, DoublePrimitive: 20})
	if len(batches) != 1 {
		t.Fatalf("non-matching composite right changed output = %#v", batches)
	}
	sendRight(joinOuterCompositeRight{ID: 3, P10: "A_1", P11: "B_1", IntPrimitive: 20, DoublePrimitive: 10})
	if len(batches) != 2 || len(batches[1].Old) != 0 || len(batches[1].New) != 1 {
		t.Fatalf("composite/coercion transition = %#v", batches)
	}
	row := joinOuterRow(t, batches[1].New[0])
	if row.Get("leftID").Any() != 1 || row.Get("rightID").Any() != 3 {
		t.Fatalf("composite/coercion row = %#v", row.AsMap())
	}
}

func TestTwoStreamOuterRangeAndArrayMatchesEsper(t *testing.T) {
	t.Run("range-and-where", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[joinOuterRangeValue](env, "JoinOuterRangeValue"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[joinOuterRangeBounds](env, "JoinOuterRangeBounds"); err != nil {
			t.Fatal(err)
		}
		query := Join(
			From[joinOuterRangeValue](env, "JoinOuterRangeValue"),
			From[joinOuterRangeBounds](env, "JoinOuterRangeBounds"),
			OnEqual(Field[joinOuterRangeValue, string]("key"), Field[joinOuterRangeBounds, string]("key")),
		).FullOuter().Select(
			SelectFrom(0, "value", JoinField[int](0, "value")),
			SelectFrom(1, "start", JoinField[int](1, "start")),
			SelectFrom(1, "end", JoinField[int](1, "end")),
		).Where(Between[int](JoinField[int](0, "value"), JoinField[int](1, "start"), JoinField[int](1, "end"))).Query(
			StatementName("join-two-outer-range"), WithOldStream(),
		)
		plan, err := env.Build(query)
		if err != nil {
			t.Fatal(err)
		}
		engine := env.NewEngine()
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer deployment.Undeploy(context.Background())
		var batches []ResultBatch
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			batches = append(batches, batch)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		sendValue := func(value int) {
			t.Helper()
			if err := engine.Send(context.Background(), "JoinOuterRangeValue", joinOuterRangeValue{Key: "K1", Value: value}); err != nil {
				t.Fatal(err)
			}
		}
		sendBounds := func(start, end int) {
			t.Helper()
			if err := engine.Send(context.Background(), "JoinOuterRangeBounds", joinOuterRangeBounds{ID: "R", Key: "K1", Start: start, End: end}); err != nil {
				t.Fatal(err)
			}
		}
		sendValue(10)
		sendBounds(20, 30)
		if len(batches) != 0 {
			t.Fatalf("range outer null-side rows were not filtered = %#v", batches)
		}
		sendValue(30)
		if len(batches) != 1 || len(batches[0].New) != 1 || joinOuterRow(t, batches[0].New[0]).Get("value").Any() != 30 {
			t.Fatalf("range outer in-bound row = %#v", batches)
		}
		sendBounds(35, 42)
		if len(batches) != 1 {
			t.Fatalf("range outer out-of-bound replacement emitted = %#v", batches)
		}
		sendValue(40)
		if len(batches) != 2 || len(batches[1].Old) != 0 || joinOuterRow(t, batches[1].New[0]).Get("start").Any() != 35 {
			t.Fatalf("range outer second match = %#v", batches)
		}
	})

	t.Run("array-full-outer", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[joinOuterArrayLeft](env, "JoinOuterArrayLeft"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[joinOuterArrayRight](env, "JoinOuterArrayRight"); err != nil {
			t.Fatal(err)
		}
		query := Join(
			From[joinOuterArrayLeft](env, "JoinOuterArrayLeft"),
			From[joinOuterArrayRight](env, "JoinOuterArrayRight"),
			OnEqual(Field[joinOuterArrayLeft, []int]("array"), Field[joinOuterArrayRight, []int]("intOne")),
		).FullOuter().Select(
			SelectFrom(0, "leftID", JoinField[string](0, "id")),
			SelectFrom(1, "rightID", JoinField[string](1, "id")),
		).Query(StatementName("join-two-outer-array"), WithOldStream())
		plan, err := env.Build(query)
		if err != nil {
			t.Fatal(err)
		}
		engine := env.NewEngine()
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer deployment.Undeploy(context.Background())
		var batches []ResultBatch
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			batches = append(batches, batch)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		sendLeft := func(id string, array []int) {
			t.Helper()
			if err := engine.Send(context.Background(), "JoinOuterArrayLeft", joinOuterArrayLeft{ID: id, Array: array}); err != nil {
				t.Fatal(err)
			}
		}
		sendRight := func(id string, array []int) {
			t.Helper()
			if err := engine.Send(context.Background(), "JoinOuterArrayRight", joinOuterArrayRight{ID: id, IntOne: array}); err != nil {
				t.Fatal(err)
			}
		}
		sendLeft("IA1", []int{1, 2})
		sendRight("MA1", []int{3, 4})
		sendRight("MA2", []int{1, 2})
		if len(batches) != 3 || len(batches[2].Old) != 0 || joinOuterRow(t, batches[2].New[0]).Get("rightID").Any() != "MA2" {
			t.Fatalf("array full outer first match = %#v", batches)
		}
		sendLeft("IA3", []int{3, 4})
		if len(batches) != 4 || len(batches[3].Old) != 0 || joinOuterRow(t, batches[3].New[0]).Get("leftID").Any() != "IA3" || joinOuterRow(t, batches[3].New[0]).Get("rightID").Any() != "MA1" {
			t.Fatalf("array full outer second match = %#v", batches)
		}
	})
}
