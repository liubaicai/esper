package esper

import (
	"context"
	"testing"
)

type enumMethodNestedItem struct {
	Values []string `esper:"values"`
}

type enumMethodNestedSource struct {
	Selector string                 `esper:"selector"`
	Direct   []string               `esper:"direct"`
	Nested   enumMethodNestedItem   `esper:"nested"`
	Groups   []enumMethodNestedItem `esper:"groups"`
}

func (s enumMethodNestedSource) GetCandidates() []string {
	return append([]string(nil), s.Direct...)
}

func TestEnumerableMethodPropertyAndNestedSourcesMatchJava(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[enumMethodNestedSource](env, "EnumMethodNestedSource"); err != nil {
		t.Fatal(err)
	}

	selector := Field[enumMethodNestedSource, string]("selector")
	matchString := Equal[string](EnumElement[string](), selector)
	directValues := Method[[]string](EventValue[enumMethodNestedSource](), "GetCandidates")
	directAny := EnumAnyOf[string](directValues, matchString)

	nestedObject := Property[enumMethodNestedItem](EventValue[enumMethodNestedSource](), "nested")
	nestedValues := Property[[]string](nestedObject, "values")
	nestedAny := EnumAnyOf[string](nestedValues, matchString)

	groupValues := Field[enumMethodNestedSource, []enumMethodNestedItem]("groups")
	groupItemValues := Property[[]string](EnumElement[enumMethodNestedItem](), "values")
	groupAny := EnumAnyOf[enumMethodNestedItem](groupValues, EnumAnyOf[string](groupItemValues, matchString))

	plan, err := env.Build(Select(From[enumMethodNestedSource](env, "EnumMethodNestedSource"),
		Alias("directAny", directAny),
		Alias("nestedAny", nestedAny),
		Alias("groupAny", groupAny),
	).Query(StatementName("enum-method-nested-sources")))
	if err != nil {
		t.Fatal(err)
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
				t.Fatalf("method/nested enum result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	events := []enumMethodNestedSource{
		{
			Selector: "b",
			Direct:   []string{"a", "b"},
			Nested:   enumMethodNestedItem{Values: []string{"c", "b"}},
			Groups: []enumMethodNestedItem{
				{Values: []string{"x"}},
				{Values: []string{"z", "b"}},
			},
		},
		{
			Selector: "q",
			Direct:   []string{"a", "b"},
			Nested:   enumMethodNestedItem{Values: []string{"c", "b"}},
			Groups: []enumMethodNestedItem{
				{Values: []string{"x"}},
				{Values: []string{"z", "b"}},
			},
		},
	}
	for _, event := range events {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("method/nested enum row count = %d", len(rows))
	}
	for _, name := range []string{"directAny", "nestedAny", "groupAny"} {
		if rows[0].Get(name).Any() != true || rows[1].Get(name).Any() != false {
			t.Fatalf("method/nested enum %s trace = %#v, %#v", name, rows[0].Get(name), rows[1].Get(name))
		}
	}
}
