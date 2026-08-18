package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type namedExpressionEvent struct {
	Value int64 `esper:"value"`
}

type namedExpressionGroupEvent struct {
	Name   string `esper:"name"`
	Amount int64  `esper:"amount"`
}

type parameterizedNamedExpressionEvent struct {
	Left  string `esper:"left"`
	Right string `esper:"right"`
}

func TestNamedExpressionReferenceEvaluatesAndEntersPlanIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[namedExpressionEvent](env, "NamedExpressionEvent"); err != nil {
		t.Fatal(err)
	}
	value := Field[namedExpressionEvent, int64]("value")
	if err := DefineExpression[int64](env, "double-value", Multiply[int64](value, Literal(int64(2)))); err != nil {
		t.Fatal(err)
	}
	if err := DefineExpression[int64](env, "quadruple-value", Multiply[int64](ExpressionRef[int64](env, "double-value"), Literal(int64(2)))); err != nil {
		t.Fatal(err)
	}

	input := From[namedExpressionEvent](env, "NamedExpressionEvent")
	ref := ExpressionRef[int64](env, "quadruple-value")
	build := func() Plan {
		plan, err := env.Build(Select(input, Alias("value", ref)).Query(StatementName("named-expression")))
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}
	first := build()
	second := build()
	if first.Hash() != second.Hash() || !reflect.DeepEqual(first.Canonical(), second.Canonical()) {
		t.Fatalf("equivalent named-expression plans differ: %s != %s", first.Hash(), second.Hash())
	}
	canonical := string(first.Canonical())
	if !strings.Contains(canonical, "expression(double-value:int64") || !strings.Contains(canonical, "expression(quadruple-value:int64") {
		t.Fatalf("named expression definitions missing from canonical plan: %s", canonical)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	var result int64
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			row, ok := batch.New[0].Row()
			if !ok {
				t.Fatalf("named expression result is not a row: %#v", batch.New[0])
			}
			result, _ = As[int64](row.Get("value"))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), namedExpressionEvent{Value: 3}); err != nil {
		t.Fatal(err)
	}
	if result != 12 {
		t.Fatalf("named expression result = %d, want 12", result)
	}
}

func TestNamedExpressionReferenceRejectsInvalidDependencies(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[namedExpressionEvent](env, "NamedExpressionInvalidEvent"); err != nil {
		t.Fatal(err)
	}
	if err := DefineExpression[int64](env, "double-value", Literal(int64(2))); err != nil {
		t.Fatal(err)
	}
	input := From[namedExpressionEvent](env, "NamedExpressionInvalidEvent")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "unknown", expr: ExpressionRef[int64](env, "missing"), want: `expression definition "missing" is not registered`},
		{name: "wrong-type", expr: ExpressionRef[string](env, "double-value"), want: `expression definition "double-value" returns int64, reference expects string`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", testCase.expr)).Query(StatementName("named-expression-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("Build error = %v, want %q", err, testCase.want)
			}
		})
	}

	other := NewEnvironment()
	if _, err := RegisterStruct[namedExpressionEvent](other, "NamedExpressionInvalidEvent"); err != nil {
		t.Fatal(err)
	}
	if err := DefineExpression[int64](other, "foreign", Literal(int64(1))); err != nil {
		t.Fatal(err)
	}
	foreign := ExpressionRef[int64](other, "foreign")
	if _, err := env.Build(Select(input, Alias("value", foreign)).Query(StatementName("named-expression-foreign"))); err == nil || !strings.Contains(err.Error(), "different environment") {
		t.Fatalf("foreign expression Build error = %v", err)
	}

	cycleEnv := NewEnvironment()
	if _, err := RegisterStruct[namedExpressionEvent](cycleEnv, "NamedExpressionCycleEvent"); err != nil {
		t.Fatal(err)
	}
	if err := DefineExpression[int64](cycleEnv, "first", ExpressionRef[int64](cycleEnv, "second")); err != nil {
		t.Fatal(err)
	}
	if err := DefineExpression[int64](cycleEnv, "second", ExpressionRef[int64](cycleEnv, "first")); err != nil {
		t.Fatal(err)
	}
	cycleInput := From[namedExpressionEvent](cycleEnv, "NamedExpressionCycleEvent")
	if _, err := cycleEnv.Build(Select(cycleInput, Alias("value", ExpressionRef[int64](cycleEnv, "first"))).Query(StatementName("named-expression-cycle"))); err == nil || !strings.Contains(err.Error(), "cyclic dependency") {
		t.Fatalf("cyclic expression Build error = %v", err)
	}

	if err := DefineExpression[int64](env, "double-value", Literal(int64(3))); err == nil || !strings.Contains(err.Error(), "A declared-expression by name 'double-value' has already been created for module 'unnamed'") {
		t.Fatalf("duplicate expression registration error = %v", err)
	}
	if err := DefineExpression[int64](env, "nil-expression", nil); err == nil || !strings.Contains(err.Error(), "requires an expression") {
		t.Fatalf("nil expression registration error = %v", err)
	}
}

