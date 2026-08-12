package esper

import (
	"encoding/json"
	"math/big"
	"reflect"
	"testing"
	"time"
)

type jsonClassSimpleParity struct {
	TheString    string `json:"theString"`
	IntPrimitive int    `json:"intPrimitive"`
}

type jsonClassSimpleRoot struct {
	Local jsonClassSimpleParity `json:"local"`
}

type jsonClassRecursiveParity struct {
	ID    string                    `json:"id"`
	Child *jsonClassRecursiveParity `json:"child"`
}

type jsonClassRecursiveRoot struct {
	Local jsonClassRecursiveParity `json:"local"`
}

type jsonClassNestedItemParity struct {
	TheString    string `json:"theString"`
	IntPrimitive int    `json:"intPrimitive"`
}

type jsonClassNestedArraysParity struct {
	C0        *jsonClassNestedItemParity     `json:"c0"`
	C0Arr     []*jsonClassNestedItemParity   `json:"c0Arr"`
	C0Arr2Dim [][]*jsonClassNestedItemParity `json:"c0Arr2Dim"`
	C0Coll    []*jsonClassNestedItemParity   `json:"c0Coll"`
}

type jsonClassNestedArraysRoot struct {
	Local jsonClassNestedArraysParity `json:"local"`
}

type jsonClassBuiltinListParity struct {
	C0  []*string  `json:"c0"`
	C1  []*rune    `json:"c1"`
	C2  []*bool    `json:"c2"`
	C3  []*int8    `json:"c3"`
	C4  []*int16   `json:"c4"`
	C5  []*int     `json:"c5"`
	C6  []*int64   `json:"c6"`
	C7  []*float64 `json:"c7"`
	C8  []*float32 `json:"c8"`
	C9  []*big.Int `json:"c9"`
	C10 []*big.Rat `json:"c10"`
}

type jsonClassBuiltinListRoot struct {
	Local jsonClassBuiltinListParity `json:"local"`
}

type jsonClassEnumParity string

type jsonClassEnumListParity struct {
	C0 []*jsonClassEnumParity `json:"c0"`
}

type jsonClassEnumListRoot struct {
	Local jsonClassEnumListParity `json:"local"`
}

type jsonVMUUIDParity struct {
	C0        *UUID     `json:"c0"`
	C0Arr     []*UUID   `json:"c0Arr"`
	C0Arr2Dim [][]*UUID `json:"c0Arr2Dim"`
	C0Coll    []*UUID   `json:"c0Coll"`
}

type jsonVMUUIDRoot struct {
	Local jsonVMUUIDParity `json:"local"`
}

type jsonVMTimeParity struct {
	C0        *time.Time     `json:"c0"`
	C0Arr     []*time.Time   `json:"c0Arr"`
	C0Arr2Dim [][]*time.Time `json:"c0Arr2Dim"`
	C0Coll    []*time.Time   `json:"c0Coll"`
}

type jsonVMTimeRoot struct {
	Local jsonVMTimeParity `json:"local"`
}

type jsonVMDateOnlyParity struct {
	C0        *DateOnly     `json:"c0"`
	C0Arr     []*DateOnly   `json:"c0Arr"`
	C0Arr2Dim [][]*DateOnly `json:"c0Arr2Dim"`
	C0Coll    []*DateOnly   `json:"c0Coll"`
}

type jsonVMDateOnlyRoot struct {
	Local jsonVMDateOnlyParity `json:"local"`
}

type jsonVMURLParity struct {
	C0        *URL     `json:"c0"`
	C0Arr     []*URL   `json:"c0Arr"`
	C0Arr2Dim [][]*URL `json:"c0Arr2Dim"`
	C0Coll    []*URL   `json:"c0Coll"`
}

type jsonVMURLRoot struct {
	Local jsonVMURLParity `json:"local"`
}

type jsonVMURIParity struct {
	C0        *URI     `json:"c0"`
	C0Arr     []*URI   `json:"c0Arr"`
	C0Arr2Dim [][]*URI `json:"c0Arr2Dim"`
	C0Coll    []*URI   `json:"c0Coll"`
}

