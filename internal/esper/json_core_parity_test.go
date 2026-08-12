package esper

import (
	"encoding/json"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

// These structures mirror the declared property families in
// EventJsonTypingCoreParse/CoreWrite. Pointers model Java's nullable wrapper
// elements; non-pointer slices model the primitive-array branches.
type jsonCoreScalarParity struct {
	C0  *string  `json:"c0"`
	C1  *rune    `json:"c1"`
	C2  *rune    `json:"c2"`
	C3  *bool    `json:"c3"`
	C4  *bool    `json:"c4"`
	C5  *int8    `json:"c5"`
	C6  *int16   `json:"c6"`
	C7  *int     `json:"c7"`
	C8  *int     `json:"c8"`
	C9  *int64   `json:"c9"`
	C10 *float64 `json:"c10"`
	C11 *float32 `json:"c11"`
	C12 any      `json:"c12"`
}

type jsonCoreArrayParity struct {
	C0  []*string  `json:"c0"`
	C1  []*rune    `json:"c1"`
	C2  []rune     `json:"c2"`
	C3  []*bool    `json:"c3"`
	C4  []bool     `json:"c4"`
	C5  []*int8    `json:"c5"`
	C6  []int8     `json:"c6"`
	C7  []*int16   `json:"c7"`
	C8  []int16    `json:"c8"`
	C9  []*int     `json:"c9"`
	C10 []int      `json:"c10"`
	C11 []*int64   `json:"c11"`
	C12 []int64    `json:"c12"`
	C13 []*float64 `json:"c13"`
	C14 []float64  `json:"c14"`
	C15 []*float32 `json:"c15"`
	C16 []float32  `json:"c16"`
}

type jsonCoreArray2DParity struct {
	C0  [][]*string  `json:"c0"`
	C1  [][]*rune    `json:"c1"`
	C2  [][]rune     `json:"c2"`
	C3  [][]*bool    `json:"c3"`
	C4  [][]bool     `json:"c4"`
	C5  [][]*int8    `json:"c5"`
	C6  [][]int8     `json:"c6"`
	C7  [][]*int16   `json:"c7"`
	C8  [][]int16    `json:"c8"`
	C9  [][]*int     `json:"c9"`
	C10 [][]int      `json:"c10"`
	C11 [][]*int64   `json:"c11"`
	C12 [][]int64    `json:"c12"`
	C13 [][]*float64 `json:"c13"`
	C14 [][]float64  `json:"c14"`
	C15 [][]*float32 `json:"c15"`
	C16 [][]float32  `json:"c16"`
}

type jsonCoreBigParity struct {
	C0 *big.Int     `json:"c0"`
	C1 *big.Rat     `json:"c1"`
	C2 []*big.Int   `json:"c2"`
	C3 []*big.Rat   `json:"c3"`
	C4 [][]*big.Int `json:"c4"`
	C5 [][]*big.Rat `json:"c5"`
}

type jsonCoreEnumParity string

type jsonCoreEnumFields struct {
	C0 *jsonCoreEnumParity     `json:"c0"`
	C1 []*jsonCoreEnumParity   `json:"c1"`
	C2 [][]*jsonCoreEnumParity `json:"c2"`
}

type jsonCoreObjectField struct {
	C0 any `json:"c0"`
}

type jsonCoreObjectArrayField struct {
	C0 []any `json:"c0"`
}

type jsonCoreMapField struct {
	C0 map[string]any `json:"c0"`
}

type jsonCoreNestedBook struct {
	BookID string   `json:"bookId"`
	Price  *big.Rat `json:"price"`
}

type jsonCoreNestedShelf struct {
	ShelfID string              `json:"shelfId"`
	Book    *jsonCoreNestedBook `json:"book"`
}

type jsonCoreNestedIsle struct {
	IsleID string               `json:"isleId"`
	Shelf  *jsonCoreNestedShelf `json:"shelf"`
}

type jsonCoreNestedLibrary struct {
	LibraryID string              `json:"libraryId"`
	Isle      *jsonCoreNestedIsle `json:"isle"`
}

type jsonCoreNestedArrayShelf struct {
	ShelfID string                `json:"shelfId"`
	Books   []*jsonCoreNestedBook `json:"books"`
}

type jsonCoreNestedArrayIsle struct {
	IsleID  string                      `json:"isleId"`
	Shelves []*jsonCoreNestedArrayShelf `json:"shelfs"`
}

type jsonCoreNestedArrayLibrary struct {
	LibraryID string                     `json:"libraryId"`
	Isles     []*jsonCoreNestedArrayIsle `json:"isles"`
}

func jsonCoreFieldNames(count int) []string {
	fields := make([]string, count)
	for index := range fields {
		fields[index] = "c" + jsonCoreIndexText(index)
	}
	return fields
}

func jsonCoreIndexText(index int) string {
	if index < 10 {
		return string(rune('0' + index))
	}
	return string([]rune{'1', rune('0' + index - 10)})
}

func jsonCoreAllFieldsValue(value string, count int) string {
	fields := make([]string, count)
	for index, name := range jsonCoreFieldNames(count) {
		fields[index] = `"` + name + `":` + value
	}
	return "{" + strings.Join(fields, ",") + "}"
}

func TestJSONCoreTypedScalarCharacterAndNumberParity(t *testing.T) {
	valid := `{"c0":"abc","c1":"xy","c2":"z","c3":true,"c4":false,"c5":1,"c6":10,"c7":11,"c8":12,"c9":13,"c10":14.0,"c11":1500.0,"c12":null}`
	_, event := parseJSONParityEvent[jsonCoreScalarParity](t, "JSONCoreScalar", valid)
	value := event.Underlying().(jsonCoreScalarParity)
	if value.C0 == nil || *value.C0 != "abc" || value.C1 == nil || *value.C1 != rune('x') || value.C2 == nil || *value.C2 != rune('z') || value.C3 == nil || !*value.C3 || value.C4 == nil || *value.C4 || value.C5 == nil || *value.C5 != 1 || value.C6 == nil || *value.C6 != 10 || value.C7 == nil || *value.C7 != 11 || value.C8 == nil || *value.C8 != 12 || value.C9 == nil || *value.C9 != 13 || value.C10 == nil || *value.C10 != 14 || value.C11 == nil || *value.C11 != 1500 || value.C12 != nil {
		t.Fatalf("core scalar value = %#v", value)
	}
	assertJSONParityRender(t, event, `{"c0":"abc","c1":"x","c2":"z","c3":true,"c4":false,"c5":1,"c6":10,"c7":11,"c8":12,"c9":13,"c10":14.0,"c11":1500.0,"c12":null}`)

	for _, payload := range []string{`{}`, `{"c0":null,"c1":null,"c2":null,"c3":null,"c4":null,"c5":null,"c6":null,"c7":null,"c8":null,"c9":null,"c10":null,"c11":null,"c12":null}`} {
		_, nullEvent := parseJSONParityEvent[jsonCoreScalarParity](t, "JSONCoreScalar", payload)
		if !reflect.DeepEqual(nullEvent.Underlying(), jsonCoreScalarParity{}) {
			t.Fatalf("core scalar null/missing value = %#v", nullEvent.Underlying())
		}
		assertJSONParityRender(t, nullEvent, `{"c0":null,"c1":null,"c2":null,"c3":null,"c4":null,"c5":null,"c6":null,"c7":null,"c8":null,"c9":null,"c10":null,"c11":null,"c12":null}`)
	}
}

func TestJSONCoreTypedEnumParity(t *testing.T) {
	full := `{"c0":"ENUM_VALUE_2","c1":["ENUM_VALUE_2","ENUM_VALUE_1"],"c2":[["ENUM_VALUE_2"],["ENUM_VALUE_1","ENUM_VALUE_3"]]}`
	_, event := parseJSONParityEvent[jsonCoreEnumFields](t, "JSONCoreEnum", full)
	want := jsonCoreEnumFields{
		C0: jsonTestPointer(jsonCoreEnumParity("ENUM_VALUE_2")),
		C1: []*jsonCoreEnumParity{jsonTestPointer(jsonCoreEnumParity("ENUM_VALUE_2")), jsonTestPointer(jsonCoreEnumParity("ENUM_VALUE_1"))},
		C2: [][]*jsonCoreEnumParity{{jsonTestPointer(jsonCoreEnumParity("ENUM_VALUE_2"))}, {jsonTestPointer(jsonCoreEnumParity("ENUM_VALUE_1")), jsonTestPointer(jsonCoreEnumParity("ENUM_VALUE_3"))}},
	}
	if !reflect.DeepEqual(event.Underlying(), want) {
		t.Fatalf("core enum value = %#v, want %#v", event.Underlying(), want)
	}
	assertJSONParityRender(t, event, full)

	_, empty := parseJSONParityEvent[jsonCoreEnumFields](t, "JSONCoreEnum", `{"c1":[],"c2":[[]]}`)
	emptyWant := jsonCoreEnumFields{C1: []*jsonCoreEnumParity{}, C2: [][]*jsonCoreEnumParity{{}}}
	if !reflect.DeepEqual(empty.Underlying(), emptyWant) {
		t.Fatalf("core enum empty value = %#v, want %#v", empty.Underlying(), emptyWant)
	}
	assertJSONParityRender(t, empty, `{"c0":null,"c1":[],"c2":[[]]}`)

	for _, payload := range []string{`{}`, `{"c0":null,"c1":null,"c2":null}`} {
		_, nullEvent := parseJSONParityEvent[jsonCoreEnumFields](t, "JSONCoreEnum", payload)
		if !reflect.DeepEqual(nullEvent.Underlying(), jsonCoreEnumFields{}) {
			t.Fatalf("core enum null value = %#v", nullEvent.Underlying())
		}
		assertJSONParityRender(t, nullEvent, `{"c0":null,"c1":null,"c2":null}`)
	}
}

func TestJSONCoreTypedArrayParity(t *testing.T) {
	full := `{"c0":["abc","def"],"c1":["x","z"],"c2":["x","y"],"c3":[true,false],"c4":[false,true],"c5":[10,11],"c6":[12,13],"c7":[20,21],"c8":[22,23],"c9":[30,31],"c10":[32,33],"c11":[40,41],"c12":[42,43],"c13":[50.0,51.0],"c14":[52.0,53.0],"c15":[60.0,61.0],"c16":[62.0,63.0]}`
	_, event := parseJSONParityEvent[jsonCoreArrayParity](t, "JSONCoreArray", full)
	want := jsonCoreArrayParity{
		C0: []*string{jsonTestPointer("abc"), jsonTestPointer("def")}, C1: []*rune{jsonTestPointer(rune('x')), jsonTestPointer(rune('z'))}, C2: []rune{'x', 'y'}, C3: []*bool{jsonTestPointer(true), jsonTestPointer(false)}, C4: []bool{false, true}, C5: []*int8{jsonTestPointer(int8(10)), jsonTestPointer(int8(11))}, C6: []int8{12, 13}, C7: []*int16{jsonTestPointer(int16(20)), jsonTestPointer(int16(21))}, C8: []int16{22, 23}, C9: []*int{jsonTestPointer(30), jsonTestPointer(31)}, C10: []int{32, 33}, C11: []*int64{jsonTestPointer(int64(40)), jsonTestPointer(int64(41))}, C12: []int64{42, 43}, C13: []*float64{jsonTestPointer(float64(50)), jsonTestPointer(float64(51))}, C14: []float64{52, 53}, C15: []*float32{jsonTestPointer(float32(60)), jsonTestPointer(float32(61))}, C16: []float32{62, 63},
	}
	if !reflect.DeepEqual(event.Underlying(), want) {
		t.Fatalf("core array value = %#v, want %#v", event.Underlying(), want)
	}
	assertJSONParityRender(t, event, full)

	empty := jsonCoreAllFieldsValue("[]", 17)
	_, emptyEvent := parseJSONParityEvent[jsonCoreArrayParity](t, "JSONCoreArray", empty)
	if !reflect.DeepEqual(emptyEvent.Underlying(), jsonCoreArrayParity{C0: []*string{}, C1: []*rune{}, C2: []rune{}, C3: []*bool{}, C4: []bool{}, C5: []*int8{}, C6: []int8{}, C7: []*int16{}, C8: []int16{}, C9: []*int{}, C10: []int{}, C11: []*int64{}, C12: []int64{}, C13: []*float64{}, C14: []float64{}, C15: []*float32{}, C16: []float32{}}) {
		t.Fatalf("core empty arrays = %#v", emptyEvent.Underlying())
	}
	assertJSONParityRender(t, emptyEvent, empty)

	nulled := jsonCoreAllFieldsValue("null", 17)
	_, nullEvent := parseJSONParityEvent[jsonCoreArrayParity](t, "JSONCoreArray", nulled)
	if !reflect.DeepEqual(nullEvent.Underlying(), jsonCoreArrayParity{}) {
		t.Fatalf("core null arrays = %#v", nullEvent.Underlying())
	}
	assertJSONParityRender(t, nullEvent, nulled)

	partial := `{"c0":[null,"def",null],"c1":["x",null],"c2":["x"],"c3":[true,null,false],"c4":[true],"c5":[null,null,null],"c6":[12],"c7":[20,21,null],"c8":[23],"c9":[null,30,null,31,null,32],"c10":[32],"c11":[null,40,41,null],"c12":[42],"c13":[null,null,51.0],"c14":[52.0],"c15":[null],"c16":[63.0]}`
	_, partialEvent := parseJSONParityEvent[jsonCoreArrayParity](t, "JSONCoreArray", partial)
	partialWant := jsonCoreArrayParity{C0: []*string{nil, jsonTestPointer("def"), nil}, C1: []*rune{jsonTestPointer(rune('x')), nil}, C2: []rune{'x'}, C3: []*bool{jsonTestPointer(true), nil, jsonTestPointer(false)}, C4: []bool{true}, C5: []*int8{nil, nil, nil}, C6: []int8{12}, C7: []*int16{jsonTestPointer(int16(20)), jsonTestPointer(int16(21)), nil}, C8: []int16{23}, C9: []*int{nil, jsonTestPointer(30), nil, jsonTestPointer(31), nil, jsonTestPointer(32)}, C10: []int{32}, C11: []*int64{nil, jsonTestPointer(int64(40)), jsonTestPointer(int64(41)), nil}, C12: []int64{42}, C13: []*float64{nil, nil, jsonTestPointer(float64(51))}, C14: []float64{52}, C15: []*float32{nil}, C16: []float32{63}}
	if !reflect.DeepEqual(partialEvent.Underlying(), partialWant) {
		t.Fatalf("core partial arrays = %#v, want %#v", partialEvent.Underlying(), partialWant)
	}
	assertJSONParityRender(t, partialEvent, partial)
}

func TestJSONCoreTypedArray2DParity(t *testing.T) {
	full := `{"c0":[["a","b"],["c"]],"c1":[["x","z"],["n"]],"c2":[["x"],["y","z"]],"c3":[[],[true,false],[]],"c4":[[false,true]],"c5":[[10],[11]],"c6":[[12,13]],"c7":[[20,21],[22,23]],"c8":[[22],[23],[]],"c9":[[],[],[30,31]],"c10":[[32],[33,34]],"c11":[[40],[],[41]],"c12":[[42,43],[44]],"c13":[[50],[51,52],[53]],"c14":[[54],[55,56]],"c15":[[60,61],[]],"c16":[[62],[63]]}`
	_, event := parseJSONParityEvent[jsonCoreArray2DParity](t, "JSONCoreArray2D", full)
	want := jsonCoreArray2DParity{C0: [][]*string{{jsonTestPointer("a"), jsonTestPointer("b")}, {jsonTestPointer("c")}}, C1: [][]*rune{{jsonTestPointer(rune('x')), jsonTestPointer(rune('z'))}, {jsonTestPointer(rune('n'))}}, C2: [][]rune{{'x'}, {'y', 'z'}}, C3: [][]*bool{{}, {jsonTestPointer(true), jsonTestPointer(false)}, {}}, C4: [][]bool{{false, true}}, C5: [][]*int8{{jsonTestPointer(int8(10))}, {jsonTestPointer(int8(11))}}, C6: [][]int8{{12, 13}}, C7: [][]*int16{{jsonTestPointer(int16(20)), jsonTestPointer(int16(21))}, {jsonTestPointer(int16(22)), jsonTestPointer(int16(23))}}, C8: [][]int16{{22}, {23}, {}}, C9: [][]*int{{}, {}, {jsonTestPointer(30), jsonTestPointer(31)}}, C10: [][]int{{32}, {33, 34}}, C11: [][]*int64{{jsonTestPointer(int64(40))}, {}, {jsonTestPointer(int64(41))}}, C12: [][]int64{{42, 43}, {44}}, C13: [][]*float64{{jsonTestPointer(float64(50))}, {jsonTestPointer(float64(51)), jsonTestPointer(float64(52))}, {jsonTestPointer(float64(53))}}, C14: [][]float64{{54}, {55, 56}}, C15: [][]*float32{{jsonTestPointer(float32(60)), jsonTestPointer(float32(61))}, {}}, C16: [][]float32{{62}, {63}}}
	if !reflect.DeepEqual(event.Underlying(), want) {
		t.Fatalf("core 2D array value = %#v, want %#v", event.Underlying(), want)
	}
	assertJSONParityRender(t, event, full)

	partial := `{"c0":[[null,"a"]],"c1":[[null],["x"]],"c2":[null,["x"]],"c3":[[null],[true]],"c4":[[true],null],"c5":[null,null],"c6":[null,[12,13]],"c7":[[21],null],"c8":[null,[23],null],"c9":[[30],null,[31]],"c10":[[]],"c11":[[],[]],"c12":[[42]],"c13":[null,[]],"c14":[[],null],"c15":[[null]],"c16":[[63]]}`
	_, partialEvent := parseJSONParityEvent[jsonCoreArray2DParity](t, "JSONCoreArray2D", partial)
	partialWant := jsonCoreArray2DParity{C0: [][]*string{{nil, jsonTestPointer("a")}}, C1: [][]*rune{{nil}, {jsonTestPointer(rune('x'))}}, C2: [][]rune{nil, {'x'}}, C3: [][]*bool{{nil}, {jsonTestPointer(true)}}, C4: [][]bool{{true}, nil}, C5: [][]*int8{nil, nil}, C6: [][]int8{nil, {12, 13}}, C7: [][]*int16{{jsonTestPointer(int16(21))}, nil}, C8: [][]int16{nil, {23}, nil}, C9: [][]*int{{jsonTestPointer(30)}, nil, {jsonTestPointer(31)}}, C10: [][]int{{}}, C11: [][]*int64{{}, {}}, C12: [][]int64{{42}}, C13: [][]*float64{nil, {}}, C14: [][]float64{{}, nil}, C15: [][]*float32{{nil}}, C16: [][]float32{{63}}}
	if !reflect.DeepEqual(partialEvent.Underlying(), partialWant) {
		t.Fatalf("core partial 2D arrays = %#v, want %#v", partialEvent.Underlying(), partialWant)
	}
	assertJSONParityRender(t, partialEvent, partial)
}

func TestJSONCoreBigIntegerDecimalNestedPrecisionParity(t *testing.T) {
	integerText := "123456789123456789123456789"
	decimalText := "123456789123456789123456789.1"
	valid := `{"c0":` + integerText + `,"c1":` + decimalText + `,"c2":[` + integerText + `],"c3":[` + decimalText + `],"c4":[[` + integerText + `]],"c5":[[` + decimalText + `]]}`
	_, event := parseJSONParityEvent[jsonCoreBigParity](t, "JSONCoreBig", valid)
	integer := jsonBigInt(integerText)
	decimal := jsonBigRat(decimalText)
	want := jsonCoreBigParity{C0: integer, C1: decimal, C2: []*big.Int{jsonBigInt(integerText)}, C3: []*big.Rat{jsonBigRat(decimalText)}, C4: [][]*big.Int{{jsonBigInt(integerText)}}, C5: [][]*big.Rat{{jsonBigRat(decimalText)}}}
	value := event.Underlying().(jsonCoreBigParity)
	if value.C0.Cmp(integer) != 0 || value.C1.Cmp(decimal) != 0 || !reflect.DeepEqual(value.C2, want.C2) || !reflect.DeepEqual(value.C3, want.C3) || !reflect.DeepEqual(value.C4, want.C4) || !reflect.DeepEqual(value.C5, want.C5) {
		t.Fatalf("core arbitrary precision value = %#v, want %#v", value, want)
	}
	assertJSONParityRender(t, event, valid)

	for _, payload := range []string{`{}`, `{"c0":null,"c1":null,"c2":null,"c3":null,"c4":null,"c5":null}`} {
		_, nullEvent := parseJSONParityEvent[jsonCoreBigParity](t, "JSONCoreBig", payload)
		if !reflect.DeepEqual(nullEvent.Underlying(), jsonCoreBigParity{}) {
			t.Fatalf("core arbitrary precision null value = %#v", nullEvent.Underlying())
		}
		assertJSONParityRender(t, nullEvent, `{"c0":null,"c1":null,"c2":null,"c3":null,"c4":null,"c5":null}`)
	}
}

func TestJSONCoreTypedMapObjectAndObjectArrayParity(t *testing.T) {
	type coreDynamic struct {
		Object      any            `json:"object"`
		ObjectArray []any          `json:"objectArray"`
		Map         map[string]any `json:"map"`
	}
	input := `{"object":{"c1":10,"c2":"abc"},"objectArray":["abc",2,{"nested":5.0}],"map":{"c1":{"c2":20},"c3":["x",1.0]}}`
	_, event := parseJSONParityEvent[coreDynamic](t, "JSONCoreDynamicContainers", input)
	value := event.Underlying().(coreDynamic)
	object, ok := value.Object.(map[string]any)
	if !ok || object["c1"] != 10 || object["c2"] != "abc" {
		t.Fatalf("typed Object value = %#v", value.Object)
	}
	if len(value.ObjectArray) != 3 || value.ObjectArray[0] != "abc" || value.ObjectArray[1] != 2 {
		t.Fatalf("typed Object[] value = %#v", value.ObjectArray)
	}
	number, ok := value.ObjectArray[2].(map[string]any)["nested"].(json.Number)
	if !ok || number.String() != "5.0" {
		t.Fatalf("typed Object[] decimal value = %#v", value.ObjectArray[2])
	}
	assertJSONParityRender(t, event, input)
}

func TestJSONCoreObjectMapAndObjectArrayMatrixParity(t *testing.T) {
	objectCases := []struct {
		name   string
		input  string
		render string
		want   any
	}{
		{name: "missing", input: `{}`, render: `{"c0":null}`, want: nil},
		{name: "integer", input: `{"c0":1}`, render: `{"c0":1}`, want: 1},
		{name: "decimal", input: `{"c0":1.0}`, render: `{"c0":1.0}`, want: json.Number("1.0")},
		{name: "null", input: `{"c0":null}`, render: `{"c0":null}`, want: nil},
		{name: "boolean", input: `{"c0":true}`, render: `{"c0":true}`, want: true},
		{name: "string", input: `{"c0":"abc"}`, render: `{"c0":"abc"}`, want: "abc"},
		{name: "array", input: `{"c0":["abc",2]}`, render: `{"c0":["abc",2]}`, want: []any{"abc", 2}},
		{name: "nested-array", input: `{"c0":[["abc"],[5.0]]}`, render: `{"c0":[["abc"],[5.0]]}`, want: []any{[]any{"abc"}, []any{json.Number("5.0")}}},
		{name: "object", input: `{"c0":{"c1":10,"c2":"abc"}}`, render: `{"c0":{"c1":10,"c2":"abc"}}`, want: map[string]any{"c1": 10, "c2": "abc"}},
	}
	for _, test := range objectCases {
		t.Run("object/"+test.name, func(t *testing.T) {
			_, event := parseJSONParityEvent[jsonCoreObjectField](t, "JSONCoreObject", test.input)
			if !reflect.DeepEqual(event.Underlying().(jsonCoreObjectField).C0, test.want) {
				t.Fatalf("Object value = %#v, want %#v", event.Underlying(), test.want)
			}
			assertJSONParityRender(t, event, test.render)
		})
	}

	arrayCases := []string{
		`{}`, `{"c0":[]}`, `{"c0":[1.0]}`, `{"c0":[null]}`,
		`{"c0":[true]}`, `{"c0":[false]}`, `{"c0":["abc"]}`,
		`{"c0":[["abc"]]}`, `{"c0":[[]]}`,
		`{"c0":[["abc",2]]}`, `{"c0":[[["abc"],[5.0]]]}`,
		`{"c0":[{"c1":10}]}`, `{"c0":[{"c1":10,"c2":"abc"}]}`,
	}
	for _, input := range arrayCases {
		render := input
		if input == `{}` {
			render = `{"c0":null}`
		}
		t.Run("object-array/"+input, func(t *testing.T) {
			_, event := parseJSONParityEvent[jsonCoreObjectArrayField](t, "JSONCoreObjectArray", input)
			assertJSONParityRender(t, event, render)
		})
	}

	mapCases := []string{
		`{}`, `{"c0":{"c1":10}}`, `{"c0":{"c1":{"c2":20}}}`,
		`{"c0":{"c1":["c2",20]}}`,
	}
	for _, input := range mapCases {
		render := input
		if input == `{}` {
			render = `{"c0":null}`
		}
		t.Run("map/"+input, func(t *testing.T) {
			_, event := parseJSONParityEvent[jsonCoreMapField](t, "JSONCoreMap", input)
			assertJSONParityRender(t, event, render)
		})
	}
}

func TestJSONCoreNestedBigDecimalWriteParity(t *testing.T) {
	full := `{"libraryId":"L","isle":{"isleId":"I1","shelf":{"shelfId":"S11","book":{"bookId":"B111","price":20}}}}`
	_, event := parseJSONParityEvent[jsonCoreNestedLibrary](t, "JSONCoreNested", full)
	value := event.Underlying().(jsonCoreNestedLibrary)
	if value.LibraryID != "L" || value.Isle == nil || value.Isle.IsleID != "I1" || value.Isle.Shelf == nil || value.Isle.Shelf.ShelfID != "S11" || value.Isle.Shelf.Book == nil || value.Isle.Shelf.Book.BookID != "B111" || value.Isle.Shelf.Book.Price == nil || value.Isle.Shelf.Book.Price.Cmp(big.NewRat(20, 1)) != 0 {
		t.Fatalf("nested BigDecimal value = %#v", value)
	}
	assertJSONParityRender(t, event, full)

	for _, test := range []struct {
		input string
		want  string
	}{
		{input: `{"libraryId":"L","isle":null}`, want: `{"libraryId":"L","isle":null}`},
		{input: `{"libraryId":"L","isle":{"isleId":"I1","shelf":null}}`, want: `{"libraryId":"L","isle":{"isleId":"I1","shelf":null}}`},
		{input: `{"libraryId":"L","isle":{"isleId":"I1","shelf":{"shelfId":"S11","book":null}}}`, want: `{"libraryId":"L","isle":{"isleId":"I1","shelf":{"shelfId":"S11","book":null}}}`},
	} {
		_, nested := parseJSONParityEvent[jsonCoreNestedLibrary](t, "JSONCoreNested", test.input)
		assertJSONParityRender(t, nested, test.want)
	}
}

func TestJSONCoreNestedArrayWriteParity(t *testing.T) {
	input := `{"libraryId":"L1","isles":[{"isleId":"I1","shelfs":[{"shelfId":"S1","books":[{"bookId":"B1","price":10}]}]}]}`
	_, event := parseJSONParityEvent[jsonCoreNestedArrayLibrary](t, "JSONCoreNestedArray", input)
	value := event.Underlying().(jsonCoreNestedArrayLibrary)
	if len(value.Isles) != 1 || value.Isles[0] == nil || len(value.Isles[0].Shelves) != 1 || value.Isles[0].Shelves[0] == nil || len(value.Isles[0].Shelves[0].Books) != 1 || value.Isles[0].Shelves[0].Books[0] == nil || value.Isles[0].Shelves[0].Books[0].Price.Cmp(big.NewRat(10, 1)) != 0 {
		t.Fatalf("nested array value = %#v", value)
	}
	assertJSONParityRender(t, event, input)

	empty := `{"libraryId":"L2","isles":[]}`
	_, emptyEvent := parseJSONParityEvent[jsonCoreNestedArrayLibrary](t, "JSONCoreNestedArray", empty)
	assertJSONParityRender(t, emptyEvent, empty)
}
