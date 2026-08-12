package esper

import (
	"context"
	"reflect"
	"testing"
)

type variantInterfaceBaseAB interface {
	GetBaseAB() string
}

type variantInterfaceB interface {
	variantInterfaceBaseAB
	GetB() string
}

type variantInterfaceA interface {
	variantInterfaceBaseAB
	GetA() string
}

type variantInterfaceSuperG interface {
	variantInterfaceA
	GetG() string
}

type variantInterfaceSuperGPlus interface {
	variantInterfaceSuperG
	GetB() string
	GetC() string
}

type variantInterfaceBValue struct{}

func (variantInterfaceBValue) GetBaseAB() string { return "base-b" }
func (variantInterfaceBValue) GetB() string      { return "b" }

type variantInterfaceSuperGValue struct{}

func (variantInterfaceSuperGValue) GetBaseAB() string { return "base-g" }
func (variantInterfaceSuperGValue) GetA() string      { return "a" }
func (variantInterfaceSuperGValue) GetG() string      { return "g" }

type variantInterfaceSuperGPlusValue struct{}

func (variantInterfaceSuperGPlusValue) GetBaseAB() string { return "base-g-plus" }
func (variantInterfaceSuperGPlusValue) GetA() string      { return "a-plus" }
func (variantInterfaceSuperGPlusValue) GetG() string      { return "g-plus" }
func (variantInterfaceSuperGPlusValue) GetB() string      { return "b-plus" }
func (variantInterfaceSuperGPlusValue) GetC() string      { return "c-plus" }

type variantInterfaceMemberOne struct{}

func (variantInterfaceMemberOne) GetP0() variantInterfaceB {
	return variantInterfaceBValue{}
}

func (variantInterfaceMemberOne) GetP1() variantInterfaceSuperG {
	return variantInterfaceSuperGValue{}
}

type variantInterfaceMemberTwo struct{}

func (variantInterfaceMemberTwo) GetP0() variantInterfaceBaseAB {
	return variantInterfaceBValue{}
}

func (variantInterfaceMemberTwo) GetP1() variantInterfaceSuperGPlus {
	return variantInterfaceSuperGPlusValue{}
}

func TestVariantCommonPropertiesWidenInterfaceTypes(t *testing.T) {
	env := NewEnvironment()
	first, err := RegisterStruct[variantInterfaceMemberOne](env, "VariantInterfaceMemberOne", WithAccessorStyle(AccessorJavaBean))
	if err != nil {
		t.Fatal(err)
	}
	second, err := RegisterStruct[variantInterfaceMemberTwo](env, "VariantInterfaceMemberTwo", WithAccessorStyle(AccessorJavaBean))
	if err != nil {
		t.Fatal(err)
	}
	variant, err := RegisterVariant(env, "VariantInterfaceMembers", first, second)
	if err != nil {
		t.Fatal(err)
	}
	baseType := reflect.TypeOf((*variantInterfaceBaseAB)(nil)).Elem()
	superGType := reflect.TypeOf((*variantInterfaceSuperG)(nil)).Elem()
	p0, ok := variant.Field("p0")
	if !ok || p0.Type != baseType {
		t.Fatalf("variant p0 type = %#v, want %v", p0.Type, baseType)
	}
	p1, ok := variant.Field("p1")
	if !ok || p1.Type != superGType {
		t.Fatalf("variant p1 type = %#v, want %v", p1.Type, superGType)
	}

	plan, err := env.Build(FromAny(env, "VariantInterfaceMembers").Query(StatementName("variant-interface-members")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var received []Event
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			event, ok := result.Event()
			if !ok {
				t.Fatalf("variant interface result = %#v", result)
			}
			received = append(received, event)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	firstEvent, err := newEvent(first, variantInterfaceMemberOne{}, engine.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Route(context.Background(), "VariantInterfaceMembers", firstEvent); err != nil {
		t.Fatal(err)
	}
	secondEvent, err := newEvent(second, variantInterfaceMemberTwo{}, engine.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Route(context.Background(), "VariantInterfaceMembers", secondEvent); err != nil {
		t.Fatal(err)
	}
	if len(received) != 2 || received[0].TypeName() != "VariantInterfaceMemberOne" || received[1].TypeName() != "VariantInterfaceMemberTwo" {
		t.Fatalf("variant interface events = %#v", received)
	}
	if _, ok := received[0].Get("p0").Any().(variantInterfaceBaseAB); !ok {
		t.Fatalf("first p0 does not satisfy shared interface: %#v", received[0].Get("p0"))
	}
	if _, ok := received[1].Get("p0").Any().(variantInterfaceBaseAB); !ok {
		t.Fatalf("second p0 does not satisfy shared interface: %#v", received[1].Get("p0"))
	}
	if _, ok := received[0].Get("p1").Any().(variantInterfaceSuperG); !ok {
		t.Fatalf("first p1 does not satisfy shared interface: %#v", received[0].Get("p1"))
	}
	if _, ok := received[1].Get("p1").Any().(variantInterfaceSuperG); !ok {
		t.Fatalf("second p1 does not satisfy shared interface: %#v", received[1].Get("p1"))
	}
}
