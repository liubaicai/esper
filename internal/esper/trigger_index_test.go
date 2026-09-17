package esper

import (
	"context"
	"reflect"
	"testing"
)

type triggerIndexBean struct {
	TheString       string   `esper:"theString"`
	IntPrimitive    int64    `esper:"intPrimitive"`
	IntBoxed        *int64   `esper:"intBoxed"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
}

// TestNamedWindowTriggerImplicitIndexLifecycle pins the observable index
// lifecycle Java exposes through assertIndexCount: equality conjuncts infer
// hash props keyed by coercion type, conjunct order is significant, shared
// specs are ref-counted across statements, and undeploying the last owner
// drops the index.
func TestNamedWindowTriggerImplicitIndexLifecycle(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[triggerIndexBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer engine.Close(context.Background())
	ctx := context.Background()

	schema, err := NewMapSchema("MyWindowCKSchema", []FieldSpec{
		FieldDef("theString", reflect.TypeOf("")),
		FieldDef("intPrimitive", reflect.TypeOf(int64(0))),
		FieldDef("intBoxed", reflect.TypeOf((*int64)(nil))),
		FieldDef("doublePrimitive", reflect.TypeOf(float64(0))),
		FieldDef("doubleBoxed", reflect.TypeOf((*float64)(nil))),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindowCK", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	window, ok := engine.NamedWindow("MyWindowCK")
	if !ok {
		t.Fatal("window missing")
	}
	if n := window.IndexCount(); n != 0 {
		t.Fatalf("initial IndexCount = %d", n)
	}
	src := From[triggerIndexBean](env, "SupportBean")
	mk := func(name string, pred Expression[bool]) *Deployment {
		plan, err := env.Build(OnEvent(src.Filter(Equal[string](Field[triggerIndexBean, string]("theString"), Literal(name)))).
			DeleteFromNamedWindow("MyWindowCK", pred).Query(StatementName(name)))
		if err != nil {
			t.Fatal(err)
		}
		d, err := engine.Deploy(ctx, plan)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	d1 := mk("d1", EqualOf(NamedWindowField[int64]("intPrimitive"), Field[triggerIndexBean, *float64]("doubleBoxed")))
	if n := window.IndexCount(); n != 1 {
		t.Fatalf("after d1 IndexCount = %d, want 1", n)
	}
	d2 := mk("d2", EqualOf(NamedWindowField[int64]("intPrimitive"), Field[triggerIndexBean, float64]("doublePrimitive")))
	if n := window.IndexCount(); n != 1 {
		t.Fatalf("after d2 IndexCount = %d, want 1 (reuse)", n)
	}
	d3 := mk("d3", EqualOf(NamedWindowField[int64]("intPrimitive"), Field[triggerIndexBean, *int64]("intBoxed")))
	if n := window.IndexCount(); n != 2 {
		t.Fatalf("after d3 IndexCount = %d, want 2", n)
	}
	d4 := mk("d4", And(
		EqualOf(NamedWindowField[int64]("intPrimitive"), Field[triggerIndexBean, int64]("intPrimitive")),
		EqualOf(NamedWindowField[float64]("doublePrimitive"), Field[triggerIndexBean, float64]("doublePrimitive"))))
	if n := window.IndexCount(); n != 3 {
		t.Fatalf("after d4 IndexCount = %d, want 3", n)
	}
	d5 := mk("d5", And(
		EqualOf(NamedWindowField[float64]("doublePrimitive"), Field[triggerIndexBean, float64]("doublePrimitive")),
		EqualOf(NamedWindowField[int64]("intPrimitive"), Field[triggerIndexBean, int64]("intPrimitive"))))
	if n := window.IndexCount(); n != 4 {
		t.Fatalf("after d5 IndexCount = %d, want 4 (order matters)", n)
	}
	// undeploy d1: shared index survives (d2 still refs) → count stays 4
	if err := d1.Undeploy(ctx); err != nil {
		t.Fatal(err)
	}
	if n := window.IndexCount(); n != 4 {
		t.Fatalf("after undeploy d1 IndexCount = %d, want 4", n)
	}
	for _, d := range []*Deployment{d2, d3, d4, d5} {
		if err := d.Undeploy(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if n := window.IndexCount(); n != 0 {
		t.Fatalf("after undeploy all IndexCount = %d, want 0", n)
	}
}
