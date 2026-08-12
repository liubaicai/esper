package esper

import (
	"context"
	"reflect"
	"testing"
)

// Parity coverage for PatternComplexPropertyAccess (see docs
// esper-go-port-implementation-plan.md): mapped, indexed, nested and
// combined property access inside pattern filter predicates. Esper's
// property-path syntax (mapped('k'), indexed[1], nested.value) maps to
// composed analyzable expressions in the fluent API: MapValue/ArrayAt for
// keyed and indexed access and Property for nested navigation, with
// out-of-range and missing-key access evaluating to Null exactly like
// Esper's safe property getters.

type patternComplexNestedNested struct {
	NestedNestedValue string `esper:"nestedNestedValue"`
}

type patternComplexNested struct {
	NestedValue  string                     `esper:"nestedValue"`
	NestedNested patternComplexNestedNested `esper:"nestedNested"`
}

type patternComplexProps struct {
	SimpleProperty string               `esper:"simpleProperty"`
	Mapped         map[string]string    `esper:"mapped"`
	Indexed        []int                `esper:"indexed"`
	MapProperty    map[string]string    `esper:"mapProperty"`
	ArrayProperty  []int                `esper:"arrayProperty"`
	Nested         patternComplexNested `esper:"nested"`
}

type patternCombinedNestedTwo struct {
	Value string `esper:"value"`
}

type patternCombinedNestedOne struct {
	Mapped map[string]patternCombinedNestedTwo `esper:"mapped"`
}

type patternCombinedProps struct {
	Indexed []patternCombinedNestedOne `esper:"indexed"`
	Array   []patternCombinedNestedOne `esper:"array"`
}

func defaultComplexProps() patternComplexProps {
	return patternComplexProps{
		SimpleProperty: "simple",
		Mapped:         map[string]string{"keyOne": "valueOne", "keyTwo": "valueTwo"},
		Indexed:        []int{1, 2},
		MapProperty:    map[string]string{"xOne": "yOne", "xTwo": "yTwo"},
		ArrayProperty:  []int{10, 20, 30},
		Nested: patternComplexNested{
			NestedValue:  "nestedValue",
			NestedNested: patternComplexNestedNested{NestedNestedValue: "nestedNestedValue"},
		},
	}
}

func defaultCombinedProps() patternCombinedProps {
	nested := []patternCombinedNestedOne{
		{Mapped: map[string]patternCombinedNestedTwo{"0ma": {Value: "0ma0"}, "0mb": {Value: "0ma1"}}},
		{Mapped: map[string]patternCombinedNestedTwo{"1ma": {Value: "1ma0"}, "1mb": {Value: "1ma1"}}},
		{Mapped: map[string]patternCombinedNestedTwo{"2ma": {Value: "valueOne"}, "2mb": {Value: "2ma1"}}},
		{}, // [3] left empty on purpose, like the Java bean
	}
	return patternCombinedProps{Indexed: nested, Array: nested}
}

type patternCpStreams struct {
	complex  Stream[patternComplexProps]
	combined Stream[patternCombinedProps]
}

func newPatternCpEnv(t *testing.T) (*Environment, *Engine, patternCpStreams) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[patternComplexProps](env, "SupportBeanComplexProps"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternCombinedProps](env, "SupportBeanCombinedProps"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	return env, engine, patternCpStreams{
		complex:  From[patternComplexProps](env, "SupportBeanComplexProps"),
		combined: From[patternCombinedProps](env, "SupportBeanCombinedProps"),
	}
}

