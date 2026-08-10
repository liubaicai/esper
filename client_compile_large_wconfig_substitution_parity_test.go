package esper

import (
	"context"
	"fmt"
	"testing"
)

const clientCompileLargeParameterCount = 5000

func clientCompileLargeParameterSelections(prefix Expression[string]) ([]Selection, ParameterValues) {
	selections := make([]Selection, clientCompileLargeParameterCount)
	values := make(ParameterValues, clientCompileLargeParameterCount)
	for index := range selections {
		parameterName := fmt.Sprintf("p%d", index)
		columnName := fmt.Sprintf("c%d", index)
		parameter := Parameter[string](parameterName)
		expression := Expression[string](parameter)
		if prefix != nil {
			expression = Concat(prefix, parameter)
		}
		selections[index] = Alias(columnName, expression)
		values[parameterName] = fmt.Sprintf("v%d", index)
	}
	return selections, values
}

func assertClientCompileLargeParameterResult(t *testing.T, result Result, prefix string) {
	t.Helper()
	row, ok := result.Row()
	if !ok {
		t.Fatalf("large parameter result is not a row: %#v", result)
	}
	if fields := row.Schema().Fields(); len(fields) != clientCompileLargeParameterCount {
		t.Fatalf("large parameter result fields = %d, want %d", len(fields), clientCompileLargeParameterCount)
	}
	for index := 0; index < clientCompileLargeParameterCount; index++ {
		columnName := fmt.Sprintf("c%d", index)
		want := prefix + fmt.Sprintf("v%d", index)
		if got := row.Get(columnName).Any(); got != want {
			t.Fatalf("large parameter %s = %#v, want %q", columnName, got, want)
		}
	}
}

func TestClientCompileLargeSubstitutionParamsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[clientCompileLargeEvent](env, "ClientCompileLargeWConfigEvent"); err != nil {
		t.Fatal(err)
	}
	selections, values := clientCompileLargeParameterSelections(nil)
	plan, err := env.Build(Select(
		From[clientCompileLargeEvent](env, "ClientCompileLargeWConfigEvent"),
		selections...,
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.DeployWithParameters(context.Background(), plan, values)
	if err != nil {
		t.Fatal(err)
	}
	// Deployment owns a binding snapshot; caller mutations after deployment
	// must not change the compiled statement's 5,000 values.
	values["p0"] = "changed"
	delete(values, "p1")

	statement, ok := deployment.Statement("s0")
	if !ok {
		t.Fatal("large parameter statement s0 is missing")
	}
	received := false
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) != 1 {
			t.Fatalf("large parameter batch = %#v", batch)
		}
		assertClientCompileLargeParameterResult(t, batch.New[0], "")
		received = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "ClientCompileLargeWConfigEvent", clientCompileLargeEvent{Value: 1}); err != nil {
		t.Fatal(err)
	}
	if !received {
		t.Fatal("large parameter statement produced no listener result")
	}
}

func TestClientCompileLargeSubstitutionParamsFAFMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	windowSchema, err := RegisterMap(env, "ClientCompileLargeWConfigWindowRow", []FieldSpec{
		FieldDef("p0", typeOf[string]()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindow", windowSchema, NamedWindowRetention(LastEvent())); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if err := engine.InsertNamedWindow(context.Background(), "MyWindow", map[string]any{"p0": "x"}); err != nil {
		t.Fatal(err)
	}

	selections, values := clientCompileLargeParameterSelections(Field[any, string]("p0"))
	plan, err := env.Build(FromNamedWindow(env, "MyWindow").Select(selections...).Query(StatementName("large-faf-parameters")))
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := engine.PrepareFireAndForget(plan)
	if err != nil {
		t.Fatal(err)
	}
	result, err := prepared.ExecuteWithParameters(context.Background(), values)
	if err != nil {
		t.Fatal(err)
	}
	results := result.Results()
	if len(results) != 1 {
		t.Fatalf("large FAF parameter results = %d, want 1", len(results))
	}
	assertClientCompileLargeParameterResult(t, results[0], "x")
}
