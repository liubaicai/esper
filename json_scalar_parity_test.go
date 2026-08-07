package esper

import (
	"encoding/json"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"
)

// jsonVMScalarEnvelope mirrors the Java VM-class regression shape: one
// scalar, a one-dimensional array, a two-dimensional array and a collection
// all use the same declared scalar type.
type jsonVMScalarEnvelope[T any] struct {
	C0        *T     `json:"c0"`
	C0Arr     []*T   `json:"c0Arr"`
	C0Arr2Dim [][]*T `json:"c0Arr2Dim"`
	C0Coll    []*T   `json:"c0Coll"`
}

func TestJSONTypedVMScalarsMatchEsper(t *testing.T) {
	uuid, err := ParseUUID("b7dc7f66-4f6d-4f03-14d7-83da210dfba6")
	if err != nil {
		t.Fatal(err)
	}
	runJSONVMScalarCase(t, "UUID", uuid, `"b7dc7f66-4f6d-4f03-14d7-83da210dfba6"`, `1`)

	location, err := parseJSONTime("2024-01-02T03:04:05.123456789+08:00")
	if err != nil {
		t.Fatal(err)
	}
	runJSONVMScalarCase(t, "OffsetTime", location, `"2024-01-02T03:04:05.123456789+08:00"`, `1`)
	runJSONVMScalarCase(t, "DateOnly", DateOnly("2024-01-02"), `"2024-01-02"`, `1`)
	runJSONVMScalarCase(t, "URL", URL("http://host:1000/file"), `"http://host:1000/file"`, `1`)
	runJSONVMScalarCase(t, "URI", URI("ftp://ftp.is.co.za/rfc/rfc1808.txt"), `"ftp://ftp.is.co.za/rfc/rfc1808.txt"`, `"a b"`)
}

