package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type exprCaseEvent struct {
	Kind   int     `esper:"kind"`
	Price  float64 `esper:"price"`
	Text   string  `esper:"text"`
	Switch bool    `esper:"switch"`
}

type exprCaseEnum string

const (
	exprCaseEnumOne   exprCaseEnum = "one"
	exprCaseEnumTwo   exprCaseEnum = "two"
	exprCaseEnumOther exprCaseEnum = "other"
)

func TestCaseExpressionsMatchJavaNullAndShortCircuitSemantics(t *testing.T) {
	searched := CaseWhen[string](Equal[int](Literal(1), Literal(2)), Literal("wrong")).
		When(Literal(true), Literal("selected")).
		Else(Literal("else"))
	if got := searched.eval(EvalContext{}); !got.Equal(Present("selected")) {
		t.Fatalf("searched case = %v", got)
	}

	noMatch := CaseWhen[string](NullLiteral[bool](), Literal("wrong")).Build()
	if got := noMatch.eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("null searched condition = %v, want null", got)
	}

	simple := CaseValue[string](Literal(int64(2)), Literal(1), Literal("one")).
		When(Literal(2), Literal("two")).
		Else(Literal("other"))
	if got := simple.eval(EvalContext{}); !got.Equal(Present("two")) {
		t.Fatalf("numeric simple case = %v", got)
	}

	nullSimple := CaseValue[string](NullLiteral[string](), NullLiteral[string](), Literal("null")).
		When(Literal("value"), Literal("value")).
		Else(Literal("other"))
	if got := nullSimple.eval(EvalContext{}); !got.Equal(Present("null")) {
		t.Fatalf("null simple case = %v", got)
	}

	array := CaseWhen[[]int](Literal(true), Literal([]int{1, 2})).Else(Literal([]int{3}))
	if got := array.eval(EvalContext{}); !got.Equal(Present([]int{1, 2})) {
		t.Fatalf("array case = %v", got)
	}

	enum := CaseValue[exprCaseEnum](Literal(2), Literal(1), Literal(exprCaseEnumOne)).
		When(Literal(2), Literal(exprCaseEnumTwo)).
		Else(Literal(exprCaseEnumOther))
	if got := enum.eval(EvalContext{}); !got.Equal(Present(exprCaseEnumTwo)) {
		t.Fatalf("enum case = %v", got)
	}

	var selected, skipped int
	lazy := CaseWhen[string](Literal(false), Func0("skipped", func() string {
		skipped++
		return "skipped"
	})).When(Literal(true), Func0("selected", func() string {
		selected++
		return "selected"
	})).Else(Func0("else", func() string {
		t.Fatalf("CASE evaluated ELSE after a matching WHEN")
		return "else"
	}))
	if got := lazy.eval(EvalContext{}); !got.Equal(Present("selected")) || selected != 1 || skipped != 0 {
		t.Fatalf("lazy case = %v, selected=%d skipped=%d", got, selected, skipped)
	}

	builder := CaseWhen[string](Literal(false), Literal("first"))
	expression := builder.Build()
	builder.When(Literal(true), Literal("late"))
	if got := expression.eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("built case changed after builder mutation = %v", got)
	}
}

