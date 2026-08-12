package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type coalesceNumericEvent struct {
	Byte   *int8    `esper:"byte"`
	Short  *int16   `esper:"short"`
	Int    *int     `esper:"int"`
	Long   *int64   `esper:"long"`
	Float  *float32 `esper:"float"`
	Double *float64 `esper:"double"`
}

type coalesceBean struct {
	Name string `esper:"name"`
}

func coalescePtr[T any](value T) *T { return &value }

func TestCoalesceExpressionsMatchJavaNumericAndNullSemantics(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[coalesceNumericEvent](env, "CoalesceNumericEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[coalesceNumericEvent](env, "CoalesceNumericEvent")
	longResult := CoalesceOf[int64](
		Field[coalesceNumericEvent, *int64]("long"),
		Field[coalesceNumericEvent, *int]("int"),
		Field[coalesceNumericEvent, *int16]("short"),
	)
	doubleResult := CoalesceOf[float64](
		NullLiteral[float64](),
		Field[coalesceNumericEvent, *int8]("byte"),
		Field[coalesceNumericEvent, *int16]("short"),
		Field[coalesceNumericEvent, *int]("int"),
		Field[coalesceNumericEvent, *int64]("long"),
		Field[coalesceNumericEvent, *float32]("float"),
		Field[coalesceNumericEvent, *float64]("double"),
	)
	nullResult := CoalesceOf[any](NullLiteral[any](), NullLiteral[any]())
	plan, err := env.Build(Select(input,
		Alias("long", longResult),
		Alias("double", doubleResult),
		Alias("null", nullResult),
	).Query(StatementName("coalesce-numeric")))
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(plan.Query())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !reflect.DeepEqual(plan.Canonical(), samePlan.Canonical()) {
		t.Fatalf("equivalent coalesce plans differ: %s != %s", plan.Hash(), samePlan.Hash())
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("coalesce result schema is missing")
	}
	if field, ok := resultSchema.Field("long"); !ok || field.Type != reflect.TypeOf(int64(0)) {
		t.Fatalf("long coalesce result type = %#v, want int64", field)
	}
	if field, ok := resultSchema.Field("double"); !ok || field.Type != reflect.TypeOf(float64(0)) {
		t.Fatalf("double coalesce result type = %#v, want float64", field)
	}
	if field, ok := resultSchema.Field("null"); !ok || field.Type != typeOf[any]() {
		t.Fatalf("null coalesce result type = %#v, want any", field)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 7)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "coalesce result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	events := []coalesceNumericEvent{
		{Long: coalescePtr(int64(1)), Int: coalescePtr(2), Short: coalescePtr(int16(3)), Double: coalescePtr(100.0)},
		{Int: coalescePtr(2)},
		{Double: coalescePtr(100.0)},
		{Float: coalescePtr(float32(10.0)), Double: coalescePtr(100.0)},
		{Int: coalescePtr(1), Long: coalescePtr(int64(5)), Float: coalescePtr(float32(10.0)), Double: coalescePtr(100.0)},
		{Byte: coalescePtr(int8(3))},
		{Long: coalescePtr(int64(5))},
	}
	for _, event := range events {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != len(events) {
		t.Fatalf("coalesce rows = %d, want %d", len(rows), len(events))
	}
	wantLong := []Value{Present(int64(1)), Present(int64(2)), Null(), Null(), Present(int64(5)), Null(), Present(int64(5))}
	wantDouble := []Value{Present(float64(3)), Present(float64(2)), Present(float64(100)), Present(float64(10)), Present(float64(1)), Present(float64(3)), Present(float64(5))}
	for index, row := range rows {
		if !row.Get("long").Equal(wantLong[index]) || !row.Get("double").Equal(wantDouble[index]) || !row.Get("null").IsNull() {
			t.Fatalf("coalesce row %d = long=%v double=%v null=%v, want long=%v double=%v null", index, row.Get("long"), row.Get("double"), row.Get("null"), wantLong[index], wantDouble[index])
		}
	}
}

func TestCoalesceExpressionsPreserveBeansAndRejectInvalidBuilders(t *testing.T) {
	bean := &coalesceBean{Name: "first"}
	value := Coalesce[*coalesceBean](NullLiteral[*coalesceBean](), Literal(bean)).eval(EvalContext{})
	got, err := As[*coalesceBean](value)
	if err != nil || got != bean {
		t.Fatalf("bean coalesce = %#v (%v), want original pointer", got, err)
	}
	if got := CoalesceOf[any](NullLiteral[any](), Literal("fallback")).eval(EvalContext{}); !got.Equal(Present("fallback")) {
		t.Fatalf("any coalesce = %v", got)
	}
	var nilBean *coalesceBean
	if got := CoalesceOf[*coalesceBean](Literal(nilBean), Literal(bean)).eval(EvalContext{}); !got.Equal(Present(bean)) {
		t.Fatalf("typed nil bean coalesce = %v, want fallback bean", got)
	}
	if got := Coalesce[string](ContextField[string]("not-present"), Literal("fallback")).eval(EvalContext{}); !got.Equal(Present("fallback")) {
		t.Fatalf("missing coalesce = %v", got)
	}

	env := NewEnvironment()
	if _, err := RegisterStruct[coalesceNumericEvent](env, "CoalesceInvalidEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[coalesceNumericEvent](env, "CoalesceInvalidEvent")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "zero-operand", expr: CoalesceOf[int64](), want: "at least two"},
		{name: "one-operand", expr: CoalesceOf[int64](Literal(int64(1))), want: "at least two"},
		{name: "nil-operand", expr: CoalesceOf[int64](nil, Literal(int64(1))), want: "operands are required"},
		{name: "wrong-type", expr: CoalesceOf[int64](Literal("x"), Literal(int64(1))), want: "operand 0"},
		{name: "wrong-type-boolean", expr: CoalesceOf[int64](Literal(true), Literal(int64(1))), want: "operand 0"},
		{name: "numeric-narrowing", expr: CoalesceOf[int64](Literal(float64(1)), Literal(int64(1))), want: "operand 0"},
		{name: "unknown-field", expr: CoalesceOf[int64](Field[coalesceNumericEvent, int64]("unknown"), Literal(int64(1))), want: "unknown field"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", testCase.expr)).Query(StatementName("coalesce-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("Build error = %v, want substring %q", err, testCase.want)
			}
		})
	}
	if _, err := env.Build(From[coalesceNumericEvent](env, "CoalesceInvalidEvent").Query(
		WithOutput(OutputWhen(CoalesceOf[bool](Literal(true)))),
	)); err == nil || !strings.Contains(err.Error(), "at least two") {
		t.Fatalf("output coalesce validation error = %v, want at-least-two diagnostic", err)
	}
}

