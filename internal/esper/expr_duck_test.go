package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type duckExpressionCommonOne struct{}

func (duckExpressionCommonOne) MakeInteger() int { return -1 }

type duckExpressionCommonTwo struct{}

func (duckExpressionCommonTwo) MakeString() string { return "mytext" }

type duckExpressionOne struct{}

func (duckExpressionOne) MakeString() string { return "x" }
func (duckExpressionOne) MakeCommon() any    { return duckExpressionCommonOne{} }
func (duckExpressionOne) ReturnDouble() float64 {
	return 12.9876
}

type duckExpressionTwo struct{}

func (duckExpressionTwo) MakeInteger() int { return -10 }
func (duckExpressionTwo) MakeCommon() any  { return duckExpressionCommonTwo{} }
func (duckExpressionTwo) ReturnDouble() float64 {
	return 11.1234
}

type duckExpressionEvent struct {
	Dynamic any `esper:"dt"`
}

func TestDuckMethodExpressionsMatchJavaDynamicDotSemantics(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[duckExpressionEvent](env, "DuckExpressionEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[duckExpressionEvent](env, "DuckExpressionEvent")
	dynamic := Field[duckExpressionEvent, any]("dt")
	common := DuckMethod[any](dynamic, "MakeCommon")
	query := Select(input,
		Alias("strval", DuckMethod[any](dynamic, "MakeString")),
		Alias("intval", DuckMethod[int](dynamic, "MakeInteger")),
		Alias("commonstrval", DuckMethod[any](common, "MakeString")),
		Alias("commonintval", DuckMethod[int](common, "MakeInteger")),
		Alias("commondoubleval", DuckMethod[float64](dynamic, "ReturnDouble")),
	).Query(StatementName("duck-method-parity"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("duck method result schema is missing")
	}
	for name, typ := range map[string]reflect.Type{
		"strval":          typeOf[any](),
		"intval":          typeOf[int](),
		"commonstrval":    typeOf[any](),
		"commonintval":    typeOf[int](),
		"commondoubleval": typeOf[float64](),
	} {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != typ {
			t.Fatalf("duck method result %q = %#v, want %v", name, field, typ)
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
				return NewError(ErrorTypeMismatch, "duck method result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []duckExpressionEvent{{Dynamic: duckExpressionOne{}}, {Dynamic: duckExpressionTwo{}}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("duck method rows = %d, want 2", len(rows))
	}
	wantFirst := map[string]Value{
		"strval":          Present(any("x")),
		"intval":          Null(),
		"commonstrval":    Null(),
		"commonintval":    Present(-1),
		"commondoubleval": Present(12.9876),
	}
	wantSecond := map[string]Value{
		"strval":          Null(),
		"intval":          Present(-10),
		"commonstrval":    Present(any("mytext")),
		"commonintval":    Null(),
		"commondoubleval": Present(11.1234),
	}
	for name, expected := range wantFirst {
		if got := rows[0].Get(name); !got.Equal(expected) {
			t.Fatalf("first duck method %s = %v, want %v", name, got, expected)
		}
	}
	for name, expected := range wantSecond {
		if got := rows[1].Get(name); !got.Equal(expected) {
			t.Fatalf("second duck method %s = %v, want %v", name, got, expected)
		}
	}
}

func TestDuckMethodExpressionsPreservePlanIdentityAndRejectInvalidBuilders(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[duckExpressionEvent](env, "DuckExpressionPlan"); err != nil {
		t.Fatal(err)
	}
	input := From[duckExpressionEvent](env, "DuckExpressionPlan")
	dynamic := Field[duckExpressionEvent, any]("dt")
	first, err := env.Build(Select(input, Alias("value", DuckMethod[any](dynamic, "MakeString"))).Query(StatementName("duck-plan")))
	if err != nil {
		t.Fatal(err)
	}
	same, err := env.Build(Select(input, Alias("value", DuckMethod[any](dynamic, "MakeString"))).Query(StatementName("duck-plan")))
	if err != nil {
		t.Fatal(err)
	}
	different, err := env.Build(Select(input, Alias("value", DuckMethod[any](dynamic, "MakeInteger"))).Query(StatementName("duck-plan")))
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash() != same.Hash() || !reflect.DeepEqual(first.Canonical(), same.Canonical()) || first.Hash() == different.Hash() {
		t.Fatal("duck method call did not participate in Plan identity")
	}
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "nil-receiver", expr: DuckMethod[any](nil, "MakeString"), want: "receiver is required"},
		{name: "blank-name", expr: DuckMethod[any](dynamic, " "), want: "method name is required"},
		{name: "nested-nil-receiver", expr: DuckMethod[any](DuckMethod[any](nil, "MakeCommon"), "MakeString"), want: "receiver is required"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", testCase.expr)).Query(StatementName("duck-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("invalid duck method error = %v, want %q", err, testCase.want)
			}
		})
	}
}
