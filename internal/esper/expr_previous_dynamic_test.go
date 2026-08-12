package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type dynamicPreviousEvent struct {
	Price  float64 `esper:"price"`
	Offset int     `esper:"offset"`
}

func TestDynamicPreviousExpressionsMatchJavaOffsetAndNullSemantics(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dynamicPreviousEvent](env, "DynamicPreviousEvent"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	input := From[dynamicPreviousEvent](env, "DynamicPreviousEvent").Window(LengthWindow(4))
	price := Field[dynamicPreviousEvent, float64]("price")
	offset := Field[dynamicPreviousEvent, int]("offset")
	query := Select(input,
		Alias("current", price),
		Alias("dynamic", PrevOf[float64](offset, price)),
		Alias("static", Prev[float64](1, price)),
	).Query(StatementName("dynamic-previous"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 5)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "dynamic previous result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []dynamicPreviousEvent{{Price: 10, Offset: 0}, {Price: 20, Offset: 1}, {Price: 30, Offset: 2}, {Price: 40, Offset: 3}, {Price: 50, Offset: 99}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	wantDynamic := []Value{Present(10.0), Present(10.0), Present(10.0), Present(10.0), Null()}
	for index, expected := range wantDynamic {
		if got := rows[index].Get("dynamic"); !got.Equal(expected) {
			t.Fatalf("dynamic previous row %d = %v, want %v", index, got, expected)
		}
	}
	if !rows[0].Get("static").IsNull() || !rows[1].Get("static").Equal(Present(10.0)) || !rows[4].Get("static").Equal(Present(40.0)) {
		t.Fatalf("static previous rows = %#v", rows)
	}

	if got := PrevOf[float64](Literal(-1), price).eval(EvalContext{Event: Event{}}); !got.IsNull() {
		t.Fatalf("negative dynamic offset = %v, want null", got)
	}
	if got := PrevOf[float64](Literal(1.5), price).eval(EvalContext{Event: Event{}}); !got.IsNull() {
		t.Fatalf("fractional dynamic offset = %v, want null", got)
	}
}

func TestDynamicPreviousExpressionsEnterPlanIdentityAndRejectInvalidBuilders(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dynamicPreviousEvent](env, "DynamicPreviousPlan"); err != nil {
		t.Fatal(err)
	}
	input := From[dynamicPreviousEvent](env, "DynamicPreviousPlan")
	price := Field[dynamicPreviousEvent, float64]("price")
	plan, err := env.Build(Select(input, Alias("value", PrevOf[float64](Literal(1), price))).Query(StatementName("previous-plan")))
	if err != nil {
		t.Fatal(err)
	}
	different, err := env.Build(Select(input, Alias("value", PrevOf[float64](Literal(2), price))).Query(StatementName("previous-plan-different")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() == different.Hash() || reflect.DeepEqual(plan.Canonical(), different.Canonical()) {
		t.Fatal("dynamic previous offset did not enter Plan identity")
	}
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "nil-offset", expr: PrevOf[float64](nil, price), want: "offset expression is required"},
		{name: "nil-value", expr: PrevOf[float64](Literal(1), nil), want: "value expression is required"},
		{name: "fractional-type", expr: PrevOf[float64](Literal(1.5), price), want: "must be integral"},
		{name: "bool-type", expr: PrevOf[float64](Literal(true), price), want: "must be integral"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", testCase.expr)).Query(StatementName("previous-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("invalid dynamic previous error = %v, want %q", err, testCase.want)
			}
		})
	}
}