type jsonVMURIRoot struct {
	Local jsonVMURIParity `json:"local"`
}

func jsonTestPointer[T any](value T) *T { return &value }

func parseJSONParityEvent[T any](t *testing.T, name, payload string) (Schema, Event) {
	t.Helper()
	schema, err := NewJSONSchemaFor[T](name, nil)
	if err != nil {
		t.Fatal(err)
	}
	event, err := ParseJSON(schema, []byte(payload), time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	return schema, event
}

func assertJSONParityRender(t *testing.T, event Event, want string) {
	t.Helper()
	got, err := RenderJSON(event)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("rendered JSON = %s, want %s", got, want)
	}
}

func TestJSONTypedClassSimpleAndRecursiveParity(t *testing.T) {
	_, simple := parseJSONParityEvent[jsonClassSimpleRoot](t, "JSONTypedClassSimple", `{"local":{"theString":"abc","intPrimitive":10}}`)
	if got, ok := simple.Get("local").Any().(jsonClassSimpleParity); !ok || got != (jsonClassSimpleParity{TheString: "abc", IntPrimitive: 10}) {
		t.Fatalf("simple typed local = %#v", simple.Get("local").Any())
	}
	assertJSONParityRender(t, simple, `{"local":{"theString":"abc","intPrimitive":10}}`)

	_, recursiveTwo := parseJSONParityEvent[jsonClassRecursiveRoot](t, "JSONTypedClassRecursive", `{"local":{"id":"a","child":{"id":"b","child":null}}}`)
	two := recursiveTwo.Get("local").Any().(jsonClassRecursiveParity)
	if two.ID != "a" || two.Child == nil || two.Child.ID != "b" || two.Child.Child != nil {
		t.Fatalf("recursive depth two = %#v", two)
	}
	assertJSONParityRender(t, recursiveTwo, `{"local":{"id":"a","child":{"id":"b","child":null}}}`)

	_, recursiveThree := parseJSONParityEvent[jsonClassRecursiveRoot](t, "JSONTypedClassRecursive", `{"local":{"id":"a","child":{"id":"b","child":{"id":"c","child":null}}}}`)
	three := recursiveThree.Get("local").Any().(jsonClassRecursiveParity)
	if three.Child == nil || three.Child.Child == nil || three.Child.Child.ID != "c" {
		t.Fatalf("recursive depth three = %#v", three)
	}
	assertJSONParityRender(t, recursiveThree, `{"local":{"id":"a","child":{"id":"b","child":{"id":"c","child":null}}}}`)
}

func jsonNestedItem(name string, value int) *jsonClassNestedItemParity {
	return &jsonClassNestedItemParity{TheString: name, IntPrimitive: value}
}

