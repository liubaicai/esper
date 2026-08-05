package esper

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRenderJSONPreservesSchemaFieldsNestedValuesAndNull(t *testing.T) {
	schema, err := NewJSONSchema("Outer", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("price", reflect.TypeOf(float64(0))),
		FieldDef("nullable", reflect.TypeOf("")),
		FieldDef("items", reflect.TypeOf([]any{})),
	}, AllowDynamicFields())
	if err != nil {
		t.Fatal(err)
	}
	event, err := newEvent(schema, map[string]any{
		"symbol":   "ESPER",
		"price":    12.5,
		"nullable": nil,
		"items": []any{
			map[string]any{"id": "A", "tags": []string{"x", "y"}},
		},
		"dynamic": "kept",
	}, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := RenderJSON(event, WithJSONTitle("outer"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(encoded), &decoded); err != nil {
		t.Fatalf("rendered JSON is invalid: %v; %s", err, encoded)
	}
	outer, ok := decoded["outer"].(map[string]any)
	if !ok || outer["symbol"] != "ESPER" || outer["price"] != 12.5 || outer["nullable"] != nil || outer["dynamic"] != "kept" {
		t.Fatalf("rendered JSON object = %#v", decoded)
	}
	items, ok := outer["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("rendered nested items = %#v", outer["items"])
	}
	if _, err := RenderJSON(event, WithJSONMaxDepth(0)); err == nil {
		t.Fatal("non-positive JSON depth should be rejected")
	}
}

func TestRenderXMLEscapesNestedArraysAndAttributes(t *testing.T) {
	schema, err := NewMapSchema("Outer", []FieldSpec{
		FieldDef("symbol", reflect.TypeOf("")),
		FieldDef("items", reflect.TypeOf([]any{})),
		FieldDef("nullable", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	event, err := newEvent(schema, map[string]any{
		"symbol": "A&B",
		"items": []any{
			map[string]any{"name": "x<y"},
			map[string]any{"name": "z>q"},
		},
		"nullable": nil,
	}, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := RenderXML(event, WithXMLTitle("outer"), WithXMLIndent(""))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<outer>`,
		`<symbol>A&amp;B</symbol>`,
		`<items><name>x&lt;y</name></items>`,
		`<items><name>z&gt;q</name></items>`,
		`<nullable></nullable>`,
		`</outer>`,
	} {
		if !strings.Contains(encoded, fragment) {
			t.Fatalf("XML %q missing %q", encoded, fragment)
		}
	}
	withAttributes, err := RenderXML(event, WithXMLDefaultAsAttribute(true), WithXMLIndent(""))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(withAttributes, `<Outer symbol="A&amp;B" nullable="">`) {
		t.Fatalf("scalar fields should render as attributes: %s", withAttributes)
	}
}

func TestRenderJSONAndXMLRejectTooDeepPayload(t *testing.T) {
	schema, err := NewMapSchema("Deep", []FieldSpec{FieldDef("value", reflect.TypeOf(any(nil)))})
	if err != nil {
		t.Fatal(err)
	}
	deep := map[string]any{}
	current := deep
	for index := 0; index < 4; index++ {
		next := map[string]any{}
		current["next"] = next
		current = next
	}
	event, err := newEvent(schema, map[string]any{"value": deep}, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RenderJSON(event, WithJSONMaxDepth(2)); err == nil {
		t.Fatal("deep JSON payload should be rejected")
	}
	if _, err := RenderXML(event, WithXMLMaxDepth(2)); err == nil {
		t.Fatal("deep XML payload should be rejected")
	}
}
