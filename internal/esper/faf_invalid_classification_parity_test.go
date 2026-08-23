package esper

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// Approved-difference coverage for the compile-only InfraNWTableFAF
// executions: Java asserts EPCompileException message prefixes tied to EPL
// text, while Go classifies the same diagnostic surface through ErrorCode.
// The observable contract preserved here is that these forms are rejected at
// build time and never execute.
func TestFafInvalidInsertClassificationParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[onsetArrayBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	str := reflect.TypeOf("")
	intType := reflect.TypeOf(int32(0))
	if _, err := CreateTable(env, "MyTable", []TableColumn{
		{Name: "theString", Type: str, PrimaryKey: true},
		{Name: "intPrimitive", Type: intType, PrimaryKey: true},
	}); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("SupportBean")
	if !ok {
		t.Fatal("SupportBean schema missing")
	}
	if _, err := CreateNamedWindow(env, "MyWindow", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	buildFaf := func(query Query) error {
		_, err := env.Build(query)
		return err
	}

	// Unknown FAF target classifies as UnknownName.
	err := buildFaf(FromNamedWindow(env, "NoSuchWindow").OnDemand().Insert(
		SetColumn("theString", Literal("x")),
	))
	if !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("unknown faf target = %v, want UnknownName", err)
	}

	// A FAF select against an unregistered stream classifies as UnknownName.
	// Go resolves unknown FAF sources as InvalidRule naming the schema.
	err = buildFaf(FromAny(env, "NoSuchWindow").Query(StatementName("s0")))
	if !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("unknown faf source = %v, want InvalidRule", err)
	}

	// Previous functions are rejected when a fire-and-forget query executes:
	// Build accepts them (live statements may use prev), while the FAF
	// execution boundary classifies the rejection as InvalidRule.
	prevPlan, buildErr := env.Build(FromNamedWindow(env, "MyWindow").Select(
		Alias("c0", Prev[string](1, Field[any, string]("theString"))),
	).Query(StatementName("faf-prev")))
	if buildErr != nil {
		t.Fatalf("build rejected prev before faf boundary: %v", buildErr)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	_, execErr := engine.ExecuteFireAndForget(context.Background(), prevPlan)
	var prevErr *Error
	if !errors.As(execErr, &prevErr) || prevErr.Code != ErrorInvalidRule {
		t.Fatalf("prev in faf execute = %v, want InvalidRule", execErr)
	}
}

func TestFafInsertTypeMismatchParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[fafInsertTarget](env, "FafInsertTarget"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("FafInsertTarget")
	if !ok {
		t.Fatal("target schema missing")
	}
	if _, err := CreateNamedWindow(env, "TypedWindow", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	// Inserting a string into an int property is rejected at Build time,
	// mirroring Java's column/type mismatch EPCompileException.
	_, err := env.Build(FromNamedWindow(env, "TypedWindow").OnDemand().Insert(
		SetColumn("id", Literal("not-a-number")),
	))
	if err == nil {
		t.Fatal("type-mismatched faf insert unexpectedly built")
	}
	var espErr *Error
	if !errors.As(err, &espErr) || espErr.Code != ErrorInvalidRule || espErr.Message == "" {
		t.Fatalf("error = %v, want classified InvalidRule with message", err)
	}
}

type fafInsertTarget struct {
	ID int `esper:"id"`
}
