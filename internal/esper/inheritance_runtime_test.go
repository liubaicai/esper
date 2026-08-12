package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

// TestSchemaInheritanceRoutesMultiLevelAndBranchedEvents mirrors the
// EventMapInheritanceRuntime and EventJsonInherits branched/multi-level
// executions. A parent source receives every descendant, while sibling
// branches stay isolated and shared ancestors are present only once.
func TestSchemaInheritanceRoutesMultiLevelAndBranchedEvents(t *testing.T) {
	env := NewEnvironment()
	stringType := reflect.TypeOf("")
	mapFields := func(fields ...string) []FieldSpec {
		result := make([]FieldSpec, 0, len(fields))
		for _, name := range fields {
			result = append(result, FieldDef(name, stringType))
		}
		return result
	}

	root, err := RegisterMap(env, "InheritanceRoot", mapFields("base"))
	if err != nil {
		t.Fatal(err)
	}
	left, err := RegisterMap(env, "InheritanceLeft", mapFields("left"), WithSchemaParent(root))
	if err != nil {
		t.Fatal(err)
	}
	right, err := RegisterMap(env, "InheritanceRight", mapFields("right"), WithSchemaParent(root))
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := RegisterMap(env, "InheritanceLeaf", mapFields("leaf"), WithSchemaParent(left), WithSchemaParent(right))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := leaf.PropertyNames(), []string{"base", "left", "right", "leaf"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("branched inherited properties = %#v, want %#v", got, want)
	}

	engine := NewEngine(env)
	type observedEvent struct {
		typeName string
		base     any
	}
	observed := make(map[string][]observedEvent)
	for _, eventType := range []string{"InheritanceRoot", "InheritanceLeft", "InheritanceRight", "InheritanceLeaf"} {
		eventType := eventType
		plan, buildErr := env.Build(FromAny(env, eventType).Query(StatementName("inherit-" + eventType)))
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		deployment, deployErr := engine.Deploy(context.Background(), plan)
		if deployErr != nil {
			t.Fatal(deployErr)
		}
		if _, subscribeErr := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				event, ok := result.Event()
				if !ok {
					t.Errorf("inheritance result for %s is not an event: %#v", eventType, result)
					continue
				}
				observed[eventType] = append(observed[eventType], observedEvent{typeName: event.TypeName(), base: event.Get("base").Any()})
			}
			return nil
		}); subscribeErr != nil {
			t.Fatal(subscribeErr)
		}
	}

	send := func(eventType string, record map[string]any) {
		t.Helper()
		if sendErr := engine.SendRecord(context.Background(), eventType, record); sendErr != nil {
			t.Fatalf("send %s: %v", eventType, sendErr)
		}
	}
	assertCounts := func(want map[string]int) {
		t.Helper()
		for eventType, count := range want {
			if got := len(observed[eventType]); got != count {
				t.Fatalf("observations for %s = %d, want %d (%#v)", eventType, got, count, observed[eventType])
			}
		}
	}

	send("InheritanceLeaf", map[string]any{"base": "B1", "left": "L1", "right": "R1", "leaf": "F1"})
	assertCounts(map[string]int{"InheritanceRoot": 1, "InheritanceLeft": 1, "InheritanceRight": 1, "InheritanceLeaf": 1})
	for _, eventType := range []string{"InheritanceRoot", "InheritanceLeft", "InheritanceRight", "InheritanceLeaf"} {
		got := observed[eventType][0]
		if got.typeName != "InheritanceLeaf" || got.base != "B1" {
			t.Fatalf("leaf event observed by %s = %#v", eventType, got)
		}
	}

	send("InheritanceLeft", map[string]any{"base": "B2", "left": "L2"})
	assertCounts(map[string]int{"InheritanceRoot": 2, "InheritanceLeft": 2, "InheritanceRight": 1, "InheritanceLeaf": 1})
	if got := observed["InheritanceLeft"][1]; got.typeName != "InheritanceLeft" || got.base != "B2" {
		t.Fatalf("left event observation = %#v", got)
	}

	// A sibling event is accepted by the shared root and its own branch, but
	// never by the other branch or the deeper leaf.
	send("InheritanceRight", map[string]any{"base": "B3", "right": "R3"})
	assertCounts(map[string]int{"InheritanceRoot": 3, "InheritanceLeft": 2, "InheritanceRight": 2, "InheritanceLeaf": 1})

	jsonRoot, err := RegisterJSON(env, "InheritanceJSONRoot", []FieldSpec{FieldDef("base", stringType)})
	if err != nil {
		t.Fatal(err)
	}
	jsonChild, err := RegisterJSON(env, "InheritanceJSONChild", []FieldSpec{FieldDef("child", stringType)}, WithSchemaParent(jsonRoot))
	if err != nil {
		t.Fatal(err)
	}
	jsonLeaf, err := RegisterJSON(env, "InheritanceJSONLeaf", []FieldSpec{FieldDef("leaf", stringType)}, WithSchemaParent(jsonChild))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := jsonLeaf.PropertyNames(), []string{"base", "child", "leaf"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON inherited properties = %#v, want %#v", got, want)
	}
	jsonPlan, err := env.Build(FromAny(env, "InheritanceJSONRoot").Query(StatementName("inherit-json-root")))
	if err != nil {
		t.Fatal(err)
	}
	jsonDeployment, err := engine.Deploy(context.Background(), jsonPlan)
	if err != nil {
		t.Fatal(err)
	}
	var jsonEvents []Event
	if _, err := jsonDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				jsonEvents = append(jsonEvents, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendJSON(context.Background(), "InheritanceJSONLeaf", []byte(`{"base":"JB","child":"JC","leaf":"JL"}`)); err != nil {
		t.Fatal(err)
	}
	if len(jsonEvents) != 1 || jsonEvents[0].TypeName() != "InheritanceJSONLeaf" || jsonEvents[0].Get("leaf").Any() != "JL" {
		t.Fatalf("JSON descendant observations = %#v", jsonEvents)
	}

	objectRoot, err := RegisterObjectArray(env, "InheritanceObjectRoot", []FieldSpec{FieldDef("base", stringType)})
	if err != nil {
		t.Fatal(err)
	}
	objectLeft, err := RegisterObjectArray(env, "InheritanceObjectLeft", []FieldSpec{FieldDef("left", stringType)}, WithSchemaParent(objectRoot))
	if err != nil {
		t.Fatal(err)
	}
	objectLeaf, err := RegisterObjectArray(env, "InheritanceObjectLeaf", []FieldSpec{FieldDef("leaf", stringType)}, WithSchemaParent(objectLeft))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := objectLeaf.PropertyNames(), []string{"base", "left", "leaf"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ObjectArray inherited properties = %#v, want %#v", got, want)
	}
	objectEvent, err := ParseObjectArray(objectLeaf, []any{"OB", "OL", "OF"}, engine.Now())
	if err != nil {
		t.Fatal(err)
	}
	if objectEvent.Get("base").Any() != "OB" || objectEvent.Get("left").Any() != "OL" || objectEvent.Get("leaf").Any() != "OF" {
		t.Fatalf("ObjectArray inherited values = %#v", objectEvent.Underlying())
	}
}

