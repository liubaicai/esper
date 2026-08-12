package esper

import (
	"reflect"
	"testing"
	"time"
)

type jsonUnderlyingTrace struct {
	payload string
	want    map[string]any
}

func TestJSONUnderlyingMapRepresentationMatchesEsper(t *testing.T) {
	intType := reflect.TypeOf(int(0))
	run := func(name string, dynamic bool, fields []FieldSpec, traces []jsonUnderlyingTrace) {
		t.Run(name, func(t *testing.T) {
			options := []SchemaOption(nil)
			if dynamic {
				options = append(options, AllowDynamicFields())
			}
			schema, err := NewJSONSchema("JSONUnderlying"+name, fields, options...)
			if err != nil {
				t.Fatal(err)
			}
			for index, trace := range traces {
				event, parseErr := ParseJSON(schema, []byte(trace.payload), time.Unix(int64(index), 0).UTC())
				if parseErr != nil {
					t.Fatalf("trace %d parse error: %v", index, parseErr)
				}
				underlying, ok := event.Underlying().(map[string]any)
				if !ok {
					t.Fatalf("trace %d underlying type = %T, want map[string]any", index, event.Underlying())
				}
				if !reflect.DeepEqual(underlying, trace.want) {
					t.Fatalf("trace %d underlying = %#v, want %#v", index, underlying, trace.want)
				}
			}
		})
	}

	declaredOne := []FieldSpec{FieldDef("a", intType)}
	declaredTwo := []FieldSpec{FieldDef("a", intType), FieldDef("b", intType)}
	run("StaticZeroDeclared", false, nil, []jsonUnderlyingTrace{
		{`{"a":1,"b":2,"c":3}`, map[string]any{}},
	})
	run("StaticOneDeclared", false, declaredOne, []jsonUnderlyingTrace{
		{`{"a":1,"b":2,"c":3}`, map[string]any{"a": 1}},
		{`{"a":10}`, map[string]any{"a": 10}},
		{`{}`, map[string]any{"a": nil}},
	})
	run("StaticTwoDeclared", false, declaredTwo, []jsonUnderlyingTrace{
		{`{"a":1,"b":2,"c":3}`, map[string]any{"a": 1, "b": 2}},
		{`{"a":10}`, map[string]any{"a": 10, "b": nil}},
		{`{}`, map[string]any{"a": nil, "b": nil}},
	})
	run("DynamicZeroDeclared", true, nil, []jsonUnderlyingTrace{
		{`{"a":1,"b":2,"c":3}`, map[string]any{"a": 1, "b": 2, "c": 3}},
		{`{"a":10}`, map[string]any{"a": 10}},
		{`{"a":null,"c":101,"d":102}`, map[string]any{"a": nil, "c": 101, "d": 102}},
		{`{}`, map[string]any{}},
	})
	run("DynamicOneDeclared", true, declaredOne, []jsonUnderlyingTrace{
		{`{"a":1,"b":2,"c":3}`, map[string]any{"a": 1, "b": 2, "c": 3}},
		{`{"a":10}`, map[string]any{"a": 10}},
		{`{"a":null,"c":101,"d":102}`, map[string]any{"a": nil, "c": 101, "d": 102}},
		{`{}`, map[string]any{"a": nil}},
	})
	run("DynamicTwoDeclared", true, declaredTwo, []jsonUnderlyingTrace{
		{`{"a":1,"b":2,"c":3}`, map[string]any{"a": 1, "b": 2, "c": 3}},
		{`{"a":10}`, map[string]any{"a": 10, "b": nil}},
		{`{"a":null,"c":101,"d":102}`, map[string]any{"a": nil, "b": nil, "c": 101, "d": 102}},
		{`{}`, map[string]any{"a": nil, "b": nil}},
	})
}
