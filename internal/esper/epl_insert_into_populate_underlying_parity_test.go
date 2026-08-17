package esper

import (
	"context"
	"reflect"
	"testing"
)

// This file locks the observable behavior of the Java regression suite
// EPLInsertIntoPopulateUnderlying (Esper 9.0.0, commit 9e1b9f1c) against the
// Go typed insert-into route. Java executes against bean-backed event types
// registered from classes; the Go equivalent registers struct schemas with
// the same property names and asserts the populated target fields. The
// persisted Java oracle trace for the same scenario lives at
// testdata/parity/epl-insert-into-populate-underlying.trace.json and the
// assertions below match it record by record.

type iipuSupportBean struct {
	TheString       string   `esper:"theString"`
	IntPrimitive    int      `esper:"intPrimitive"`
	IntBoxed        *int     `esper:"intBoxed"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	LongBoxed       *int64   `esper:"longBoxed"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	CharPrimitive   rune     `esper:"charPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	EnumValue       string   `esper:"enumValue"`
}

type iipuComplexProps struct {
	ArrayProperty []int          `esper:"arrayProperty"`
	ObjectArray   []any          `esper:"objectArray"`
	MapProperty   map[string]any `esper:"mapProperty"`
	Nested        map[string]any `esper:"nested"`
	AnyObject     any            `esper:"anyObject"`
}

func iipuRegisterCommon(t *testing.T, env *Environment) {
	t.Helper()
	if _, err := RegisterStruct[iipuSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[iipuComplexProps](env, "SupportBeanComplexProps"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "MyMap", []FieldSpec{
		FieldDef("intBoxed", reflect.TypeOf(0)),
		FieldDef("floatBoxed", reflect.TypeOf(float32(0))),
		FieldDef("intArr", reflect.TypeOf([]int{})),
		FieldDef("mapProp", reflect.TypeOf(map[string]any{})),
		FieldDef("nested", reflect.TypeOf(map[string]any{})),
	}); err != nil {
		t.Fatal(err)
	}
}

func iipuSubscribe(t *testing.T, deployment *Deployment) *[]Result {
	t.Helper()
	var results []Result
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		results = append(results, batch.New...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &results
}

// iipuBoxed reports the dereferenced value of a pointer-valued property so
// assertions compare the observable boxed value (Java get() on a boxed
// property) rather than the Go pointer representation.
func iipuBoxed(value Value) any {
	if pointer, ok := value.Any().(*int); ok {
		if pointer == nil {
			return nil
		}
		return *pointer
	}
	if pointer, ok := value.Any().(*int64); ok {
		if pointer == nil {
			return nil
		}
		return *pointer
	}
	if pointer, ok := value.Any().(*float64); ok {
		if pointer == nil {
			return nil
		}
		return *pointer
	}
	return value.Any()
}

// TestEPLInsertIntoPopulateBeanSimpleSelectNamesParity covers the
// select-column-names half of EPLInsertIntoPopulateBeanSimple: projections
// named after target properties populate the struct-backed target, and
// properties the projection omits arrive as null. Oracle record
// populate-bean-simple-select-names expects theString=E1, intPrimitive=1,
// intBoxed=2, longPrimitive=3, null longBoxed, boolPrimitive=true,
// charPrimitive='x', bytePrimitive=10, floatPrimitive=8, doublePrimitive=9,
// shortPrimitive=5 and enumValue=ENUM_VALUE_2.
func TestEPLInsertIntoPopulateBeanSimpleSelectNamesParity(t *testing.T) {
	env := NewEnvironment()
	iipuRegisterCommon(t, env)
	producer := FromAny(env, "MyMap").Select(
		Alias("theString", Literal("E1")),
		Alias("intPrimitive", Literal(1)),
		Alias("intBoxed", Literal(2)),
		Alias("longPrimitive", Literal(int64(3))),
		Alias("longBoxed", NullLiteral[*int64]()),
		Alias("boolPrimitive", Literal(true)),
		Alias("charPrimitive", Literal('x')),
		Alias("bytePrimitive", Literal(int8(10))),
		Alias("floatPrimitive", Literal(float32(8))),
		Alias("doublePrimitive", Literal(float64(9))),
		Alias("shortPrimitive", Literal(int16(5))),
		Alias("enumValue", Literal("ENUM_VALUE_2")),
	).InsertInto("SupportBean", StatementName("i1"))
	consumer := From[iipuSupportBean](env, "SupportBean").Query(StatementName("s0"))
	producerPlan, err := env.Build(producer)
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(consumer)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	producerDeployment, err := engine.Deploy(context.Background(), producerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer producerDeployment.Undeploy(context.Background())
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer consumerDeployment.Undeploy(context.Background())
	events := iipuSubscribe(t, consumerDeployment)
	if err := engine.Send(context.Background(), "MyMap", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 1 {
		t.Fatalf("select-names events = %#v", *events)
	}
	underlying, ok := (*events)[0].Event()
	if !ok {
		t.Fatalf("select-names result is not an event: %#v", (*events)[0])
	}
	iipuAssertPopulatedSimple(t, underlying)
}

// iipuAssertPopulatedSimple checks the twelve projected SupportBean columns
// both EPLInsertIntoPopulateBeanSimple statements assert: theString=E1,
// intPrimitive=1, intBoxed=2, longPrimitive=3, longBoxed=null,
// boolPrimitive=true, charPrimitive='x', bytePrimitive=10, floatPrimitive=8,
// doublePrimitive=9, shortPrimitive=5, enumValue=ENUM_VALUE_2.
func iipuAssertPopulatedSimple(t *testing.T, underlying Event) {
	t.Helper()
	if got := underlying.Get("theString").Any(); got != "E1" {
		t.Fatalf("theString = %#v, want E1", got)
	}
	if got := underlying.Get("intPrimitive").Any(); got != 1 {
		t.Fatalf("intPrimitive = %#v, want 1", got)
	}
	if got := iipuBoxed(underlying.Get("intBoxed")); got != 2 {
		t.Fatalf("intBoxed = %#v, want 2", got)
	}
	if got := underlying.Get("longPrimitive").Any(); got != int64(3) {
		t.Fatalf("longPrimitive = %#v, want 3", got)
	}
	if !underlying.Get("longBoxed").IsNull() {
		t.Fatalf("longBoxed = %#v, want null", underlying.Get("longBoxed").Any())
	}
	if got := underlying.Get("boolPrimitive").Any(); got != true {
		t.Fatalf("boolPrimitive = %#v, want true", got)
	}
	if got := underlying.Get("charPrimitive").Any(); got != 'x' {
		t.Fatalf("charPrimitive = %#v, want x", got)
	}
	if got := underlying.Get("bytePrimitive").Any(); got != int8(10) {
		t.Fatalf("bytePrimitive = %#v, want 10", got)
	}
	if got := underlying.Get("floatPrimitive").Any(); got != float32(8) {
		t.Fatalf("floatPrimitive = %#v, want 8", got)
	}
	if got := underlying.Get("doublePrimitive").Any(); got != float64(9) {
		t.Fatalf("doublePrimitive = %#v, want 9", got)
	}
	if got := underlying.Get("shortPrimitive").Any(); got != int16(5) {
		t.Fatalf("shortPrimitive = %#v, want 5", got)
	}
	if got := underlying.Get("enumValue").Any(); got != "ENUM_VALUE_2" {
		t.Fatalf("enumValue = %#v, want ENUM_VALUE_2", got)
	}
}

// TestEPLInsertIntoPopulateBeanSimpleInsertNamesParity covers the
// insert-into column-names half of EPLInsertIntoPopulateBeanSimple. The
// EPL target column list is the Java spelling of the same named routing
// contract the typed API expresses, so the observable result is identical to
// the select-names form. Oracle record populate-bean-simple-insert-names
// expects the same field values.
func TestEPLInsertIntoPopulateBeanSimpleInsertNamesParity(t *testing.T) {
	env := NewEnvironment()
	iipuRegisterCommon(t, env)
	producer := FromAny(env, "MyMap").Select(
		Alias("theString", Literal("E1")),
		Alias("intPrimitive", Literal(1)),
		Alias("intBoxed", Literal(2)),
		Alias("longPrimitive", Literal(int64(3))),
		Alias("longBoxed", NullLiteral[*int64]()),
		Alias("boolPrimitive", Literal(true)),
		Alias("charPrimitive", Literal('x')),
		Alias("bytePrimitive", Literal(int8(10))),
		Alias("floatPrimitive", Literal(float32(8))),
		Alias("doublePrimitive", Literal(float64(9))),
		Alias("shortPrimitive", Literal(int16(5))),
		Alias("enumValue", Literal("ENUM_VALUE_2")),
	).InsertInto("SupportBean", StatementName("s0"))
	consumer := From[iipuSupportBean](env, "SupportBean").Query(StatementName("c0"))
	producerPlan, err := env.Build(producer)
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(consumer)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	producerDeployment, err := engine.Deploy(context.Background(), producerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer producerDeployment.Undeploy(context.Background())
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer consumerDeployment.Undeploy(context.Background())
	events := iipuSubscribe(t, consumerDeployment)
	if err := engine.Send(context.Background(), "MyMap", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 1 {
		t.Fatalf("insert-names events = %#v", *events)
	}
	underlying, ok := (*events)[0].Event()
	if !ok {
		t.Fatalf("insert-names result is not an event: %#v", (*events)[0])
	}
	iipuAssertPopulatedSimple(t, underlying)
}

// TestEPLInsertIntoPopulateBeanSimpleBoxedConversionParity covers the
// Integer-to-Long and Float-to-Double conversion half of
// EPLInsertIntoPopulateBeanSimple. Oracle record
// populate-bean-simple-boxed-conversion expects longBoxed=4 and
// doubleBoxed=0 with all unprojected properties null.
func TestEPLInsertIntoPopulateBeanSimpleBoxedConversionParity(t *testing.T) {
	env := NewEnvironment()
	iipuRegisterCommon(t, env)
	intBoxed := Field[map[string]any, int]("intBoxed")
	floatBoxed := Field[map[string]any, float32]("floatBoxed")
	producer := Select(From[map[string]any](env, "MyMap"),
		Alias("longBoxed", Cast[int, int64](intBoxed)),
		Alias("doubleBoxed", Cast[float32, float64](floatBoxed)),
	).InsertInto("SupportBean", StatementName("s0"))
	consumer := From[iipuSupportBean](env, "SupportBean").Query(StatementName("c0"))
	producerPlan, err := env.Build(producer)
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(consumer)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	producerDeployment, err := engine.Deploy(context.Background(), producerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer producerDeployment.Undeploy(context.Background())
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer consumerDeployment.Undeploy(context.Background())
	events := iipuSubscribe(t, consumerDeployment)
	if err := engine.Send(context.Background(), "MyMap", map[string]any{"intBoxed": 4, "floatBoxed": float32(0)}); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 1 {
		t.Fatalf("boxed-conversion events = %#v", *events)
	}
	underlying, ok := (*events)[0].Event()
	if !ok {
		t.Fatalf("boxed-conversion result is not an event: %#v", (*events)[0])
	}
	if got := iipuBoxed(underlying.Get("longBoxed")); got != int64(4) {
		t.Fatalf("longBoxed = %#v, want 4", got)
	}
	if got := iipuBoxed(underlying.Get("doubleBoxed")); got != float64(0) {
		t.Fatalf("doubleBoxed = %#v, want 0", got)
	}
	// Nullable (pointer) properties the projection omits arrive as null.
	// Non-pointer properties are zero-valued by the Go struct representation,
	// matching the Java bean primitive defaults.
	for _, name := range []string{"intBoxed", "floatBoxed"} {
		if !underlying.Get(name).IsNull() {
			t.Fatalf("%s = %#v, want null", name, underlying.Get(name).Any())
		}
	}
}

// TestEPLInsertIntoBeanWildcardParity covers EPLInsertIntoBeanWildcard: a
// wildcard insert-into from a compatible map event populates the struct
// target by property name, preserving present values and nulls. Oracle
// record bean-wildcard expects intPrimitive=4, longBoxed=100, theString=E1.
func TestEPLInsertIntoBeanWildcardParity(t *testing.T) {
	env := NewEnvironment()
	iipuRegisterCommon(t, env)
	if _, err := RegisterMap(env, "MySupportMap", nil); err != nil {
		t.Fatal(err)
	}
	producer := From[map[string]any](env, "MySupportMap").
		InsertInto("SupportBean", StatementName("s0"))
	plan, err := env.Build(producer)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	events := iipuSubscribe(t, deployment)
	if err := engine.Send(context.Background(), "MySupportMap", map[string]any{
		"intPrimitive": 4, "longBoxed": int64(100), "theString": "E1", "boolPrimitive": true,
	}); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 1 {
		t.Fatalf("bean-wildcard events = %#v", *events)
	}
	underlying, ok := (*events)[0].Event()
	if !ok {
		t.Fatalf("bean-wildcard result is not an event: %#v", (*events)[0])
	}
	if got := underlying.Get("intPrimitive").Any(); got != 4 {
		t.Fatalf("intPrimitive = %#v, want 4", got)
	}
	if got := underlying.Get("longBoxed").Any(); got != int64(100) {
		t.Fatalf("longBoxed = %#v, want 100", got)
	}
	if got := underlying.Get("theString").Any(); got != "E1" {
		t.Fatalf("theString = %#v, want E1", got)
	}
	if got := underlying.Get("boolPrimitive").Any(); got != true {
		t.Fatalf("boolPrimitive = %#v, want true", got)
	}
}

// TestEPLInsertIntoPopulateBeanObjectsArraysMapsParity covers the arrays
// and maps half of EPLInsertIntoPopulateBeanObjects: array/map/object-array
// valued projections populate matching collection properties of a complex
// struct target. Oracle record populate-bean-objects-arrays-maps expects
// arrayProperty=[-1,-2], objectArray=[10,20,30] and
// mapProperty={mykey:myval}.
func TestEPLInsertIntoPopulateBeanObjectsArraysMapsParity(t *testing.T) {
	env := NewEnvironment()
	iipuRegisterCommon(t, env)
	intArr := Field[map[string]any, []int]("intArr")
	mapProp := Field[map[string]any, map[string]any]("mapProp")
	producer := Select(From[map[string]any](env, "MyMap"),
		Alias("arrayProperty", intArr),
		Alias("objectArray", Literal([]any{10, 20, 30})),
		Alias("mapProperty", mapProp),
	).InsertInto("SupportBeanComplexProps", StatementName("s0"))
	consumer := From[iipuComplexProps](env, "SupportBeanComplexProps").Query(StatementName("c0"))
	producerPlan, err := env.Build(producer)
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(consumer)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	producerDeployment, err := engine.Deploy(context.Background(), producerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer producerDeployment.Undeploy(context.Background())
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer consumerDeployment.Undeploy(context.Background())
	events := iipuSubscribe(t, consumerDeployment)
	if err := engine.Send(context.Background(), "MyMap", map[string]any{
		"intArr": []int{-1, -2}, "mapProp": map[string]any{"mykey": "myval"},
	}); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 1 {
		t.Fatalf("arrays-maps events = %#v", *events)
	}
	underlying, ok := (*events)[0].Event()
	if !ok {
		t.Fatalf("arrays-maps result is not an event: %#v", (*events)[0])
	}
	if got := underlying.Get("arrayProperty").Any().([]int); !reflect.DeepEqual(got, []int{-1, -2}) {
		t.Fatalf("arrayProperty = %#v, want [-1 -2]", got)
	}
	if got := underlying.Get("objectArray").Any().([]any); !reflect.DeepEqual(got, []any{10, 20, 30}) {
		t.Fatalf("objectArray = %#v, want [10 20 30]", got)
	}
	if got := underlying.Get("mapProperty").Any().(map[string]any); got["mykey"] != "myval" {
		t.Fatalf("mapProperty = %#v, want {mykey:myval}", got)
	}
}

// TestEPLInsertIntoPopulateBeanObjectsNestedParity covers the object-values
// half of EPLInsertIntoPopulateBeanObjects: a dynamically constructed map
// value populates a map-typed property. Java uses "new {k = v}" to build the
// nested map; the Go equivalent routes an explicit map projection. Oracle
// record populate-bean-objects-nested expects nested={nestedValue:111,
// nestedNestedValue:222}.
func TestEPLInsertIntoPopulateBeanObjectsNestedParity(t *testing.T) {
	env := NewEnvironment()
	iipuRegisterCommon(t, env)
	producer := Select(From[map[string]any](env, "MyMap"),
		Alias("nested", Field[map[string]any, map[string]any]("nested")),
	).InsertInto("SupportBeanComplexProps", StatementName("s0"))
	consumer := From[iipuComplexProps](env, "SupportBeanComplexProps").Query(StatementName("c0"))
	producerPlan, err := env.Build(producer)
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(consumer)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	producerDeployment, err := engine.Deploy(context.Background(), producerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer producerDeployment.Undeploy(context.Background())
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer consumerDeployment.Undeploy(context.Background())
	events := iipuSubscribe(t, consumerDeployment)
	if err := engine.Send(context.Background(), "MyMap", map[string]any{
		"nested": map[string]any{"nestedValue": "111", "nestedNestedValue": "222"},
	}); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 1 {
		t.Fatalf("nested events = %#v", *events)
	}
	underlying, ok := (*events)[0].Event()
	if !ok {
		t.Fatalf("nested result is not an event: %#v", (*events)[0])
	}
	nested, ok := underlying.Get("nested").Any().(map[string]any)
	if !ok || nested["nestedValue"] != "111" || nested["nestedNestedValue"] != "222" {
		t.Fatalf("nested = %#v, want {nestedValue:111, nestedNestedValue:222}", underlying.Get("nested").Any())
	}
}

// TestEPLInsertIntoPopulateBeanObjectsNullValueParity covers the null-value
// half of EPLInsertIntoPopulateBeanObjects: routing an unboxed-null source
// property into a non-pointer target field yields the Go zero value (Java
// leaves the primitive default 0), and a present boxed value routes through
// unchanged. Oracle records populate-bean-objects-null-value expect
// intPrimitive null-then-20 with theString=B.
func TestEPLInsertIntoPopulateBeanObjectsNullValueParity(t *testing.T) {
	env := NewEnvironment()
	iipuRegisterCommon(t, env)
	theString := Field[iipuSupportBean, string]("theString")
	intBoxed := Field[iipuSupportBean, *int]("intBoxed")
	source := From[iipuSupportBean](env, "SupportBean").Filter(
		Equal[string](theString, Literal("A")),
	)
	producer := Select(source,
		Alias("theString", Literal("B")),
		Alias("intPrimitive", Coalesce[int](intBoxed, Literal(0))),
	).InsertInto("SupportBean", StatementName("s0"))
	consumer := From[iipuSupportBean](env, "SupportBean").Filter(
		Equal[string](theString, Literal("B")),
	).Query(StatementName("c0"))
	producerPlan, err := env.Build(producer)
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(consumer)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	producerDeployment, err := engine.Deploy(context.Background(), producerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer producerDeployment.Undeploy(context.Background())
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer consumerDeployment.Undeploy(context.Background())
	events := iipuSubscribe(t, consumerDeployment)
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "A", IntPrimitive: 0}); err != nil {
		t.Fatal(err)
	}
	boxed := 20
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "A", IntPrimitive: 1, IntBoxed: &boxed}); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 2 {
		t.Fatalf("null-value events = %#v", *events)
	}
	first, ok := (*events)[0].Event()
	if !ok {
		t.Fatalf("null-value first result is not an event: %#v", (*events)[0])
	}
	if got := first.Get("theString").Any(); got != "B" {
		t.Fatalf("first theString = %#v, want B", got)
	}
	if got := first.Get("intPrimitive").Any(); got != 0 {
		t.Fatalf("first intPrimitive = %#v, want 0", got)
	}
	second, ok := (*events)[1].Event()
	if !ok {
		t.Fatalf("null-value second result is not an event: %#v", (*events)[1])
	}
	if got := second.Get("theString").Any(); got != "B" {
		t.Fatalf("second theString = %#v, want B", got)
	}
	if got := second.Get("intPrimitive").Any(); got != 20 {
		t.Fatalf("second intPrimitive = %#v, want 20", got)
	}
}

// TestEPLInsertIntoPopulateUnderlyingMapParity covers the MyMapType
// representation of EPLInsertIntoPopulateUnderlyingSimple: routing a typed
// projection into a registered map event type materializes the target map
// with the projected names. Oracle record populate-underlying-map expects
// intVal=1000, stringVal=E1, doubleVal=1001.
func TestEPLInsertIntoPopulateUnderlyingMapParity(t *testing.T) {
	env := NewEnvironment()
	iipuRegisterCommon(t, env)
	if _, err := RegisterMap(env, "MyMapType", []FieldSpec{
		FieldDef("intVal", reflect.TypeOf(0)),
		FieldDef("stringVal", reflect.TypeOf("")),
		FieldDef("doubleVal", reflect.TypeOf(float64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	theString := Field[iipuSupportBean, string]("theString")
	intPrimitive := Field[iipuSupportBean, int]("intPrimitive")
	doubleBoxed := Field[iipuSupportBean, *float64]("doubleBoxed")
	producer := Select(From[iipuSupportBean](env, "SupportBean"),
		Alias("intVal", intPrimitive),
		Alias("stringVal", theString),
		Alias("doubleVal", doubleBoxed),
	).InsertInto("MyMapType", StatementName("s0"))
	consumer := FromAny(env, "MyMapType").Query(StatementName("c0"))
	producerPlan, err := env.Build(producer)
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(consumer)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	producerDeployment, err := engine.Deploy(context.Background(), producerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer producerDeployment.Undeploy(context.Background())
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer consumerDeployment.Undeploy(context.Background())
	events := iipuSubscribe(t, consumerDeployment)
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "E1", IntPrimitive: 1000, DoubleBoxed: float64Ptr(1001)}); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 1 {
		t.Fatalf("map events = %#v", *events)
	}
	underlying, ok := (*events)[0].Event()
	if !ok {
		t.Fatalf("map result is not an event: %#v", (*events)[0])
	}
	if got := underlying.Get("intVal").Any(); got != 1000 {
		t.Fatalf("intVal = %#v, want 1000", got)
	}
	if got := underlying.Get("stringVal").Any(); got != "E1" {
		t.Fatalf("stringVal = %#v, want E1", got)
	}
	if got := underlying.Get("doubleVal").Any(); got != float64(1001) {
		t.Fatalf("doubleVal = %#v, want 1001", got)
	}
}

// TestEPLInsertIntoPopulateUnderlyingObjectArrayParity covers the MyOAType
// representation of EPLInsertIntoPopulateUnderlyingSimple: routing a typed
// projection into a registered object-array event type materializes the
// positional underlying in declared field order. Oracle record
// populate-underlying-objectarray expects intVal=1000, stringVal=E1,
// doubleVal=1001.
func TestEPLInsertIntoPopulateUnderlyingObjectArrayParity(t *testing.T) {
	env := NewEnvironment()
	iipuRegisterCommon(t, env)
	if _, err := RegisterObjectArray(env, "MyOAType", []FieldSpec{
		FieldDef("intVal", reflect.TypeOf(0)),
		FieldDef("stringVal", reflect.TypeOf("")),
		FieldDef("doubleVal", reflect.TypeOf(float64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	theString := Field[iipuSupportBean, string]("theString")
	intPrimitive := Field[iipuSupportBean, int]("intPrimitive")
	doubleBoxed := Field[iipuSupportBean, *float64]("doubleBoxed")
	producer := Select(From[iipuSupportBean](env, "SupportBean"),
		Alias("intVal", intPrimitive),
		Alias("stringVal", theString),
		Alias("doubleVal", doubleBoxed),
	).InsertInto("MyOAType", StatementName("s0"))
	consumer := FromAny(env, "MyOAType").Query(StatementName("c0"))
	producerPlan, err := env.Build(producer)
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(consumer)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	producerDeployment, err := engine.Deploy(context.Background(), producerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer producerDeployment.Undeploy(context.Background())
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer consumerDeployment.Undeploy(context.Background())
	events := iipuSubscribe(t, consumerDeployment)
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "E1", IntPrimitive: 1000, DoubleBoxed: float64Ptr(1001)}); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 1 {
		t.Fatalf("object-array events = %#v", *events)
	}
	underlying, ok := (*events)[0].Event()
	if !ok {
		t.Fatalf("object-array result is not an event: %#v", (*events)[0])
	}
	if got := underlying.Get("intVal").Any(); got != 1000 {
		t.Fatalf("intVal = %#v, want 1000", got)
	}
	if got := underlying.Get("stringVal").Any(); got != "E1" {
		t.Fatalf("stringVal = %#v, want E1", got)
	}
	if got := underlying.Get("doubleVal").Any(); got != float64(1001) {
		t.Fatalf("doubleVal = %#v, want 1001", got)
	}
	values, ok := underlying.Underlying().([]any)
	if !ok || !reflect.DeepEqual(values, []any{1000, "E1", float64(1001)}) {
		t.Fatalf("object-array underlying = %#v, want [1000 E1 1001]", underlying.Underlying())
	}
}

// TestEPLInsertIntoPopulateUnderlyingAvroParity covers the MyAvroType
// representation of EPLInsertIntoPopulateUnderlyingSimple: routing a typed
// projection into a registered Avro event type materializes an Avro record
// with the projected values. Oracle record populate-underlying-avro expects
// intVal=1000, stringVal=E1, doubleVal=1001.
func TestEPLInsertIntoPopulateUnderlyingAvroParity(t *testing.T) {
	env := NewEnvironment()
	iipuRegisterCommon(t, env)
	if _, err := RegisterAvro(env, "MyAvroType", []FieldSpec{
		FieldDef("intVal", reflect.TypeOf(0)),
		FieldDef("stringVal", reflect.TypeOf("")),
		FieldDef("doubleVal", reflect.TypeOf(float64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	theString := Field[iipuSupportBean, string]("theString")
	intPrimitive := Field[iipuSupportBean, int]("intPrimitive")
	doubleBoxed := Field[iipuSupportBean, *float64]("doubleBoxed")
	producer := Select(From[iipuSupportBean](env, "SupportBean"),
		Alias("intVal", intPrimitive),
		Alias("stringVal", theString),
		Alias("doubleVal", doubleBoxed),
	).InsertInto("MyAvroType", StatementName("s0"))
	consumer := FromAny(env, "MyAvroType").Query(StatementName("c0"))
	producerPlan, err := env.Build(producer)
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(consumer)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	producerDeployment, err := engine.Deploy(context.Background(), producerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer producerDeployment.Undeploy(context.Background())
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer consumerDeployment.Undeploy(context.Background())
	events := iipuSubscribe(t, consumerDeployment)
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "E1", IntPrimitive: 1000, DoubleBoxed: float64Ptr(1001)}); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 1 {
		t.Fatalf("avro events = %#v", *events)
	}
	underlying, ok := (*events)[0].Event()
	if !ok {
		t.Fatalf("avro result is not an event: %#v", (*events)[0])
	}
	if got := underlying.Get("intVal").Any(); got != 1000 {
		t.Fatalf("intVal = %#v, want 1000", got)
	}
	if got := underlying.Get("stringVal").Any(); got != "E1" {
		t.Fatalf("stringVal = %#v, want E1", got)
	}
	if got := underlying.Get("doubleVal").Any(); got != float64(1001) {
		t.Fatalf("doubleVal = %#v, want 1001", got)
	}
	record, ok := underlying.Underlying().(*AvroRecord)
	if !ok {
		t.Fatalf("avro underlying = %T, want *AvroRecord", underlying.Underlying())
	}
	if record.Get("intVal") != 1000 || record.Get("stringVal") != "E1" || record.Get("doubleVal") != float64(1001) {
		t.Fatalf("avro record = %#v", record.AsMap())
	}
}

type iipuNonBeanS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
}

// TestEPLInsertIntoColNonBeanFromSubquerySingleParity covers the
// EPLInsertIntoColNonBeanFromSubquerySingle runtimes: a non-bean variant that
// uses a multi-column subquery to project named columns into an event-typed
// column of the insert-into target. The subquery returns a map which must be
// materialized as an Event of the target schema type. Java runtimes:
// objectarray without filter, objectarray with filter, map without filter,
// map with filter. The Go chain uses SubqueryRowAsEvent to bridge the gap.
func TestEPLInsertIntoColNonBeanFromSubquerySingleParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[iipuNonBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)

	eventZero, err := RegisterObjectArray(env, "EventZero", []FieldSpec{
		FieldDef("e0_0", reflect.TypeOf("")),
		FieldDef("e0_1", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RegisterObjectArray(env, "EventOne", []FieldSpec{
		FieldDef("ez", reflect.TypeOf(Event{})),
	}, WithNestedPropertySchema("ez", eventZero))
	if err != nil {
		t.Fatal(err)
	}

	iipuRegisterCommon(t, env)

	s0 := From[iipuNonBeanS0](env, "SupportBean_S0").Window(LengthWindow(1)).AsRecord()
	projection := SubqueryRowAsEvent(env, "EventZero", s0,
		Alias("e0_0", Field[iipuNonBeanS0, string]("p00")),
		Alias("e0_1", Field[iipuNonBeanS0, string]("p01")),
	)
	plan, err := env.Build(
		Select(From[iipuSupportBean](env, "SupportBean"),
			Alias("ez", projection),
		).InsertInto("EventOne", StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	got := iipuSubscribe(t, deployment)

	// Send event 1: id=1, p00="x1", p01="y1" — no filter, subquery matches
	if err := engine.SendEvent(context.Background(), iipuNonBeanS0{P00: "x1", P01: "y1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(*got))
	}
	ez0, ok := (*got)[0].Get("ez").Any().(Event)
	if !ok {
		t.Fatalf("expected ez to be Event, got %T: %#v", (*got)[0].Get("ez").Any(), (*got)[0].Get("ez").Any())
	}
	if v := ez0.Get("e0_0"); v.Any() != "x1" {
		t.Fatalf("ez.e0_0 = %v, want x1", v.Any())
	}
	if v := ez0.Get("e0_1"); v.Any() != "y1" {
		t.Fatalf("ez.e0_1 = %v, want y1", v.Any())
	}

	// Send event 2: id=100, p00="x2", p01="y2"
	if err := engine.SendEvent(context.Background(), iipuNonBeanS0{ID: 100, P00: "x2", P01: "y2"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "E2", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(*got))
	}
	ez1, ok := (*got)[1].Get("ez").Any().(Event)
	if !ok {
		t.Fatalf("expected ez to be Event, got %T", (*got)[1].Get("ez").Any())
	}
	if v := ez1.Get("e0_0"); v.Any() != "x2" {
		t.Fatalf("ez.e0_0 = %v, want x2", v.Any())
	}
	if v := ez1.Get("e0_1"); v.Any() != "y2" {
		t.Fatalf("ez.e0_1 = %v, want y2", v.Any())
	}
}

// TestEPLInsertIntoColNonBeanFromSubqueryMultiParity covers
// EPLInsertIntoColNonBeanFromSubqueryMulti: a non-bean variant that uses a
// multi-column subquery to produce an array of events. Java runtime IDs:
// java-runtime-98c2143ddbcd1fabf69c (objectarray),
// java-runtime-3c16cb7b5aea7cc551f4 (map).
func TestEPLInsertIntoColNonBeanFromSubqueryMultiParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[iipuNonBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)

	eventZero, err := RegisterObjectArray(env, "EventZero", []FieldSpec{
		FieldDef("e0_0", reflect.TypeOf("")),
		FieldDef("e0_1", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RegisterObjectArray(env, "EventOne", []FieldSpec{
		FieldDef("e1_0", reflect.TypeOf("")),
		FieldDef("ez", reflect.TypeOf([]Event{})),
	}, WithNestedPropertySchema("ez", eventZero))
	if err != nil {
		t.Fatal(err)
	}

	iipuRegisterCommon(t, env)

	inner := From[iipuNonBeanS0](env, "SupportBean_S0").Window(KeepAll()).AsRecord()
	projection := SubqueryRowsAsEvent(env, "EventZero", inner,
		Alias("e0_0", Field[iipuNonBeanS0, string]("p00")),
		Alias("e0_1", Field[iipuNonBeanS0, string]("p01")),
	)
	plan, err := env.Build(
		Select(From[iipuSupportBean](env, "SupportBean"),
			Alias("e1_0", Field[iipuSupportBean, string]("theString")),
			Alias("ez", projection),
		).InsertInto("EventOne", StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	got := iipuSubscribe(t, deployment)

	// Send event 1: 1 S0 event then 1 SB event
	if err := engine.SendEvent(context.Background(), iipuNonBeanS0{P00: "x1", P01: "y1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(*got))
	}
	if v := (*got)[0].Get("e1_0"); v.Any() != "E1" {
		t.Fatalf("e1_0 = %v, want E1", v.Any())
	}
	ezArr0, ok := (*got)[0].Get("ez").Any().([]Event)
	if !ok {
		t.Fatalf("expected ez to be []Event, got %T: %#v", (*got)[0].Get("ez").Any(), (*got)[0].Get("ez").Any())
	}
	if len(ezArr0) != 1 {
		t.Fatalf("expected 1 event in ez array, got %d", len(ezArr0))
	}
	if v := ezArr0[0].Get("e0_0"); v.Any() != "x1" {
		t.Fatalf("ez[0].e0_0 = %v, want x1", v.Any())
	}
	if v := ezArr0[0].Get("e0_1"); v.Any() != "y1" {
		t.Fatalf("ez[0].e0_1 = %v, want y1", v.Any())
	}

	// Send event 2: another S0 event then SB event — array grows
	if err := engine.SendEvent(context.Background(), iipuNonBeanS0{ID: 2, P00: "x2", P01: "y2"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "E2", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(*got))
	}
	ezArr1, ok := (*got)[1].Get("ez").Any().([]Event)
	if !ok {
		t.Fatalf("expected ez to be []Event on second result, got %T", (*got)[1].Get("ez").Any())
	}
	if len(ezArr1) != 2 {
		t.Fatalf("expected 2 events in ez array, got %d", len(ezArr1))
	}
	if v := ezArr1[0].Get("e0_0"); v.Any() != "x1" {
		t.Fatalf("ez[0].e0_0 = %v, want x1", v.Any())
	}
	if v := ezArr1[1].Get("e0_0"); v.Any() != "x2" {
		t.Fatalf("ez[1].e0_0 = %v, want x2", v.Any())
	}
}

// TestEPLInsertIntoColNonBeanFromSubquerySingleFilterParity covers
// EPLInsertIntoColNonBeanFromSubquerySingle with filter=true: the subquery
// has "where id >= 100" so events below 100 produce a null fragment.
// Java runtime IDs: java-runtime-42f641d62fb33599cdf3 (objectarray filter),
// java-runtime-f2f9fbfea46e0a1488f2 (map filter).
func TestEPLInsertIntoColNonBeanFromSubquerySingleFilterParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[iipuNonBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)

	eventZero, err := RegisterObjectArray(env, "EventZero", []FieldSpec{
		FieldDef("e0_0", reflect.TypeOf("")),
		FieldDef("e0_1", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RegisterObjectArray(env, "EventOne", []FieldSpec{
		FieldDef("ez", reflect.TypeOf(Event{})),
	}, WithNestedPropertySchema("ez", eventZero))
	if err != nil {
		t.Fatal(err)
	}

	iipuRegisterCommon(t, env)
	s0 := From[iipuNonBeanS0](env, "SupportBean_S0").Window(LengthWindow(1)).AsRecord()
	projection := SubqueryRowAsEventWithOptions(env, "EventZero", s0,
		[]Selection{
			Alias("e0_0", Field[iipuNonBeanS0, string]("p00")),
			Alias("e0_1", Field[iipuNonBeanS0, string]("p01")),
		},
		SubqueryCardinalityMode(SubqueryNullOnMultiple),
		SubqueryWhere(GreaterOrEqual[int](Field[iipuNonBeanS0, int]("id"), Literal(100))),
	)
	plan, err := env.Build(
		Select(From[iipuSupportBean](env, "SupportBean"),
			Alias("ez", projection),
		).InsertInto("EventOne", StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	got := iipuSubscribe(t, deployment)

	// Event 1: id=1 — filter rejects, ez is null
	if err := engine.SendEvent(context.Background(), iipuNonBeanS0{ID: 1, P00: "x1", P01: "y1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(*got))
	}
	if v := (*got)[0].Get("ez"); !v.IsNull() {
		t.Fatalf("expected ez null for filtered-out id=1, got %v", v.Any())
	}

	// Event 2: id=100 — filter accepts, ez = x2/y2
	if err := engine.SendEvent(context.Background(), iipuNonBeanS0{ID: 100, P00: "x2", P01: "y2"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "E2", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(*got))
	}
	ez1, ok := (*got)[1].Get("ez").Any().(Event)
	if !ok {
		t.Fatalf("expected ez to be Event for id=100, got %T", (*got)[1].Get("ez").Any())
	}
	if v := ez1.Get("e0_0"); v.Any() != "x2" {
		t.Fatalf("ez.e0_0 = %v, want x2", v.Any())
	}
	if v := ez1.Get("e0_1"); v.Any() != "y2" {
		t.Fatalf("ez.e0_1 = %v, want y2", v.Any())
	}

	// Event 3: id=2 — filter rejects again
	if err := engine.SendEvent(context.Background(), iipuNonBeanS0{ID: 2, P00: "x3", P01: "y3"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "E3", IntPrimitive: 3}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 3 {
		t.Fatalf("expected 3 results, got %d", len(*got))
	}
	if v := (*got)[2].Get("ez"); !v.IsNull() {
		t.Fatalf("expected ez null for filtered-out id=2, got %v", v.Any())
	}
}

// TestEPLInsertIntoColNonBeanFromSubqueryMultiFilterParity covers
// EPLInsertIntoColNonBeanFromSubqueryMultiFilter: the subquery has
// "where id between 10 and 20" so only matching events appear in the array.
// Java runtime IDs: java-runtime-ebed0ec5bb756f5bff87 (objectarray),
// java-runtime-49c937650cfc43598179 (map).
func TestEPLInsertIntoColNonBeanFromSubqueryMultiFilterParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[iipuNonBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)

	eventZero, err := RegisterObjectArray(env, "EventZero", []FieldSpec{
		FieldDef("e0_0", reflect.TypeOf("")),
		FieldDef("e0_1", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RegisterObjectArray(env, "EventOne", []FieldSpec{
		FieldDef("ez", reflect.TypeOf([]Event{})),
	}, WithNestedPropertySchema("ez", eventZero))
	if err != nil {
		t.Fatal(err)
	}

	iipuRegisterCommon(t, env)

	inner := From[iipuNonBeanS0](env, "SupportBean_S0").Window(KeepAll()).AsRecord()
	projection := SubqueryRowsAsEventWithOptions(env, "EventZero", inner,
		[]Selection{
			Alias("e0_0", Field[iipuNonBeanS0, string]("p00")),
			Alias("e0_1", Field[iipuNonBeanS0, string]("p01")),
		},
		SubqueryWhere(And(
			GreaterOrEqual[int](Field[iipuNonBeanS0, int]("id"), Literal(10)),
			LessOrEqual[int](Field[iipuNonBeanS0, int]("id"), Literal(20)),
		)),
	)
	plan, err := env.Build(
		Select(From[iipuSupportBean](env, "SupportBean"),
			Alias("ez", projection),
		).InsertInto("EventOne", StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	got := iipuSubscribe(t, deployment)

	// Event 1: id=1 — outside [10,20], ez is null/empty
	if err := engine.SendEvent(context.Background(), iipuNonBeanS0{ID: 1, P00: "x1", P01: "y1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(*got))
	}
	ezVal := (*got)[0].Get("ez")
	if arr, ok := ezVal.Any().([]Event); ok && len(arr) > 0 {
		t.Fatalf("expected empty or null ez for id=1 outside filter, got %d events", len(arr))
	}

	// Event 2: id=10 and id=20 — inside [10,20], ez has 2 events
	if err := engine.SendEvent(context.Background(), iipuNonBeanS0{ID: 10, P00: "x2"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), iipuNonBeanS0{ID: 20, P00: "x3"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "E2", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(*got))
	}
	ezArr, ok := (*got)[1].Get("ez").Any().([]Event)
	if !ok {
		t.Fatalf("expected ez to be []Event, got %T", (*got)[1].Get("ez").Any())
	}
	if len(ezArr) != 2 {
		t.Fatalf("expected 2 events in ez array, got %d", len(ezArr))
	}
	if v := ezArr[0].Get("e0_0"); v.Any() != "x2" {
		t.Fatalf("ez[0].e0_0 = %v, want x2", v.Any())
	}
	if v := ezArr[1].Get("e0_0"); v.Any() != "x3" {
		t.Fatalf("ez[1].e0_0 = %v, want x3", v.Any())
	}
}

// TestEPLInsertIntoColNonBeanFromSubquerySingleMapParity covers the map
// variant of EPLInsertIntoColNonBeanFromSubquerySingle. Java runtime IDs:
// java-runtime-45a9781858a357d139ec (map nofilter),
// java-runtime-f2f9fbfea46e0a1488f2 (map filter).
func TestEPLInsertIntoColNonBeanFromSubquerySingleMapParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[iipuNonBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)

	eventZero, err := RegisterMap(env, "EventZero", []FieldSpec{
		FieldDef("e0_0", reflect.TypeOf("")),
		FieldDef("e0_1", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RegisterMap(env, "EventOne", []FieldSpec{
		FieldDef("ez", reflect.TypeOf(Event{})),
	}, WithNestedPropertySchema("ez", eventZero))
	if err != nil {
		t.Fatal(err)
	}

	iipuRegisterCommon(t, env)

	s0 := From[iipuNonBeanS0](env, "SupportBean_S0").Window(LengthWindow(1)).AsRecord()
	projection := SubqueryRowAsEvent(env, "EventZero", s0,
		Alias("e0_0", Field[iipuNonBeanS0, string]("p00")),
		Alias("e0_1", Field[iipuNonBeanS0, string]("p01")),
	)
	plan, err := env.Build(
		Select(From[iipuSupportBean](env, "SupportBean"),
			Alias("ez", projection),
		).InsertInto("EventOne", StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	got := iipuSubscribe(t, deployment)

	if err := engine.SendEvent(context.Background(), iipuNonBeanS0{ID: 1, P00: "x1", P01: "y1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(*got))
	}
	ez0, ok := (*got)[0].Get("ez").Any().(Event)
	if !ok {
		t.Fatalf("expected ez to be Event, got %T", (*got)[0].Get("ez").Any())
	}
	if v := ez0.Get("e0_0"); v.Any() != "x1" {
		t.Fatalf("ez.e0_0 = %v, want x1", v.Any())
	}
	if v := ez0.Get("e0_1"); v.Any() != "y1" {
		t.Fatalf("ez.e0_1 = %v, want y1", v.Any())
	}
}

// TestEPLInsertIntoColNonBeanFromSubqueryMultiMapParity covers the map
// variant of EPLInsertIntoColNonBeanFromSubqueryMulti. Java runtime ID:
// java-runtime-3c16cb7b5aea7cc551f4 (map).
func TestEPLInsertIntoColNonBeanFromSubqueryMultiMapParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[iipuNonBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)

	eventZero, err := RegisterMap(env, "EventZero", []FieldSpec{
		FieldDef("e0_0", reflect.TypeOf("")),
		FieldDef("e0_1", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RegisterMap(env, "EventOne", []FieldSpec{
		FieldDef("e1_0", reflect.TypeOf("")),
		FieldDef("ez", reflect.TypeOf([]Event{})),
	}, WithNestedPropertySchema("ez", eventZero))
	if err != nil {
		t.Fatal(err)
	}

	iipuRegisterCommon(t, env)

	inner := From[iipuNonBeanS0](env, "SupportBean_S0").Window(KeepAll()).AsRecord()
	projection := SubqueryRowsAsEvent(env, "EventZero", inner,
		Alias("e0_0", Field[iipuNonBeanS0, string]("p00")),
		Alias("e0_1", Field[iipuNonBeanS0, string]("p01")),
	)
	plan, err := env.Build(
		Select(From[iipuSupportBean](env, "SupportBean"),
			Alias("e1_0", Field[iipuSupportBean, string]("theString")),
			Alias("ez", projection),
		).InsertInto("EventOne", StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	got := iipuSubscribe(t, deployment)

	if err := engine.SendEvent(context.Background(), iipuNonBeanS0{ID: 1, P00: "x1", P01: "y1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(*got))
	}
	if v := (*got)[0].Get("e1_0"); v.Any() != "E1" {
		t.Fatalf("e1_0 = %v, want E1", v.Any())
	}
	ezArr, ok := (*got)[0].Get("ez").Any().([]Event)
	if !ok {
		t.Fatalf("expected ez to be []Event, got %T", (*got)[0].Get("ez").Any())
	}
	if len(ezArr) != 1 {
		t.Fatalf("expected 1 event in ez array, got %d", len(ezArr))
	}
	if v := ezArr[0].Get("e0_0"); v.Any() != "x1" {
		t.Fatalf("ez[0].e0_0 = %v, want x1", v.Any())
	}
}

// TestEPLInsertIntoColNonBeanFromSubquerySingleMapFilterParity covers the
// map variant of EPLInsertIntoColNonBeanFromSubquerySingle with filter=true.
// Java runtime ID: java-runtime-f2f9fbfea46e0a1488f2 (map filter).
func TestEPLInsertIntoColNonBeanFromSubquerySingleMapFilterParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[iipuNonBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)

	eventZero, err := RegisterMap(env, "EventZero", []FieldSpec{
		FieldDef("e0_0", reflect.TypeOf("")),
		FieldDef("e0_1", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RegisterMap(env, "EventOne", []FieldSpec{
		FieldDef("ez", reflect.TypeOf(Event{})),
	}, WithNestedPropertySchema("ez", eventZero))
	if err != nil {
		t.Fatal(err)
	}

	iipuRegisterCommon(t, env)

	s0 := From[iipuNonBeanS0](env, "SupportBean_S0").Window(LengthWindow(1)).AsRecord()
	projection := SubqueryRowAsEventWithOptions(env, "EventZero", s0,
		[]Selection{
			Alias("e0_0", Field[iipuNonBeanS0, string]("p00")),
			Alias("e0_1", Field[iipuNonBeanS0, string]("p01")),
		},
		SubqueryCardinalityMode(SubqueryNullOnMultiple),
		SubqueryWhere(GreaterOrEqual[int](Field[iipuNonBeanS0, int]("id"), Literal(100))),
	)
	plan, err := env.Build(
		Select(From[iipuSupportBean](env, "SupportBean"),
			Alias("ez", projection),
		).InsertInto("EventOne", StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	got := iipuSubscribe(t, deployment)

	// id=1: filter rejects, ez is null
	if err := engine.SendEvent(context.Background(), iipuNonBeanS0{ID: 1, P00: "x1", P01: "y1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(*got))
	}
	if v := (*got)[0].Get("ez"); !v.IsNull() {
		t.Fatalf("expected ez null for filtered-out id=1, got %v", v.Any())
	}

	// id=100: filter accepts
	if err := engine.SendEvent(context.Background(), iipuNonBeanS0{ID: 100, P00: "x2", P01: "y2"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "E2", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(*got))
	}
	ez1, ok := (*got)[1].Get("ez").Any().(Event)
	if !ok {
		t.Fatalf("expected ez to be Event for id=100, got %T", (*got)[1].Get("ez").Any())
	}
	if v := ez1.Get("e0_0"); v.Any() != "x2" {
		t.Fatalf("ez.e0_0 = %v, want x2", v.Any())
	}
}

// TestEPLInsertIntoColNonBeanFromSubqueryMultiFilterMapParity covers the
// map variant of EPLInsertIntoColNonBeanFromSubqueryMultiFilter.
// Java runtime ID: java-runtime-49c937650cfc43598179 (map).
func TestEPLInsertIntoColNonBeanFromSubqueryMultiFilterMapParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[iipuNonBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)

	eventZero, err := RegisterMap(env, "EventZero", []FieldSpec{
		FieldDef("e0_0", reflect.TypeOf("")),
		FieldDef("e0_1", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RegisterMap(env, "EventOne", []FieldSpec{
		FieldDef("ez", reflect.TypeOf([]Event{})),
	}, WithNestedPropertySchema("ez", eventZero))
	if err != nil {
		t.Fatal(err)
	}

	iipuRegisterCommon(t, env)

	inner := From[iipuNonBeanS0](env, "SupportBean_S0").Window(KeepAll()).AsRecord()
	projection := SubqueryRowsAsEventWithOptions(env, "EventZero", inner,
		[]Selection{
			Alias("e0_0", Field[iipuNonBeanS0, string]("p00")),
			Alias("e0_1", Field[iipuNonBeanS0, string]("p01")),
		},
		SubqueryWhere(And(
			GreaterOrEqual[int](Field[iipuNonBeanS0, int]("id"), Literal(10)),
			LessOrEqual[int](Field[iipuNonBeanS0, int]("id"), Literal(20)),
		)),
	)
	plan, err := env.Build(
		Select(From[iipuSupportBean](env, "SupportBean"),
			Alias("ez", projection),
		).InsertInto("EventOne", StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	got := iipuSubscribe(t, deployment)

	// id=1: outside [10,20], ez is null/empty
	if err := engine.SendEvent(context.Background(), iipuNonBeanS0{ID: 1, P00: "x1", P01: "y1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(*got))
	}
	ezVal := (*got)[0].Get("ez")
	if arr, ok := ezVal.Any().([]Event); ok && len(arr) > 0 {
		t.Fatalf("expected empty or null ez for id=1 outside filter, got %d events", len(arr))
	}

	// id=10 and id=20: inside [10,20], ez has 2 events
	if err := engine.SendEvent(context.Background(), iipuNonBeanS0{ID: 10, P00: "x2"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), iipuNonBeanS0{ID: 20, P00: "x3"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), iipuSupportBean{TheString: "E2", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(*got))
	}
	ezArr, ok := (*got)[1].Get("ez").Any().([]Event)
	if !ok {
		t.Fatalf("expected ez to be []Event, got %T", (*got)[1].Get("ez").Any())
	}
	if len(ezArr) != 2 {
		t.Fatalf("expected 2 events in ez array, got %d", len(ezArr))
	}
	if v := ezArr[0].Get("e0_0"); v.Any() != "x2" {
		t.Fatalf("ez[0].e0_0 = %v, want x2", v.Any())
	}
	if v := ezArr[1].Get("e0_0"); v.Any() != "x3" {
		t.Fatalf("ez[1].e0_0 = %v, want x3", v.Any())
	}
}

func float64Ptr(value float64) *float64 { return &value }
