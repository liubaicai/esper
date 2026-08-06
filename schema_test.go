package esper

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"
)

type schemaTestTrade struct {
	Symbol  string  `esper:"symbol"`
	Price   float64 `esper:"price"`
	Qty     int     `json:"qty"`
	Note    *string `esper:"note,optional"`
	Address struct {
		City string `esper:"city"`
	} `esper:"address"`
}

func TestStructSchemaAndTaggedPropertyAccess(t *testing.T) {
	schema, err := StructSchema[schemaTestTrade]("Trade")
	if err != nil {
		t.Fatal(err)
	}
	event, err := newEvent(schema, schemaTestTrade{Symbol: "A", Price: 12.5, Qty: 3, Address: struct {
		City string `esper:"city"`
	}{City: "Shanghai"}}, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := As[string](event.Get("symbol")); err != nil || got != "A" {
		t.Fatalf("symbol = %q, %v", got, err)
	}
	if got, err := As[float64](event.Get("price")); err != nil || got != 12.5 {
		t.Fatalf("price = %v, %v", got, err)
	}
	if !event.Get("note").IsNull() {
		t.Fatalf("nil pointer field must be Null, got %v", event.Get("note"))
	}
	if !event.Get("missing").IsMissing() {
		t.Fatalf("unknown field must be Missing, got %v", event.Get("missing"))
	}
	if got, err := As[string](event.Get("address.city")); err != nil || got != "Shanghai" {
		t.Fatalf("nested city = %q, %v", got, err)
	}
}

func TestMapSchemaCaseResolutionAndJSON(t *testing.T) {
	schema, err := NewJSONSchema("Quote", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("price", reflect.TypeOf(float64(0))),
	}, WithPropertyResolution(PropertyCaseInsensitive))
	if err != nil {
		t.Fatal(err)
	}
	event, err := ParseJSON(schema, []byte(`{"SYMBOL":"A","price":12.5}`), time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got := event.Get("symbol").Any(); got != "A" {
		t.Fatalf("case-insensitive symbol = %#v", got)
	}
	if got, err := As[float64](event.Get("price")); err != nil || got != 12.5 {
		t.Fatalf("JSON price = %v, %v", got, err)
	}
}

func TestSchemaRejectsDuplicateFields(t *testing.T) {
	_, err := NewSchema("Bad", FieldDef("x", reflect.TypeOf(0)), FieldDef("x", reflect.TypeOf("")))
	if err == nil {
		t.Fatal("duplicate field should fail schema construction")
	}
}

func TestXMLAndAvroJSONSchemasParseTypedValues(t *testing.T) {
	fields := []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("price", reflect.TypeOf(float64(0))),
		FieldDef("qty", reflect.TypeOf(int64(0))),
	}
	xmlSchema, err := NewXMLSchema("TradeXML", fields)
	if err != nil {
		t.Fatal(err)
	}
	xmlEvent, err := ParseXML(xmlSchema, []byte(`<trade><symbol>ESPER</symbol><price>12.5</price><qty>3</qty></trade>`), time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if xmlEvent.Get("symbol").Any() != "ESPER" || xmlEvent.Get("price").Any() != float64(12.5) || xmlEvent.Get("qty").Any() != int64(3) {
		t.Fatalf("XML values = symbol=%v price=%v qty=%v", xmlEvent.Get("symbol"), xmlEvent.Get("price"), xmlEvent.Get("qty"))
	}
	avroSchema, err := NewAvroSchema("TradeAvro", fields)
	if err != nil {
		t.Fatal(err)
	}
	avroEvent, err := ParseAvroJSON(avroSchema, []byte(`{"symbol":"ESPER","price":12.5,"qty":3}`), time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if avroEvent.Get("symbol").Any() != "ESPER" || avroEvent.Get("price").Any() != float64(12.5) || avroEvent.Get("qty").Any() != int64(3) {
		t.Fatalf("Avro values = symbol=%v price=%v qty=%v", avroEvent.Get("symbol"), avroEvent.Get("price"), avroEvent.Get("qty"))
	}
}

func TestXMLParserRejectsMalformedInput(t *testing.T) {
	schema, err := NewXMLSchema("TradeXML", []FieldSpec{FieldDef("symbol", reflect.TypeOf(""))})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseXML(schema, []byte(`<trade>`), time.Unix(0, 0)); err == nil {
		t.Fatal("malformed XML was accepted")
	}
}

func TestXMLSchemaAttributesRepeatedElementsAndIndexedPaths(t *testing.T) {
	schema, err := NewXMLSchema("TradeXML", []FieldSpec{
		FieldDef("trade.@id", reflect.TypeOf("")),
		FieldDef("trade.item[0].name", reflect.TypeOf("")),
		FieldDef("trade.item[1].price", reflect.TypeOf(int64(0))),
		FieldDef("names", reflect.TypeOf([]string{})),
	})
	if err != nil {
		t.Fatal(err)
	}
	event, err := ParseXML(schema, []byte(`<trade id="T"><item><name>A&amp;B</name><price>1</price></item><item><name>C</name><price>2</price></item></trade>`), time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got := event.Get("trade.@id").Any(); got != "T" {
		t.Fatalf("XML attribute = %#v", got)
	}
	if got := event.Get("trade.item[0].name").Any(); got != "A&B" {
		t.Fatalf("first repeated XML item = %#v", got)
	}
	if got, err := As[int64](event.Get("trade.item[1].price")); err != nil || got != 2 {
		t.Fatalf("second repeated XML price = %v, %v", got, err)
	}
	if got := event.Get("names"); !got.IsMissing() {
		t.Fatalf("unconfigured leaf array should not fabricate a value: %v", got)
	}
	if _, err := ParseXMLWithOptions(schema, []byte(`<trade><a><b><c>1</c></b></a></trade>`), time.Unix(0, 0), WithXMLParseMaxDepth(2)); err == nil {
		t.Fatal("deep XML should be rejected by max depth")
	}
}

type schemaTestIndexedMapped struct {
	Array  []string          `esper:"array"`
	Mapped map[string]string `esper:"mapped"`
	Nested []struct {
		Name string `esper:"name"`
	} `esper:"nested"`
}

func TestIndexedMappedAndEscapedPropertyAccess(t *testing.T) {
	schema, err := StructSchema[schemaTestIndexedMapped]("IndexedMapped")
	if err != nil {
		t.Fatal(err)
	}
	event, err := newEvent(schema, schemaTestIndexedMapped{
		Array:  []string{"a", "b"},
		Mapped: map[string]string{"a.b": "mapped", "plain": "value"},
		Nested: []struct {
			Name string `esper:"name"`
		}{{Name: "nested"}},
	}, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name string
		want any
	}{
		{name: "array[0]", want: "a"},
		{name: "array[1]?", want: "b"},
		{name: "nested[0].name", want: "nested"},
		{name: `mapped('a.b')`, want: "mapped"},
		{name: `mapped("plain")?`, want: "value"},
	}
	for _, check := range checks {
		if got := event.Get(check.name); !got.IsPresent() || got.Any() != check.want {
			t.Errorf("Get(%q) = %v (%v), want %v", check.name, got.Any(), got.State(), check.want)
		}
	}
	if !event.Get("array[2]").IsMissing() {
		t.Fatal("out-of-range indexed property must be Missing")
	}
	if !event.Get(`mapped('missing')`).IsMissing() {
		t.Fatal("unknown mapped key must be Missing")
	}
	if !event.Get("array[-1]").IsMissing() {
		t.Fatal("negative indexed property must be Missing")
	}
	if !event.Get("nested[0].missing").IsMissing() {
		t.Fatal("unknown nested property must be Missing")
	}
}

type schemaTestGetterOnly struct {
	value string
}

func (e schemaTestGetterOnly) GetValue() string { return e.value }

func TestJavaBeanGetterFallbackForDynamicProperty(t *testing.T) {
	schema, err := StructSchema[schemaTestGetterOnly]("GetterOnly", WithAccessorStyle(AccessorJavaBean))
	if err != nil {
		t.Fatal(err)
	}
	event, err := newEvent(schema, schemaTestGetterOnly{value: "getter"}, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got := event.Get("value"); !got.IsPresent() || got.Any() != "getter" {
		t.Fatalf("getter property = %v (%v)", got.Any(), got.State())
	}
}

type schemaTestSetterBean struct {
	value string
}

func (e schemaTestSetterBean) GetValue() string { return e.value }
func (e *schemaTestSetterBean) SetValue(value string) error {
	if value == "reject" {
		return fmt.Errorf("rejected value")
	}
	e.value = value
	return nil
}

type schemaTestConflictingSetterBean struct{}

func (schemaTestConflictingSetterBean) GetValue() string { return "" }
func (*schemaTestConflictingSetterBean) SetValue(int)    {}

type schemaTestWriteOnlyBean struct{}

func (*schemaTestWriteOnlyBean) SetValue(string) {}

func TestJavaBeanSetterMaterializesGetterOnlyStruct(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[schemaTestSetterBean](env, "SetterBean", WithAccessorStyle(AccessorJavaBean))
	if err != nil {
		t.Fatal(err)
	}
	underlying, err := projectMapToSchema(schema, map[string]any{"value": "abc"})
	if err != nil {
		t.Fatal(err)
	}
	bean, ok := underlying.(schemaTestSetterBean)
	if !ok || bean.GetValue() != "abc" {
		t.Fatalf("setter-backed underlying = %#v", underlying)
	}
	event, err := newEvent(schema, underlying, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got := event.Get("value"); !got.IsPresent() || got.Any() != "abc" {
		t.Fatalf("setter-backed getter value = %v (%v)", got.Any(), got.State())
	}
	if _, err := projectMapToSchema(schema, map[string]any{"value": "reject"}); err == nil || !strings.Contains(err.Error(), "rejected value") {
		t.Fatalf("setter error = %v", err)
	}
	plan, err := env.Build(From[schemaTestSetterBean](env, "SetterBean").Query())
	if err != nil {
		t.Fatal(err)
	}
	if canonical := string(plan.Canonical()); !strings.Contains(canonical, "setters=value:string:method:SetValue") {
		t.Fatalf("setter missing from plan canonical: %s", canonical)
	}
	if _, err := StructSchema[schemaTestConflictingSetterBean]("ConflictingSetterBean", WithAccessorStyle(AccessorJavaBean)); err == nil {
		t.Fatal("JavaBean schema accepted conflicting getter and setter types")
	}
}

func TestJavaBeanWriteOnlySetterIsNotReadableProperty(t *testing.T) {
	schema, err := StructSchema[schemaTestWriteOnlyBean]("WriteOnlyBean", WithAccessorStyle(AccessorJavaBean))
	if err != nil {
		t.Fatal(err)
	}
	if names := schema.PropertyNames(); len(names) != 0 {
		t.Fatalf("write-only JavaBean properties = %#v, want none", names)
	}
	event, err := newEvent(schema, schemaTestWriteOnlyBean{}, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !event.Get("value").IsMissing() {
		t.Fatalf("write-only JavaBean value = %v, want Missing", event.Get("value"))
	}
}

func TestExplicitPropertySetterCallbackMaterializesStruct(t *testing.T) {
	schema, err := StructSchema[schemaTestGetterOnly]("ExplicitSetterBean",
		WithAccessorStyle(AccessorExplicit),
		WithPropertyMethod("value", "GetValue"),
		WithTypedPropertySetter[string]("value", func(underlying any, value string) error {
			bean, ok := underlying.(*schemaTestGetterOnly)
			if !ok {
				return fmt.Errorf("setter underlying = %T", underlying)
			}
			bean.value = value
			return nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	underlying, err := projectMapToSchema(schema, map[string]any{"value": "callback"})
	if err != nil {
		t.Fatal(err)
	}
	if bean, ok := underlying.(schemaTestGetterOnly); !ok || bean.GetValue() != "callback" {
		t.Fatalf("callback setter underlying = %#v", underlying)
	}
	if _, err := NewMapSchema("InvalidMapSetter", nil, WithTypedPropertySetter[string]("value", func(any, string) error { return nil })); err == nil {
		t.Fatal("map schema accepted a struct property setter")
	}
}

type schemaTestJavaBeanMetadata struct {
	value string
	ready bool
	url   string
}

func (e schemaTestJavaBeanMetadata) GetValue() string { return e.value }
func (e schemaTestJavaBeanMetadata) IsReady() bool    { return e.ready }
func (e schemaTestJavaBeanMetadata) GetURL() string   { return e.url }

func TestJavaBeanAccessorPublishesGetterMetadataAndRenders(t *testing.T) {
	schema, err := StructSchema[schemaTestJavaBeanMetadata]("JavaBeanMetadata", WithAccessorStyle(AccessorJavaBean))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"value", "ready", "URL"} {
		if _, ok := schema.Property(name); !ok {
			t.Fatalf("JavaBean property %q missing from metadata: %#v", name, schema.PropertyNames())
		}
	}
	if descriptor, ok := schema.Property("ready"); !ok || descriptor.Type != reflect.TypeOf(false) {
		t.Fatalf("ready descriptor = %#v, %v", descriptor, ok)
	}
	event, err := newEvent(schema, schemaTestJavaBeanMetadata{value: "getter", ready: true, url: "https://example"}, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if event.Get("value").Any() != "getter" || event.Get("ready").Any() != true || event.Get("URL").Any() != "https://example" {
		t.Fatalf("JavaBean values = value=%v ready=%v URL=%v", event.Get("value"), event.Get("ready"), event.Get("URL"))
	}
	rendered, err := RenderJSON(event)
	if err != nil || !strings.Contains(rendered, `"value":"getter"`) || !strings.Contains(rendered, `"ready":true`) {
		t.Fatalf("JavaBean render = %s, %v", rendered, err)
	}
}

type schemaTestExplicitNested struct {
	FieldNestedValue string `esper:"fieldNestedValue"`
}

func (e schemaTestExplicitNested) ReadNestedValue() string { return e.FieldNestedValue }

type schemaTestExplicitRoot struct {
	FieldLegacyVal   string                   `esper:"fieldLegacyVal"`
	FieldStringArray []string                 `esper:"fieldStringArray"`
	FieldNested      schemaTestExplicitNested `esper:"fieldNested"`
	mapValues        map[string]string
}

func (e schemaTestExplicitRoot) ReadLegacyBeanVal() string  { return e.FieldLegacyVal }
func (e schemaTestExplicitRoot) ReadAlternativeVal() string { return e.FieldLegacyVal }
func (e schemaTestExplicitRoot) ReadStringArray() []string  { return e.FieldStringArray }
func (e schemaTestExplicitRoot) ReadStringIndexed(index int) string {
	return e.FieldStringArray[index]
}
func (e schemaTestExplicitRoot) ReadMapByKey(key string) string { return e.mapValues[key] }
func (e schemaTestExplicitRoot) ReadNested() schemaTestExplicitNested {
	return e.FieldNested
}

func TestExplicitAccessorFieldMethodPathAndNestedSchema(t *testing.T) {
	stringType := reflect.TypeOf("")
	nestedSchema, err := StructSchema[schemaTestExplicitNested]("ExplicitNested",
		WithAccessorStyle(AccessorExplicit),
		WithPropertyPath("fieldNestedClassValue", "FieldNestedValue", stringType),
		WithPropertyMethod("readNestedClassValue", "ReadNestedValue", stringType),
	)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := StructSchema[schemaTestExplicitRoot]("ExplicitRoot",
		WithAccessorStyle(AccessorExplicit),
		WithPropertyPath("explicitFSimple", "FieldLegacyVal", stringType),
		WithPropertyPath("explicitFIndexed", "FieldStringArray", reflect.TypeOf([]string{})),
		WithPropertyPath("explicitFNested", "FieldNested", reflect.TypeOf(schemaTestExplicitNested{})),
		WithPropertyMethod("explicitMSimple", "ReadLegacyBeanVal", stringType),
		WithPropertyMethod("explicitMArray", "ReadStringArray", reflect.TypeOf([]string{})),
		WithPropertyMethod("explicitMIndexed", "ReadStringIndexed", reflect.TypeOf([]string{})),
		WithPropertyMethod("explicitMMapped", "ReadMapByKey", reflect.TypeOf(map[string]string{})),
		WithPropertyMethod("explicitMNested", "ReadNested", reflect.TypeOf(schemaTestExplicitNested{})),
		WithNestedPropertySchema("explicitFNested", nestedSchema),
		WithNestedPropertySchema("explicitMNested", nestedSchema),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := schema.Field("fieldLegacyVal"); ok {
		t.Fatal("AccessorExplicit must not publish an unregistered field")
	}
	if _, ok := schema.Property("explicitMNested.readNestedClassValue"); !ok {
		t.Fatal("nested explicit method property is missing from metadata")
	}
	if descriptor, ok := schema.Property(`explicitMIndexed[1]`); !ok || descriptor.Type != stringType || descriptor.Kind != PropertyIndexed {
		t.Fatalf("indexed method descriptor = %#v, %v", descriptor, ok)
	}

	event, err := newEvent(schema, schemaTestExplicitRoot{
		FieldLegacyVal:   "field",
		FieldStringArray: []string{"zero", "one"},
		FieldNested:      schemaTestExplicitNested{FieldNestedValue: "nested"},
		mapValues:        map[string]string{"key2": "mapped"},
	}, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name string
		want any
	}{
		{name: "explicitFSimple", want: "field"},
		{name: "explicitFIndexed[0]", want: "zero"},
		{name: "explicitMArray[1]", want: "one"},
		{name: "explicitMIndexed[1]", want: "one"},
		{name: `explicitMMapped('key2')`, want: "mapped"},
		{name: "explicitFNested.fieldNestedClassValue", want: "nested"},
		{name: "explicitMNested.readNestedClassValue", want: "nested"},
	}
	for _, check := range checks {
		if got := event.Get(check.name); !got.IsPresent() || got.Any() != check.want {
			t.Errorf("Get(%q) = %v (%v), want %v", check.name, got.Any(), got.State(), check.want)
		}
	}
	fragment, ok := event.GetFragment("explicitMNested")
	if !ok || fragment.Get("readNestedClassValue").Any() != "nested" {
		t.Fatalf("explicit nested fragment = %#v, ok=%v", fragment, ok)
	}
	renderedJSON, err := RenderJSON(event)
	if err != nil || !strings.Contains(renderedJSON, `"explicitFNested":{"fieldNestedClassValue":"nested","readNestedClassValue":"nested"}`) {
		t.Fatalf("explicit nested JSON render = %s, %v", renderedJSON, err)
	}
	renderedXML, err := RenderXML(event)
	if err != nil || !strings.Contains(renderedXML, "<explicitFNested>") || !strings.Contains(renderedXML, "<fieldNestedClassValue>nested</fieldNestedClassValue>") {
		t.Fatalf("explicit nested XML render = %s, %v", renderedXML, err)
	}
	if !event.Get("fieldLegacyVal").IsMissing() || !event.Get("unknown").IsMissing() {
		t.Fatal("unregistered explicit properties must be Missing")
	}
}

func TestExplicitAccessorRejectsInvalidRegistration(t *testing.T) {
	if _, err := StructSchema[schemaTestExplicitRoot]("BadPath", WithAccessorStyle(AccessorExplicit), WithPropertyPath("value", "")); err == nil {
		t.Fatal("empty explicit property path should fail schema construction")
	}
	if _, err := StructSchema[schemaTestExplicitRoot]("BadMethod", WithAccessorStyle(AccessorExplicit), WithPropertyMethod("value", "MissingMethod")); err == nil {
		t.Fatal("unknown explicit property method should fail schema construction")
	}
}

func TestPublicAccessorKeepsFieldsAndAddsExplicitMethods(t *testing.T) {
	schema, err := StructSchema[schemaTestExplicitRoot]("PublicRoot",
		WithAccessorStyle(AccessorPublic),
		WithPropertyMethod("explicitMSimple", "ReadLegacyBeanVal", reflect.TypeOf("")),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := schema.Field("fieldLegacyVal"); !ok {
		t.Fatal("AccessorPublic must retain exported fields")
	}
	if _, ok := schema.Property("explicitMSimple"); !ok {
		t.Fatal("AccessorPublic explicit method is missing from metadata")
	}
	event, err := newEvent(schema, schemaTestExplicitRoot{FieldLegacyVal: "public"}, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if event.Get("fieldLegacyVal").Any() != "public" || event.Get("explicitMSimple").Any() != "public" {
		t.Fatalf("public accessor values = field=%v method=%v", event.Get("fieldLegacyVal"), event.Get("explicitMSimple"))
	}
}

func TestAccessorRegistrationEntersPlanCanonicalIdentity(t *testing.T) {
	buildPlan := func(method string) (Plan, error) {
		env := NewEnvironment()
		if _, err := RegisterStruct[schemaTestExplicitRoot](env, "AccessorPlanRoot", WithPropertyMethod("value", method, reflect.TypeOf(""))); err != nil {
			return Plan{}, err
		}
		query := Select(From[schemaTestExplicitRoot](env, "AccessorPlanRoot"),
			Alias("value", Field[schemaTestExplicitRoot, string]("value")),
		).Query(StatementName("accessor-plan"))
		return env.Build(query)
	}
	first, err := buildPlan("ReadLegacyBeanVal")
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildPlan("ReadAlternativeVal")
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash() == second.Hash() || string(first.Canonical()) == string(second.Canonical()) {
		t.Fatal("different explicit property methods must produce different plan identities")
	}
	if !strings.Contains(string(first.Canonical()), "method:ReadLegacyBeanVal") {
		t.Fatalf("plan canonical omitted accessor method: %s", first.Canonical())
	}
}

func TestPropertyExpressionUsesIndexedAndMappedPath(t *testing.T) {
	value := map[string]any{
		"items":  []any{map[string]any{"id": "first"}},
		"labels": map[string]string{"primary": "A"},
	}
	items := Literal(value)
	if got := Property[string](items, "items[0].id").eval(EvalContext{}); !got.Equal(Present("first")) {
		t.Fatalf("nested indexed Property = %v", got)
	}
	if got := Property[string](items, `labels('primary')`).eval(EvalContext{}); !got.Equal(Present("A")) {
		t.Fatalf("mapped Property = %v", got)
	}
}

type schemaJSONEnum string

type schemaJSONNested struct {
	ID     string  `json:"id"`
	Values []int64 `json:"values"`
}

func TestJSONSchemaDirectedNestedTypedConversion(t *testing.T) {
	schema, err := NewJSONSchema("TypedJSON", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("count", reflect.TypeOf(int64(0))),
		FieldDef("values", reflect.TypeOf([]int64{})),
		FieldDef("matrix", reflect.TypeOf([][]string{})),
		FieldDef("labels", reflect.TypeOf(map[string]int64{})),
		FieldDef("nested", reflect.TypeOf(schemaJSONNested{})),
		FieldDef("enum", reflect.TypeOf(schemaJSONEnum(""))),
		FieldDef("when", reflect.TypeOf(time.Time{})),
		FieldDef("bigint", reflect.TypeOf(big.Int{})),
		FieldDef("decimal", reflect.TypeOf(big.Rat{})),
		FieldDef("optional", reflect.TypeOf((*int64)(nil))),
	}, WithPropertyResolution(PropertyCaseInsensitive))
	if err != nil {
		t.Fatal(err)
	}
	event, err := ParseJSON(schema, []byte(`{
		"SYMBOL":"ESPER",
		"count":3,
		"values":[1,2,3],
		"matrix":[["a"],["b","c"]],
		"labels":{"x":7},
		"nested":{"id":"N1","values":[8,9]},
		"enum":"READY",
		"when":"2026-08-05T08:00:00Z",
		"bigint":123456789123456789123456789,
		"decimal":123456789123456789123456789.1,
		"optional":null
	}`), time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := As[string](event.Get("symbol")); err != nil || got != "ESPER" {
		t.Fatalf("symbol = %q, %v", got, err)
	}
	if got, err := As[int64](event.Get("count")); err != nil || got != 3 {
		t.Fatalf("count = %v, %v", got, err)
	}
	if got, err := As[[]int64](event.Get("values")); err != nil || !reflect.DeepEqual(got, []int64{1, 2, 3}) {
		t.Fatalf("values = %#v, %v", got, err)
	}
	if got, err := As[[][]string](event.Get("matrix")); err != nil || !reflect.DeepEqual(got, [][]string{{"a"}, {"b", "c"}}) {
		t.Fatalf("matrix = %#v, %v", got, err)
	}
	if got, err := As[map[string]int64](event.Get("labels")); err != nil || got["x"] != 7 {
		t.Fatalf("labels = %#v, %v", got, err)
	}
	if got, err := As[schemaJSONNested](event.Get("nested")); err != nil || got.ID != "N1" || !reflect.DeepEqual(got.Values, []int64{8, 9}) {
		t.Fatalf("nested = %#v, %v", got, err)
	}
	if got, err := As[schemaJSONEnum](event.Get("enum")); err != nil || got != schemaJSONEnum("READY") {
		t.Fatalf("enum = %q, %v", got, err)
	}
	if got, err := As[time.Time](event.Get("when")); err != nil || !got.Equal(time.Date(2026, 8, 5, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("when = %v, %v", got, err)
	}
	integer, err := As[big.Int](event.Get("bigint"))
	if err != nil || integer.String() != "123456789123456789123456789" {
		t.Fatalf("bigint = %s, %v", integer.String(), err)
	}
	rational, err := As[big.Rat](event.Get("decimal"))
	expectedRational, _ := new(big.Rat).SetString("123456789123456789123456789.1")
	if err != nil || rational.Cmp(expectedRational) != 0 {
		t.Fatalf("decimal = %s, %v", rational.String(), err)
	}
	if !event.Get("optional").IsNull() {
		t.Fatal("null pointer JSON field must remain Null")
	}
	encoded, err := RenderJSON(event)
	if err != nil || !strings.Contains(encoded, `"bigint":123456789123456789123456789`) || !strings.Contains(encoded, `"decimal":123456789123456789123456789.1`) {
		t.Fatalf("typed JSON render = %s, %v", encoded, err)
	}
}

func TestJSONParseOptionsRejectUnknownTrailingAndDeepInput(t *testing.T) {
	schema, err := NewJSONSchema("StrictJSON", []FieldSpec{FieldDef("value", reflect.TypeOf(int64(0)))})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseJSONWithOptions(schema, []byte(`{"value":1,"other":2}`), time.Unix(0, 0), WithJSONRejectUnknownFields()); err == nil {
		t.Fatal("unknown JSON field should be rejected when requested")
	}
	if _, err := ParseJSON(schema, []byte(`{"value":1}{"value":2}`), time.Unix(0, 0)); err == nil {
		t.Fatal("trailing JSON object should be rejected")
	}
	if _, err := ParseJSONWithOptions(schema, []byte(`{"value":{"nested":1}}`), time.Unix(0, 0), WithJSONParseMaxDepth(1)); err == nil {
		t.Fatal("deep JSON should be rejected by max depth")
	}
}

func TestJSONParserLaxScalarArrayAndShapeMatrix(t *testing.T) {
	schema, err := NewJSONSchema("JSONLaxMatrix", []FieldSpec{
		FieldDef("text", reflect.TypeOf("")),
		FieldDef("boolean", reflect.TypeOf(false)),
		FieldDef("booleanArray", reflect.TypeOf([]bool{})),
		FieldDef("integer", reflect.TypeOf(int64(0))),
		FieldDef("integerArray", reflect.TypeOf([]int64{})),
		FieldDef("object", reflect.TypeOf(map[string]int64{})),
	})
	if err != nil {
		t.Fatal(err)
	}
	event, err := ParseJSON(schema, []byte(`{
		"text":true,
		"boolean":"true",
		"booleanArray":["false",true],
		"integer":"12",
		"integerArray":["1",2],
		"object":{"x":"7"}
	}`), time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := As[string](event.Get("text")); err != nil || got != "true" {
		t.Fatalf("lax string conversion = %q, %v", got, err)
	}
	if got, err := As[bool](event.Get("boolean")); err != nil || !got {
		t.Fatalf("lax bool conversion = %v, %v", got, err)
	}
	if got, err := As[[]bool](event.Get("booleanArray")); err != nil || !reflect.DeepEqual(got, []bool{false, true}) {
		t.Fatalf("lax bool array conversion = %#v, %v", got, err)
	}
	if got, err := As[int64](event.Get("integer")); err != nil || got != 12 {
		t.Fatalf("lax integer conversion = %v, %v", got, err)
	}
	if got, err := As[[]int64](event.Get("integerArray")); err != nil || !reflect.DeepEqual(got, []int64{1, 2}) {
		t.Fatalf("lax integer array conversion = %#v, %v", got, err)
	}
	if got, err := As[map[string]int64](event.Get("object")); err != nil || got["x"] != 7 {
		t.Fatalf("lax object conversion = %#v, %v", got, err)
	}

	shapeEvent, err := ParseJSON(schema, []byte(`{"text":["not-a-string"],"boolean":{},"integer":[],"integerArray":{},"object":"not-an-object"}`), time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"text", "boolean", "integer", "integerArray", "object"} {
		if !shapeEvent.Get(name).IsNull() {
			t.Fatalf("incompatible JSON shape %q = %#v, want null", name, shapeEvent.Get(name).Any())
		}
	}

	invalid := []string{
		`{"integer":"x"}`,
		`{"boolean":"x"}`,
		`{"integerArray":["x"]}`,
		`{"object":{"x":"x"}}`,
	}
	for _, payload := range invalid {
		if _, err := ParseJSON(schema, []byte(payload), time.Unix(0, 0)); err == nil {
			t.Fatalf("invalid JSON conversion %s unexpectedly succeeded", payload)
		}
	}
}

func TestSchemaAndEventPropertyMetadata(t *testing.T) {
	schema, err := StructSchema[schemaTestIndexedMapped]("Metadata")
	if err != nil {
		t.Fatal(err)
	}
	if got := schema.PropertyNames(); !reflect.DeepEqual(got, []string{"array", "mapped", "nested"}) {
		t.Fatalf("property names = %#v", got)
	}
	array, ok := schema.Property("array[0]")
	if !ok || array.Kind != PropertyIndexed || array.Type != reflect.TypeOf("") {
		t.Fatalf("indexed descriptor = %#v", array)
	}
	mapped, ok := schema.Property(`mapped('a')`)
	if !ok || mapped.Kind != PropertyMapped || mapped.Type != reflect.TypeOf("") {
		t.Fatalf("mapped descriptor = %#v", mapped)
	}
	nested, ok := schema.Property("nested[0].name")
	if !ok || nested.Kind != PropertyIndexed || nested.Type != reflect.TypeOf("") {
		t.Fatalf("nested descriptor = %#v", nested)
	}
	if typ, ok := schema.PropertyType("nested[0].name"); !ok || typ != reflect.TypeOf("") {
		t.Fatalf("nested property type = %v, %v", typ, ok)
	}
	event, err := newEvent(schema, schemaTestIndexedMapped{}, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !event.HasProperty("array[0]") || len(event.PropertyNames()) != 3 {
		t.Fatalf("event metadata = %#v", event.PropertyNames())
	}
	unknownSchema, err := NewMapSchema("DynamicMetadata", nil, AllowDynamicFields())
	if err != nil {
		t.Fatal(err)
	}
	unknown, ok := unknownSchema.Property("runtime")
	if !ok || unknown.Kind != PropertyDynamic || unknown.Type != typeOf[any]() {
		t.Fatalf("dynamic descriptor = %#v", unknown)
	}
}

func TestSchemaInheritanceMergesFieldsAndObjectArrayOrder(t *testing.T) {
	parent, err := NewMapSchema("Parent", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
		FieldDef("created", reflect.TypeOf(int64(0))),
	}, AllowDynamicFields())
	if err != nil {
		t.Fatal(err)
	}
	child, err := NewJSONSchema("Child", []FieldSpec{FieldDef("price", reflect.TypeOf(float64(0)))}, WithSchemaParent(parent))
	if err != nil {
		t.Fatal(err)
	}
	if got := child.ParentNames(); !reflect.DeepEqual(got, []string{"Parent"}) {
		t.Fatalf("parent names = %#v", got)
	}
	if got := child.PropertyNames(); !reflect.DeepEqual(got, []string{"id", "created", "price"}) {
		t.Fatalf("inherited property order = %#v", got)
	}
	event, err := newEvent(child, map[string]any{"id": "A", "created": int64(7), "price": 12.5, "extra": true}, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if event.Get("id").Any() != "A" || event.Get("created").Any() != int64(7) || event.Get("price").Any() != 12.5 || event.Get("extra").Any() != true {
		t.Fatalf("inherited event values = %#v", event.Underlying())
	}
	if !child.AllowsDynamicProperties() {
		t.Fatal("dynamic property policy should be inherited")
	}

	objectParent, err := NewObjectArraySchema("ObjectParent", []FieldSpec{FieldDef("id", reflect.TypeOf(""))})
	if err != nil {
		t.Fatal(err)
	}
	objectChild, err := NewObjectArraySchema("ObjectChild", []FieldSpec{FieldDef("price", reflect.TypeOf(float64(0)))}, WithSchemaParent(objectParent))
	if err != nil {
		t.Fatal(err)
	}
	objectEvent, err := ParseObjectArray(objectChild, []any{"A", 12.5}, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if objectEvent.Get("id").Any() != "A" || objectEvent.Get("price").Any() != 12.5 {
		t.Fatalf("inherited object-array values = %#v", objectEvent.Underlying())
	}
	if _, err := NewMapSchema("Broken", nil, WithSchemaParent(Schema{})); err == nil {
		t.Fatal("invalid parent schema should be rejected")
	}
}
