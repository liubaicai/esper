package esper

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type scriptProviderInput struct {
	Text   string `esper:"text"`
	Number int64  `esper:"number"`
}

type scriptProviderChild struct {
	Value string `esper:"value"`
}

func TestScriptProviderContextAndContainedExpansionMatchEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[scriptProviderInput](env, "ScriptProviderInput"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[scriptProviderChild](env, "ScriptProviderChild"); err != nil {
		t.Fatal(err)
	}
	invocations := 0
	if err := RegisterScript[[]scriptProviderChild](env, "split-script", "go", func(script ScriptContext) ([]scriptProviderChild, error) {
		invocations++
		if script.Name != "split-script" || script.Dialect != "go" || script.Environment != env {
			t.Fatalf("script metadata = %#v", script)
		}
		if script.Evaluation.Event.TypeName() != "ScriptProviderInput" {
			t.Fatalf("script event type = %q", script.Evaluation.Event.TypeName())
		}
		arguments := script.Arguments
		if len(arguments) != 1 || !arguments[0].IsPresent() {
			t.Fatalf("script arguments = %#v", arguments)
		}
		text, err := As[string](arguments[0])
		if err != nil {
			return nil, err
		}
		parts := strings.Split(text, ",")
		children := make([]scriptProviderChild, 0, len(parts))
		for _, part := range parts {
			children = append(children, scriptProviderChild{Value: strings.ToUpper(part)})
		}
		return children, nil
	}, ScriptDialect("go"), ScriptArgumentTypes(reflect.TypeOf(""))); err != nil {
		t.Fatal(err)
	}

	input := From[scriptProviderInput](env, "ScriptProviderInput")
	children := Unnest[scriptProviderInput, scriptProviderChild](input, ScriptCall[[]scriptProviderChild](env, "split-script", Field[scriptProviderInput, string]("text")))
	plan, err := env.Build(Select(children,
		Alias("value", Field[scriptProviderChild, string]("value")),
	).Query(StatementName("script-contained")))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plan.Canonical()), "script(split-script:go") {
		t.Fatalf("script metadata missing from canonical plan: %s", plan.Canonical())
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	values := make([]string, 0, 3)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "script result is not a row")
			}
			value, ok := row.Get("value").Any().(string)
			if !ok {
				return NewError(ErrorTypeMismatch, "script child value is not a string")
			}
			values = append(values, value)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), scriptProviderInput{Text: "a,b,c"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(values, []string{"A", "B", "C"}) || invocations != 1 {
		t.Fatalf("script contained values=%v invocations=%d", values, invocations)
	}
}

func TestScriptProviderPlanIdentityAndRuntimeNull(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[scriptProviderInput](env, "ScriptIdentityInput"); err != nil {
		t.Fatal(err)
	}
	if err := RegisterScript[int64](env, "script-number", "go", func(script ScriptContext) (int64, error) {
		value, err := As[int64](script.Argument(0))
		if err != nil {
			return 0, err
		}
		return value * 2, nil
	}, ScriptArgumentTypes(reflect.TypeOf(int64(0)))); err != nil {
		t.Fatal(err)
	}
	if err := RegisterValueScript(env, "script-error", "go", reflect.TypeOf(""), ScriptProvider(func(_ ScriptContext) (Value, error) {
		return Null(), context.Canceled
	}), ScriptArgumentTypes()); err != nil {
		t.Fatal(err)
	}
	input := From[scriptProviderInput](env, "ScriptIdentityInput")
	number := ScriptCall[int64](env, "script-number", Field[scriptProviderInput, int64]("number"))
	build := func() Plan {
		plan, err := env.Build(Select(input, Alias("value", number)).Query(StatementName("script-identity")))
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}
	first := build()
	second := build()
	if first.Hash() != second.Hash() || !reflect.DeepEqual(first.Canonical(), second.Canonical()) {
		t.Fatalf("equivalent script plans differ: %s != %s", first.Hash(), second.Hash())
	}

	errorPlan, err := env.Build(Select(input, Alias("value", ScriptCall[string](env, "script-error"))).Query(StatementName("script-error-runtime")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), errorPlan)
	if err != nil {
		t.Fatal(err)
	}
	var value Value
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			row, ok := batch.New[0].Row()
			if !ok {
				t.Fatalf("script error result is not a row: %#v", batch.New[0])
			}
			value = row.Get("value")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), scriptProviderInput{}); err != nil {
		t.Fatal(err)
	}
	if !value.IsNull() {
		t.Fatalf("script provider error value = %v, want Null", value)
	}
}