// runPatternCpCase deploys one filter pattern and replays the Java event
// set (e1 = default complex bean, e2 = default combined bean), collecting
// the ids of the events that fired it.
func runPatternCpCase(t *testing.T, build func(s patternCpStreams) PatternStream, want []string) {
	t.Helper()
	env, engine, streams := newPatternCpEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	plan, err := env.Build(build(streams).Select(Alias("fired", Literal(1))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	fires := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		fires += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var buckets []string
	if err := engine.SendEvent(context.Background(), defaultComplexProps()); err != nil {
		t.Fatal(err)
	}
	if fires > 0 {
		buckets = append(buckets, "e1")
	}
	before := fires
	if err := engine.SendEvent(context.Background(), defaultCombinedProps()); err != nil {
		t.Fatal(err)
	}
	if fires > before {
		buckets = append(buckets, "e2")
	}
	if len(buckets) != len(want) {
		t.Fatalf("fired buckets = %v, want %v", buckets, want)
	}
	for index := range want {
		if buckets[index] != want[index] {
			t.Fatalf("fired buckets = %v, want %v", buckets, want)
		}
	}
}

func cpMappedValue(key string) Expression[string] {
	return MapValue[string](Field[patternComplexProps, map[string]string]("mapped"), Literal(key))
}

func cpIndexedValue(index int) Expression[int] {
	return ArrayAt[int](Field[patternComplexProps, []int]("indexed"), Literal(index))
}

func cpArrayPropertyValue(index int) Expression[int] {
	return ArrayAt[int](Field[patternComplexProps, []int]("arrayProperty"), Literal(index))
}

func cmbMappedProperty(field string, index int, key string) Expression[string] {
	return Property[string](
		MapValue[patternCombinedNestedTwo](
			Property[map[string]patternCombinedNestedTwo](
				ArrayAt[patternCombinedNestedOne](Field[patternCombinedProps, []patternCombinedNestedOne](field), Literal(index)),
				"mapped"),
			Literal(key)),
		"value")
}

// TestPatternComplexPropertiesMatchesEsper mirrors PatternComplexProperties:
// every filter form of the Java case list, including the no-fire cases for
// wrong values, missing map keys and out-of-range indexes.
func TestPatternComplexPropertiesMatchesEsper(t *testing.T) {
	cases := []struct {
		name  string
		build func(s patternCpStreams) PatternStream
		want  []string
	}{
		{"mapped-key", func(s patternCpStreams) PatternStream {
			return PatternFrom(s.complex, "s", Equal[string](cpMappedValue("keyOne"), Literal("valueOne")))
		}, []string{"e1"}},
		{"indexed-1-eq-2", func(s patternCpStreams) PatternStream {
			return PatternFrom(s.complex, "s", Equal[int](cpIndexedValue(1), Literal(2)))
		}, []string{"e1"}},
		{"indexed-0-eq-2-no-fire", func(s patternCpStreams) PatternStream {
			return PatternFrom(s.complex, "s", Equal[int](cpIndexedValue(0), Literal(2)))
		}, nil},
		{"array-1-eq-20", func(s patternCpStreams) PatternStream {
			return PatternFrom(s.complex, "s", Equal[int](cpArrayPropertyValue(1), Literal(20)))
		}, []string{"e1"}},
		{"array-1-in-range", func(s patternCpStreams) PatternStream {
			return PatternFrom(s.complex, "s", BetweenOf(cpArrayPropertyValue(1), Literal(10), Literal(30)))
		}, []string{"e1"}},
		{"array-2-eq-20-no-fire", func(s patternCpStreams) PatternStream {
			return PatternFrom(s.complex, "s", Equal[int](cpArrayPropertyValue(2), Literal(20)))
		}, nil},
		{"nested-value", func(s patternCpStreams) PatternStream {
			return PatternFrom(s.complex, "s", Equal[string](
				Property[string](Field[patternComplexProps, patternComplexNested]("nested"), "nestedValue"),
				Literal("nestedValue")))
		}, []string{"e1"}},
		{"nested-value-no-fire", func(s patternCpStreams) PatternStream {
			return PatternFrom(s.complex, "s", Equal[string](
				Property[string](Field[patternComplexProps, patternComplexNested]("nested"), "nestedValue"),
				Literal("dummy")))
		}, nil},
		{"nested-nested-value", func(s patternCpStreams) PatternStream {
			return PatternFrom(s.complex, "s", Equal[string](
				Property[string](
					Property[patternComplexNestedNested](Field[patternComplexProps, patternComplexNested]("nested"), "nestedNested"),
					"nestedNestedValue"),
				Literal("nestedNestedValue")))
		}, []string{"e1"}},
		{"nested-nested-value-no-fire", func(s patternCpStreams) PatternStream {
			return PatternFrom(s.complex, "s", Equal[string](
				Property[string](
					Property[patternComplexNestedNested](Field[patternComplexProps, patternComplexNested]("nested"), "nestedNested"),
					"nestedNestedValue"),
				Literal("x")))
		}, nil},
		{"combined-indexed-mapped", func(s patternCpStreams) PatternStream {
			return PatternFrom(s.combined, "s", Equal[string](cmbMappedProperty("indexed", 1, "1mb"), Literal("1ma1")))
		}, []string{"e2"}},
		{"combined-indexed-mapped-no-fire", func(s patternCpStreams) PatternStream {
			return PatternFrom(s.combined, "s", Equal[string](cmbMappedProperty("indexed", 0, "1ma"), Literal("x")))
		}, nil},
		{"combined-array-mapped", func(s patternCpStreams) PatternStream {
			return PatternFrom(s.combined, "s", Equal[string](cmbMappedProperty("array", 0, "0ma"), Literal("0ma0")))
		}, []string{"e2"}},
		{"combined-array-mapped-missing-key", func(s patternCpStreams) PatternStream {
			return PatternFrom(s.combined, "s", Equal[string](cmbMappedProperty("array", 2, "x"), Literal("x")))
		}, nil},
		{"combined-array-out-of-range", func(s patternCpStreams) PatternStream {
			return PatternFrom(s.combined, "s", Equal[string](cmbMappedProperty("array", 879787, "x"), Literal("x")))
		}, nil},
		{"combined-array-unknown-key", func(s patternCpStreams) PatternStream {
			return PatternFrom(s.combined, "s", Equal[string](cmbMappedProperty("array", 0, "xxx"), Literal("x")))
		}, nil},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			runPatternCpCase(t, testCase.build, testCase.want)
		})
	}
}