// TestJSONSchemaInheritancePreservesNestedFragmentsAndDynamicProperties
// mirrors EventJsonInheritsTwoLevelWArrayAndObject and the two dynamic-property
// executions. Nested schemas declared by a parent remain visible to a child,
// while dynamic lookup follows the Java parent-only/child-only policy.
func TestJSONSchemaInheritancePreservesNestedFragmentsAndDynamicProperties(t *testing.T) {
	stringType := reflect.TypeOf("")
	nested, err := NewJSONSchema("InheritanceNestedObject", []FieldSpec{
		FieldDef("n1", stringType),
	})
	if err != nil {
		t.Fatal(err)
	}
	parent, err := NewJSONSchema("InheritanceNestedParent", []FieldSpec{
		FieldDef("pn", reflect.TypeOf(map[string]any{})),
		FieldDef("pa", reflect.TypeOf([]int{})),
	}, WithNestedPropertySchema("pn", nested))
	if err != nil {
		t.Fatal(err)
	}
	child, err := NewJSONSchema("InheritanceNestedChild", []FieldSpec{
		FieldDef("cn", reflect.TypeOf(map[string]any{})),
		FieldDef("ca", reflect.TypeOf([]int{})),
	}, WithSchemaParent(parent), WithNestedPropertySchema("cn", nested))
	if err != nil {
		t.Fatal(err)
	}

	event, err := ParseJSON(child, []byte(`{
		"pn":{"n1":"a"}, "pa":[1,2],
		"cn":{"n1":"b"}, "ca":[3,4]
	}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := child.PropertyNames(), []string{"pn", "pa", "cn", "ca"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("nested inherited property order = %#v, want %#v", got, want)
	}
	if got := event.Get("pn.n1").Any(); got != "a" {
		t.Fatalf("inherited nested property = %#v, want a", got)
	}
	if got := event.Get("ca[1]").Any(); got != 4 {
		t.Fatalf("inherited child array property = %#v, want 4", got)
	}
	fragment, ok := event.GetFragment("pn")
	if !ok || fragment.TypeName() != nested.Name() || fragment.Get("n1").Any() != "a" {
		t.Fatalf("inherited nested fragment = %#v, ok=%t", fragment, ok)
	}
	if got := event.Get("cn.n1").Any(); got != "b" {
		t.Fatalf("child nested property = %#v, want b", got)
	}

	parentDynamic, err := NewJSONSchema("InheritanceDynamicParent", nil, AllowDynamicFields())
	if err != nil {
		t.Fatal(err)
	}
	childFromParent, err := NewJSONSchema("InheritanceDynamicChild", nil, WithSchemaParent(parentDynamic))
	if err != nil {
		t.Fatal(err)
	}
	parentEvent, err := ParseJSON(childFromParent, []byte(`{"value":10}`), time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got := parentEvent.Get("value").Any(); got != 10 {
		t.Fatalf("parent-provided dynamic value = %#v, want 10", got)
	}
	if descriptor, ok := childFromParent.Property("value"); !ok || descriptor.Kind != PropertyDynamic {
		t.Fatalf("parent-provided dynamic descriptor = %#v, ok=%t", descriptor, ok)
	}

	staticParent, err := NewJSONSchema("InheritanceStaticParent", nil)
	if err != nil {
		t.Fatal(err)
	}
	childDynamic, err := NewJSONSchema("InheritanceChildDynamic", nil, WithSchemaParent(staticParent), AllowDynamicFields())
	if err != nil {
		t.Fatal(err)
	}
	childEvent, err := ParseJSON(childDynamic, []byte(`{"value":"abc"}`), time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got := childEvent.Get("value").Any(); got != "abc" {
		t.Fatalf("child-provided dynamic value = %#v, want abc", got)
	}

	env := NewEnvironment()
	if err := env.RegisterSchema(parentDynamic); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(childFromParent); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, parentDynamic.Name()).Query(StatementName("inherit-dynamic-parent")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var routed []Event
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				routed = append(routed, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendJSON(context.Background(), childFromParent.Name(), []byte(`{"value":20}`)); err != nil {
		t.Fatal(err)
	}
	if len(routed) != 1 || routed[0].TypeName() != childFromParent.Name() || routed[0].Get("value").Any() != 20 {
		t.Fatalf("dynamic inherited runtime route = %#v", routed)
	}
}
