package esper

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"
)

type dataflowOptionRuntime struct{}

func (dataflowOptionRuntime) Process(_ context.Context, input DataflowInput) ([]DataflowEmission, error) {
	return []DataflowEmission{Emit(input.Value)}, nil
}

type dataflowInjectedRuntime struct{}

func (dataflowInjectedRuntime) Process(context.Context, DataflowInput) ([]DataflowEmission, error) {
	return []DataflowEmission{Emit("operator-injected")}, nil
}

type dataflowInjectedSource struct{}

func (dataflowInjectedSource) Run(ctx context.Context, emitter *DataflowEmitter) error {
	return emitter.Submit(ctx, "source-injected")
}

func TestDataflowParameterProviderMatchesEsperInstantiationOptions(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[runtimeTestTrade](env, "Trade")
	if err != nil {
		t.Fatal(err)
	}
	event, err := newEvent(schema, runtimeTestTrade{Symbol: "options", Price: 1}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	var captured DataflowOperatorContext
	customFactory := func(ctx DataflowOperatorContext) (DataflowOperatorRuntime, error) {
		captured = ctx
		return dataflowOptionRuntime{}, nil
	}
	definition, err := DefineDataflow(env, "parameter-options-flow").
		BeaconSource("source", event).
		CustomWithOptions("custom", customFactory, DataflowOperatorOptions{
			Properties:     map[string]any{"propOne": "abc", "propThree": "xyz"},
			ParameterNames: []string{"propOne", "propTwo", "propThree"},
		}).
		Emitter("sink").
		Connect("source", "custom").
		Connect("custom", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	var contexts []DataflowParameterContext
	instance, err := NewEngine(env).InstantiateDataflowWithOptions(context.Background(), definition, DataflowOptions{
		InstanceID: "parameter-instance",
		UserObject: "owner",
		ParameterProvider: func(ctx DataflowParameterContext) (any, bool) {
			contexts = append(contexts, ctx)
			if ctx.ParameterName == "propTwo" {
				return "def", true
			}
			return nil, false
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if captured.DataflowName != "parameter-options-flow" || captured.InstanceID != "parameter-instance" || captured.UserObject != "owner" || captured.OperatorName != "custom" || captured.OperatorNum != 1 {
		t.Fatalf("operator context = %#v", captured)
	}
	if captured.Factory.Kind != CustomKind || captured.Factory.OperatorFactory == nil || captured.Factory.SourceFactory != nil || captured.Factory.IsBuiltin() {
		t.Fatalf("operator factory metadata = %#v", captured.Factory)
	}
	if reflect.ValueOf(captured.Factory.OperatorFactory).Pointer() != reflect.ValueOf(customFactory).Pointer() {
		t.Fatal("operator context did not retain the definition factory")
	}
	if got := []string{captured.InputPorts[0].Name, captured.OutputPorts[0].Name}; !reflect.DeepEqual(got, []string{"in", "out"}) {
		t.Fatalf("operator context ports = %#v", captured)
	}
	if !reflect.DeepEqual(captured.Properties, map[string]any{"propOne": "abc", "propTwo": "def", "propThree": "xyz"}) {
		t.Fatalf("resolved properties = %#v", captured.Properties)
	}
	if len(contexts) != 6 {
		t.Fatalf("parameter callback count = %d, want 6: %#v", len(contexts), contexts)
	}
	if got := []string{contexts[0].ParameterName, contexts[1].ParameterName, contexts[2].ParameterName}; !reflect.DeepEqual(got, []string{"initialDelay", "interval", "iterations"}) {
		t.Fatalf("built-in parameter callback order = %#v", got)
	}
	for _, parameter := range contexts[:3] {
		if parameter.OperatorName != "source" || parameter.OperatorNum != 0 || parameter.Factory.Kind != BeaconSourceKind || !parameter.Factory.IsBuiltin() {
			t.Fatalf("built-in parameter context = %#v", parameter)
		}
	}
	customContexts := contexts[3:]
	if got := []string{customContexts[0].ParameterName, customContexts[1].ParameterName, customContexts[2].ParameterName}; !sort.StringsAreSorted(got) {
		t.Fatalf("parameter callback order = %#v, want deterministic order", got)
	}
	for _, parameter := range customContexts {
		if parameter.DataflowName != "parameter-options-flow" || parameter.InstanceID != "parameter-instance" || parameter.OperatorName != "custom" || parameter.OperatorNum != 1 {
			t.Fatalf("parameter context = %#v", parameter)
		}
		if parameter.Factory.Kind != CustomKind || parameter.Factory.OperatorFactory == nil || parameter.Factory.SourceFactory != nil || parameter.Factory.IsBuiltin() {
			t.Fatalf("parameter factory metadata = %#v", parameter.Factory)
		}
		if parameter.ParameterName == "propOne" && parameter.DefaultValue != "abc" {
			t.Fatalf("propOne default = %#v", parameter.DefaultValue)
		}
	}
	if outputs := instance.Outputs(); len(outputs) != 1 {
		t.Fatalf("parameter flow outputs = %#v", outputs)
	}
}

func TestDataflowOperatorAndSourceProvidersOverrideFactories(t *testing.T) {
	t.Run("operator", func(t *testing.T) {
		env := NewEnvironment()
		schema, err := RegisterStruct[runtimeTestTrade](env, "Trade")
		if err != nil {
			t.Fatal(err)
		}
		event, err := newEvent(schema, runtimeTestTrade{Symbol: "provider", Price: 2}, time.Unix(0, 0).UTC())
		if err != nil {
			t.Fatal(err)
		}
		factoryCalled := false
		var provided DataflowOperatorContext
		originalFactory := func(DataflowOperatorContext) (DataflowOperatorRuntime, error) {
			factoryCalled = true
			return nil, errors.New("factory should not be called")
		}
		definition, err := DefineDataflow(env, "operator-provider-flow").
			BeaconSource("source", event).
			CustomWithOptions("custom", originalFactory, DataflowOperatorOptions{Properties: map[string]any{"mode": "factory"}}).
			Emitter("sink").
			Connect("source", "custom").
			Connect("custom", "sink").
			Build()
		if err != nil {
			t.Fatal(err)
		}
		instance, err := NewEngine(env).InstantiateDataflowWithOptions(context.Background(), definition, DataflowOptions{
			OperatorProvider: func(ctx DataflowOperatorContext) (DataflowOperatorRuntime, error) {
				provided = ctx
				return dataflowInjectedRuntime{}, nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := instance.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		if factoryCalled {
			t.Fatal("operator factory was called despite provider override")
		}
		if provided.OperatorName != "custom" || provided.Properties["mode"] != "factory" {
			t.Fatalf("operator provider context = %#v", provided)
		}
		if provided.Factory.Kind != CustomKind || provided.Factory.OperatorFactory == nil || provided.Factory.SourceFactory != nil || provided.Factory.IsBuiltin() {
			t.Fatalf("operator provider factory metadata = %#v", provided.Factory)
		}
		if reflect.ValueOf(provided.Factory.OperatorFactory).Pointer() != reflect.ValueOf(originalFactory).Pointer() {
			t.Fatal("operator provider did not receive the definition factory")
		}
		if outputs := instance.Outputs(); len(outputs) != 1 || outputs[0] != "operator-injected" {
			t.Fatalf("operator provider outputs = %#v", outputs)
		}
	})

	t.Run("source", func(t *testing.T) {
		env := NewEnvironment()
		factoryCalled := false
		var provided DataflowOperatorContext
		originalFactory := func(DataflowOperatorContext) (DataflowSourceRuntime, error) {
			factoryCalled = true
			return nil, errors.New("source factory should not be called")
		}
		definition, err := DefineDataflow(env, "source-provider-flow").
			CustomSourceWithOptions("source", originalFactory, DataflowOperatorOptions{Properties: map[string]any{"mode": "factory"}}).
			Emitter("sink").
			Connect("source", "sink").
			Build()
		if err != nil {
			t.Fatal(err)
		}
		instance, err := NewEngine(env).InstantiateDataflowWithOptions(context.Background(), definition, DataflowOptions{
			SourceProvider: func(ctx DataflowOperatorContext) (DataflowSourceRuntime, error) {
				provided = ctx
				return dataflowInjectedSource{}, nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := instance.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		if factoryCalled {
			t.Fatal("source factory was called despite provider override")
		}
		if provided.OperatorName != "source" || provided.Properties["mode"] != "factory" {
			t.Fatalf("source provider context = %#v", provided)
		}
		if provided.Factory.Kind != CustomSourceKind || provided.Factory.SourceFactory == nil || provided.Factory.OperatorFactory != nil || provided.Factory.IsBuiltin() {
			t.Fatalf("source provider factory metadata = %#v", provided.Factory)
		}
		if reflect.ValueOf(provided.Factory.SourceFactory).Pointer() != reflect.ValueOf(originalFactory).Pointer() {
			t.Fatal("source provider did not receive the definition factory")
		}
		if outputs := instance.Outputs(); len(outputs) != 1 || outputs[0] != "source-injected" {
			t.Fatalf("source provider outputs = %#v", outputs)
		}
	})
}

func TestDataflowOperatorOptionsRejectDuplicateParameters(t *testing.T) {
	env := NewEnvironment()
	if _, err := DefineDataflow(env, "invalid-parameter-options").
		CustomWithOptions("custom", func(DataflowOperatorContext) (DataflowOperatorRuntime, error) {
			return dataflowOptionRuntime{}, nil
		}, DataflowOperatorOptions{ParameterNames: []string{"same", "same"}}).
		Build(); err == nil {
		t.Fatal("duplicate parameter names were accepted")
	}
}

func TestDataflowOperatorOptionsRejectNonCanonicalNames(t *testing.T) {
	env := NewEnvironment()
	factory := func(DataflowOperatorContext) (DataflowOperatorRuntime, error) {
		return dataflowOptionRuntime{}, nil
	}
	for _, options := range []DataflowOperatorOptions{
		{Properties: map[string]any{" ": 1}},
		{Properties: map[string]any{" mode ": 1}},
		{ParameterNames: []string{" "}},
		{ParameterNames: []string{" mode "}},
	} {
		if _, err := DefineDataflow(env, "invalid-operator-option-name").
			CustomWithOptions("custom", factory, options).
			Build(); err == nil {
			t.Fatalf("operator options accepted non-canonical name: %#v", options)
		}
	}
}

func TestDataflowCustomPropertiesAreTypedAndInstanceIsolated(t *testing.T) {
	env := NewEnvironment()
	settings := map[string]any{
		"parameterOne": "ValueOne",
		"labels":       []string{"a", "b"},
	}
	properties := map[string]any{
		"enabled":  true,
		"settings": settings,
		"catchAll": map[string]int{"limit": 10},
	}
	var contexts []DataflowOperatorContext
	definition, err := DefineDataflow(env, "custom-properties-flow").
		CustomWithOptions("custom", func(operator DataflowOperatorContext) (DataflowOperatorRuntime, error) {
			contexts = append(contexts, operator)
			return dataflowOptionRuntime{}, nil
		}, DataflowOperatorOptions{Properties: properties}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	settings["parameterOne"] = "mutated-after-build"
	settings["labels"].([]string)[0] = "mutated-after-build"
	properties["enabled"] = false
	engine := NewEngine(env)
	if _, err := engine.InstantiateDataflow(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	if len(contexts) != 1 {
		t.Fatalf("custom property contexts = %#v", contexts)
	}
	if enabled, ok := DataflowProperty[bool](contexts[0], "enabled"); !ok || !enabled {
		t.Fatalf("typed enabled property = %v, %t", enabled, ok)
	}
	if parameter, ok := DataflowProperty[string](contexts[0], "settings", "parameterOne"); !ok || parameter != "ValueOne" {
		t.Fatalf("typed nested property = %q, %t", parameter, ok)
	}
	labels, ok := DataflowProperty[[]string](contexts[0], "settings", "labels")
	if !ok || !reflect.DeepEqual(labels, []string{"a", "b"}) {
		t.Fatalf("typed array property = %#v, %t", labels, ok)
	}
	if limit, ok := DataflowProperty[int](contexts[0], "catchAll", "limit"); !ok || limit != 10 {
		t.Fatalf("typed catch-all property = %d, %t", limit, ok)
	}
	if _, ok := DataflowProperty[string](contexts[0], "settings", "missing"); ok {
		t.Fatal("missing nested property was reported present")
	}

	contexts[0].Properties["settings"].(map[string]any)["parameterOne"] = "mutated-instance"
	labels[0] = "mutated-instance"
	if _, err := engine.InstantiateDataflow(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	if parameter, ok := DataflowProperty[string](contexts[1], "settings", "parameterOne"); !ok || parameter != "ValueOne" {
		t.Fatalf("second instance nested property = %q, %t", parameter, ok)
	}
	if labels, ok := DataflowProperty[[]string](contexts[1], "settings", "labels"); !ok || !reflect.DeepEqual(labels, []string{"a", "b"}) {
		t.Fatalf("second instance array property = %#v, %t", labels, ok)
	}
	definitionProperties := definition.Operators()[0].Properties
	definitionProperties["settings"].(map[string]any)["parameterOne"] = "mutated-view"
	if parameter := definition.Operators()[0].Properties["settings"].(map[string]any)["parameterOne"]; parameter != "ValueOne" {
		t.Fatalf("definition property view leaked nested mutation: %#v", parameter)
	}
}

func TestDataflowCustomPropertiesHaveStablePlanIdentity(t *testing.T) {
	build := func(properties map[string]any, parameters []string) Plan {
		t.Helper()
		env := NewEnvironment()
		if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
			t.Fatal(err)
		}
		if _, err := DefineDataflow(env, "custom-property-plan").
			CustomWithOptions("custom", func(DataflowOperatorContext) (DataflowOperatorRuntime, error) {
				return dataflowOptionRuntime{}, nil
			}, DataflowOperatorOptions{Properties: properties, ParameterNames: parameters}).
			Build(); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query())
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}
	first := build(map[string]any{"mode": "one", "nested": map[string]any{"values": []int{1, 2}}}, []string{"second", "first"})
	reordered := build(map[string]any{"nested": map[string]any{"values": []int{1, 2}}, "mode": "one"}, []string{"first", "second"})
	differentValue := build(map[string]any{"mode": "two", "nested": map[string]any{"values": []int{1, 2}}}, []string{"first", "second"})
	differentParameter := build(map[string]any{"mode": "one", "nested": map[string]any{"values": []int{1, 2}}}, []string{"first"})
	if first.Hash() != reordered.Hash() {
		t.Fatalf("custom property insertion order changed plan hash: %s != %s", first.Hash(), reordered.Hash())
	}
	if first.Hash() == differentValue.Hash() || first.Hash() == differentParameter.Hash() {
		t.Fatalf("different custom property configuration shares plan hash %s", first.Hash())
	}
}