// TestPatternIndexedFilterPropMatchesEsper mirrors PatternIndexedFilterProp:
// every a=SupportBeanComplexProps(indexed[0]=3) fires only for events whose
// first indexed value is 3. Java asserts event identity with assertSame;
// Go events are values, so the assertion compares the captured event's
// underlying value instead.
func TestPatternIndexedFilterPropMatchesEsper(t *testing.T) {
	env, engine, streams := newPatternCpEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	pattern := PatternFrom(streams.complex, "a", Equal[int](cpIndexedValue(0), Literal(3))).Every()
	plan, err := env.Build(pattern.Select(Alias("a", PatternEvent("a"))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var captured []patternComplexProps
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatal("expected row")
			}
			event, ok := row.Get("a").Any().(Event)
			if !ok {
				t.Fatal("expected captured event")
			}
			underlying, ok := event.Underlying().(patternComplexProps)
			if !ok {
				t.Fatalf("unexpected underlying %T", event.Underlying())
			}
			captured = append(captured, underlying)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	eventOne := patternComplexProps{Indexed: []int{3, 4}}
	eventTwo := patternComplexProps{Indexed: []int{6}}
	eventThree := patternComplexProps{Indexed: []int{3}}
	for _, event := range []patternComplexProps{eventOne, eventTwo, eventThree} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(captured) != 2 {
		t.Fatalf("captured = %v, want two events", captured)
	}
	if !reflect.DeepEqual(captured[0], eventOne) || !reflect.DeepEqual(captured[1], eventThree) {
		t.Fatalf("captured = %v, want [eventOne eventThree]", captured)
	}
}

// TestPatternIndexedValuePropMatchesEsper mirrors PatternIndexedValueProp,
// PatternIndexedValuePropOM and PatternIndexedValuePropCompile: every a ->
// b=SupportBeanComplexProps(indexed[0] = a.indexed[0]) correlates the
// followed-by filter with the captured tag's indexed property. All three
// Java executions compile the same pattern text through different front
// ends, so the fluent form replays the expectations three times.
func TestPatternIndexedValuePropMatchesEsper(t *testing.T) {
	for _, variant := range []string{"epl", "object-model", "compile"} {
		t.Run(variant, func(t *testing.T) {
			env, engine, streams := newPatternCpEnv(t)
			defer func() { _ = engine.Close(context.Background()) }()
			pattern := PatternFrom(streams.complex, "a", Literal(true)).Every().
				Then(PatternFrom(streams.complex, "b", Equal[int](
					cpIndexedValue(0),
					ArrayAt[int](TagField[[]int]("a", "indexed"), Literal(0)))))
			plan, err := env.Build(pattern.Select(
				Alias("a", TagField[string]("a", "simpleProperty")),
				Alias("b", TagField[string]("b", "simpleProperty")),
			).Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			var fires [][2]string
			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
				for _, result := range batch.New {
					row, ok := result.Row()
					if !ok {
						t.Fatal("expected row")
					}
					a, _ := row.Get("a").Any().(string)
					b, _ := row.Get("b").Any().(string)
					fires = append(fires, [2]string{a, b})
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			// Java distinguishes the two {3} events by identity; Go events
			// are values, so simpleProperty carries the marker. The pattern
			// logic itself only reads indexed, exactly like the Java bean.
			for _, event := range []patternComplexProps{
				{SimpleProperty: "eventOne", Indexed: []int{3}},
				{SimpleProperty: "other", Indexed: []int{6}},
				{SimpleProperty: "eventTwo", Indexed: []int{3}},
			} {
				if err := engine.SendEvent(context.Background(), event); err != nil {
					t.Fatal(err)
				}
			}
			if len(fires) != 1 || fires[0] != [2]string{"eventOne", "eventTwo"} {
				t.Fatalf("fires = %v, want [[eventOne eventTwo]]", fires)
			}
		})
	}
}