func TestCaseExpressionsBuildFluentQueriesAndPlanIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprCaseEvent](env, "ExprCaseEvent"); err != nil {
		t.Fatal(err)
	}
	stream := From[exprCaseEvent](env, "ExprCaseEvent")
	kind := Field[exprCaseEvent, int]("kind")
	label := CaseValue[string](kind, Literal(1), Literal("one")).
		When(Literal(2), Literal("two")).
		Else(Literal("other"))

	build := func(result string) Plan {
		expression := CaseValue[string](kind, Literal(1), Literal("one")).
			When(Literal(2), Literal(result)).
			Else(Literal("other"))
		plan, err := env.Build(Select(stream, Alias("label", expression)).Query(StatementName("expr-case-plan")))
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}
	first := build("two")
	same := build("two")
	different := build("changed")
	if first.Hash() != same.Hash() || !reflect.DeepEqual(first.Canonical(), same.Canonical()) {
		t.Fatalf("equivalent CASE plans differ: %s != %s", first.Hash(), same.Hash())
	}
	if first.Hash() == different.Hash() || reflect.DeepEqual(first.Canonical(), different.Canonical()) {
		t.Fatalf("different CASE branches share plan identity: %s", first.Hash())
	}

	plan, err := env.Build(Select(stream, Alias("label", label)).Query(StatementName("expr-case-runtime")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, 3)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("CASE result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []exprCaseEvent{{Kind: 1}, {Kind: 2}, {Kind: 3}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"one", "two", "other"}
	if len(rows) != len(want) {
		t.Fatalf("CASE query rows = %#v", rows)
	}
	for index, expected := range want {
		if got := rows[index].Get("label").Any(); got != expected {
			t.Fatalf("CASE row %d = %#v, want %q", index, got, expected)
		}
	}

	nested := Multiply[int64](
		CaseValue[int64](kind, Literal(1), Literal(int64(2))).
			When(Literal(2), Literal(int64(3))).
			Else(Literal(int64(10))),
		Literal(int64(2)),
	)
	nestedPlan, err := env.Build(Select(stream, Alias("value", nested)).Query(StatementName("expr-case-nested")))
	if err != nil {
		t.Fatal(err)
	}
	nestedEngine := NewEngine(env)
	nestedDeployment, err := nestedEngine.Deploy(context.Background(), nestedPlan)
	if err != nil {
		t.Fatal(err)
	}
	var nestedValue any
	if _, err := nestedDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) != 0 {
			row, ok := batch.New[0].Row()
			if !ok {
				t.Fatalf("nested CASE result is not a row: %#v", batch.New[0])
			}
			nestedValue = row.Get("value").Any()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := nestedEngine.SendEvent(context.Background(), exprCaseEvent{Kind: 2}); err != nil {
		t.Fatal(err)
	}
	if nestedValue != int64(6) {
		t.Fatalf("nested CASE value = %#v, want %d", nestedValue, 6)
	}
}

func TestCaseExpressionsPreserveAggregateContext(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprCaseEvent](env, "ExprCaseAggregateEvent"); err != nil {
		t.Fatal(err)
	}
	stream := From[exprCaseEvent](env, "ExprCaseAggregateEvent")
	kind := Field[exprCaseEvent, int]("kind")
	price := Field[exprCaseEvent, float64]("price")
	value := CaseValue[float64](kind, Literal(1), Sum[float64](price)).
		When(Literal(2), Cast[int64, float64](CountAll())).
		Else(Avg[float64](price))
	if !isAggregateExpression(value) {
		t.Fatal("CASE containing aggregate branches was not classified as aggregate")
	}
	plan, err := env.Build(stream.Aggregate(Alias("value", value)).Query(StatementName("expr-case-aggregate")))
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, ok := plan.ResultSchema()
	resultField, fieldOK := resultSchema.Field("value")
	if !ok || !fieldOK || resultField.Type != reflect.TypeOf(float64(0)) {
		t.Fatalf("CASE aggregate result schema = %#v", resultSchema)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	values := make([]float64, 0, 4)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("aggregate CASE result is not a row: %#v", result)
			}
			got, ok := row.Get("value").Any().(float64)
			if !ok {
				t.Fatalf("aggregate CASE value = %#v", row.Get("value"))
			}
			values = append(values, got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []exprCaseEvent{
		{Kind: 1, Price: 5},
		{Kind: 1, Price: 7},
		{Kind: 2, Price: 9},
		{Kind: 3, Price: 3},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	want := []float64{5, 12, 3, 6}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("aggregate CASE values = %#v, want %#v", values, want)
	}
}

func TestCaseExpressionsRejectInvalidBuildersAtBuild(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[exprCaseEvent](env, "ExprCaseInvalidEvent"); err != nil {
		t.Fatal(err)
	}
	stream := From[exprCaseEvent](env, "ExprCaseInvalidEvent")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "empty", expr: Case[int]().Build(), want: "at least one WHEN"},
		{name: "nil-condition", expr: Case[int]().When(nil, Literal(1)).Build(), want: "WHEN condition"},
		{name: "nil-result", expr: CaseWhen[int](Literal(true), nil).Build(), want: "THEN result"},
		{name: "nil-value", expr: CaseValue[string](nil, Literal(1), Literal("one")).Build(), want: "case value"},
		{name: "nil-match", expr: CaseValue[string](Literal(1), nil, Literal("one")).Build(), want: "WHEN value"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(stream, Alias("value", testCase.expr)).Query(StatementName("expr-case-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("invalid CASE error = %v, want substring %q", err, testCase.want)
			}
		})
	}

	builder := CaseWhen[string](Literal(true), Literal("a"))
	_ = builder.Else(Literal("b"))
	duplicateElse := builder.Else(Literal("c"))
	if _, err := env.Build(Select(stream, Alias("value", duplicateElse)).Query(StatementName("expr-case-duplicate-else"))); err == nil || !strings.Contains(err.Error(), "only be specified once") {
		t.Fatalf("duplicate ELSE error = %v", err)
	}
}
