package esper

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

type exprDefineCaseNewEvent struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func exprDefineCaseNewMap(values []Value) (map[string]any, error) {
	if len(values) != 2 || !values[0].IsPresent() || !values[1].IsPresent() {
		return nil, fmt.Errorf("case-new map requires col1 and col2")
	}
	col1, col1OK := values[0].Any().(string)
	col2, col2OK := values[1].Any().(int)
	if !col1OK || !col2OK {
		return nil, fmt.Errorf("case-new map requires string and int columns")
	}
	return map[string]any{"col1": col1, "col2": col2}, nil
}

// TestExprDefineCaseNewMultiReturnNoElseParity covers Java's
// ExprDefineCaseNewMultiReturnNoElse. An unmatched CASE has a null map result;
// matching branches return an anonymous map that remains readable after
// routing through an intermediate stream.
func TestExprDefineCaseNewMultiReturnNoElseParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprDefineCaseNewEvent](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "OtherStream", []FieldSpec{
		FieldDef("val0", reflect.TypeOf(map[string]any{})),
	}); err != nil {
		t.Fatal(err)
	}

	parameter := ExpressionParam[exprDefineCaseNewEvent]("x")
	result := CaseWhen[map[string]any](
		Equal[string](Property[string](parameter, "theString"), Literal("A")),
		Construct("expr-define-case-new", exprDefineCaseNewMap, Literal("X"), Literal(10)),
	).When(
		Equal[string](Property[string](parameter, "theString"), Literal("B")),
		Construct("expr-define-case-new", exprDefineCaseNewMap, Literal("Y"), Literal(20)),
	).Build()
	if err := env.DefineExpression("gettotal", result); err != nil {
		t.Fatal(err)
	}

	producerPlan, err := env.Build(Select(From[exprDefineCaseNewEvent](env, "SupportBean"),
		Alias("val0", ExpressionRef[map[string]any](env, "gettotal", EventValue[exprDefineCaseNewEvent]())),
	).InsertInto("OtherStream", StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, ok := producerPlan.ResultSchema()
	if !ok {
		t.Fatal("case-new producer result schema is missing")
	}
	field, ok := resultSchema.Field("val0")
	if !ok || field.Type != reflect.TypeOf(map[string]any{}) {
		t.Fatalf("case-new val0 field = %#v, want map[string]any", field)
	}

	val0 := Field[Event, map[string]any]("val0")
	consumerPlan, err := env.Build(FromAny(env, "OtherStream").Select(
		Alias("c1", Property[string](val0, "col1")),
		Alias("c2", Property[int](val0, "col2")),
	).Query(StatementName("s1")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	producerDeployment, err := engine.Deploy(context.Background(), producerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	producerRows := collectDotRows(t, producerDeployment)
	consumerRows := collectDotRows(t, consumerDeployment)

	tests := []struct {
		event exprDefineCaseNewEvent
		col1  Value
		col2  Value
	}{
		{event: exprDefineCaseNewEvent{TheString: "E1", IntPrimitive: 1}, col1: Null(), col2: Null()},
		{event: exprDefineCaseNewEvent{TheString: "A", IntPrimitive: 2}, col1: Present("X"), col2: Present(10)},
		{event: exprDefineCaseNewEvent{TheString: "B", IntPrimitive: 3}, col1: Present("Y"), col2: Present(20)},
	}
	for index, test := range tests {
		if err := engine.SendEvent(context.Background(), test.event); err != nil {
			t.Fatal(err)
		}
		if len(*producerRows) != index+1 || len(*consumerRows) != index+1 {
			t.Fatalf("case-new row counts producer=%d consumer=%d, want %d", len(*producerRows), len(*consumerRows), index+1)
		}

		producerValue := (*producerRows)[index].Get("val0")
		if !test.col1.IsPresent() {
			if !producerValue.IsNull() {
				t.Fatalf("case-new unmatched val0 = %v, want null", producerValue)
			}
		} else {
			value, ok := producerValue.Any().(map[string]any)
			if !ok || value["col1"] != test.col1.Any() || value["col2"] != test.col2.Any() {
				t.Fatalf("case-new producer val0 = %#v, want col1=%v col2=%v", producerValue.Any(), test.col1, test.col2)
			}
		}
		if !(*consumerRows)[index].Get("c1").Equal(test.col1) || !(*consumerRows)[index].Get("c2").Equal(test.col2) {
			t.Fatalf("case-new consumer row = %#v, want c1=%v c2=%v", (*consumerRows)[index], test.col1, test.col2)
		}
	}
}
