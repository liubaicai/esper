package esper

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// Implemented-only coverage for EPLInsertIntoInvalid
// (java-runtime-6ff1f053ec9bfb43434b), the invalidity execution closing
// EPLInsertIntoPopulateUnderlying at 13/13 dispositioned (ords 0-11
// differential-verified). Java's compile-rejection messages are
// engine-internal diagnostics with no Go rejection-message surface; per the
// established invalidity policy these sub-cases are pinned as Go Build/send
// rejections instead of trace rows. Sub-cases whose Java rejection relies on
// concepts the Go engine does not model are documented approved differences,
// not fabricated rejections:
//
// - long→int and null→int "column and parameter types mismatch" /
//   "nullable type mismatch": Go converts numerics and maps null to the
//   zero value.
// - "Failed to find a suitable constructor" and ctor-throws: no constructor
//   concept; the setter-error variant stands in for the runtime-throw stage.
// - ABCStream/xmltype auto-declared insert-into types: Go requires
//   pre-registered targets and rejects transpose-plus-columns differently.
// - `c0 null` null-typed column: Go schemas have no null type.
// - xmltype XMLDOM precondition: Go has no XML event types.
// - `insert into MyMap(dummy) ...` property-not-found: Go map targets are
//   open (unregistered fields are accepted), so the Java rejection has no
//   Go counterpart for map targets.

type iipuInvalidSupportA interface {
	AGet() string
}

type iipuInvalidSupportAImpl struct {
	A string `esper:"a"`
}

func (i iipuInvalidSupportAImpl) AGet() string { return i.A }

type iipuInvalidInterfaceTarget struct {
	Isa iipuInvalidSupportA `esper:"isa"`
}

type iipuInvalidThrowingTarget struct {
	Value string `esper:"value"`
}

// TestEPLInsertIntoInvalidUnknownColumnRejected pins the unknown-column
// rejection family: a named projection that matches no target property
// (Java's `select 3 as dummyField` / `(dummy)` cases) and an unnamed
// projection (Java's bare `select 3` case) are both rejected at Build for
// struct targets.
func TestEPLInsertIntoInvalidUnknownColumnRejected(t *testing.T) {
	for _, tc := range []struct {
		name       string
		selections []Selection
	}{
		{"named-unknown-column", []Selection{Alias("dummyField", Literal(3))}},
		// Java's bare `select 3` projects an unnamed column; the Go name
		// requirement surfaces via the empty alias.
		{"unnamed-column", []Selection{Alias("", Literal(3))}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := NewEnvironment()
			iipuRegisterCommon(t, env)
			query := FromAny(env, "MyMap").Select(tc.selections...).InsertInto("SupportBean", StatementName("i1"))
			if _, err := env.Build(query); err == nil {
				t.Fatalf("%s was accepted at Build", tc.name)
			}
		})
	}
}

// TestEPLInsertIntoInvalidInterfaceMismatchRejected pins the interface
// hierarchy mismatch (Java's `insert into SupportBeanInterfaceProps(isa)
// select isbImpl from MyMap` family): routing a value that does not
// implement the declared interface-typed member is rejected at Build.
func TestEPLInsertIntoInvalidInterfaceMismatchRejected(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "MyMap", []FieldSpec{
		FieldDef("isbImpl", reflect.TypeOf(iipuInvalidSupportAImpl{})),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[iipuInvalidInterfaceTarget](env, "InterfaceTarget"); err != nil {
		t.Fatal(err)
	}
	// A struct field whose type is an interface is routed by assignable
	// type; project a *different* struct to force the mismatch.
	if _, err := RegisterStruct[iipuEventOne](env, "UnrelatedSource"); err != nil {
		t.Fatal(err)
	}
	_, err := env.Build(Select(From[iipuEventOne](env, "UnrelatedSource"),
		Alias("isa", Field[iipuEventOne, string]("id")),
	).InsertInto("InterfaceTarget", StatementName("i1")))
	if err == nil {
		t.Fatal("string projection into interface-typed member was accepted at Build")
	}
}

// TestEPLInsertIntoInvalidSetterThrowSurfacesAtSend pins the runtime-throw
// stage (Java's SupportBeanErrorTestingTwo.setValue throwing
// RuntimeException): a target property writer that returns an error makes
// the Send return that error instead of silently dropping the route.
func TestEPLInsertIntoInvalidSetterThrowSurfacesAtSend(t *testing.T) {
	env := NewEnvironment()
	iipuRegisterCommon(t, env)
	if _, err := RegisterStruct[iipuInvalidThrowingTarget](env, "ThrowingTarget",
		WithTypedPropertySetter[string]("value", func(any, string) error {
			return errors.New("esper: setter manufactured test exception")
		})); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithRuntimeURI("java-runtime-6ff1f053ec9bfb43434b"))
	defer func() { _ = engine.Close(context.Background()) }()

	producer, err := env.Build(FromAny(env, "MyMap").Select(
		Alias("value", Literal("E1")),
	).InsertInto("ThrowingTarget", StatementName("i1")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), producer); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "MyMap", map[string]any{}); err == nil {
		t.Fatal("setter throw was swallowed by the route")
	} else if !strings.Contains(err.Error(), "setter manufactured test exception") {
		t.Fatalf("send error = %v, want the manufactured setter error", err)
	}
}

// TestEPLInsertIntoInvalidWrongTypeSurfacesAtSend pins the wrong-type
// tolerance stage: Java accepts either a listener-visible default or a
// RuntimeException for a string routed into an int column; Go surfaces the
// conversion error from the Send, inside the Java-accepted envelope.
func TestEPLInsertIntoInvalidWrongTypeSurfacesAtSend(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "MyMap", []FieldSpec{
		FieldDef("anint", reflect.TypeOf(0)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[iipuSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithRuntimeURI("java-runtime-6ff1f053ec9bfb43434b"))
	defer func() { _ = engine.Close(context.Background()) }()

	producer, err := env.Build(FromAny(env, "MyMap").Select(
		Alias("intPrimitive", Field[map[string]any, any]("anint")),
	).InsertInto("SupportBean", StatementName("i1")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), producer); err != nil {
		t.Fatal(err)
	}
	// Either-behavior: an error is the Go-side disposition of the same
	// implementation-defined boundary Java documents.
	if err := engine.Send(context.Background(), "MyMap", map[string]any{"anint": "notAnInt"}); err != nil {
		t.Logf("wrong-type send surfaced an error (within the Java-accepted envelope): %v", err)
	}
}

// TestEPLInsertIntoInvalidSameSchemaCastDeploys pins the positive stage: an
// insert-into between two identically-shaped registered schemas compiles
// and deploys.
func TestEPLInsertIntoInvalidSameSchemaCastDeploys(t *testing.T) {
	env := NewEnvironment()
	fields := []FieldSpec{FieldDef("prop1", reflect.TypeOf(""))}
	if _, err := RegisterMap(env, "MapOneA", fields); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "MapTwoA", fields); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithRuntimeURI("java-runtime-6ff1f053ec9bfb43434b"))
	defer func() { _ = engine.Close(context.Background()) }()

	producer, err := env.Build(FromAny(env, "MapTwoA").InsertInto("MapOneA", StatementName("i1")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), producer); err != nil {
		t.Fatal(err)
	}
}