func TestParameterizedNamedExpressionReferenceMatchesJavaValueParameterSemantics(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[parameterizedNamedExpressionEvent](env, "ParameterizedNamedExpressionEvent"); err != nil {
		t.Fatal(err)
	}
	left := ExpressionParam[string]("left")
	right := ExpressionParam[string]("right")
	if err := DefineExpression[string](env, "join-values", Concat(left, right)); err != nil {
		t.Fatal(err)
	}
	value := ExpressionParam[string]("value")
	if err := DefineExpression[string](env, "decorate-value", Concat(ExpressionRef[string](env, "join-values", value, Literal("!")), Literal("?"))); err != nil {
		t.Fatal(err)
	}

	input := From[parameterizedNamedExpressionEvent](env, "ParameterizedNamedExpressionEvent")
	plan, err := env.Build(Select(input,
		Alias("joined", ExpressionRef[string](env, "join-values",
			Field[parameterizedNamedExpressionEvent, string]("left"),
			Field[parameterizedNamedExpressionEvent, string]("right"))),
		Alias("decorated", ExpressionRef[string](env, "decorate-value",
			Field[parameterizedNamedExpressionEvent, string]("left"))),
	).Query(StatementName("parameterized-named-expression")))
	if err != nil {
		t.Fatal(err)
	}
	canonical := string(plan.Canonical())
	if !strings.Contains(canonical, "expression(join-values:string:left:string,right:string:") ||
		!strings.Contains(canonical, "expression(decorate-value:string:value:string:") {
		t.Fatalf("parameterized expression metadata missing from canonical plan: %s", canonical)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var result Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) == 0 {
			return nil
		}
		var ok bool
		result, ok = batch.New[0].Row()
		if !ok {
			return NewError(ErrorTypeMismatch, "parameterized named expression result is not a row")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), parameterizedNamedExpressionEvent{Left: "A", Right: "B"}); err != nil {
		t.Fatal(err)
	}
	if !result.Get("joined").Equal(Present("AB")) || !result.Get("decorated").Equal(Present("A!?")) {
		t.Fatalf("parameterized named expression result = %#v", result)
	}

	definition, ok := env.Expression("join-values")
	if !ok || len(definition.Parameters) != 2 || definition.Parameters[0].Name != "left" || definition.Parameters[1].Name != "right" {
		t.Fatalf("parameterized definition metadata = %#v", definition)
	}
}

func TestParameterizedNamedExpressionReferenceRejectsArityAndTypeMismatch(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[parameterizedNamedExpressionEvent](env, "ParameterizedNamedExpressionInvalidEvent"); err != nil {
		t.Fatal(err)
	}
	if err := DefineExpression[string](env, "join-values",
		Concat(ExpressionParam[string]("left"), ExpressionParam[string]("right"))); err != nil {
		t.Fatal(err)
	}
	input := From[parameterizedNamedExpressionEvent](env, "ParameterizedNamedExpressionInvalidEvent")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{
			name: "arity",
			expr: ExpressionRef[string](env, "join-values", Literal("only")),
			want: `expression definition "join-values" expects 2 arguments, received 1`,
		},
		{
			name: "type",
			expr: ExpressionRef[string](env, "join-values", Literal(int64(1)), Literal("ok")),
			want: `expression definition "join-values" argument 0 (left) expects string, received int64`,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", testCase.expr)).Query(StatementName("parameterized-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("Build error = %v, want %q", err, testCase.want)
			}
		})
	}
}

