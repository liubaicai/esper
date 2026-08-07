package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

type variantCoercionPrimitive struct {
	TheString     string  `esper:"theString"`
	BoolBoxed     *bool   `esper:"boolBoxed"`
	IntPrimitive  int     `esper:"intPrimitive"`
	LongPrimitive int64   `esper:"longPrimitive"`
	DoubleValue   float64 `esper:"doublePrimitive"`
	EnumValue     string  `esper:"enumValue"`
}

type variantCoercionWrapper struct {
	TheString     string  `esper:"theString"`
	BoolBoxed     bool    `esper:"boolBoxed"`
	IntPrimitive  *int    `esper:"intPrimitive"`
	LongPrimitive int     `esper:"longPrimitive"`
	DoubleValue   float32 `esper:"doublePrimitive"`
	EnumValue     string  `esper:"enumValue"`
}

type variantWrappedSource struct {
	ID    string `esper:"id"`
	Value int    `esper:"value"`
}

type variantWrappedSecondSource struct {
	ID string `esper:"id"`
}

type variantPatternCandidate struct {
	ID string `esper:"id"`
}

type variantRichCollection interface {
	Values() []string
}

type variantRichCollectionValue struct {
	Items []string
}

func (variantRichCollectionValue) Values() []string { return []string{"value"} }

type variantRichInner struct {
	Val string `esper:"val"`
}

type variantRichMemberOne struct{}

func (variantRichMemberOne) GetP0() variantInterfaceB       { return variantInterfaceBValue{} }
func (variantRichMemberOne) GetP1() variantInterfaceSuperG  { return variantInterfaceSuperGValue{} }
func (variantRichMemberOne) GetP2() variantRichCollection   { return variantRichCollectionValue{} }
func (variantRichMemberOne) GetP3() []string                { return []string{"p3-one"} }
func (variantRichMemberOne) GetP4() variantRichCollection   { return variantRichCollectionValue{} }
func (variantRichMemberOne) GetP5() variantRichCollection   { return variantRichCollectionValue{} }
func (variantRichMemberOne) GetIndexed() []int              { return []int{1, 2, 3} }
func (variantRichMemberOne) GetMapped() map[string]string   { return map[string]string{"a": "val1"} }
func (variantRichMemberOne) GetInneritem() variantRichInner { return variantRichInner{Val: "i1"} }

type variantRichMemberTwo struct{}

func (variantRichMemberTwo) GetP0() variantInterfaceBaseAB { return variantInterfaceBValue{} }
func (variantRichMemberTwo) GetP1() variantInterfaceSuperGPlus {
	return variantInterfaceSuperGPlusValue{}
}
func (variantRichMemberTwo) GetP2() variantRichCollectionValue { return variantRichCollectionValue{} }
func (variantRichMemberTwo) GetP3() []string                   { return []string{"p3-two"} }
func (variantRichMemberTwo) GetP4() variantRichCollectionValue { return variantRichCollectionValue{} }
func (variantRichMemberTwo) GetP5() variantRichCollection      { return variantRichCollectionValue{} }
func (variantRichMemberTwo) GetIndexed() []int                 { return []int{10, 20, 30} }
func (variantRichMemberTwo) GetMapped() map[string]string      { return map[string]string{"a": "val2"} }
func (variantRichMemberTwo) GetInneritem() variantRichInner    { return variantRichInner{Val: "i2"} }

