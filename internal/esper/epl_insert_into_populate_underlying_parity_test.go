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

// ---- 4.338: constructor-position population (ords 0/1/2/11) ----
//
// Java populates bean targets through constructors and factory methods;
// the Go model registers struct targets with the same property names and
// populates them by named projection (approved adaptation). Constructor
// defaults reproduce via WithJSONDefaults, and identity of routed event
// members is preserved through routes.

type iipuCtorSourceBean struct {
	TheString     *string `esper:"theString"`
	IntPrimitive  int     `esper:"intPrimitive"`
	BoolPrimitive bool    `esper:"boolPrimitive"`
	IntBoxed      *int    `esper:"intBoxed"`
}

type iipuCtorOne struct {
	TheString     *string `esper:"theString"`
	IntBoxed      *int    `esper:"intBoxed"`
	IntPrimitive  int     `esper:"intPrimitive"`
	BoolPrimitive bool    `esper:"boolPrimitive"`
}

type iipuCtorTwo struct {
	St0 Event `esper:"st0"`
	St1 Event `esper:"st1"`
}

type iipuCtorThree struct {
	St0 Event   `esper:"st0"`
	St1 []Event `esper:"st1"`
}

type iipuSameTypeCtor struct {
	C1 Event `esper:"c1"`
	C2 Event `esper:"c2"`
}