func TestJSONTypedClassNestedArraysNullAndCollectionParity(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    string
		local   jsonClassNestedArraysParity
	}{
		{
			name:    "filled",
			payload: `{"local":{"c0":{"theString":"E1","intPrimitive":1},"c0Arr":[{"theString":"E2","intPrimitive":2}],"c0Arr2Dim":[[{"theString":"E3","intPrimitive":3}]],"c0Coll":[{"theString":"E4","intPrimitive":4}]}}`,
			want:    `{"local":{"c0":{"theString":"E1","intPrimitive":1},"c0Arr":[{"theString":"E2","intPrimitive":2}],"c0Arr2Dim":[[{"theString":"E3","intPrimitive":3}]],"c0Coll":[{"theString":"E4","intPrimitive":4}]}}`,
			local: jsonClassNestedArraysParity{
				C0:        jsonNestedItem("E1", 1),
				C0Arr:     []*jsonClassNestedItemParity{jsonNestedItem("E2", 2)},
				C0Arr2Dim: [][]*jsonClassNestedItemParity{{jsonNestedItem("E3", 3)}},
				C0Coll:    []*jsonClassNestedItemParity{jsonNestedItem("E4", 4)},
			},
		},
		{
			name:    "nulled",
			payload: `{"local":{"c0":null,"c0Arr":[null],"c0Arr2Dim":[[null]],"c0Coll":[null]}}`,
			want:    `{"local":{"c0":null,"c0Arr":[null],"c0Arr2Dim":[[null]],"c0Coll":[null]}}`,
			local: jsonClassNestedArraysParity{
				C0Arr:     []*jsonClassNestedItemParity{nil},
				C0Arr2Dim: [][]*jsonClassNestedItemParity{{nil}},
				C0Coll:    []*jsonClassNestedItemParity{nil},
			},
		},
		{
			name:    "half-filled",
			payload: `{"local":{"c0":{"theString":"E1","intPrimitive":1},"c0Arr":[null,{"theString":"E2","intPrimitive":2}],"c0Arr2Dim":[null,[{"theString":"E3","intPrimitive":3},null],null],"c0Coll":[{"theString":"E4","intPrimitive":4},null]}}`,
			want:    `{"local":{"c0":{"theString":"E1","intPrimitive":1},"c0Arr":[null,{"theString":"E2","intPrimitive":2}],"c0Arr2Dim":[null,[{"theString":"E3","intPrimitive":3},null],null],"c0Coll":[{"theString":"E4","intPrimitive":4},null]}}`,
			local: jsonClassNestedArraysParity{
				C0:        jsonNestedItem("E1", 1),
				C0Arr:     []*jsonClassNestedItemParity{nil, jsonNestedItem("E2", 2)},
				C0Arr2Dim: [][]*jsonClassNestedItemParity{nil, {jsonNestedItem("E3", 3), nil}, nil},
				C0Coll:    []*jsonClassNestedItemParity{jsonNestedItem("E4", 4), nil},
			},
		},
		{
			name:    "empty",
			payload: `{"local":{"c0":null,"c0Arr":[],"c0Arr2Dim":[],"c0Coll":[]}}`,
			want:    `{"local":{"c0":null,"c0Arr":[],"c0Arr2Dim":[],"c0Coll":[]}}`,
			local: jsonClassNestedArraysParity{
				C0Arr:     []*jsonClassNestedItemParity{},
				C0Arr2Dim: [][]*jsonClassNestedItemParity{},
				C0Coll:    []*jsonClassNestedItemParity{},
			},
		},
		{
			name:    "multiple",
			payload: `{"local":{"c0":{"theString":"E1","intPrimitive":1},"c0Arr":[{"theString":"E2","intPrimitive":10},{"theString":"E2","intPrimitive":11},{"theString":"E2","intPrimitive":12}],"c0Arr2Dim":[[{"theString":"E3","intPrimitive":30},{"theString":"E3","intPrimitive":31}],[{"theString":"E3","intPrimitive":32},{"theString":"E3","intPrimitive":33}]],"c0Coll":[{"theString":"E4","intPrimitive":40},{"theString":"E4","intPrimitive":41}]}}`,
			want:    `{"local":{"c0":{"theString":"E1","intPrimitive":1},"c0Arr":[{"theString":"E2","intPrimitive":10},{"theString":"E2","intPrimitive":11},{"theString":"E2","intPrimitive":12}],"c0Arr2Dim":[[{"theString":"E3","intPrimitive":30},{"theString":"E3","intPrimitive":31}],[{"theString":"E3","intPrimitive":32},{"theString":"E3","intPrimitive":33}]],"c0Coll":[{"theString":"E4","intPrimitive":40},{"theString":"E4","intPrimitive":41}]}}`,
			local: jsonClassNestedArraysParity{
				C0:        jsonNestedItem("E1", 1),
				C0Arr:     []*jsonClassNestedItemParity{jsonNestedItem("E2", 10), jsonNestedItem("E2", 11), jsonNestedItem("E2", 12)},
				C0Arr2Dim: [][]*jsonClassNestedItemParity{{jsonNestedItem("E3", 30), jsonNestedItem("E3", 31)}, {jsonNestedItem("E3", 32), jsonNestedItem("E3", 33)}},
				C0Coll:    []*jsonClassNestedItemParity{jsonNestedItem("E4", 40), jsonNestedItem("E4", 41)},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, event := parseJSONParityEvent[jsonClassNestedArraysRoot](t, "JSONTypedClassNestedArrays", test.payload)
			got := event.Get("local").Any().(jsonClassNestedArraysParity)
			if !reflect.DeepEqual(got, test.local) {
				t.Fatalf("nested arrays value = %#v, want %#v", got, test.local)
			}
			assertJSONParityRender(t, event, test.want)
		})
	}
}