func runJSONVMScalarCase[T any](t *testing.T, name string, expected T, validJSON, invalidJSON string) {
	t.Helper()
	schema, err := NewJSONSchemaFor[jsonVMScalarEnvelope[T]]("VM"+name, nil)
	if err != nil {
		t.Fatal(err)
	}
	valid := `{"c0":` + validJSON + `,"c0Arr":[` + validJSON + `],"c0Arr2Dim":[[` + validJSON + `]],"c0Coll":[` + validJSON + `]}`
	event, err := ParseJSON(schema, []byte(valid), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	underlying, ok := event.Underlying().(jsonVMScalarEnvelope[T])
	if !ok {
		t.Fatalf("typed %s underlying = %T", name, event.Underlying())
	}
	if underlying.C0 == nil || !reflect.DeepEqual(*underlying.C0, expected) || len(underlying.C0Arr) != 1 || !reflect.DeepEqual(*underlying.C0Arr[0], expected) || len(underlying.C0Arr2Dim) != 1 || len(underlying.C0Arr2Dim[0]) != 1 || !reflect.DeepEqual(*underlying.C0Arr2Dim[0][0], expected) || len(underlying.C0Coll) != 1 || !reflect.DeepEqual(*underlying.C0Coll[0], expected) {
		t.Fatalf("typed %s values = %#v", name, underlying)
	}
	rendered, err := RenderJSON(event)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEquivalent(t, valid, rendered)

	nullEvent, err := ParseJSON(schema, []byte(`{"c0":null,"c0Arr":[null],"c0Arr2Dim":[[null]],"c0Coll":[null]}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	nullUnderlying, ok := nullEvent.Underlying().(jsonVMScalarEnvelope[T])
	if !ok || nullUnderlying.C0 != nil || nullUnderlying.C0Arr[0] != nil || nullUnderlying.C0Arr2Dim[0][0] != nil || nullUnderlying.C0Coll[0] != nil {
		t.Fatalf("typed %s null values = %#v", name, nullEvent.Underlying())
	}

	if _, err := ParseJSON(schema, []byte(`{"c0":`+invalidJSON+`}`), time.Unix(0, 0).UTC()); err == nil {
		t.Fatalf("typed %s invalid scalar unexpectedly succeeded", name)
	}
	shapeEvent, err := ParseJSON(schema, []byte(`{"c0":{}}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	shapeUnderlying, ok := shapeEvent.Underlying().(jsonVMScalarEnvelope[T])
	if !ok || shapeUnderlying.C0 != nil {
		t.Fatalf("typed %s incompatible shapes = %#v", name, shapeEvent.Underlying())
	}
}

type jsonLaxNumericEvent struct {
	Byte       int8      `json:"cbyte"`
	Short      int16     `json:"cshort"`
	Int        int       `json:"cint"`
	Long       int64     `json:"clong"`
	Double     float64   `json:"cdouble"`
	Float      float32   `json:"cfloat"`
	BigInt     big.Int   `json:"cbigint"`
	BigDecimal big.Rat   `json:"cbigdec"`
	ByteA      []int8    `json:"cbytea1"`
	ShortA     []int16   `json:"cshorta1"`
	IntA       []int     `json:"cinta1"`
	LongA      []int64   `json:"clonga1"`
	DoubleA    []float64 `json:"cdoublea1"`
	FloatA     []float32 `json:"cfloata1"`
	BigIntA    []big.Int `json:"cbiginta1"`
	BigDecA    []big.Rat `json:"cbigdeca1"`
}

func TestJSONParserLaxNumericTypesAndInvalidValuesMatchEsper(t *testing.T) {
	schema, err := NewJSONSchemaFor[jsonLaxNumericEvent]("JSONLaxNumeric", nil)
	if err != nil {
		t.Fatal(err)
	}
	valid := []byte(`{"cbyte":"1","cshort":"1","cint":"1","clong":"1","cdouble":"1","cfloat":"1","cbigint":"1","cbigdec":"1","cbytea1":["1"],"cshorta1":["1"],"cinta1":["1"],"clonga1":["1"],"cdoublea1":["1"],"cfloata1":["1"],"cbiginta1":["1"],"cbigdeca1":["1"]}`)
	event, err := ParseJSON(schema, valid, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	underlying, ok := event.Underlying().(jsonLaxNumericEvent)
	if !ok {
		t.Fatalf("lax numeric underlying = %T", event.Underlying())
	}
	if underlying.Byte != 1 || underlying.Short != 1 || underlying.Int != 1 || underlying.Long != 1 || underlying.Double != 1 || underlying.Float != 1 || underlying.BigInt.Cmp(big.NewInt(1)) != 0 || underlying.BigDecimal.Cmp(big.NewRat(1, 1)) != 0 || !reflect.DeepEqual(underlying.ByteA, []int8{1}) || !reflect.DeepEqual(underlying.ShortA, []int16{1}) || !reflect.DeepEqual(underlying.IntA, []int{1}) || !reflect.DeepEqual(underlying.LongA, []int64{1}) || !reflect.DeepEqual(underlying.DoubleA, []float64{1}) || !reflect.DeepEqual(underlying.FloatA, []float32{1}) || len(underlying.BigIntA) != 1 || underlying.BigIntA[0].Cmp(big.NewInt(1)) != 0 || len(underlying.BigDecA) != 1 || underlying.BigDecA[0].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatalf("lax numeric values = %#v", underlying)
	}

	invalidFields := []string{"cbyte", "cshort", "cint", "clong", "cdouble", "cfloat", "cbigint", "cbigdec", "cbytea1", "cshorta1", "cinta1", "clonga1", "cdoublea1", "cfloata1", "cbiginta1", "cbigdeca1"}
	for _, field := range invalidFields {
		payload := `{"` + field + `":"x"}`
		if strings.HasSuffix(field, "a1") {
			payload = `{"` + field + `":["x"]}`
		}
		if _, err := ParseJSON(schema, []byte(payload), time.Unix(0, 0).UTC()); err == nil {
			t.Fatalf("invalid numeric field %s unexpectedly succeeded", field)
		}
	}
	shape, err := ParseJSON(schema, []byte(`{"cint":{},"cbigint":[],"cbytea1":{}}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	shapeValue := shape.Underlying().(jsonLaxNumericEvent)
	if shapeValue.Int != 0 || !reflect.DeepEqual(shapeValue.BigInt, big.Int{}) || shapeValue.ByteA != nil {
		t.Fatalf("incompatible numeric shapes = %#v", shapeValue)
	}
}

func TestJSONParserMalformedAndUndeclaredPoliciesMatchEsper(t *testing.T) {
	schema, err := NewJSONSchema("JSONMalformed", []FieldSpec{FieldDef("p1", reflect.TypeOf(""))})
	if err != nil {
		t.Fatal(err)
	}
	for _, payload := range []string{"", `{}{}`, `{{}`} {
		if _, err := ParseJSON(schema, []byte(payload), time.Unix(0, 0).UTC()); err == nil {
			t.Fatalf("malformed JSON %q unexpectedly succeeded", payload)
		}
	}
	staticEvent, err := ParseJSON(schema, []byte(`{"p1":1,"unknown":{"x":2}}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if staticEvent.Get("p1").Any() != "1" || !staticEvent.Get("unknown").IsMissing() {
		t.Fatalf("static undeclared policy = p1=%v unknown=%v", staticEvent.Get("p1"), staticEvent.Get("unknown"))
	}
	dynamicSchema, err := NewJSONSchema("JSONMalformedDynamic", []FieldSpec{FieldDef("p1", reflect.TypeOf(""))}, AllowDynamicFields())
	if err != nil {
		t.Fatal(err)
	}
	dynamicEvent, err := ParseJSON(dynamicSchema, []byte(`{"p1":1,"unknown":{"x":2}}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if dynamicEvent.Get("unknown.x").Any() != 2 {
		t.Fatalf("dynamic undeclared policy = %#v", dynamicEvent.Underlying())
	}
}

func TestJSONScalarRoundTripTypes(t *testing.T) {
	values := []any{URL("http://host:1000/file"), URI("ftp://ftp.is.co.za/rfc/rfc1808.txt"), DateOnly("2024-01-02")}
	for _, value := range values {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(encoded), `"`) {
			t.Fatalf("scalar %T encoded as %s", value, encoded)
		}
	}
}