func TestCoalesceExpressionsPreservePatternEventIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[coalesceBean](env, "CoalesceBean"); err != nil {
		t.Fatal(err)
	}
	input := From[coalesceBean](env, "CoalesceBean")
	left := PatternFrom(input, "a", Equal[string](Field[coalesceBean, string]("name"), Literal("s0")))
	right := PatternFrom(input, "b", Equal[string](Field[coalesceBean, string]("name"), Literal("s1")))
	plan, err := env.Build(left.Or(right).Every().Select(
		Alias("name", Coalesce[string](TagField[string]("a", "name"), TagField[string]("b", "name"))),
		Alias("event", CoalesceOf[Event](PatternEvent("a"), PatternEvent("b"))),
	).Query(StatementName("coalesce-pattern-beans")))
	if err != nil {
		t.Fatal(err)
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
				return NewError(ErrorTypeMismatch, "coalesce pattern result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	first := &coalesceBean{Name: "s0"}
	second := &coalesceBean{Name: "s1"}
	for _, event := range []*coalesceBean{first, second} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("coalesce pattern rows = %d, want 2", len(rows))
	}
	for index, expected := range []*coalesceBean{first, second} {
		if rows[index].Get("name").Any() != expected.Name {
			t.Fatalf("coalesce pattern row %d name = %v, want %q", index, rows[index].Get("name"), expected.Name)
		}
		event, err := As[Event](rows[index].Get("event"))
		if err != nil || event.Underlying() != expected {
			t.Fatalf("coalesce pattern row %d event = %v (%v), want underlying %p", index, event, err, expected)
		}
	}
}
