package esper

import (
	"context"
	"reflect"
	"testing"
)

type logicParityEvent struct {
	IntValue      int    `esper:"int_value"`
	BoolPrimitive bool   `esper:"bool_primitive"`
	BoolBoxed     *bool  `esper:"bool_boxed"`
	Symbol        string `esper:"symbol"`
}

func TestLogicalExpressionsMatchJavaThreeValuedTruthTables(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[logicParityEvent](env, "LogicParityEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[logicParityEvent](env, "LogicParityEvent")
	intValue := Field[logicParityEvent, int]("int_value")
	primitive := Field[logicParityEvent, bool]("bool_primitive")
	boxed := Property[bool](EventValue[logicParityEvent](), "BoolBoxed")
	plan, err := env.Build(Select(input,
		Alias("combined_or", Or(Equal[int](intValue, Literal(1)), Equal[int](intValue, Literal(2)))),
		Alias("combined_and", And(Greater[int](intValue, Literal(0)), Less[int](intValue, Literal(3)))),
		Alias("combined_not", Not(Equal[int](intValue, Literal(2)))),
		Alias("null_and", And(NullLiteral[bool](), NullLiteral[bool]())),
		Alias("bool_and", And(primitive, boxed)),
		Alias("bool_or", Or(primitive, boxed)),
		Alias("not_boxed", Not(boxed)),
	).Query(StatementName("logical-truth-table")))
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(plan.Query())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !reflect.DeepEqual(plan.Canonical(), samePlan.Canonical()) {
		t.Fatalf("equivalent logical plans differ: %s != %s", plan.Hash(), samePlan.Hash())
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("logical result schema is missing")
	}
	for _, name := range []string{"combined_or", "combined_and", "combined_not", "null_and", "bool_and", "bool_or", "not_boxed"} {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != typeOf[bool]() {
			t.Fatalf("logical field %q = %#v, want bool", name, field)
		}
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 6)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "logical result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	trueValue := true
	probeSchema, err := StructSchema[logicParityEvent]("LogicProbe")
	if err != nil {
		t.Fatal(err)
	}
	probeEvent, err := newEvent(probeSchema, logicParityEvent{BoolBoxed: &trueValue}, engine.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := boolValue(boxed.eval(EvalContext{Event: probeEvent})); !ok || !got {
		t.Fatalf("boxed property = %v, want true", boxed.eval(EvalContext{Event: probeEvent}))
	}
	events := []logicParityEvent{
		{IntValue: 1, BoolPrimitive: true, BoolBoxed: &trueValue},
		{IntValue: 2, BoolPrimitive: false, BoolBoxed: func() *bool { value := false; return &value }()},
		{IntValue: 3, BoolPrimitive: false},
		{BoolPrimitive: true},
		{BoolPrimitive: true, BoolBoxed: func() *bool { value := false; return &value }()},
		{BoolPrimitive: false, BoolBoxed: &trueValue},
	}
	for _, event := range events {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != len(events) {
		t.Fatalf("logical rows = %d, want %d", len(rows), len(events))
	}
	want := []struct {
		combinedOr, combinedAnd, combinedNot Value
		boolAnd, boolOr, notBoxed            Value
	}{
		{Present(true), Present(true), Present(true), Present(true), Present(true), Present(false)},
		{Present(true), Present(true), Present(false), Present(false), Present(false), Present(true)},
		{Present(false), Present(false), Present(true), Present(false), Null(), Null()},
		{Present(false), Present(false), Present(true), Null(), Present(true), Null()},
		{Present(false), Present(false), Present(true), Present(false), Present(true), Present(true)},
		{Present(false), Present(false), Present(true), Present(false), Present(true), Present(false)},
	}
	for index, row := range rows {
		expected := want[index]
		if !row.Get("combined_or").Equal(expected.combinedOr) ||
			!row.Get("combined_and").Equal(expected.combinedAnd) ||
			!row.Get("combined_not").Equal(expected.combinedNot) ||
			!row.Get("null_and").IsNull() ||
			!row.Get("bool_and").Equal(expected.boolAnd) ||
			!row.Get("bool_or").Equal(expected.boolOr) ||
			!row.Get("not_boxed").Equal(expected.notBoxed) {
			t.Fatalf("logical row %d = %#v, want %#v", index, row, expected)
		}
	}
}

func TestLogicalExpressionVariableUpdatesMatchEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[logicParityEvent](env, "LogicVariableEvent"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("thing", "Hello World"); err != nil {
		t.Fatal(err)
	}
	input := From[logicParityEvent](env, "LogicVariableEvent")
	expression := Not(Contains(VariableRef[string]("thing"), Field[logicParityEvent, string]("symbol")))
	plan, err := env.Build(Select(input, Alias("matches", expression)).Query(StatementName("logical-variable")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	values := make([]Value, 0, 4)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "logical variable result is not a row")
			}
			values = append(values, row.Get("matches"))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"World", "x"} {
		if err := engine.SendEvent(context.Background(), logicParityEvent{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.SetVariable(context.Background(), "thing", "5 x 5"); err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"World", "x"} {
		if err := engine.SendEvent(context.Background(), logicParityEvent{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	want := []Value{Present(false), Present(true), Present(true), Present(false)}
	if len(values) != len(want) {
		t.Fatalf("logical variable rows = %d, want %d", len(values), len(want))
	}
	for index := range want {
		if !values[index].Equal(want[index]) {
			t.Fatalf("logical variable row %d = %v, want %v", index, values[index], want[index])
		}
	}
}
