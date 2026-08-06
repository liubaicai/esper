package esper

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type constructExpressionValue struct {
	Label string `esper:"label"`
	Value int    `esper:"value"`
}

type constructExpressionEvent struct {
	Label string `esper:"label"`
	Value int    `esper:"value"`
	Size  int    `esper:"size"`
}

func TestConstructExpressionsMatchJavaFactoryAndArraySemantics(t *testing.T) {
	factory := ValueConstructor[constructExpressionValue](func(values []Value) (constructExpressionValue, error) {
		if len(values) != 2 {
			return constructExpressionValue{}, errors.New("wrong arity")
		}
		label, _ := As[string](values[0])
		value, _ := As[int](values[1])
		return constructExpressionValue{Label: label, Value: value}, nil
	})
	if got := Construct[constructExpressionValue]("value", factory, Literal("A"), Literal(2)).eval(EvalContext{}); !got.Equal(Present(constructExpressionValue{Label: "A", Value: 2})) {
		t.Fatalf("constructed value = %v, want A/2", got)
	}
	if got := Construct[constructExpressionValue]("value", factory, NullLiteral[string](), Literal(2)).eval(EvalContext{}); !got.Equal(Present(constructExpressionValue{Label: "", Value: 2})) {
		t.Fatalf("constructor Null argument = %v, want zero string and 2", got)
	}
	failing := ValueConstructor[constructExpressionValue](func([]Value) (constructExpressionValue, error) {
		return constructExpressionValue{}, errors.New("factory failed")
	})
	if got := Construct[constructExpressionValue]("failing", failing).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("constructor error = %v, want null", got)
	}
	if got := MakeArray[int](Literal(3)).eval(EvalContext{}); !got.Equal(Present([]int{0, 0, 0})) {
		t.Fatalf("new array = %v, want three zeroes", got)
	}
	if got := MakeArray2D[int](Literal(2), Literal(2)).eval(EvalContext{}); !got.Equal(Present([][]int{{0, 0}, {0, 0}})) {
		t.Fatalf("two-dimensional new array = %v, want 2x2 zeroes", got)
	}
	if got := MakeArray[int](Literal(-1)).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("negative array dimension = %v, want null", got)
	}
}

func TestConstructExpressionsBuildLiveProjectionAndPlanIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[constructExpressionEvent](env, "ConstructParityEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[constructExpressionEvent](env, "ConstructParityEvent")
	label := Field[constructExpressionEvent, string]("label")
	value := Field[constructExpressionEvent, int]("value")
	size := Field[constructExpressionEvent, int]("size")
	factory := ValueConstructor[constructExpressionValue](func(values []Value) (constructExpressionValue, error) {
		if len(values) != 2 {
			return constructExpressionValue{}, errors.New("wrong arity")
		}
		labelValue, _ := As[string](values[0])
		intValue, _ := As[int](values[1])
		return constructExpressionValue{Label: labelValue, Value: intValue}, nil
	})
	query := Select(input,
		Alias("constructed", Construct[constructExpressionValue]("value", factory, label, value)),
		Alias("array", MakeArray[int](size)),
		Alias("array2d", MakeArray2D[int](size, Literal(2))),
	).Query(StatementName("construct-parity"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !reflect.DeepEqual(plan.Canonical(), samePlan.Canonical()) {
		t.Fatalf("equivalent constructor plans differ: %s != %s", plan.Hash(), samePlan.Hash())
	}
	different, err := env.Build(Select(input,
		Alias("constructed", Construct[constructExpressionValue]("other-value", factory, label, value)),
		Alias("array", MakeArray[int](size)),
		Alias("array2d", MakeArray2D[int](size, Literal(2))),
	).Query(StatementName("construct-parity")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() == different.Hash() {
		t.Fatal("constructor name did not enter Plan identity")
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("constructor result schema is missing")
	}
	for name, typ := range map[string]reflect.Type{
		"constructed": reflect.TypeOf(constructExpressionValue{}),
		"array":       reflect.TypeOf([]int{}),
		"array2d":     reflect.TypeOf([][]int{}),
	} {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != typ {
			t.Fatalf("constructor result %q = %#v, want %v", name, field, typ)
		}
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 1)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "constructor result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), constructExpressionEvent{Label: "A", Value: 2, Size: 3}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("constructor rows = %d, want 1", len(rows))
	}
	if got := rows[0].Get("constructed").Any(); !reflect.DeepEqual(got, constructExpressionValue{Label: "A", Value: 2}) {
		t.Fatalf("constructed row = %#v", got)
	}
	if got := rows[0].Get("array").Any(); !reflect.DeepEqual(got, []int{0, 0, 0}) {
		t.Fatalf("array row = %#v", got)
	}
	if got := rows[0].Get("array2d").Any(); !reflect.DeepEqual(got, [][]int{{0, 0}, {0, 0}, {0, 0}}) {
		t.Fatalf("array2d row = %#v", got)
	}
}

func TestConstructExpressionsRejectInvalidBuilders(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[constructExpressionEvent](env, "ConstructInvalidEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[constructExpressionEvent](env, "ConstructInvalidEvent")
	factory := ValueConstructor[constructExpressionValue](func([]Value) (constructExpressionValue, error) {
		return constructExpressionValue{}, nil
	})
	var nilFactory ValueConstructor[constructExpressionValue]
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "blank-name", expr: Construct[constructExpressionValue](" ", factory), want: "name is required"},
		{name: "nil-factory", expr: Construct[constructExpressionValue]("value", nilFactory), want: "requires a factory"},
		{name: "nil-argument", expr: Construct[constructExpressionValue]("value", factory, nil), want: "argument 0 is required"},
		{name: "nil-length", expr: MakeArray[int](nil), want: "length expression is required"},
		{name: "fractional-length", expr: MakeArray[int](Literal(1.5)), want: "length must be integral"},
		{name: "bool-length", expr: MakeArray[int](Literal(true)), want: "length must be integral"},
		{name: "nested-invalid", expr: Construct[constructExpressionValue]("value", factory, EqualOf(Literal(true), Literal(1))), want: "not compatible"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", testCase.expr)).Query(StatementName("construct-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("invalid constructor error = %v, want %q", err, testCase.want)
			}
		})
	}
}