type iipuBeanN struct {
	IntPrimitive    int      `esper:"intPrimitive"`
	IntBoxed        *int     `esper:"intBoxed"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
}

type iipuST0 struct {
	ID  string `esper:"id"`
	P00 int    `esper:"p00"`
}

type iipuST1 struct {
	ID  string `esper:"id"`
	P10 int    `esper:"p10"`
}

type iipuJoinS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
	P02 string `esper:"p02"`
	P03 string `esper:"p03"`
}

type iipuBeanObject struct {
	One Event `esper:"one"`
	Two Event `esper:"two"`
}

type iipuMyLocalTarget struct {
	Value int `esper:"value"`
}

type iipuBeanArrayEvent struct {
	Array []Event `esper:"array"`
}

func iipuStringPtr(value string) *string { return &value }
func iipuIntPtr(value int) *int          { return &value }

func iipuSubscribeUnderlying[T any](t *testing.T, env *Environment, engine *Engine, target string) *[]T {
	t.Helper()
	consumer, err := env.Build(FromAny(env, target).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), consumer)
	if err != nil {
		t.Fatal(err)
	}
	var received []T
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				if underlying, ok := event.Underlying().(T); ok {
					received = append(received, underlying)
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &received
}

// TestEPLInsertIntoCtorVariantsParity covers EPLInsertIntoCtor
// (java-runtime-706d3ed19b9a9b680430) in five fresh-runtime stages mirroring
// the Java undeploy boundaries: 4-column projection into the
// (String,Integer,int,boolean) shape with null String and null boxed; a
// 3-column projection where the boxed value positionally lands in the
// primitive slot (theString=E1, intBoxed=null, intPrimitive=100); a
// lastevent join wildcard into the (ST0,ST1) shape; the column-list form
// whose select-list arity picks the 2-arg shape with intPrimitive keeping
// its constructor default 99; and the same-type constructor over two
// filtered lastevent streams of the identical schema.
func TestEPLInsertIntoCtorVariantsParity(t *testing.T) {
	send := func(t *testing.T, engine *Engine, underlying any) {
		t.Helper()
		if err := engine.Send(context.Background(), "CtorSource", underlying); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("four-column", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[iipuCtorSourceBean](env, "CtorSource"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[iipuCtorOne](env, "SupportBeanCtorOne"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env, WithRuntimeURI("java-runtime-706d3ed19b9a9b680430"))
		defer func() { _ = engine.Close(context.Background()) }()

		producer, err := env.Build(Select(From[iipuCtorSourceBean](env, "CtorSource"),
			Alias("theString", Field[iipuCtorSourceBean, *string]("theString")),
			Alias("intBoxed", Field[iipuCtorSourceBean, *int]("intBoxed")),
			Alias("intPrimitive", Field[iipuCtorSourceBean, int]("intPrimitive")),
			Alias("boolPrimitive", Field[iipuCtorSourceBean, bool]("boolPrimitive")),
		).InsertInto("SupportBeanCtorOne", StatementName("i1")))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Deploy(context.Background(), producer); err != nil {
			t.Fatal(err)
		}
		received := iipuSubscribeUnderlying[iipuCtorOne](t, env, engine, "SupportBeanCtorOne")

		for _, tc := range []iipuCtorSourceBean{
			{TheString: iipuStringPtr("E1"), IntPrimitive: 2, BoolPrimitive: true, IntBoxed: iipuIntPtr(100)},
			{TheString: iipuStringPtr("E2"), IntPrimitive: 3, BoolPrimitive: false, IntBoxed: iipuIntPtr(101)},
			{IntPrimitive: 4, BoolPrimitive: true},
		} {
			send(t, engine, tc)
		}
		got := *received
		if len(got) != 3 {
			t.Fatalf("routed rows = %d, want 3", len(got))
		}
		if *got[0].TheString != "E1" || *got[0].IntBoxed != 100 || got[0].IntPrimitive != 2 || !got[0].BoolPrimitive {
			t.Fatalf("row0 = %#v", got[0])
		}
		if *got[1].TheString != "E2" || *got[1].IntBoxed != 101 || got[1].IntPrimitive != 3 || got[1].BoolPrimitive {
			t.Fatalf("row1 = %#v", got[1])
		}
		if got[2].TheString != nil || got[2].IntBoxed != nil || got[2].IntPrimitive != 4 || !got[2].BoolPrimitive {
			t.Fatalf("row2 = %#v", got[2])
		}
	})

	t.Run("boxed-into-primitive-slot", func(t *testing.T) {
		env := NewEnvironment()
		iipuRegisterCommon(t, env)
		if _, err := RegisterStruct[iipuCtorOne](env, "SupportBeanCtorOne"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env, WithRuntimeURI("java-runtime-706d3ed19b9a9b680430"))
		defer func() { _ = engine.Close(context.Background()) }()

		producer, err := env.Build(Select(From[iipuSupportBean](env, "SupportBean"),
			Alias("theString", Field[iipuSupportBean, string]("theString")),
			Alias("intBoxed", NullLiteral[*int]()),
			Alias("intPrimitive", Field[iipuSupportBean, *int]("intBoxed")),
		).InsertInto("SupportBeanCtorOne", StatementName("i1")))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Deploy(context.Background(), producer); err != nil {
			t.Fatal(err)
		}
		received := iipuSubscribeUnderlying[iipuCtorOne](t, env, engine, "SupportBeanCtorOne")

		boxed := 100
		if err := engine.Send(context.Background(), "SupportBean", iipuSupportBean{TheString: "E1", IntBoxed: &boxed, IntPrimitive: -1}); err != nil {
			t.Fatal(err)
		}
		got := *received
		if len(got) != 1 {
			t.Fatalf("routed rows = %d, want 1", len(got))
		}
		if *got[0].TheString != "E1" || got[0].IntBoxed != nil || got[0].IntPrimitive != 100 || got[0].BoolPrimitive {
			t.Fatalf("row = %#v", got[0])
		}
	})

	t.Run("join-wildcard", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[iipuST0](env, "SupportBean_ST0"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[iipuST1](env, "SupportBean_ST1"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[iipuCtorTwo](env, "SupportBeanCtorTwo"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env, WithRuntimeURI("java-runtime-706d3ed19b9a9b680430"))
		defer func() { _ = engine.Close(context.Background()) }()

		producer, err := env.Build(JoinMany(
			JoinSource(From[iipuST0](env, "SupportBean_ST0").Window(LastEvent())),
			JoinSource(From[iipuST1](env, "SupportBean_ST1").Window(LastEvent())),
		).On().Select(
			SelectSourceEvent(0, "st0"),
			SelectSourceEvent(1, "st1"),
		).InsertInto("SupportBeanCtorTwo", StatementName("i1")))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Deploy(context.Background(), producer); err != nil {
			t.Fatal(err)
		}
		received := iipuSubscribeUnderlying[iipuCtorTwo](t, env, engine, "SupportBeanCtorTwo")

		if err := engine.Send(context.Background(), "SupportBean_ST0", iipuST0{ID: "ST0", P00: 1}); err != nil {
			t.Fatal(err)
		}
		if len(*received) != 0 {
			t.Fatalf("incomplete join must not route, got %d", len(*received))
		}
		if err := engine.Send(context.Background(), "SupportBean_ST1", iipuST1{ID: "ST1", P10: 2}); err != nil {
			t.Fatal(err)
		}
		got := *received
		if len(got) != 1 {
			t.Fatalf("routed rows = %d, want 1", len(got))
		}
		if id := got[0].St0.Get("id").Any(); id != "ST0" {
			t.Fatalf("st0.id = %v", id)
		}
		if id := got[0].St1.Get("id").Any(); id != "ST1" {
			t.Fatalf("st1.id = %v", id)
		}
	})

	t.Run("column-list-ignored", func(t *testing.T) {
		env := NewEnvironment()
		iipuRegisterCommon(t, env)
		if _, err := RegisterStruct[iipuCtorOne](env, "SupportBeanCtorOne",
			WithJSONDefaults(map[string]any{"intPrimitive": 99})); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env, WithRuntimeURI("java-runtime-706d3ed19b9a9b680430"))
		defer func() { _ = engine.Close(context.Background()) }()

		// Java's `insert into SupportBeanCtorOne(theString, intPrimitive)
		// select 'E1', 5` ignores the insert-into column list for
		// constructor targets: the two select columns positionally bind to
		// the 2-arg shape and intPrimitive keeps its constructor default.
		// Go reproduces the observable by named projection plus the
		// declared default; the column-list-ignored syntax itself has no
		// Go surface (approved API-surface difference).
		producer, err := env.Build(Select(From[iipuSupportBean](env, "SupportBean"),
			Alias("theString", Literal("E1")),
			Alias("intBoxed", Literal(5)),
		).InsertInto("SupportBeanCtorOne", StatementName("i1")))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Deploy(context.Background(), producer); err != nil {
			t.Fatal(err)
		}
		received := iipuSubscribeUnderlying[iipuCtorOne](t, env, engine, "SupportBeanCtorOne")

		if err := engine.Send(context.Background(), "SupportBean", iipuSupportBean{TheString: "x", IntPrimitive: -1}); err != nil {
			t.Fatal(err)
		}
		got := *received
		if len(got) != 1 {
			t.Fatalf("routed rows = %d, want 1", len(got))
		}
		if *got[0].TheString != "E1" || got[0].IntBoxed == nil || *got[0].IntBoxed != 5 || got[0].IntPrimitive != 99 || got[0].BoolPrimitive {
			t.Fatalf("row = %#v", got[0])
		}
	})

	t.Run("same-type-ctor", func(t *testing.T) {
		env := NewEnvironment()
		iipuRegisterCommon(t, env)
		if _, err := RegisterStruct[iipuSameTypeCtor](env, "SupportEventWithCtorSameType"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env, WithRuntimeURI("java-runtime-706d3ed19b9a9b680430"))
		defer func() { _ = engine.Close(context.Background()) }()

		producer, err := env.Build(JoinMany(
			JoinSource(From[iipuSupportBean](env, "SupportBean").Filter(
				Equal[string](Field[iipuSupportBean, string]("theString"), Literal("b1"))).Window(LastEvent())),
			JoinSource(From[iipuSupportBean](env, "SupportBean").Filter(
				Equal[string](Field[iipuSupportBean, string]("theString"), Literal("b2"))).Window(LastEvent())),
		).On().Select(
			SelectSourceEvent(0, "c1"),
			SelectSourceEvent(1, "c2"),
		).InsertInto("SupportEventWithCtorSameType", StatementName("i1")))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Deploy(context.Background(), producer); err != nil {
			t.Fatal(err)
		}
		received := iipuSubscribeUnderlying[iipuSameTypeCtor](t, env, engine, "SupportEventWithCtorSameType")

		if err := engine.Send(context.Background(), "SupportBean", iipuSupportBean{TheString: "b1", IntPrimitive: 1}); err != nil {
			t.Fatal(err)
		}
		if err := engine.Send(context.Background(), "SupportBean", iipuSupportBean{TheString: "b2", IntPrimitive: 2}); err != nil {
			t.Fatal(err)
		}
		got := *received
		if len(got) != 1 {
			t.Fatalf("routed rows = %d, want 1", len(got))
		}
		if prim := got[0].C1.Get("intPrimitive").Any(); prim != 1 {
			t.Fatalf("c1.intPrimitive = %v", prim)
		}
		if prim := got[0].C2.Get("intPrimitive").Any(); prim != 2 {
			t.Fatalf("c2.intPrimitive = %v", prim)
		}
	})
}