func jsonBigInt(text string) *big.Int {
	value, ok := new(big.Int).SetString(text, 10)
	if !ok {
		panic(text)
	}
	return value
}

func jsonBigRat(text string) *big.Rat {
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		panic(text)
	}
	return value
}

func TestJSONTypedClassBuiltinListParity(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    string
		local   jsonClassBuiltinListParity
	}{
		{
			name:    "filled",
			payload: `{"local":{"c0":["abc","def"],"c1":["x","y"],"c2":[true,false],"c3":[10,11],"c4":[20,21],"c5":[30,31],"c6":[40,41],"c7":[50.0,51.0],"c8":[60.0,61.0],"c9":[70,71],"c10":[80,81]}}`,
			want:    `{"local":{"c0":["abc","def"],"c1":["x","y"],"c2":[true,false],"c3":[10,11],"c4":[20,21],"c5":[30,31],"c6":[40,41],"c7":[50.0,51.0],"c8":[60.0,61.0],"c9":[70,71],"c10":[80,81]}}`,
			local: jsonClassBuiltinListParity{
				C0:  []*string{jsonTestPointer("abc"), jsonTestPointer("def")},
				C1:  []*rune{jsonTestPointer(rune('x')), jsonTestPointer(rune('y'))},
				C2:  []*bool{jsonTestPointer(true), jsonTestPointer(false)},
				C3:  []*int8{jsonTestPointer(int8(10)), jsonTestPointer(int8(11))},
				C4:  []*int16{jsonTestPointer(int16(20)), jsonTestPointer(int16(21))},
				C5:  []*int{jsonTestPointer(30), jsonTestPointer(31)},
				C6:  []*int64{jsonTestPointer(int64(40)), jsonTestPointer(int64(41))},
				C7:  []*float64{jsonTestPointer(float64(50)), jsonTestPointer(float64(51))},
				C8:  []*float32{jsonTestPointer(float32(60)), jsonTestPointer(float32(61))},
				C9:  []*big.Int{jsonBigInt("70"), jsonBigInt("71")},
				C10: []*big.Rat{jsonBigRat("80"), jsonBigRat("81")},
			},
		},
		{
			name:    "unfilled",
			payload: `{"local":{}}`,
			want:    `{"local":{"c0":null,"c1":null,"c2":null,"c3":null,"c4":null,"c5":null,"c6":null,"c7":null,"c8":null,"c9":null,"c10":null}}`,
			local:   jsonClassBuiltinListParity{},
		},
		{
			name:    "empty",
			payload: `{"local":{"c0":[],"c1":[],"c2":[],"c3":[],"c4":[],"c5":[],"c6":[],"c7":[],"c8":[],"c9":[],"c10":[]}}`,
			want:    `{"local":{"c0":[],"c1":[],"c2":[],"c3":[],"c4":[],"c5":[],"c6":[],"c7":[],"c8":[],"c9":[],"c10":[]}}`,
			local: jsonClassBuiltinListParity{
				C0: []*string{}, C1: []*rune{}, C2: []*bool{}, C3: []*int8{}, C4: []*int16{}, C5: []*int{}, C6: []*int64{}, C7: []*float64{}, C8: []*float32{}, C9: []*big.Int{}, C10: []*big.Rat{},
			},
		},
		{
			name:    "null",
			payload: `{"local":{"c0":null,"c1":null,"c2":null,"c3":null,"c4":null,"c5":null,"c6":null,"c7":null,"c8":null,"c9":null,"c10":null}}`,
			want:    `{"local":{"c0":null,"c1":null,"c2":null,"c3":null,"c4":null,"c5":null,"c6":null,"c7":null,"c8":null,"c9":null,"c10":null}}`,
			local:   jsonClassBuiltinListParity{},
		},
		{
			name:    "partial",
			payload: `{"local":{"c0":["abc",null],"c1":["x",null],"c2":[true,null],"c3":[10,null],"c4":[20,null],"c5":[30,null],"c6":[40,null],"c7":[50.0,null],"c8":[60.0,null],"c9":[70,null],"c10":[80,null]}}`,
			want:    `{"local":{"c0":["abc",null],"c1":["x",null],"c2":[true,null],"c3":[10,null],"c4":[20,null],"c5":[30,null],"c6":[40,null],"c7":[50.0,null],"c8":[60.0,null],"c9":[70,null],"c10":[80,null]}}`,
			local: jsonClassBuiltinListParity{
				C0:  []*string{jsonTestPointer("abc"), nil},
				C1:  []*rune{jsonTestPointer(rune('x')), nil},
				C2:  []*bool{jsonTestPointer(true), nil},
				C3:  []*int8{jsonTestPointer(int8(10)), nil},
				C4:  []*int16{jsonTestPointer(int16(20)), nil},
				C5:  []*int{jsonTestPointer(30), nil},
				C6:  []*int64{jsonTestPointer(int64(40)), nil},
				C7:  []*float64{jsonTestPointer(float64(50)), nil},
				C8:  []*float32{jsonTestPointer(float32(60)), nil},
				C9:  []*big.Int{jsonBigInt("70"), nil},
				C10: []*big.Rat{jsonBigRat("80"), nil},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, event := parseJSONParityEvent[jsonClassBuiltinListRoot](t, "JSONTypedClassBuiltinList", test.payload)
			got := event.Get("local").Any().(jsonClassBuiltinListParity)
			if !reflect.DeepEqual(got, test.local) {
				t.Fatalf("builtin list value = %#v, want %#v", got, test.local)
			}
			assertJSONParityRender(t, event, test.want)
		})
	}
}

