package esper

import (
	"reflect"
	"testing"
	"time"
)

// Probe: how does RenderJSON handle programmatic values (no raw tree)?
type jsonAuditProbeChar struct {
	C rune     `json:"c"`
	D rune     `json:"d"`
	E *rune    `json:"e"`
	F float64  `json:"f"`
	G *float64 `json:"g"`
	H []rune   `json:"h"`
	I []*rune  `json:"i"`
}

type jsonAuditProbeNested struct {
	Local jsonAuditProbeChar `json:"local"`
}

func TestJSONAuditProbeProgrammaticRender(t *testing.T) {
	ch := rune('x')
	schema, err := NewJSONSchemaFor[jsonAuditProbeChar]("ProbeChar", nil)
	if err != nil {
		t.Fatal(err)
	}
	event, err := newEvent(schema, jsonAuditProbeChar{C: ch, D: rune(120), E: &ch, G: jsonAuditPtr(1.0), H: []rune{'a', 'b'}, I: []*rune{&ch}}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := RenderJSON(event)
	t.Logf("programmatic char render = %q err=%v", rendered, err)

	nested, err := NewJSONSchemaFor[jsonAuditProbeNested]("ProbeNested", nil)
	if err != nil {
		t.Fatal(err)
	}
	nestedEvent, err := newEvent(nested, jsonAuditProbeNested{Local: jsonAuditProbeChar{C: rune('n'), H: []rune{'x'}}}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	nestedRender, err := RenderJSON(nestedEvent)
	t.Logf("nested char render = %q err=%v", nestedRender, err)
}

// Probe adapter with pointer-declared field.
type jsonAuditProbeAdapterPtr struct {
	When *time.Time `json:"when"`
}

func TestJSONAuditProbeAdapterPointerField(t *testing.T) {
	dateAdapter := NewJSONFieldAdapter(func(text string) (time.Time, error) {
		return time.Parse("02-01-2006", text)
	}, func(value time.Time) (string, error) {
		return value.Format("02-01-2006"), nil
	})
	schema, err := NewJSONSchema("ProbeAdapterPtr", []FieldSpec{
		FieldDef("when", reflect.TypeOf((*time.Time)(nil))),
	}, WithJSONFieldAdapter("when", dateAdapter))
	if err != nil {
		t.Fatal(err)
	}
	event, err := ParseJSON(schema, []byte(`{"when":"22-09-2018"}`), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("adapter pointer parse: %v", err)
	}
	rendered, err := RenderJSON(event)
	t.Logf("adapter pointer render = %q err=%v", rendered, err)
}

func jsonAuditPtr[T any](value T) *T { return &value }
