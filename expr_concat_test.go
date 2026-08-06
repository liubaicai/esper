package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type concatParityEvent struct {
	First  *string `esper:"first"`
	Second *string `esper:"second"`
	Third  *string `esper:"third"`
}

func TestConcatExpressionsMatchJavaNullEmptyAndMixedTextSemantics(t *testing.T) {
	if got := Concat(Literal("a"), Literal("b")).eval(EvalContext{}); !got.Equal(Present("ab")) {
		t.Fatalf("basic concat = %v, want ab", got)
	}
	if got := ConcatOf(Literal("a"), Literal("b"), Literal("c")).eval(EvalContext{}); !got.Equal(Present("abc")) {
		t.Fatalf("three-part concat = %v, want abc", got)
	}
	if got := ConcatOf(Literal("a"), Literal(""), Literal("b")).eval(EvalContext{}); !got.Equal(Present("ab")) {
		t.Fatalf("empty-string concat = %v, want ab", got)
	}
	if got := ConcatOf(Literal("a"), NullLiteral[string](), Literal("b")).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("null concat = %v, want null", got)
	}
	if got := ConcatOf(Literal(123), Literal("-"), Literal(4.5)).eval(EvalContext{}); !got.Equal(Present("123-4.5")) {
		t.Fatalf("mixed numeric concat = %v, want 123-4.5", got)
	}
	var nilText *string
	if got := ConcatOf(Literal("a"), Literal(nilText)).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("typed-nil concat = %v, want null", got)
	}
}

func TestConcatExpressionsBuildLiveProjectionAndPlanIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[concatParityEvent](env, "ConcatParityEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[concatParityEvent](env, "ConcatParityEvent")
	first := Field[concatParityEvent, *string]("first")
	second := Field[concatParityEvent, *string]("second")
	third := Field[concatParityEvent, *string]("third")
	query := Select(input,
		Alias("two", ConcatOf(first, second)),
		Alias("three", ConcatOf(first, second, third)),
		Alias("with_separator", ConcatOf(first, Literal("|"), second)),
	).Query(StatementName("concat-parity"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !reflect.DeepEqual(plan.Canonical(), samePlan.Canonical()) {
		t.Fatalf("equivalent concat plans differ: %s != %s", plan.Hash(), samePlan.Hash())
	}
	different, err := env.Build(Select(input,
		Alias("two", ConcatOf(first, second)),
		Alias("three", ConcatOf(first, second, third)),
		Alias("with_separator", ConcatOf(first, Literal("/"), second)),
	).Query(StatementName("concat-different")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() == different.Hash() {
		t.Fatal("concat operand change did not enter Plan identity")
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("concat result schema is missing")
	}
	for _, name := range []string{"two", "three", "with_separator"} {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != typeOf[string]() {
			t.Fatalf("concat result %q = %#v, want string", name, field)
		}
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 2)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "concat result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	a, b, c := "a", "b", "c"
	if err := engine.SendEvent(context.Background(), concatParityEvent{First: &a, Second: &b, Third: &c}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !rows[0].Get("two").Equal(Present("ab")) || !rows[0].Get("three").Equal(Present("abc")) || !rows[0].Get("with_separator").Equal(Present("a|b")) {
		t.Fatalf("concat first row = %#v", rows)
	}
	if err := engine.SendEvent(context.Background(), concatParityEvent{First: &a, Second: nil, Third: &c}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !rows[1].Get("two").IsNull() || !rows[1].Get("three").IsNull() || !rows[1].Get("with_separator").IsNull() {
		t.Fatalf("concat null row = %#v", rows)
	}
}

func TestConcatExpressionsRejectInvalidBuilders(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[concatParityEvent](env, "ConcatInvalid"); err != nil {
		t.Fatal(err)
	}
	input := From[concatParityEvent](env, "ConcatInvalid")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "empty", expr: ConcatOf(), want: "at least one operand"},
		{name: "nil", expr: ConcatOf(Literal("a"), nil), want: "operand 1 is required"},
		{name: "bool", expr: ConcatOf(Literal(true), Literal("a")), want: "text-compatible"},
		{name: "array", expr: ConcatOf(Literal([]string{"a"})), want: "text-compatible"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", testCase.expr)).Query(StatementName("concat-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("invalid concat error = %v, want %q", err, testCase.want)
			}
		})
	}
}