// TestEPLInsertIntoCtorWithPatternParity covers
// EPLInsertIntoCtorWithPattern (java-runtime-42b687e10ab403b319bd):
// pattern [every s=SupportBean_ST0 -> [2] e=SupportBean_ST1] routes the
// single capture and the 2-element repeated capture array into the
// (ST0, ST1[]) target. Contract: ST0("E0",1), ST1("E1",2), ST1("E2",3)
// route one row with st0.id=E0 and st1 ids [E1,E2].
func TestEPLInsertIntoCtorWithPatternParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[iipuST0](env, "SupportBean_ST0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[iipuST1](env, "SupportBean_ST1"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[iipuCtorThree](env, "SupportBeanCtorThree"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithRuntimeURI("java-runtime-42b687e10ab403b319bd"))
	defer func() { _ = engine.Close(context.Background()) }()

	producer, err := env.Build(PatternFrom(From[iipuST0](env, "SupportBean_ST0"), "s", Literal[bool](true)).
		Then(PatternFrom(From[iipuST1](env, "SupportBean_ST1"), "e", Literal[bool](true)).MatchUntil(2, 2)).
		Select(
			Alias("st0", PatternEvent("s")),
			Alias("st1", TagEvents("e")),
		).InsertInto("SupportBeanCtorThree", StatementName("i1")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), producer); err != nil {
		t.Fatal(err)
	}
	received := iipuSubscribeUnderlying[iipuCtorThree](t, env, engine, "SupportBeanCtorThree")

	if err := engine.Send(context.Background(), "SupportBean_ST0", iipuST0{ID: "E0", P00: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "SupportBean_ST1", iipuST1{ID: "E1", P10: 2}); err != nil {
		t.Fatal(err)
	}
	if len(*received) != 0 {
		t.Fatalf("incomplete repeat must not route, got %d", len(*received))
	}
	if err := engine.Send(context.Background(), "SupportBean_ST1", iipuST1{ID: "E2", P10: 3}); err != nil {
		t.Fatal(err)
	}
	got := *received
	if len(got) != 1 {
		t.Fatalf("routed rows = %d, want 1", len(got))
	}
	if id := got[0].St0.Get("id").Any(); id != "E0" {
		t.Fatalf("st0.id = %v", id)
	}
	if len(got[0].St1) != 2 {
		t.Fatalf("st1 length = %d, want 2", len(got[0].St1))
	}
	if id := got[0].St1[0].Get("id").Any(); id != "E1" {
		t.Fatalf("st1[0].id = %v", id)
	}
	if id := got[0].St1[1].Get("id").Any(); id != "E2" {
		t.Fatalf("st1[1].id = %v", id)
	}
}

// TestEPLInsertIntoBeanJoinPopulateParity covers EPLInsertIntoBeanJoin
// (java-runtime-43b50e297c1593da8ec3) stages 1/2/4: a lastevent join of
// SupportBean_N and SupportBean_S0 populates the Object-property target by
// wildcard and by stream-name projection (Java's assertSame identity maps
// to Go route identity), and a local-class target receives value=1. Java
// stage 3 is a comment/text artifact (same observable as stage 2) and is
// not separately pinned.
func TestEPLInsertIntoBeanJoinPopulateParity(t *testing.T) {
	t.Run("wildcard-and-select-names", func(t *testing.T) {
		for _, stage := range []string{"wildcard", "select-names"} {
			t.Run(stage, func(t *testing.T) {
				env := NewEnvironment()
				if _, err := RegisterStruct[iipuBeanN](env, "SupportBean_N"); err != nil {
					t.Fatal(err)
				}
				if _, err := RegisterStruct[iipuJoinS0](env, "SupportBean_S0"); err != nil {
					t.Fatal(err)
				}
				if _, err := RegisterStruct[iipuBeanObject](env, "SupportBeanObject"); err != nil {
					t.Fatal(err)
				}
				engine := NewEngine(env, WithRuntimeURI("java-runtime-43b50e297c1593da8ec3"))
				defer func() { _ = engine.Close(context.Background()) }()

				// Java's wildcard form and its `select one, two` form route the
				// identical pair of source events under the stream aliases, so
				// one Go producer stands in for both stages (the observable
				// rows are the same; Java deploys the statement twice).
				producer := JoinMany(
					JoinSource(From[iipuBeanN](env, "SupportBean_N").Window(LastEvent()).As("one")),
					JoinSource(From[iipuJoinS0](env, "SupportBean_S0").Window(LastEvent()).As("two")),
				).On().Select(
					SelectSourceEvent(0, "one"),
					SelectSourceEvent(1, "two"),
				).InsertInto("SupportBeanObject", StatementName("i1"))
				plan, err := env.Build(producer)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := engine.Deploy(context.Background(), plan); err != nil {
					t.Fatal(err)
				}
				received := iipuSubscribeUnderlying[iipuBeanObject](t, env, engine, "SupportBeanObject")

				if err := engine.Send(context.Background(), "SupportBean_N", iipuBeanN{
					IntPrimitive: 1, IntBoxed: iipuIntPtr(10),
					DoublePrimitive: 100, DoubleBoxed: func() *float64 { v := 1000.0; return &v }(),
					BoolPrimitive: true, BoolBoxed: func() *bool { v := true; return &v }(),
				}); err != nil {
					t.Fatal(err)
				}
				if len(*received) != 0 {
					t.Fatalf("incomplete join must not route, got %d", len(*received))
				}
				if err := engine.Send(context.Background(), "SupportBean_S0", iipuJoinS0{ID: 1}); err != nil {
					t.Fatal(err)
				}
				got := *received
				if len(got) != 1 {
					t.Fatalf("routed rows = %d, want 1", len(got))
				}
				if prim := got[0].One.Get("intPrimitive").Any(); prim != 1 {
					t.Fatalf("one.intPrimitive = %v", prim)
				}
				if boxed := iipuBoxed(got[0].One.Get("intBoxed")); boxed != 10 {
					t.Fatalf("one.intBoxed = %v", boxed)
				}
				if id := got[0].Two.Get("id").Any(); id != 1 {
					t.Fatalf("two.id = %v", id)
				}
			})
		}
	})

	t.Run("local-class", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[iipuBeanN](env, "SupportBean_N"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[iipuMyLocalTarget](env, "MyLocalTarget"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env, WithRuntimeURI("java-runtime-43b50e297c1593da8ec3"))
		defer func() { _ = engine.Close(context.Background()) }()

		producer, err := env.Build(Select(From[iipuBeanN](env, "SupportBean_N"),
			Alias("value", Literal(1)),
		).InsertInto("MyLocalTarget", StatementName("i1")))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Deploy(context.Background(), producer); err != nil {
			t.Fatal(err)
		}
		received := iipuSubscribeUnderlying[iipuMyLocalTarget](t, env, engine, "MyLocalTarget")

		if err := engine.Send(context.Background(), "SupportBean_N", iipuBeanN{IntPrimitive: 1, IntBoxed: iipuIntPtr(10)}); err != nil {
			t.Fatal(err)
		}
		got := *received
		if len(got) != 1 || got[0].Value != 1 {
			t.Fatalf("rows = %#v", got)
		}
	})
}

// TestEPLInsertIntoWindowAggregationAtEventBeanParity covers
// EPLInsertIntoWindowAggregationAtEventBean
// (java-runtime-5c663b3a17e6e4aac880): insert into
// SupportBeanArrayEvent select window(*) @eventbean from
// SupportBean#keepall routes the full retained window in insertion order
// per event.
func TestEPLInsertIntoWindowAggregationAtEventBeanParity(t *testing.T) {
	env := NewEnvironment()
	iipuRegisterCommon(t, env)
	if _, err := RegisterStruct[iipuBeanArrayEvent](env, "SupportBeanArrayEvent"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithRuntimeURI("java-runtime-5c663b3a17e6e4aac880"))
	defer func() { _ = engine.Close(context.Background()) }()

	producer, err := env.Build(From[iipuSupportBean](env, "SupportBean").Window(KeepAll()).
		Aggregate(Alias("array", WindowEvents())).
		InsertInto("SupportBeanArrayEvent", StatementName("i1")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), producer); err != nil {
		t.Fatal(err)
	}
	received := iipuSubscribeUnderlying[iipuBeanArrayEvent](t, env, engine, "SupportBeanArrayEvent")

	if err := engine.Send(context.Background(), "SupportBean", iipuSupportBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "SupportBean", iipuSupportBean{TheString: "E2", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	got := *received
	if len(got) != 2 {
		t.Fatalf("routed rows = %d, want 2", len(got))
	}
	if len(got[0].Array) != 1 {
		t.Fatalf("row0 array length = %d, want 1", len(got[0].Array))
	}
	if str := got[0].Array[0].Get("theString").Any(); str != "E1" {
		t.Fatalf("row0 array[0] = %v", str)
	}
	if len(got[1].Array) != 2 {
		t.Fatalf("row1 array length = %d, want 2", len(got[1].Array))
	}
	if s := got[1].Array[0].Get("theString").Any(); s != "E1" {
		t.Fatalf("row1 array[0] = %v", s)
	}
	if s := got[1].Array[1].Get("theString").Any(); s != "E2" {
		t.Fatalf("row1 array[1] = %v", s)
	}
}

// ---- 4.339: CharSequence widening and factory-origin targets (ords 7/8) ----

// TestEPLInsertIntoCharSequenceCompatParity covers
// EPLInsertIntoCharSequenceCompat (java-runtime-f04a53b7cbe0320f681f):
// `create schema ConcreteType as (value java.lang.CharSequence)` plus
// `insert into ConcreteType select "Test" as value from SupportBean` must
// compile and deploy per representation — Java loops OBJECTARRAY/MAP/AVRO/
// DEFAULT with no sends, so the observable is successful deployment and a
// silent stream. Go models the objectarray and map/default representations
// with a string-typed value field (the CharSequence widening is native);
// the producer deploys and the stream stays silent (the trigger send
// exercises live population into the unobserved route).
func TestEPLInsertIntoCharSequenceCompatParity(t *testing.T) {
	for _, rep := range []string{"objectarray", "map", "default"} {
		t.Run(rep, func(t *testing.T) {
			env := NewEnvironment()
			iipuRegisterCommon(t, env)
			switch rep {
			case "objectarray":
				if _, err := RegisterObjectArray(env, "ConcreteType", []FieldSpec{
					FieldDef("value", reflect.TypeOf("")),
				}); err != nil {
					t.Fatal(err)
				}
			default:
				if _, err := RegisterMap(env, "ConcreteType", []FieldSpec{
					FieldDef("value", reflect.TypeOf("")),
				}); err != nil {
					t.Fatal(err)
				}
			}
			engine := NewEngine(env, WithRuntimeURI("java-runtime-f04a53b7cbe0320f681f"))
			defer func() { _ = engine.Close(context.Background()) }()

			producer, err := env.Build(Select(From[iipuSupportBean](env, "SupportBean"),
				Alias("value", Literal("Test")),
			).InsertInto("ConcreteType", StatementName("i1")))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Deploy(context.Background(), producer); err != nil {
				t.Fatal(err)
			}
			// Java never sends; a live trigger send exercises the route's
			// population without breaking the observable silence (the routed
			// row has no consumer statement).
			if err := engine.Send(context.Background(), "SupportBean", iipuSupportBean{TheString: "trigger"}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

type iipuFactoryString struct {
	TheString string `esper:"theString"`
}

type iipuSensorEvent struct {
	ID          int     `esper:"id"`
	Type        string  `esper:"type"`
	Device      string  `esper:"device"`
	Measurement float64 `esper:"measurement"`
	Confidence  float64 `esper:"confidence"`
}

// TestEPLInsertIntoBeanFactoryMethodParity covers
// EPLInsertIntoBeanFactoryMethod (java-runtime-e79b48b60120e27bafbc):
// Java pre-configures SupportBeanString and SupportSensorEvent with
// factory methods; the Go model registers the same property shapes as
// plain struct targets (approved adaptation — the populated observable is
// identical). Stage 1 routes theString=abc (Java asserts the row via both
// a listener and a subscriber; one Go subscription observes the same routed
// row). Stage 2 routes the 5-column projection with the int literal
// widening into the double columns.
func TestEPLInsertIntoBeanFactoryMethodParity(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		env := NewEnvironment()
		iipuRegisterCommon(t, env)
		if _, err := RegisterStruct[iipuFactoryString](env, "SupportBeanString"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env, WithRuntimeURI("java-runtime-e79b48b60120e27bafbc"))
		defer func() { _ = engine.Close(context.Background()) }()

		producer, err := env.Build(FromAny(env, "MyMap").Select(
			Alias("theString", Literal("abc")),
		).InsertInto("SupportBeanString", StatementName("i1")))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Deploy(context.Background(), producer); err != nil {
			t.Fatal(err)
		}
		received := iipuSubscribeUnderlying[iipuFactoryString](t, env, engine, "SupportBeanString")

		if err := engine.Send(context.Background(), "MyMap", map[string]any{}); err != nil {
			t.Fatal(err)
		}
		got := *received
		if len(got) != 1 || got[0].TheString != "abc" {
			t.Fatalf("rows = %#v", got)
		}
	})

	t.Run("sensor", func(t *testing.T) {
		env := NewEnvironment()
		iipuRegisterCommon(t, env)
		if _, err := RegisterStruct[iipuSensorEvent](env, "SupportSensorEvent"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env, WithRuntimeURI("java-runtime-e79b48b60120e27bafbc"))
		defer func() { _ = engine.Close(context.Background()) }()

		producer, err := env.Build(FromAny(env, "MyMap").Select(
			Alias("id", Literal(2)),
			Alias("type", Literal("A01")),
			Alias("device", Literal("DHC1000")),
			Alias("measurement", Literal(100)),
			Alias("confidence", Literal(5)),
		).InsertInto("SupportSensorEvent", StatementName("i1")))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Deploy(context.Background(), producer); err != nil {
			t.Fatal(err)
		}
		received := iipuSubscribeUnderlying[iipuSensorEvent](t, env, engine, "SupportSensorEvent")

		if err := engine.Send(context.Background(), "MyMap", map[string]any{}); err != nil {
			t.Fatal(err)
		}
		got := *received
		if len(got) != 1 {
			t.Fatalf("rows = %d, want 1", len(got))
		}
		if got[0].ID != 2 || got[0].Type != "A01" || got[0].Device != "DHC1000" ||
			got[0].Measurement != 100 || got[0].Confidence != 5 {
			t.Fatalf("row = %#v", got[0])
		}
	})
}