func TestScriptProviderAttributesPersistAndPreserveNull(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[scriptProviderInput](env, "ScriptAttributeInput"); err != nil {
		t.Fatal(err)
	}
	if err := RegisterScript[int64](env, "script-attributes", "go", func(script ScriptContext) (int64, error) {
		count, exists := script.GetScriptAttribute("count")
		if !exists {
			count = Present(int64(0))
		}
		current, err := As[int64](count)
		if err != nil {
			return 0, err
		}
		current++
		script.SetScriptAttribute("count", current)
		script.SetScriptAttribute("nil-value", nil)
		stored, ok := script.GetScriptAttribute("nil-value")
		if !ok || !stored.IsNull() {
			return 0, fmt.Errorf("nil script attribute = %#v, exists=%t", stored, ok)
		}
		return current, nil
	}); err != nil {
		t.Fatal(err)
	}

	input := From[scriptProviderInput](env, "ScriptAttributeInput")
	plan, err := env.Build(Select(input, Alias("count", ScriptCall[int64](env, "script-attributes"))).Query(StatementName("script-attributes")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	counts := make([]int64, 0, 2)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "script attribute result is not a row")
			}
			value, err := As[int64](row.Get("count"))
			if err != nil {
				return err
			}
			counts = append(counts, value)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := engine.SendEvent(context.Background(), scriptProviderInput{}); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(counts, []int64{1, 2}) {
		t.Fatalf("script attribute counts = %#v, want [1 2]", counts)
	}
}

func TestScriptProviderRejectsMissingAndMismatchedDefinitionsAtBuild(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[scriptProviderInput](env, "ScriptInvalidInput"); err != nil {
		t.Fatal(err)
	}
	if err := DefineScript[int64](env, "typed-script", "go", func(EvalContext, []Value) (int64, error) { return 1, nil },
		ScriptArgumentTypes(reflect.TypeOf(""))); err != nil {
		t.Fatal(err)
	}
	input := From[scriptProviderInput](env, "ScriptInvalidInput")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "unknown", expr: ScriptCall[string](env, "missing-script"), want: `script "missing-script" is not registered`},
		{name: "wrong-result", expr: ScriptCall[string](env, "typed-script", Literal("x")), want: `script "typed-script" returns int64, expression expects string`},
		{name: "wrong-argument", expr: ScriptCall[int64](env, "typed-script", Field[scriptProviderInput, int64]("number")), want: `script "typed-script" argument 0 expects string, received int64`},
		{name: "wrong-arity", expr: ScriptCall[int64](env, "typed-script"), want: `script "typed-script" expects 1 arguments, received 0`},
		{name: "nil-argument", expr: ScriptCall[int64](env, "typed-script", nil), want: `script "typed-script" argument 0 is required`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", testCase.expr)).Query(StatementName("script-invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("Build error = %v, want %q", err, testCase.want)
			}
		})
	}

	other := NewEnvironment()
	if _, err := RegisterStruct[scriptProviderInput](other, "ScriptInvalidInput"); err != nil {
		t.Fatal(err)
	}
	foreign := ScriptCall[int64](other, "typed-script", Literal("x"))
	if _, err := env.Build(Select(input, Alias("value", foreign)).Query(StatementName("script-foreign-environment"))); err == nil || !strings.Contains(err.Error(), "different or nil environment") {
		t.Fatalf("foreign script Build error = %v", err)
	}

	if err := DefineScript[string](env, "typed-script", "go", func(EvalContext, []Value) (string, error) { return "duplicate", nil }); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("duplicate script registration error = %v", err)
	}
}
