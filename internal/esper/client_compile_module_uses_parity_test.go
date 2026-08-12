package esper

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type clientCompileModuleUsesEvent struct {
	ID string `esper:"id"`
}

func moduleOrderItem(id, name string, uses ...string) ModuleOrderItem[string] {
	return NewModuleOrderItem(name, id, uses...)
}

func moduleOrderValues(items []ModuleOrderItem[string]) []string {
	values := make([]string, len(items))
	for index, item := range items {
		values[index] = item.Value
	}
	return values
}

func assertModuleOrder(t *testing.T, items []ModuleOrderItem[string], expected []string, options ...ModuleOrderOption) {
	t.Helper()
	ordered, err := OrderModules(items, nil, options...)
	if err != nil {
		t.Fatal(err)
	}
	if actual := moduleOrderValues(ordered); !reflect.DeepEqual(actual, expected) {
		t.Fatalf("module order = %v, want %v", actual, expected)
	}
}

func TestClientCompileModuleUsesOrderMatchesEsper(t *testing.T) {
	assertModuleOrder(t, []ModuleOrderItem[string]{
		moduleOrderItem("C", "C", "A", "B", "D"),
		moduleOrderItem("D", "D", "A", "B"),
		moduleOrderItem("B", "B", "A"),
		moduleOrderItem("A", "A"),
	}, []string{"A", "B", "D", "C"})

	assertModuleOrder(t, nil, []string{})
	assertModuleOrder(t, []ModuleOrderItem[string]{moduleOrderItem("A", "A")}, []string{"A"})
	assertModuleOrder(t, []ModuleOrderItem[string]{
		moduleOrderItem("B", "B"),
		moduleOrderItem("A", "A", "B"),
	}, []string{"B", "A"})

	three := []ModuleOrderItem[string]{
		moduleOrderItem("B", "B"),
		moduleOrderItem("C", "C", "B"),
		moduleOrderItem("D", "D"),
	}
	assertModuleOrder(t, three, []string{"B", "C", "D"})
	assertModuleOrder(t, []ModuleOrderItem[string]{three[2], three[1], three[0]}, []string{"B", "D", "C"})

	assertModuleOrder(t, []ModuleOrderItem[string]{
		moduleOrderItem("C", "C", "D"),
		moduleOrderItem("B", "B"),
		moduleOrderItem("A", "A", "B"),
		moduleOrderItem("D", "D"),
	}, []string{"B", "D", "C", "A"})

	five := map[string]ModuleOrderItem[string]{
		"A": moduleOrderItem("A", "A", "C"),
		"B": moduleOrderItem("B", "B"),
		"C": moduleOrderItem("C", "C", "B"),
		"D": moduleOrderItem("D", "D", "C", "E"),
		"E": moduleOrderItem("E", "E"),
	}
	assertModuleOrder(t, []ModuleOrderItem[string]{five["A"], five["B"], five["C"], five["D"], five["E"]}, []string{"B", "C", "E", "A", "D"})
	assertModuleOrder(t, []ModuleOrderItem[string]{five["B"], five["E"], five["C"], five["A"], five["D"]}, []string{"B", "E", "C", "A", "D"})
	assertModuleOrder(t, []ModuleOrderItem[string]{five["A"], five["D"], five["E"], five["C"], five["B"]}, []string{"B", "E", "C", "A", "D"})

	assertModuleOrder(t, []ModuleOrderItem[string]{
		moduleOrderItem("anonymous-a", "", "C", "A", "B", "D"),
		moduleOrderItem("anonymous-b", "", "C"),
		moduleOrderItem("A", "A"),
		moduleOrderItem("B", "B", "A", "C"),
		moduleOrderItem("C", "C"),
	}, []string{"A", "C", "B", "anonymous-a", "anonymous-b"}, AllowUnresolvedModuleUses())

	assertModuleOrder(t, []ModuleOrderItem[string]{
		moduleOrderItem("A-first", "A", "C"),
		moduleOrderItem("B", "B", "C"),
		moduleOrderItem("A-second", "A", "B"),
		moduleOrderItem("D", "D", "A"),
		moduleOrderItem("C", "C"),
	}, []string{"C", "B", "A-first", "A-second", "D"}, AllowUnresolvedModuleUses())
}