func TestJSONTypedClassEnumListParity(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    string
		value   []*jsonClassEnumParity
	}{
		{name: "filled", payload: `{"local":{"c0":["ENUM_VALUE_1","ENUM_VALUE_2"]}}`, want: `{"local":{"c0":["ENUM_VALUE_1","ENUM_VALUE_2"]}}`, value: []*jsonClassEnumParity{jsonTestPointer(jsonClassEnumParity("ENUM_VALUE_1")), jsonTestPointer(jsonClassEnumParity("ENUM_VALUE_2"))}},
		{name: "unfilled", payload: `{"local":{}}`, want: `{"local":{"c0":null}}`, value: nil},
		{name: "empty", payload: `{"local":{"c0":[]}}`, want: `{"local":{"c0":[]}}`, value: []*jsonClassEnumParity{}},
		{name: "null", payload: `{"local":{"c0":null}}`, want: `{"local":{"c0":null}}`, value: nil},
		{name: "partial", payload: `{"local":{"c0":["ENUM_VALUE_3",null]}}`, want: `{"local":{"c0":["ENUM_VALUE_3",null]}}`, value: []*jsonClassEnumParity{jsonTestPointer(jsonClassEnumParity("ENUM_VALUE_3")), nil}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, event := parseJSONParityEvent[jsonClassEnumListRoot](t, "JSONTypedClassEnumList", test.payload)
			got := event.Get("local").Any().(jsonClassEnumListParity)
			if !reflect.DeepEqual(got.C0, test.value) {
				t.Fatalf("enum list value = %#v, want %#v", got.C0, test.value)
			}
			assertJSONParityRender(t, event, test.want)
		})
	}
}

func jsonStringValue(text string) string {
	encoded, _ := json.Marshal(text)
	return string(encoded)
}

func jsonVMFilledPayload(text string) string {
	quoted := jsonStringValue(text)
	return `{"local":{"c0":` + quoted + `,"c0Arr":[` + quoted + `],"c0Arr2Dim":[[` + quoted + `]],"c0Coll":[` + quoted + `]}}`
}

func jsonVMNullPayload() string {
	return `{"local":{"c0":null,"c0Arr":[null],"c0Arr2Dim":[[null]],"c0Coll":[null]}}`
}

func jsonVMInvalidPayload(token string) string {
	return `{"local":{"c0":` + token + `}}`
}

