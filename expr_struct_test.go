package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type structExpressionParityEvent struct {
	Kind  string `esper:"kind"`
	Text  string `esper:"text"`
	Value int    `esper:"value"`
}

func TestStructExpressionsMatchJavaAnonymousMapAndNullSemantics(t *testing.T) {
	nested := StructOf(
		StructField("a", Literal("x")),
		StructField("b.c", Literal(2)),
		StructField("}", NullLiteral[string]()),
	)
	expression := StructOf(
		StructField("text", Literal("value")),
		StructField("number", Literal(2)),
		StructField("null", NullLiteral[string]()),
		StructField("nested", nested),
	)
	got := expression.eval(EvalContext{})
	want := map[string]any{
		"text":   "value",
		"number": 2,
		"null":   nil,
		"nested": map[string]any{"a": "x", "b.c": 2, "}": nil},
	}
	if !got.Equal(Present(want)) {
		t.Fatalf("anonymous struct = %#v, want %#v", got.Any(), want)
	}
	nestedGot, ok := got.Any().(map[string]any)["nested"].(map[string]any)
	if !ok || nestedGot["}"] != nil {
		t.Fatal("special-name null field was not materialized")
	}
}

func TestStructExpressionsBuildCaseProjectionAndPlanIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[structExpressionParityEvent](env, "StructParityEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[structExpressionParityEvent](env, "StructParityEvent")
	kind := Field[structExpressionParityEvent, string]("kind")
	text := Field[structExpressionParityEvent, string]("text")
	value := Field[structExpressionParityEvent, int]("value")
	whenA := StructOf(
		Alias("text", Literal("Q")),
		Alias("value", value),
		Alias("col2", ConcatOf(text, Literal("A"))),
	)
	whenB := StructOf(
		Alias("text", text),
		Alias("value", Literal(10)),
		Alias("col2", ConcatOf(text, Literal("B"))),
	)
	whenElse := StructOf(Alias("text", Literal("Z")), Alias("value", Literal(30)))
	projection := CaseValue[map[string]any](kind, Literal("A"), whenA).
		When(Literal("B"), whenB).
		Else(whenElse)
	query := Select(input, Alias("value", projection)).Query(StatementName("struct-parity"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	samePlan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != samePlan.Hash() || !reflect.DeepEqual(plan.Canonical(), samePlan.Canonical()) {
		t.Fatalf("equivalent struct plans differ: %s != %s", plan.Hash(), samePlan.Hash())
	}
	different, err := env.Build(Select(input, Alias("value", CaseValue[map[string]any](kind, Literal("A"),
		StructOf(Alias("text", Literal("changed")))).Else(whenElse))).Query(StatementName("struct-parity")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() == different.Hash() {
		t.Fatal("struct child expression did not enter Plan identity")
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("struct result schema is missing")
	}
	field, ok := resultSchema.Field("value")
	if !ok || field.Type != typeOf[map[string]any]() {
		t.Fatalf("struct result field = %#v, want map[string]any", field)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 4)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "struct result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []structExpressionParityEvent{
		{Kind: "E", Text: "E1", Value: 1},
		{Kind: "A", Text: "A", Value: 2},
		{Kind: "B", Text: "B", Value: 3},
		{Kind: "C", Text: "C", Value: 4},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 4 {
		t.Fatalf("struct rows = %d, want 4", len(rows))
	}
	assertStructRow := func(index int, want map[string]any) {
		got, ok := rows[index].Get("value").Any().(map[string]any)
		if !ok || !reflect.DeepEqual(got, want) {
			t.Fatalf("struct row %d = %#v, want %#v", index, got, want)
		}
	}
	assertStructRow(0, map[string]any{"text": "Z", "value": 30})
	assertStructRow(1, map[string]any{"text": "Q", "value": 2, "col2": "AA"})
	assertStructRow(2, map[string]any{"text": "B", "value": 10, "col2": "BB"})
	assertStructRow(3, map[string]any{"text": "Z", "value": 30})
}

func TestStructExpressionsRejectInvalidBuilders(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[structExpressionParityEvent](env, "StructInvalidEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[structExpressionParityEvent](env, "StructInvalidEvent")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "duplicate", expr: StructOf(Alias("x", Literal(1)), Alias("x", Literal(2))), want: "declared more than once"},
		{name: "empty-name", expr: StructOf(Alias(" ", Literal(1))), want: "field name"},
		{name: "nil-expression", expr: StructOf(Selection{Name: "x"}), want: "requires an expression"},
		{name: "nested-invalid", expr: StructOf(Alias("x", EqualOf(Literal(true), Literal(1)))), want: "not compatible"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", testCase.expr)).Query(StatementName("struct-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("invalid struct error = %v, want %q", err, testCase.want)
			}
		})
	}
}
