package esper

import (
	"context"
	"reflect"
	"testing"
)

func TestVariantPredefinedAcceptsInheritedMemberTypes(t *testing.T) {
	env := NewEnvironment()
	root, err := RegisterMap(env, "VariantRoot", []FieldSpec{
		FieldDef("common", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "VariantLeaf", []FieldSpec{
		FieldDef("leaf", reflect.TypeOf(int64(0))),
	}, WithSchemaParent(root)); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterVariant(env, "VariantRootOrDescendant", root); err != nil {
		t.Fatal(err)
	}

	stream := FromAny(env, "VariantRootOrDescendant").Filter(
		Equal[string](Field[any, string]("common"), Literal("keep")),
	)
	plan, err := env.Build(stream.Query(StatementName("variant-inherited-member")))
	if err != nil {
		t.Fatal(err)
	}
	routePlan, err := env.Build(FromAny(env, "VariantLeaf").Query(
		StatementName("variant-inherited-route"), RouteTo("VariantRootOrDescendant"),
	))
	if err != nil {
		t.Fatalf("inherited member route rejected: %v", err)
	}

	engine := NewEngine(env)
	listenerDeployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	routeDeployment, err := engine.Deploy(context.Background(), routePlan)
	if err != nil {
		t.Fatal(err)
	}
	var received []Event
	if _, err := listenerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("variant inherited result = %#v", result)
			}
			received = append(received, event)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendRecord(context.Background(), "VariantLeaf", map[string]any{"common": "keep", "leaf": int64(7)}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 || received[0].TypeName() != "VariantLeaf" || received[0].Get("common").Any() != "keep" || received[0].Get("leaf").Any() != int64(7) {
		t.Fatalf("variant inherited events = %#v", received)
	}
	if err := engine.Undeploy(context.Background(), routeDeployment.ID()); err != nil {
		t.Fatal(err)
	}
}
