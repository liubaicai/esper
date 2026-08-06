package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type scalarMinMaxEvent struct {
	Long  *int64 `esper:"long_boxed"`
	Int   *int   `esper:"int_boxed"`
	Short *int16 `esper:"short_boxed"`
	Text  string `esper:"text"`
	Flag  bool   `esper:"flag"`
}

func TestScalarMinMaxExpressionsMatchJavaNullAndPromotionSemantics(t *testing.T) {
	longValue := int64(10)
	intValue := 20
	shortValue := int16(4)
	if got := MinOf[int64](Literal(longValue), Literal(intValue), Literal(shortValue)).eval(EvalContext{}); !got.Equal(Present(int64(4))) {
		t.Fatalf("mixed scalar min = %v, want 4", got)
	}
	if got := MaxOf[int64](Literal(longValue), Literal(intValue), Literal(shortValue)).eval(EvalContext{}); !got.Equal(Present(int64(20))) {
		t.Fatalf("mixed scalar max = %v, want 20", got)
	}
	if got := MinOf[string](Literal("b"), Literal("a"), Literal("c")).eval(EvalContext{}); !got.Equal(Present("a")) {
		t.Fatalf("string scalar min = %v, want a", got)
	}
	if got := MaxOf[string](Literal("b"), Literal("a"), Literal("c")).eval(EvalContext{}); !got.Equal(Present("c")) {
		t.Fatalf("string scalar max = %v, want c", got)
	}
	if got := MaxOf[int64](Literal(int64(10)), NullLiteral[int](), Literal(int16(2))).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("scalar max with null = %v, want null", got)
	}
	var nilInt *int
	if got := MinOf[int64](Literal(int64(10)), Literal(nilInt)).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("scalar min with typed nil = %v, want null", got)
	}
}

func TestScalarMinMaxExpressionsBuildTypedLiveProjectionAndPlanIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[scalarMinMaxEvent](env, "ScalarMinMaxEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[scalarMinMaxEvent](env, "ScalarMinMaxEvent")
	longField := Field[scalarMinMaxEvent, *int64]("long_boxed")
	intField := Field[scalarMinMaxEvent, *int]("int_boxed")
	shortField := Field[scalarMinMaxEvent, *int16]("short_boxed")
	textField := Field[scalarMinMaxEvent, string]("text")
	query := Select(input,
		Alias("minimum", MinOf[int64](longField, intField, shortField)),
		Alias("maximum", MaxOf[int64](longField, intField, shortField)),
		Alias("text_minimum", MinOf[string](textField, Literal("m"))),
	).Query(StatementName("scalar-minmax"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !reflect.DeepEqual(plan.Canonical(), samePlan.Canonical()) {
		t.Fatalf("equivalent scalar min/max plans differ: %s != %s", plan.Hash(), samePlan.Hash())
	}
	different, err := env.Build(Select(input,
		Alias("minimum", MinOf[int64](longField, intField)),
		Alias("maximum", MaxOf[int64](longField, intField, shortField)),
		Alias("text_minimum", MinOf[string](textField, Literal("m"))),
	).Query(StatementName("scalar-minmax")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() == different.Hash() {
		t.Fatal("operand count did not enter scalar min/max Plan identity")
	}
	schema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("scalar min/max result schema is missing")
	}
	for _, name := range []string{"minimum", "maximum"} {
		field, ok := schema.Field(name)
		if !ok || field.Type != typeOf[int64]() {
			t.Fatalf("scalar min/max field %q = %#v, want int64", name, field)
		}
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "scalar min/max result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	longValue := int64(10)
	intValue := 20
	shortValue := int16(4)
	if err := engine.SendEvent(context.Background(), scalarMinMaxEvent{Long: &longValue, Int: &intValue, Short: &shortValue, Text: "z"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("scalar min/max rows = %d, want 1", len(rows))
	}
	if !rows[0].Get("minimum").Equal(Present(int64(4))) ||
		!rows[0].Get("maximum").Equal(Present(int64(20))) ||
		!rows[0].Get("text_minimum").Equal(Present("m")) {
		t.Fatalf("scalar min/max row = %#v", rows[0])
	}
	if err := engine.SendEvent(context.Background(), scalarMinMaxEvent{Long: &longValue, Int: nil, Short: &shortValue, Text: "z"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !rows[1].Get("minimum").IsNull() || !rows[1].Get("maximum").IsNull() {
		t.Fatalf("scalar min/max null row = %#v", rows)
	}
}

func TestScalarMinMaxExpressionsRejectInvalidBuilders(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[scalarMinMaxEvent](env, "ScalarMinMaxInvalid"); err != nil {
		t.Fatal(err)
	}
	input := From[scalarMinMaxEvent](env, "ScalarMinMaxInvalid")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "too-few", expr: MinOf[int](Literal(1)), want: "at least two"},
		{name: "nil-operand", expr: MaxOf[int](Literal(1), nil), want: "operand 1"},
		{name: "unordered", expr: MinOf[int](Literal(1), Literal(true)), want: "must be ordered"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", testCase.expr)).Query(StatementName("scalar-minmax-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("invalid scalar min/max error = %v, want %q", err, testCase.want)
			}
		})
	}
}