func TestClientCompileModuleUsesCircularMatchesEsper(t *testing.T) {
	threeCycle := []ModuleOrderItem[string]{
		moduleOrderItem("C", "C", "D"),
		moduleOrderItem("D", "D", "B"),
		moduleOrderItem("B", "B", "C"),
	}
	if _, err := OrderModules(threeCycle, nil); err == nil || !errors.Is(err, ErrorDependency) || !strings.Contains(err.Error(), "C -> D -> B") {
		t.Fatalf("three-module cycle error = %v", err)
	}

	assertModuleOrder(t, []ModuleOrderItem[string]{
		moduleOrderItem("C", "C", "D"),
		moduleOrderItem("D", "D", "B"),
		moduleOrderItem("B", "B", "B"),
	}, []string{"B", "D", "C"})

	twoCycle := []ModuleOrderItem[string]{
		moduleOrderItem("C", "C", "B"),
		moduleOrderItem("B", "B", "C"),
	}
	if _, err := OrderModules(twoCycle, nil); err == nil || !errors.Is(err, ErrorDependency) || !strings.Contains(err.Error(), "C -> B") {
		t.Fatalf("two-module cycle error = %v", err)
	}
	assertModuleOrder(t, twoCycle, []string{"B", "C"}, AllowCircularModuleUses())
}

func TestClientCompileModuleUsesUnresolvedMatchesEsper(t *testing.T) {
	if _, err := OrderModules([]ModuleOrderItem[string]{moduleOrderItem("B", "B", "C")}, nil); err == nil || !errors.Is(err, ErrorDependency) || !strings.Contains(err.Error(), `module "B"`) || !strings.Contains(err.Error(), `module "C"`) {
		t.Fatalf("single unresolved dependency error = %v", err)
	}

	chain := []ModuleOrderItem[string]{
		moduleOrderItem("B", "B", "C"),
		moduleOrderItem("C", "C", "D"),
		moduleOrderItem("D", "D", "x"),
	}
	if _, err := OrderModules(chain, nil); err == nil || !errors.Is(err, ErrorDependency) || !strings.Contains(err.Error(), `module "D"`) || !strings.Contains(err.Error(), `module "x"`) {
		t.Fatalf("chain unresolved dependency error = %v", err)
	}
	assertModuleOrder(t, chain, []string{"D", "C", "B"}, AllowUnresolvedModuleUses())

	ordered, err := OrderModules([]ModuleOrderItem[string]{moduleOrderItem("B", "B", "C")}, []string{"C"})
	if err != nil || !reflect.DeepEqual(moduleOrderValues(ordered), []string{"B"}) {
		t.Fatalf("deployed dependency order = %v, err=%v", moduleOrderValues(ordered), err)
	}
}

func TestClientCompileModuleUsesIgnorableNameMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[clientCompileModuleUsesEvent](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("MYVAR", 10); err != nil {
		t.Fatal(err)
	}
	path := env.Path().UsesNames("dummy", "dummy")
	variable, err := ModulePathVariableRef[int](path, "MYVAR")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := path.Build(Select(
		From[clientCompileModuleUsesEvent](env, "SupportBean"),
		Alias("value", variable),
	).Query(StatementName("uses-dummy")))
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.Query().ModuleUses(); !reflect.DeepEqual(got, []string{"dummy"}) || !strings.Contains(string(plan.Canonical()), "uses(dummy)") {
		t.Fatalf("ignorable uses identity = %v canonical=%s", got, plan.Canonical())
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var values []int
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			values = append(values, row.Get("value").Any().(int))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientCompileModuleUsesEvent{ID: "E1"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(values, []int{10}) {
		t.Fatalf("ignorable uses result = %v, want [10]", values)
	}

	invalidPath := env.Path().UsesNames(" ")
	if _, err := invalidPath.Build(From[clientCompileModuleUsesEvent](env, "SupportBean").Query()); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("blank uses name error = %v", err)
	}
}
