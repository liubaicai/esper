package esper

import (
	"context"
	"reflect"
	"testing"
)

type variableDeployBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestVariableDeployOrderParity mirrors the shared variable-deploy parity
// scenario (Java EPLVariableOnSetWDeploy): a select referencing a variable is
// deployed before an on-set trigger, and later on-set events update the value
// seen by the select.
func TestVariableDeployOrderParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[variableDeployBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var1RTC", 10); err != nil {
		t.Fatal(err)
	}
	source := From[variableDeployBean](env, "SupportBean")
	selectPlan, err := env.Build(Select(source.Filter(
		StartsWith(Field[variableDeployBean, string]("theString"), Literal("E")),
	),
		Alias("var1RTC", VariableRef[int]("var1RTC")),
		Alias("theString", Field[variableDeployBean, string]("theString")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	setPlan, err := env.Build(OnEvent(source.Filter(
		StartsWith(Field[variableDeployBean, string]("theString"), Literal("S")),
	)).SetVariable("var1RTC", Field[variableDeployBean, int]("intPrimitive")).Query(StatementName("set")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deploy := func(plan Plan) *Statement {
		t.Helper()
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		return deployment.Statements()[0]
	}
	selectStatement := deploy(selectPlan)
	setStatement := deploy(setPlan)

	type row struct {
		fields map[string]any
	}
	var selectRows []row
	if _, err := selectStatement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			r, ok := result.Row()
			if !ok {
				t.Fatalf("select result is not a row: %#v", result)
			}
			selectRows = append(selectRows, row{map[string]any{
				"var1RTC":   r.Get("var1RTC").Any(),
				"theString": r.Get("theString").Any(),
			}})
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var setRows []row
	if _, err := setStatement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			r, ok := result.Row()
			if !ok {
				t.Fatalf("set result is not a row: %#v", result)
			}
			setRows = append(setRows, row{map[string]any{"var1RTC": r.Get("var1RTC").Any()}})
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(theString string, intPrimitive int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), variableDeployBean{TheString: theString, IntPrimitive: intPrimitive}); err != nil {
			t.Fatal(err)
		}
	}
	send("E1", 1)
	send("E2", 2)
	send("S1", 3)
	send("E3", 4)
	send("S2", -1)
	send("E4", 5)

	wantSelect := []map[string]any{
		{"var1RTC": 10, "theString": "E1"},
		{"var1RTC": 10, "theString": "E2"},
		{"var1RTC": 3, "theString": "E3"},
		{"var1RTC": -1, "theString": "E4"},
	}
	if len(selectRows) != len(wantSelect) {
		t.Fatalf("select rows = %#v, want %#v", selectRows, wantSelect)
	}
	for index := range wantSelect {
		if !reflect.DeepEqual(selectRows[index].fields, wantSelect[index]) {
			t.Fatalf("select rows[%d] = %#v, want %#v", index, selectRows[index].fields, wantSelect[index])
		}
	}
	wantSet := []map[string]any{{"var1RTC": 3}, {"var1RTC": -1}}
	if len(setRows) != len(wantSet) {
		t.Fatalf("set rows = %#v, want %#v", setRows, wantSet)
	}
	for index := range wantSet {
		if !reflect.DeepEqual(setRows[index].fields, wantSet[index]) {
			t.Fatalf("set rows[%d] = %#v, want %#v", index, setRows[index].fields, wantSet[index])
		}
	}
}
