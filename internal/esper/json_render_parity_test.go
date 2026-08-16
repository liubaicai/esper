package esper

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The Java-side contract for JSON field adapters is the nullable object:
// JsonWriteForgeProvidedStringAdapter generates `adapter.write(field, writer)`
// for the stored (possibly null) property value
// (common/.../event/json/write/JsonWriteForgeProvidedStringAdapter.java), and
// the reference adapter handles null explicitly
// (regression-lib/.../support/json/SupportJsonFieldAdapterStringDate.java).
// Go's nullable form is *T, so adapter round trips must work for
// pointer-declared fields as well as for the bare value type.

func TestJSONFieldAdapterNullablePointerRoundTrip(t *testing.T) {
	dateAdapter := NewJSONFieldAdapter(func(text string) (time.Time, error) {
		return time.Parse("02-01-2006", text)
	}, func(value time.Time) (string, error) {
		return value.Format("02-01-2006"), nil
	})
	intAdapter := NewJSONFieldAdapter(func(text string) (int64, error) {
		return strconv.ParseInt(text, 10, 64)
	}, func(value int64) (string, error) {
		return strconv.FormatInt(value, 10), nil
	})
	ratioAdapter := NewJSONFieldAdapter(func(text string) (float32, error) {
		parsed, err := strconv.ParseFloat(text, 32)
		return float32(parsed), err
	}, func(value float32) (string, error) {
		return strconv.FormatFloat(float64(value), 'g', -1, 32), nil
	})
	flagAdapter := NewJSONFieldAdapter(func(text string) (bool, error) {
		return strconv.ParseBool(text)
	}, func(value bool) (string, error) {
		return strconv.FormatBool(value), nil
	})

	schema, err := NewJSONSchema("NullableAdapterJSON", []FieldSpec{
		FieldDef("when", reflect.TypeOf((*time.Time)(nil))),
		FieldDef("count", reflect.TypeOf((*int64)(nil))),
		FieldDef("ratio", reflect.TypeOf((*float32)(nil))),
		FieldDef("flag", reflect.TypeOf((*bool)(nil))),
	}, WithJSONFieldAdapter("when", dateAdapter), WithJSONFieldAdapter("count", intAdapter),
		WithJSONFieldAdapter("ratio", ratioAdapter), WithJSONFieldAdapter("flag", flagAdapter))
	if err != nil {
		t.Fatal(err)
	}

	event, err := ParseJSON(schema, []byte(`{"when":"22-09-2018","count":"7","ratio":"1.5","flag":"true"}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("nullable adapter parse: %v", err)
	}
	whenPtr, err := As[*time.Time](event.Get("when"))
	if err != nil || whenPtr == nil || whenPtr.Format("02-01-2006") != "22-09-2018" {
		t.Fatalf("adapted pointer date = %v (%v)", whenPtr, err)
	}
	countPtr, err := As[*int64](event.Get("count"))
	if err != nil || countPtr == nil || *countPtr != 7 {
		t.Fatalf("adapted pointer count = %v (%v)", countPtr, err)
	}
	ratioPtr, err := As[*float32](event.Get("ratio"))
	if err != nil || ratioPtr == nil || *ratioPtr != 1.5 {
		t.Fatalf("adapted pointer ratio = %v (%v)", ratioPtr, err)
	}
	flagPtr, err := As[*bool](event.Get("flag"))
	if err != nil || flagPtr == nil || !*flagPtr {
		t.Fatalf("adapted pointer flag = %v (%v)", flagPtr, err)
	}
	// The adapter round trip must render through Write with the stored
	// pointer value unboxed to the adapter's value type.
	if rendered, err := RenderJSON(event); err != nil || rendered != `{"when":"22-09-2018","count":"7","ratio":"1.5","flag":"true"}` {
		t.Fatalf("nullable adapter render = %s (%v)", rendered, err)
	}

	// Explicit null stays the Null state and renders as JSON null without
	// invoking the adapter's write function.
	nullEvent, err := ParseJSON(schema, []byte(`{"when":null,"count":null,"ratio":null,"flag":null}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("nullable adapter null parse: %v", err)
	}
	for _, name := range []string{"when", "count", "ratio", "flag"} {
		if !nullEvent.Get(name).IsNull() {
			t.Fatalf("null adapter field %q = %v", name, nullEvent.Get(name))
		}
	}
	if rendered, err := RenderJSON(nullEvent); err != nil || rendered != `{"when":null,"count":null,"ratio":null,"flag":null}` {
		t.Fatalf("nullable adapter null render = %s (%v)", rendered, err)
	}
}

func TestJSONFieldAdapterNullablePointerTypedStructRoundTrip(t *testing.T) {
	// Mirrors the Java adapter shape (EventJsonAdapter.java
	// EventJsonAdapterInsertInto / EventJsonAdapterCreateSchemaWStringTransform):
	// fields declared with nullable object types, adapters translating the JSON
	// string form. Here the schema is a typed struct whose fields are pointers,
	// exercising the materialize + render path without any shared-core change.
	pointAdapter := NewJSONFieldAdapter(func(text string) (jsonAdapterPoint, error) {
		parts := strings.Split(text, ",")
		if len(parts) != 2 {
			return jsonAdapterPoint{}, fmt.Errorf("invalid point %q", text)
		}
		x, errX := strconv.Atoi(strings.TrimSpace(parts[0]))
		y, errY := strconv.Atoi(strings.TrimSpace(parts[1]))
		if errX != nil || errY != nil {
			return jsonAdapterPoint{}, fmt.Errorf("invalid point %q", text)
		}
		return jsonAdapterPoint{X: x, Y: y}, nil
	}, func(point jsonAdapterPoint) (string, error) {
		return strconv.Itoa(point.X) + "," + strconv.Itoa(point.Y), nil
	})
	dateAdapter := NewJSONFieldAdapter(func(text string) (time.Time, error) {
		return time.Parse("2006-01-02T15:04:05.999", text)
	}, func(value time.Time) (string, error) {
		return value.Format("2006-01-02T15:04:05.999"), nil
	})

	schema, err := NewJSONSchemaFor[jsonAdapterNullableTyped]("AdapterTypedPtr", nil,
		WithJSONFieldAdapter("point", pointAdapter), WithJSONFieldAdapter("myDate", dateAdapter))
	if err != nil {
		t.Fatal(err)
	}
	event, err := ParseJSON(schema, []byte(`{"point":"7,14","myDate":"2002-05-01T08:00:01.999"}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("typed nullable adapter parse: %v", err)
	}
	underlying, ok := event.Underlying().(jsonAdapterNullableTyped)
	if !ok {
		t.Fatalf("typed underlying = %T", event.Underlying())
	}
	if underlying.Point == nil || underlying.Point.X != 7 || underlying.Point.Y != 14 {
		t.Fatalf("adapted typed point = %#v", underlying.Point)
	}
	if underlying.MyDate == nil || underlying.MyDate.Format("2006-01-02T15:04:05.999") != "2002-05-01T08:00:01.999" {
		t.Fatalf("adapted typed date = %v", underlying.MyDate)
	}
	if rendered, err := RenderJSON(event); err != nil || rendered != `{"point":"7,14","myDate":"2002-05-01T08:00:01.999"}` {
		t.Fatalf("typed nullable adapter render = %s (%v)", rendered, err)
	}

	// A null field materializes as a nil pointer and renders as JSON null.
	nullEvent, err := ParseJSON(schema, []byte(`{"point":null,"myDate":null}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("typed nullable adapter null parse: %v", err)
	}
	if rendered, err := RenderJSON(nullEvent); err != nil || rendered != `{"point":null,"myDate":null}` {
		t.Fatalf("typed nullable adapter null render = %s (%v)", rendered, err)
	}
}

type jsonAdapterNullableTyped struct {
	Point  *jsonAdapterPoint `json:"point"`
	MyDate *time.Time        `json:"myDate"`
}

func TestJSONFieldAdapterFuncsNullablePointerRoundTrip(t *testing.T) {
	// JSONFieldAdapterFuncs with a non-pointer declared type must receive the
	// unboxed value through Write exactly like the typed adapter, while an
	// adapter declared for a pointer type keeps receiving the pointer.
	dateType := reflect.TypeOf(time.Time{})
	dateAdapter := JSONFieldAdapterFuncs{
		Name:      "date-dmy",
		Type:      dateType,
		ParseFunc: func(text string) (any, error) { return time.Parse("02-01-2006", text) },
		WriteFunc: func(value any) (string, error) {
			// JSONFieldAdapterFuncs passes the stored value as-is on direct
			// calls; the renderer unboxes to the declared non-pointer type.
			switch typed := value.(type) {
			case time.Time:
				return typed.Format("02-01-2006"), nil
			case *time.Time:
				return typed.Format("02-01-2006"), nil
			}
			return "", fmt.Errorf("unexpected date value %T", value)
		},
	}
	pointPtrAdapter := JSONFieldAdapterFuncs{
		Name: "point-ptr",
		Type: reflect.TypeOf((*jsonAdapterPoint)(nil)),
		ParseFunc: func(text string) (any, error) {
			parts := strings.Split(text, ",")
			x, _ := strconv.Atoi(parts[0])
			y, _ := strconv.Atoi(parts[1])
			return &jsonAdapterPoint{X: x, Y: y}, nil
		},
		WriteFunc: func(value any) (string, error) {
			point := value.(*jsonAdapterPoint)
			return strconv.Itoa(point.X) + "," + strconv.Itoa(point.Y), nil
		},
	}
	schema, err := NewJSONSchema("FuncsNullableAdapterJSON", []FieldSpec{
		FieldDef("when", reflect.TypeOf((*time.Time)(nil))),
		FieldDef("point", reflect.TypeOf((*jsonAdapterPoint)(nil))),
	}, WithJSONFieldAdapter("when", dateAdapter), WithJSONFieldAdapter("point", pointPtrAdapter))
	if err != nil {
		t.Fatal(err)
	}
	event, err := ParseJSON(schema, []byte(`{"when":"22-09-2018","point":"7,14"}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("funcs nullable adapter parse: %v", err)
	}
	if rendered, err := RenderJSON(event); err != nil || rendered != `{"when":"22-09-2018","point":"7,14"}` {
		t.Fatalf("funcs nullable adapter render = %s (%v)", rendered, err)
	}
	// The non-pointer-typed Funcs adapter gets the unboxed value.
	if _, err := dateAdapter.Write(event.Get("when").Any()); err != nil {
		t.Fatalf("funcs date write with stored pointer: %v", err)
	}
	// The pointer-typed Funcs adapter keeps the pointer.
	if _, err := pointPtrAdapter.Write(event.Get("point").Any()); err != nil {
		t.Fatalf("funcs point write with stored pointer: %v", err)
	}
}

func TestJSONFieldAdapterTypedWriteAcceptsStoredPointer(t *testing.T) {
	// The typed adapter's Write must accept the pointer form that ParseJSON
	// stores for nullable fields when called directly with the underlying.
	dateAdapter := NewJSONFieldAdapter(func(text string) (time.Time, error) {
		return time.Parse("02-01-2006", text)
	}, func(value time.Time) (string, error) {
		return value.Format("02-01-2006"), nil
	})
	schema, err := NewJSONSchema("DirectWritePtr", []FieldSpec{
		FieldDef("when", reflect.TypeOf((*time.Time)(nil))),
	}, WithJSONFieldAdapter("when", dateAdapter))
	if err != nil {
		t.Fatal(err)
	}
	event, err := ParseJSON(schema, []byte(`{"when":"22-09-2018"}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := dateAdapter.Write(event.Get("when").Any())
	if err != nil {
		t.Fatalf("typed write with stored pointer: %v", err)
	}
	if encoded != "22-09-2018" {
		t.Fatalf("typed write result = %q", encoded)
	}
	// Bare value still works, as does the unboxed path.
	bare, err := dateAdapter.Write(time.Date(2018, time.September, 22, 0, 0, 0, 0, time.UTC))
	if err != nil || bare != "22-09-2018" {
		t.Fatalf("typed write bare = %q (%v)", bare, err)
	}
}

// Character fields are the one scalar whose JSON form is not numeric:
// JsonForgeFactoryBuiltinClassTyped binds Character to
// JsonWriteForgeStringWithToString, i.e. JsonWriteUtil.
// writeNullableStringToString writes the Character's string value
// (common/.../event/json/write/JsonWriteForgeStringWithToString.java).
// Parsed events preserve the original text through the raw tree; programmatic
// events must still render the rune as its JSON string rather than a number.

type jsonRenderCharRoot struct {
	Local jsonRenderCharFields `json:"local"`
}

type jsonRenderCharFields struct {
	C0 rune     `json:"c0"`
	C1 *rune    `json:"c1"`
	C2 []rune   `json:"c2"`
	C3 []*rune  `json:"c3"`
	C4 [][]rune `json:"c4"`
}

func TestJSONRenderCharacterSpecialValues(t *testing.T) {
	ch := rune('x')
	schema, err := NewJSONSchemaFor[jsonRenderCharRoot]("RenderChar", nil)
	if err != nil {
		t.Fatal(err)
	}
	event, err := newEvent(schema, jsonRenderCharRoot{Local: jsonRenderCharFields{
		C0: rune('a'),
		C1: &ch,
		C2: []rune{'a', 'b'},
		C3: []*rune{&ch},
		C4: [][]rune{{'y'}},
	}}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := RenderJSON(event)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"local":{"c0":"a","c1":"x","c2":["a","b"],"c3":["x"],"c4":[["y"]]}}`
	if rendered != want {
		t.Fatalf("programmatic char render = %s, want %s", rendered, want)
	}
	// Property access still exposes the numeric code point.
	if event.Get("local.c0").Any() != rune('a') {
		t.Fatalf("char property value = %v", event.Get("local.c0"))
	}

	// Parsed events keep their original text (charAt(0) for multi-run strings)
	// and a numeric raw value stays numeric (the lax parse accepts it, the
	// renderer must not reinterpret it as a character string).
	parsed, err := ParseJSON(schema, []byte(`{"local":{"c0":"xy","c1":null,"c2":["z"],"c3":[null],"c4":[[]]}}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	parsedRender, err := RenderJSON(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if parsedRender != `{"local":{"c0":"x","c1":null,"c2":["z"],"c3":[null],"c4":[[]]}}` {
		t.Fatalf("parsed char render = %s", parsedRender)
	}
	numeric, err := ParseJSON(schema, []byte(`{"local":{"c0":120}}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	numericRender, err := RenderJSON(numeric)
	if err != nil {
		t.Fatal(err)
	}
	if numericRender != `{"local":{"c0":120,"c1":null,"c2":null,"c3":null,"c4":null}}` {
		t.Fatalf("numeric char render = %s", numericRender)
	}
}