func TestNamedExpressionGroupedSubquerySnapshotAndEnumChainMatchEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[namedExpressionEvent](env, "NamedExpressionTrigger"); err != nil {
		t.Fatal(err)
	}
	groupSchema, err := RegisterStruct[namedExpressionGroupEvent](env, "NamedExpressionGroupEvent")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "NamedExpressionGroups", groupSchema); err != nil {
		t.Fatal(err)
	}

	groups := FromNamedWindow(env, "NamedExpressionGroups")
	name := Field[any, string]("name")
	amount := Field[any, int64]("amount")
	groupRows := SubqueryGroupRows(groups, name, []Selection{
		Alias("name", name),
		Alias("total", Sum[int64](amount)),
	})
	if err := DefineExpression[[]map[string]any](env, "getGroups", groupRows); err != nil {
		t.Fatal(err)
	}

	trigger := From[namedExpressionEvent](env, "NamedExpressionTrigger")
	allGroups := ExpressionRef[[]map[string]any](env, "getGroups")
	limitedGroups := EnumTake[map[string]any](ExpressionRef[[]map[string]any](env, "getGroups"), 1)
	plan, err := env.Build(Select(trigger,
		Alias("groups", allGroups),
		Alias("limited", limitedGroups),
	).Query(StatementName("named-expression-grouped-subquery")))
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
				return NewError(ErrorTypeMismatch, "named expression grouped result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	insert := func(name string, amount int64) {
		t.Helper()
		if err := engine.InsertNamedWindow(context.Background(), "NamedExpressionGroups", namedExpressionGroupEvent{Name: name, Amount: amount}); err != nil {
			t.Fatal(err)
		}
	}
	triggerOnce := func() {
		t.Helper()
		if err := engine.SendEvent(context.Background(), namedExpressionEvent{}); err != nil {
			t.Fatal(err)
		}
	}

	triggerOnce()
	if len(rows) != 1 {
		t.Fatalf("empty named expression rows = %d", len(rows))
	}
	if got := rows[0].Get("groups"); !got.IsNull() {
		t.Fatalf("empty groups = %#v, want null (Java groupKeys.isEmpty -> constantNull)", got)
	}
	if got := rows[0].Get("limited"); !got.IsNull() {
		t.Fatalf("empty limited groups = %#v, want null (.take over null stays null)", got)
	}

	insert("E1", 20)
	triggerOnce()
	if len(rows) != 2 {
		t.Fatalf("first grouped snapshot rows = %d", len(rows))
	}
	wantOne := []map[string]any{{"name": "E1", "total": int64(20)}}
	if got := rows[1].Get("groups").Any(); !reflect.DeepEqual(got, wantOne) {
		t.Fatalf("first groups = %#v, want %#v", got, wantOne)
	}
	if got := rows[1].Get("limited").Any(); !reflect.DeepEqual(got, wantOne) {
		t.Fatalf("first limited groups = %#v, want %#v", got, wantOne)
	}

	insert("E2", 30)
	triggerOnce()
	if len(rows) != 3 {
		t.Fatalf("second grouped snapshot rows = %d", len(rows))
	}
	wantTwo := []map[string]any{
		{"name": "E1", "total": int64(20)},
		{"name": "E2", "total": int64(30)},
	}
	if got := rows[2].Get("groups").Any(); !reflect.DeepEqual(got, wantTwo) {
		t.Fatalf("second groups = %#v, want %#v", got, wantTwo)
	}
	if got := rows[2].Get("limited").Any(); !reflect.DeepEqual(got, wantOne) {
		t.Fatalf("second limited groups = %#v, want %#v", got, wantOne)
	}
}