func runJSONVMParityCase[T, L any](t *testing.T, name, text, invalidToken string, assertFilled func(L), assertNull func(L)) {
	t.Helper()
	_, filled := parseJSONParityEvent[T](t, name, jsonVMFilledPayload(text))
	assertFilled(filled.Get("local").Any().(L))
	assertJSONParityRender(t, filled, jsonVMFilledPayload(text))

	_, nulled := parseJSONParityEvent[T](t, name, jsonVMNullPayload())
	assertNull(nulled.Get("local").Any().(L))
	assertJSONParityRender(t, nulled, jsonVMNullPayload())

	schema, err := NewJSONSchemaFor[T](name, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseJSON(schema, []byte(jsonVMInvalidPayload(invalidToken)), time.Unix(0, 0)); err == nil {
		t.Fatalf("invalid %s JSON value unexpectedly parsed", name)
	}
}

func TestJSONTypedClassVMScalarParity(t *testing.T) {
	uuid, err := ParseUUID("b7dc7f66-4f6d-4f03-14d7-83da210dfba6")
	if err != nil {
		t.Fatal(err)
	}
	runJSONVMParityCase[jsonVMUUIDRoot, jsonVMUUIDParity](t, "JSONTypedClassVMUUID", uuid.String(), "1", func(root jsonVMUUIDParity) {
		if root.C0 == nil || *root.C0 != uuid || !reflect.DeepEqual(root.C0Arr, []*UUID{jsonTestPointer(uuid)}) || !reflect.DeepEqual(root.C0Arr2Dim, [][]*UUID{{jsonTestPointer(uuid)}}) || !reflect.DeepEqual(root.C0Coll, []*UUID{jsonTestPointer(uuid)}) {
			t.Fatalf("UUID VM value = %#v", root)
		}
	}, func(root jsonVMUUIDParity) {
		if root.C0 != nil || root.C0Arr[0] != nil || root.C0Arr2Dim[0][0] != nil || root.C0Coll[0] != nil {
			t.Fatalf("UUID VM null value = %#v", root)
		}
	})

	offsetText := "2026-08-05T08:00:00.123+08:00"
	offset, err := time.Parse(time.RFC3339Nano, offsetText)
	if err != nil {
		t.Fatal(err)
	}
	runJSONVMParityCase[jsonVMTimeRoot, jsonVMTimeParity](t, "JSONTypedClassVMOffsetDateTime", offsetText, "1", func(root jsonVMTimeParity) {
		if root.C0 == nil || !root.C0.Equal(offset) || !reflect.DeepEqual(root.C0Arr, []*time.Time{jsonTestPointer(offset)}) || !reflect.DeepEqual(root.C0Arr2Dim, [][]*time.Time{{jsonTestPointer(offset)}}) || !reflect.DeepEqual(root.C0Coll, []*time.Time{jsonTestPointer(offset)}) {
			t.Fatalf("OffsetDateTime VM value = %#v", root)
		}
	}, func(root jsonVMTimeParity) {
		if root.C0 != nil || root.C0Arr[0] != nil || root.C0Arr2Dim[0][0] != nil || root.C0Coll[0] != nil {
			t.Fatalf("OffsetDateTime VM null value = %#v", root)
		}
	})

	dateText := "2026-08-05"
	date := DateOnly(dateText)
	runJSONVMParityCase[jsonVMDateOnlyRoot, jsonVMDateOnlyParity](t, "JSONTypedClassVMLocalDate", dateText, "1", func(root jsonVMDateOnlyParity) {
		if root.C0 == nil || *root.C0 != date || !reflect.DeepEqual(root.C0Arr, []*DateOnly{jsonTestPointer(date)}) || !reflect.DeepEqual(root.C0Arr2Dim, [][]*DateOnly{{jsonTestPointer(date)}}) || !reflect.DeepEqual(root.C0Coll, []*DateOnly{jsonTestPointer(date)}) {
			t.Fatalf("LocalDate VM value = %#v", root)
		}
	}, func(root jsonVMDateOnlyParity) {
		if root.C0 != nil || root.C0Arr[0] != nil || root.C0Arr2Dim[0][0] != nil || root.C0Coll[0] != nil {
			t.Fatalf("LocalDate VM null value = %#v", root)
		}
	})

	localDateTimeText := "2026-08-05T08:00:00.123"
	localDateTime, err := time.Parse("2006-01-02T15:04:05.999999999", localDateTimeText)
	if err != nil {
		t.Fatal(err)
	}
	runJSONVMParityCase[jsonVMTimeRoot, jsonVMTimeParity](t, "JSONTypedClassVMLocalDateTime", localDateTimeText, "1", func(root jsonVMTimeParity) {
		if root.C0 == nil || !root.C0.Equal(localDateTime) {
			t.Fatalf("LocalDateTime VM value = %#v", root)
		}
	}, func(root jsonVMTimeParity) {
		if root.C0 != nil || root.C0Arr[0] != nil || root.C0Arr2Dim[0][0] != nil || root.C0Coll[0] != nil {
			t.Fatalf("LocalDateTime VM null value = %#v", root)
		}
	})

	zonedText := "2026-08-05T08:00:00.123+08:00[Asia/Shanghai]"
	zoned, err := time.Parse(time.RFC3339Nano, "2026-08-05T08:00:00.123+08:00")
	if err != nil {
		t.Fatal(err)
	}
	runJSONVMParityCase[jsonVMTimeRoot, jsonVMTimeParity](t, "JSONTypedClassVMZonedDateTime", zonedText, "1", func(root jsonVMTimeParity) {
		if root.C0 == nil || !root.C0.Equal(zoned) {
			t.Fatalf("ZonedDateTime VM value = %#v", root)
		}
	}, func(root jsonVMTimeParity) {
		if root.C0 != nil || root.C0Arr[0] != nil || root.C0Arr2Dim[0][0] != nil || root.C0Coll[0] != nil {
			t.Fatalf("ZonedDateTime VM null value = %#v", root)
		}
	})

	urlText := "http://host:1000/file"
	parsedURL, err := ParseURL(urlText)
	if err != nil {
		t.Fatal(err)
	}
	runJSONVMParityCase[jsonVMURLRoot, jsonVMURLParity](t, "JSONTypedClassVMURL", urlText, "1", func(root jsonVMURLParity) {
		if root.C0 == nil || *root.C0 != parsedURL || !reflect.DeepEqual(root.C0Arr, []*URL{jsonTestPointer(parsedURL)}) || !reflect.DeepEqual(root.C0Arr2Dim, [][]*URL{{jsonTestPointer(parsedURL)}}) || !reflect.DeepEqual(root.C0Coll, []*URL{jsonTestPointer(parsedURL)}) {
			t.Fatalf("URL VM value = %#v", root)
		}
	}, func(root jsonVMURLParity) {
		if root.C0 != nil || root.C0Arr[0] != nil || root.C0Arr2Dim[0][0] != nil || root.C0Coll[0] != nil {
			t.Fatalf("URL VM null value = %#v", root)
		}
	})

	uriText := "ftp://ftp.is.co.za/rfc/rfc1808.txt"
	parsedURI, err := ParseURI(uriText)
	if err != nil {
		t.Fatal(err)
	}
	runJSONVMParityCase[jsonVMURIRoot, jsonVMURIParity](t, "JSONTypedClassVMURI", uriText, `"a b"`, func(root jsonVMURIParity) {
		if root.C0 == nil || *root.C0 != parsedURI || !reflect.DeepEqual(root.C0Arr, []*URI{jsonTestPointer(parsedURI)}) || !reflect.DeepEqual(root.C0Arr2Dim, [][]*URI{{jsonTestPointer(parsedURI)}}) || !reflect.DeepEqual(root.C0Coll, []*URI{jsonTestPointer(parsedURI)}) {
			t.Fatalf("URI VM value = %#v", root)
		}
	}, func(root jsonVMURIParity) {
		if root.C0 != nil || root.C0Arr[0] != nil || root.C0Arr2Dim[0][0] != nil || root.C0Coll[0] != nil {
			t.Fatalf("URI VM null value = %#v", root)
		}
	})
}