func TestVariantCommonMetadataWideningMatchesJavaBoxedAndNumericProperties(t *testing.T) {
	env := NewEnvironment()
	primitive, err := RegisterStruct[variantCoercionPrimitive](env, "VariantPrimitive")
	if err != nil {
		t.Fatal(err)
	}
	wrapper, err := RegisterStruct[variantCoercionWrapper](env, "VariantWrapper")
	if err != nil {
		t.Fatal(err)
	}
	variant, err := RegisterVariant(env, "VariantCoercion", primitive, wrapper)
	if err != nil {
		t.Fatal(err)
	}

	boolField, ok := variant.Field("boolBoxed")
	if !ok || boolField.Type != reflect.TypeOf((*bool)(nil)) || !boolField.Optional {
		t.Fatalf("variant bool metadata = %#v, ok=%t", boolField, ok)
	}
	intField, ok := variant.Field("intPrimitive")
	if !ok || intField.Type != reflect.TypeOf((*int)(nil)) || !intField.Optional {
		t.Fatalf("variant int metadata = %#v, ok=%t", intField, ok)
	}
	longField, ok := variant.Field("longPrimitive")
	if !ok || longField.Type != reflect.TypeOf(int64(0)) {
		t.Fatalf("variant long metadata = %#v, ok=%t", longField, ok)
	}
	doubleField, ok := variant.Field("doublePrimitive")
	if !ok || doubleField.Type != reflect.TypeOf(float64(0)) {
		t.Fatalf("variant double metadata = %#v, ok=%t", doubleField, ok)
	}
	getter, ok := variant.Getter("longPrimitive")
	if !ok || getter.Type() != reflect.TypeOf(int64(0)) {
		t.Fatalf("variant getter metadata = %#v, ok=%t", getter.Property(), ok)
	}

	plan, err := env.Build(FromAny(env, "VariantCoercion").Filter(
		Equal[int64](Field[any, int64]("longPrimitive"), Literal[int64](7)),
	).Query(StatementName("variant-coercion")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var received []Event
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				received = append(received, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	primitiveValue := variantCoercionPrimitive{TheString: "p", LongPrimitive: 7}
	primitiveEvent, err := newEvent(primitive, primitiveValue, engine.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Route(context.Background(), "VariantCoercion", primitiveEvent); err != nil {
		t.Fatal(err)
	}
	wrapperValue := variantCoercionWrapper{TheString: "w", LongPrimitive: 7}
	wrapperEvent, err := newEvent(wrapper, wrapperValue, engine.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Route(context.Background(), "VariantCoercion", wrapperEvent); err != nil {
		t.Fatal(err)
	}
	if len(received) != 2 || received[0].Get("longPrimitive").Any() != int64(7) || received[1].Get("longPrimitive").Any() != int(7) {
		t.Fatalf("variant widened numeric events = %#v", received)
	}
}

func TestVariantMetadataAndGetterCacheCoversIndexedMappedAndFragments(t *testing.T) {
	env := NewEnvironment()
	one, err := RegisterStruct[variantRichMemberOne](env, "VariantRichOne", WithAccessorStyle(AccessorJavaBean))
	if err != nil {
		t.Fatal(err)
	}
	two, err := RegisterStruct[variantRichMemberTwo](env, "VariantRichTwo", WithAccessorStyle(AccessorJavaBean))
	if err != nil {
		t.Fatal(err)
	}
	variant, err := RegisterVariant(env, "VariantRich", one, two)
	if err != nil {
		t.Fatal(err)
	}

	baseType := reflect.TypeOf((*variantInterfaceBaseAB)(nil)).Elem()
	superGType := reflect.TypeOf((*variantInterfaceSuperG)(nil)).Elem()
	collectionType := reflect.TypeOf((*variantRichCollection)(nil)).Elem()
	wantTypes := map[string]reflect.Type{
		"p0":        baseType,
		"p1":        superGType,
		"p2":        collectionType,
		"p3":        reflect.TypeOf([]string{}),
		"p4":        collectionType,
		"p5":        collectionType,
		"indexed":   reflect.TypeOf([]int{}),
		"mapped":    reflect.TypeOf(map[string]string{}),
		"inneritem": reflect.TypeOf(variantRichInner{}),
	}
	for name, want := range wantTypes {
		field, ok := variant.Field(name)
		if !ok || field.Type != want {
			t.Fatalf("variant rich field %q = %#v, want %v", name, field, want)
		}
		getter, ok := variant.Getter(name)
		if !ok || getter.Type() != want {
			t.Fatalf("variant rich getter %q = %#v, want %v", name, getter, want)
		}
	}
	for name, want := range map[string]reflect.Type{
		"indexed[0]":    reflect.TypeOf(int(0)),
		"mapped('a')":   reflect.TypeOf(""),
		"inneritem.val": reflect.TypeOf(""),
	} {
		descriptor, ok := variant.Property(name)
		if !ok || descriptor.Type != want {
			t.Fatalf("variant rich property %q = %#v, want %v", name, descriptor, want)
		}
		getter, ok := variant.Getter(name)
		if !ok || getter.Type() != want {
			t.Fatalf("variant rich path getter %q = %#v, want %v", name, getter, want)
		}
	}

	firstEvent, err := newEvent(one, variantRichMemberOne{}, timeZero())
	if err != nil {
		t.Fatal(err)
	}
	indexedGetter, _ := variant.Getter("indexed[0]")
	mappedGetter, _ := variant.Getter("mapped('a')")
	innerGetter, _ := variant.Getter("inneritem.val")
	if got := indexedGetter.Get(firstEvent.Underlying()); got.Any() != 1 {
		t.Fatalf("indexed getter = %#v, want 1", got)
	}
	if got := mappedGetter.Get(firstEvent.Underlying()); got.Any() != "val1" {
		t.Fatalf("mapped getter = %#v, want val1", got)
	}
	if got := innerGetter.Get(firstEvent.Underlying()); got.Any() != "i1" {
		t.Fatalf("inner getter = %#v, want i1", got)
	}

	query := FromAny(env, "VariantRich").Select(
		Alias("p6", ArrayAt[int](Field[any, []int]("indexed"), Literal[int64](0))),
		Alias("p7", ArrayAt[int](Field[any, []int]("indexed"), Literal[int64](1))),
		Alias("p8", MapAt[string](Field[any, map[string]string]("mapped"), Literal("a"))),
		Alias("p9", Field[any, variantRichInner]("inneritem")),
		Alias("p10", Property[string](Field[any, variantRichInner]("inneritem"), "val")),
	).Query(StatementName("variant-rich-properties"))
	plan, err := env.Build(query)
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
			if ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	secondEvent, err := newEvent(two, variantRichMemberTwo{}, engine.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Route(context.Background(), "VariantRich", secondEvent); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("p6").Any() != 10 || rows[0].Get("p7").Any() != 20 || rows[0].Get("p8").Any() != "val2" || rows[0].Get("p10").Any() != "i2" {
		t.Fatalf("variant rich projected row = %#v", rows)
	}
}

func TestVariantAnyWrapperAndStaggeredDerivedStreamsMatchEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[variantWrappedSource](env, "WrappedSource"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[variantWrappedSecondSource](env, "WrappedSecondSource"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "WrappedOne", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
		FieldDef("value", reflect.TypeOf(int(0))),
		FieldDef("field", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "WrappedTwo", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
		FieldDef("field", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterVariantAny(env, "WrappedVariant"); err != nil {
		t.Fatal(err)
	}

	firstPlan, err := env.Build(Select(
		From[variantWrappedSource](env, "WrappedSource"),
		Alias("id", Field[variantWrappedSource, string]("id")),
		Alias("value", Field[variantWrappedSource, int]("value")),
		Alias("field", Literal("a")),
	).InsertInto("WrappedOne", StatementName("wrapped-one")))
	if err != nil {
		t.Fatal(err)
	}
	secondPlan, err := env.Build(FromAny(env, "WrappedOne").InsertInto("WrappedVariant", StatementName("wrapped-one-variant")))
	if err != nil {
		t.Fatal(err)
	}
	thirdPlan, err := env.Build(Select(
		From[variantWrappedSecondSource](env, "WrappedSecondSource"),
		Alias("id", Field[variantWrappedSecondSource, string]("id")),
		Alias("field", Literal("b")),
	).InsertInto("WrappedTwo", StatementName("wrapped-two")))
	if err != nil {
		t.Fatal(err)
	}
	fourthPlan, err := env.Build(FromAny(env, "WrappedTwo").InsertInto("WrappedVariant", StatementName("wrapped-two-variant")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromAny(env, "WrappedVariant").Window(KeepAll()).Query(StatementName("wrapped-consumer")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	for _, plan := range []Plan{firstPlan, secondPlan, thirdPlan, fourthPlan} {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	consumer, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var received []Event
	if _, err := consumer.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				received = append(received, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), variantWrappedSource{ID: "E1", Value: 3}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), variantWrappedSecondSource{ID: "E2"}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 2 {
		t.Fatalf("wrapped variant events = %#v", received)
	}
	if received[0].TypeName() != "WrappedOne" || received[0].StreamType() != "WrappedVariant" || received[0].Get("field").Any() != "a" || received[0].Get("value").Any() != int(3) {
		t.Fatalf("wrapped first event = %#v/%s/%s", received[0].Underlying(), received[0].TypeName(), received[0].StreamType())
	}
	if received[1].TypeName() != "WrappedTwo" || received[1].Get("field").Any() != "b" || received[1].Get("value").IsPresent() {
		t.Fatalf("wrapped second event = %#v/%s", received[1].Underlying(), received[1].TypeName())
	}
}

func TestVariantPatternAndSubqueryFlowsMatchEsper(t *testing.T) {
	env := NewEnvironment()
	order, err := RegisterStruct[variantOrder](env, "PatternVariantOrder")
	if err != nil {
		t.Fatal(err)
	}
	quote, err := RegisterStruct[variantQuote](env, "PatternVariantQuote")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[variantPatternCandidate](env, "PatternVariantCandidate"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterVariant(env, "PatternVariant", order, quote); err != nil {
		t.Fatal(err)
	}

	pattern := PatternFromRecord(
		FromAny(env, "PatternVariant"),
		"a",
		Literal(true),
	).FollowedBy("b", Literal(true))
	patternPlan, err := env.Build(pattern.Select(
		Alias("a", PatternEvent("a")),
		Alias("b", PatternEvent("b")),
	).Query(StatementName("variant-pattern")))
	if err != nil {
		t.Fatal(err)
	}
	inner := FromAny(env, "PatternVariant").Window(LastEvent())
	subqueryPlan, err := env.Build(Select(
		From[variantPatternCandidate](env, "PatternVariantCandidate").Filter(
			SubqueryExists(inner, Equal[string](
				Field[any, string]("id"),
				OuterField[string]("id"),
			)),
		),
		Alias("id", Field[variantPatternCandidate, string]("id")),
	).Query(StatementName("variant-subquery")))
	if err != nil {
		t.Fatal(err)
	}
	orderRoute, err := env.Build(From[variantOrder](env, "PatternVariantOrder").InsertInto("PatternVariant", StatementName("variant-pattern-order")))
	if err != nil {
		t.Fatal(err)
	}
	quoteRoute, err := env.Build(From[variantQuote](env, "PatternVariantQuote").InsertInto("PatternVariant", StatementName("variant-pattern-quote")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, plan := range []Plan{orderRoute, quoteRoute} {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	patternDeployment, err := engine.Deploy(context.Background(), patternPlan)
	if err != nil {
		t.Fatal(err)
	}
	subqueryDeployment, err := engine.Deploy(context.Background(), subqueryPlan)
	if err != nil {
		t.Fatal(err)
	}
	var patternRows, subqueryRows []Row
	if _, err := patternDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				patternRows = append(patternRows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := subqueryDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				subqueryRows = append(subqueryRows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), variantOrder{ID: "E1", Common: "pattern", Amount: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), variantQuote{ID: "E2", Common: "pattern", Bid: 2}); err != nil {
		t.Fatal(err)
	}
	if len(patternRows) != 1 {
		t.Fatalf("variant pattern rows = %#v", patternRows)
	}
	first, err := As[Event](patternRows[0].Get("a"))
	if err != nil || first.TypeName() != "PatternVariantOrder" || first.Get("id").Any() != "E1" {
		t.Fatalf("variant pattern first = %#v, err=%v", patternRows[0].Get("a"), err)
	}
	second, err := As[Event](patternRows[0].Get("b"))
	if err != nil || second.TypeName() != "PatternVariantQuote" || second.Get("id").Any() != "E2" {
		t.Fatalf("variant pattern second = %#v, err=%v", patternRows[0].Get("b"), err)
	}
	if err := engine.SendEvent(context.Background(), variantPatternCandidate{ID: "E1"}); err != nil {
		t.Fatal(err)
	}
	if len(subqueryRows) != 0 {
		t.Fatalf("variant subquery early rows = %#v", subqueryRows)
	}
	if err := engine.SendEvent(context.Background(), variantPatternCandidate{ID: "E2"}); err != nil {
		t.Fatal(err)
	}
	if len(subqueryRows) != 1 || subqueryRows[0].Get("id").Any() != "E2" {
		t.Fatalf("variant subquery rows = %#v", subqueryRows)
	}
}

func TestVariantAnyAcceptsLateRegisteredMapAndObjectArrayMembers(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterVariantAny(env, "LateVariant"); err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromAny(env, "LateVariant").Window(KeepAll()).Query(StatementName("late-variant-consumer")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "LateMap", []FieldSpec{FieldDef("id", reflect.TypeOf(""))}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterObjectArray(env, "LateArray", []FieldSpec{FieldDef("id", reflect.TypeOf(""))}); err != nil {
		t.Fatal(err)
	}
	mapRoute, err := env.Build(FromAny(env, "LateMap").InsertInto("LateVariant", StatementName("late-map-route")))
	if err != nil {
		t.Fatal(err)
	}
	arrayRoute, err := env.Build(FromAny(env, "LateArray").InsertInto("LateVariant", StatementName("late-array-route")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	consumer, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	for _, plan := range []Plan{mapRoute, arrayRoute} {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	var received []Event
	if _, err := consumer.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				received = append(received, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendRecord(context.Background(), "LateMap", map[string]any{"id": "M1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendObjectArray(context.Background(), "LateArray", []any{"A1"}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 2 || received[0].TypeName() != "LateMap" || received[1].TypeName() != "LateArray" || received[0].Get("id").Any() != "M1" || received[1].Get("id").Any() != "A1" {
		t.Fatalf("late variant events = %#v", received)
	}
}

func TestVariantAnyMetadataAndDynamicGetterMatchEsper(t *testing.T) {
	env := NewEnvironment()
	order, err := RegisterStruct[variantOrder](env, "MetadataAnyOrder")
	if err != nil {
		t.Fatal(err)
	}
	quote, err := RegisterStruct[variantQuote](env, "MetadataAnyQuote")
	if err != nil {
		t.Fatal(err)
	}
	variant, err := RegisterVariantAny(env, "MetadataAnyVariant")
	if err != nil {
		t.Fatal(err)
	}
	if names := variant.PropertyNames(); len(names) != 0 {
		t.Fatalf("ANY declared properties = %#v, want empty", names)
	}
	for _, name := range []string{"bid", "amount", "notDeclared"} {
		descriptor, ok := variant.Property(name)
		if !ok || descriptor.Kind != PropertyDynamic || descriptor.Type != reflect.TypeOf((*any)(nil)).Elem() {
			t.Fatalf("ANY dynamic property %q = %#v, ok=%t", name, descriptor, ok)
		}
		getter, ok := variant.Getter(name)
		if !ok || getter.Type() != reflect.TypeOf((*any)(nil)).Elem() || getter.Property().Kind != PropertyDynamic {
			t.Fatalf("ANY dynamic getter %q = %#v, ok=%t", name, getter, ok)
		}
	}

	orderEvent, err := newEvent(order, variantOrder{ID: "O1", Common: "same", Amount: 10}, timeZero())
	if err != nil {
		t.Fatal(err)
	}
	quoteEvent, err := newEvent(quote, variantQuote{ID: "Q1", Common: "same", Bid: 12.5}, timeZero())
	if err != nil {
		t.Fatal(err)
	}
	getter, _ := variant.Getter("bid")
	if got := getter.Get(orderEvent.Underlying()); !got.IsMissing() {
		t.Fatalf("dynamic getter on order = %#v, want missing", got)
	}
	if got := getter.Get(quoteEvent.Underlying()); !got.IsPresent() || got.Any() != 12.5 {
		t.Fatalf("dynamic getter on quote = %#v, want 12.5", got)
	}
	missingGetter, _ := variant.Getter("notDeclared")
	if got := missingGetter.Get(quoteEvent.Underlying()); !got.IsMissing() {
		t.Fatalf("missing dynamic getter = %#v, want missing", got)
	}
}

func TestVariantLengthWindowPreservesMemberOldNewStreams(t *testing.T) {
	env := NewEnvironment()
	order, err := RegisterStruct[variantOrder](env, "LengthVariantOrder")
	if err != nil {
		t.Fatal(err)
	}
	quote, err := RegisterStruct[variantQuote](env, "LengthVariantQuote")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterVariant(env, "LengthVariant", order, quote); err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromAny(env, "LengthVariant").Window(LengthWindow(2)).Query(StatementName("variant-length"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	orderRoute, err := env.Build(From[variantOrder](env, "LengthVariantOrder").InsertInto("LengthVariant", StatementName("length-order")))
	if err != nil {
		t.Fatal(err)
	}
	quoteRoute, err := env.Build(From[variantQuote](env, "LengthVariantQuote").InsertInto("LengthVariant", StatementName("length-quote")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, plan := range []Plan{orderRoute, quoteRoute} {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	consumer, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := consumer.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), variantOrder{ID: "E1", Common: "one", Amount: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), variantQuote{ID: "E2", Common: "two", Bid: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), variantQuote{ID: "E3", Common: "three", Bid: 3}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 3 || len(batches[0].New) != 1 || len(batches[1].New) != 1 || len(batches[2].New) != 1 || len(batches[2].Old) != 1 {
		t.Fatalf("variant length new/old batches = %#v", batches)
	}
	if event, ok := batches[2].Old[0].Event(); !ok || event.TypeName() != "LengthVariantOrder" || event.Get("id").Any() != "E1" {
		t.Fatalf("variant length old event = %#v", batches[2].Old[0])
	}
}

func TestVariantNamedWindowUniqueReplacementPreservesMemberIdentity(t *testing.T) {
	env := NewEnvironment()
	order, err := RegisterStruct[variantOrder](env, "NamedVariantOrder")
	if err != nil {
		t.Fatal(err)
	}
	quote, err := RegisterStruct[variantQuote](env, "NamedVariantQuote")
	if err != nil {
		t.Fatal(err)
	}
	variant, err := RegisterVariant(env, "NamedVariant", order, quote)
	if err != nil {
		t.Fatal(err)
	}
	const windowName = "NamedVariantWindow"
	if _, err := CreateNamedWindow(env, windowName, variant, NamedWindowRetention(Unique(Field[any, string]("id")))); err != nil {
		t.Fatal(err)
	}
	routeOrder, err := env.Build(From[variantOrder](env, "NamedVariantOrder").InsertInto("NamedVariant", StatementName("named-variant-order")))
	if err != nil {
		t.Fatal(err)
	}
	routeQuote, err := env.Build(From[variantQuote](env, "NamedVariantQuote").InsertInto("NamedVariant", StatementName("named-variant-quote")))
	if err != nil {
		t.Fatal(err)
	}
	routes := []Plan{routeOrder, routeQuote}
	windowPlan, err := env.Build(OnRecord(FromAny(env, "NamedVariant")).MergeIntoNamedWindowWhen(windowName, nil, WhenNotMatchedAny()).Query(StatementName("named-variant-insert")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, plan := range routes {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := engine.Deploy(context.Background(), windowPlan); err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow(windowName)
	if !ok {
		t.Fatal("variant named window is missing")
	}
	var deltas []NamedWindowDelta
	if _, err := window.Subscribe(func(_ context.Context, delta NamedWindowDelta) error {
		deltas = append(deltas, delta)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), variantOrder{ID: "E1", Common: "first", Amount: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), variantQuote{ID: "E2", Common: "second", Bid: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), variantQuote{ID: "E1", Common: "replacement", Bid: 3}); err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 3 || len(deltas[2].Old) != 1 || len(deltas[2].New) != 1 {
		t.Fatalf("variant named window deltas = %#v", deltas)
	}
	if deltas[2].Old[0].Schema().Name() != "NamedVariantOrder" || deltas[2].New[0].Schema().Name() != "NamedVariantQuote" {
		t.Fatalf("variant named window identities = old:%s new:%s", deltas[2].Old[0].Schema().Name(), deltas[2].New[0].Schema().Name())
	}
}

func TestVariantWildcardJoinAndDerivedProjectionMatchEsper(t *testing.T) {
	env := NewEnvironment()
	leftName := "VariantWildcardLeft"
	rightName := "VariantWildcardRight"
	if _, err := RegisterStruct[insertJoinWildcardLeft](env, leftName); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[insertJoinWildcardRight](env, rightName); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterVariantAny(env, "VariantWildcardAny"); err != nil {
		t.Fatal(err)
	}

	query := Join(
		From[insertJoinWildcardLeft](env, leftName),
		From[insertJoinWildcardRight](env, rightName).Window(KeepAll()),
	).Unidirectional(JoinLeft).Select(
		SelectSourceEvent(0, "sb"),
		SelectSourceEvent(1, "s0"),
	).InsertInto("VariantWildcardAny", StatementName("variant-wildcard-join"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromAny(env, "VariantWildcardAny").Query(StatementName("variant-wildcard-consumer")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	consumer, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var received []Event
	if _, err := consumer.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				received = append(received, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), insertJoinWildcardRight{Key: "J-1", Value: 22}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), insertJoinWildcardLeft{Key: "J-1", Value: 11}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 {
		t.Fatalf("variant wildcard join events = %#v", received)
	}
	left, err := As[Event](received[0].Get("sb"))
	if err != nil || left.TypeName() != leftName || left.Get("key").Any() != "J-1" || left.Get("leftValue").Any() != int64(11) {
		t.Fatalf("variant wildcard left = %#v, err=%v", received[0].Get("sb"), err)
	}
	right, err := As[Event](received[0].Get("s0"))
	if err != nil || right.TypeName() != rightName || right.Get("key").Any() != "J-1" || right.Get("rightValue").Any() != int64(22) {
		t.Fatalf("variant wildcard right = %#v, err=%v", received[0].Get("s0"), err)
	}
}

func TestVariantPlanCanonicalIncludesMemberIdentity(t *testing.T) {
	env := NewEnvironment()
	first, err := RegisterStruct[variantOrder](env, "PlanVariantOrder")
	if err != nil {
		t.Fatal(err)
	}
	second, err := RegisterStruct[variantQuote](env, "PlanVariantQuote")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterVariant(env, "PlanVariant", first, second); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "PlanVariant").Query(StatementName("variant-plan")))
	if err != nil {
		t.Fatal(err)
	}
	same, err := env.Build(FromAny(env, "PlanVariant").Query(StatementName("variant-plan")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != same.Hash() || !reflect.DeepEqual(plan.Canonical(), same.Canonical()) {
		t.Fatal("equivalent variant plans are not stable")
	}
	canonical := string(plan.Canonical())
	if !strings.Contains(canonical, "variant=0") || !strings.Contains(canonical, "PlanVariantOrder,PlanVariantQuote") {
		t.Fatalf("variant member identity missing from canonical plan: %s", canonical)
	}
}

func timeZero() time.Time { return time.Unix(0, 0) }
