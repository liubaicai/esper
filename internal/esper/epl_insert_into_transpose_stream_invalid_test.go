package esper

import (
	"errors"
	"strings"
	"testing"
)

// Build-time rejection for the two INVALIDITY executions of
// EPLInsertIntoTransposeStream (Esper 9.0.0, commit 9e1b9f1c):
//
//   - execution 5 EPLInsertIntoTransposeSingleColumnInsertInvalid (runtime
//     java-runtime-699333da3ee37518415a)
//   - execution 9 EPLInsertIntoInvalidTranspose (runtime
//     java-runtime-d0c4e373409e2703b6ef)
//
// Java flags these executions INVALIDITY, so their protocol has no trace
// step; they are registered as implemented (not differential-verified) and
// the Go equivalents are Go-unit build-error tests below. The messages follow
// the runbook freeze: type-coercion errors are rejected at Build
// (plan.go validateRoute + validateTransposeExpression), and a top-level
// transpose in a non-route select is validated by validateTransposeSelectShape
// (plan.go), which now matches Java's rejection of transpose(null).
// The transpose(null) message is implemented in plan.go but has no typed Go
// API surface that can construct it (Transpose takes exactly one argument, so
// the null-type child is unreachable); coverage for the rejected branch is at
// the mechanism level only. The Java "e2.* as event" unwrap rule likewise has
// no Go statement shape and is documented below as an approved API-surface
// difference.

// assertTransposeInvalidBuild asserts that a query is rejected at Build with
// one of the route/projection error codes and that the error message contains
// every fragment.
func assertTransposeInvalidBuild(t *testing.T, env *Environment, name string, query Query, fragments ...string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		_, err := env.Build(query)
		if err == nil {
			t.Fatalf("invalid transpose %s was accepted", name)
		}
		if !errors.Is(err, ErrorInvalidRule) && !errors.Is(err, ErrorUnknownName) && !errors.Is(err, ErrorTypeMismatch) {
			t.Fatalf("invalid transpose %s error = %v, want InvalidRule/UnknownName/TypeMismatch", name, err)
		}
		for _, fragment := range fragments {
			if !strings.Contains(err.Error(), fragment) {
				t.Fatalf("invalid transpose %s error = %v, missing fragment %q", name, err, fragment)
			}
		}
	})
}

// transposeInvalidCustomOne is the typed payload function mirroring Java's
// customOne('O', 10): both valid (exec 4) and invalid (exec 5/9) transpose
// statements call it.
func transposeInvalidCustomOne(name string) Expr {
	return Transpose[*epSupportBean](Func2[string, int, *epSupportBean](name, func(s string, i int) *epSupportBean {
		return &epSupportBean{TheString: s, IntPrimitive: i}
	}, Literal("O"), Literal(10)))
}

// TestEPLInsertIntoTransposeSingleColumnInsertInvalidBuild covers the
// build-time rejections of EPLInsertIntoTransposeSingleColumnInsertInvalid.
// Java registers SupportBean and SupportBeanNumeric bean event types and a
// map schema SomeOtherStream(); the Go equivalents register the matching Go
// struct/map schemas.
func TestEPLInsertIntoTransposeSingleColumnInsertInvalidBuild(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[epSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[filterBeanNumeric](env, "SupportBeanNumeric"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "SomeOtherStream", []FieldSpec{}); err != nil {
		t.Fatal(err)
	}

	target := func(name string, transpose Expr) Query {
		return Select(
			From[epSupportBean](env, "SupportBean"),
			Selection{Name: "col", Expr: transpose},
		).InsertInto(name, StatementName("s0"))
	}

	// invalid wrong-bean target: insert into SupportBeanNumeric select
	// transpose(customOne('O', 10)) from SupportBean. Java: "Expression-
	// returned value of type '<SupportBean>' cannot be converted to target
	// event type 'SupportBeanNumeric' with underlying type
	// '<SupportBeanNumeric>'".
	assertTransposeInvalidBuild(t, env, "wrong-bean-target",
		target("SupportBeanNumeric", transposeInvalidCustomOne("customOne")),
		"cannot be converted to target event type \"SupportBeanNumeric\"",
		"esper.filterBeanNumeric")

	// invalid additional properties: insert into SupportBean select 1 as
	// dummy, transpose(...) from SupportBean. Java: "Cannot transpose
	// additional properties in the select-clause to target event type
	// 'SupportBean' ... the transpose function must occur alone in the select
	// clause".
	assertTransposeInvalidBuild(t, env, "additional-properties",
		Select(
			From[epSupportBean](env, "SupportBean"),
			Alias("dummy", Literal(1)),
			Selection{Name: "col", Expr: transposeInvalidCustomOne("customOne")},
		).InsertInto("SupportBean", StatementName("s0")),
		"cannot transpose additional properties in the select-clause",
		"the transpose function must occur alone in the select clause")

	// invalid occurs twice: select transpose(...), transpose(...). Java:
	// "A column name must be supplied for all but one stream if multiple
	// streams are selected via the stream.* notation".
	assertTransposeInvalidBuild(t, env, "transpose-twice",
		Select(
			From[epSupportBean](env, "SupportBean"),
			Selection{Name: "c1", Expr: transposeInvalidCustomOne("one")},
			Selection{Name: "c2", Expr: transposeInvalidCustomOne("two")},
		).InsertInto("SupportBean", StatementName("s0")),
		"a column name must be supplied for all but one stream")

	// invalid wrong-type target: insert into SomeOtherStream (a map schema)
	// select transpose(...) from SupportBean. Java: the same
	// "Expression-returned value ... cannot be converted" message with
	// underlying type 'java.util.Map'.
	assertTransposeInvalidBuild(t, env, "wrong-map-target",
		target("SomeOtherStream", transposeInvalidCustomOne("customOne")),
		"cannot be converted to target event type \"SomeOtherStream\"",
		"map[string]interface {}")

	// invalid two parameters: select transpose(customOne('O', 10),
	// customOne('O', 10)) from SupportBean. Java: "The transpose function
	// requires a single parameter expression". Go's Transpose takes a single
	// generic argument, so the two-parameter shape is only expressible as the
	// no-parameter Transpose(nil), which hits the same single-parameter
	// message in both the route and plain-select paths.
	assertTransposeInvalidBuild(t, env, "two-parameters",
		Select(
			From[epSupportBean](env, "SupportBean"),
			Selection{Name: "t", Expr: Transpose[any](nil)},
		).Query(StatementName("s0")),
		"transpose function requires a single parameter expression")

	// transpose not a top-level function is valid: used in a where-clause is
	// possible but not useful. Java: select * from SupportBean where
	// transpose(customOne('O', 10)) is not null compiles and deploys.
	notNull := Not(IsNull[Event](Transpose[*epSupportBean](
		Func2[string, int, *epSupportBean]("customOne", func(s string, i int) *epSupportBean {
			return &epSupportBean{TheString: s, IntPrimitive: i}
		}, Literal("O"), Literal(10)))))
	if _, err := env.Build(From[epSupportBean](env, "SupportBean").
		Filter(notNull).Query(StatementName("where-transpose-is-not-null"))); err != nil {
		t.Fatalf("where-clause transpose is not null was rejected: %v", err)
	}
	// select transpose(customOne(...)) is not null from SupportBean also
	// compiles in Java; the non-top-level transpose selection is valid.
	if _, err := env.Build(Select(
		From[epSupportBean](env, "SupportBean"),
		Selection{Name: "tr", Expr: notNull},
	).Query(StatementName("select-transpose-is-not-null"))); err != nil {
		t.Fatalf("select transpose is not null was rejected: %v", err)
	}

	// invalid insert of object-array into an undefined stream: insert into
	// SomeOther select transpose(generateOA('a', 1)) from SupportBean. Java
	// auto-creates the target from the transpose and rejects the Object[]
	// return type ("Invalid expression return type 'Object[]' for transpose
	// function"). The Go typed API requires a registered route target, so the
	// unregistered "SomeOther" is rejected before payload coercion with
	// ErrorUnknownName. The runtime-only part of the Java rule (transpose of
	// an Object[] into a registered OA target) is covered by the exec 1 oa
	// parity test.
	assertTransposeInvalidBuild(t, env, "object-array-undefined-target",
		Select(
			From[epSupportBean](env, "SupportBean"),
			Selection{Name: "col", Expr: Transpose[[]any](
				Func2[string, int, []any]("generateOA", func(s string, i int) []any {
					values := make([]any, 2)
					values[0] = s
					values[1] = i
					return values
				}, Literal("a"), Literal(1))),
			}).InsertInto("SomeOther", StatementName("s0")),
		"route target \"SomeOther\" is not registered")

	// insert into SomeOtherStream select transpose(customOne) is invalid
	// because SomeOtherStream is a pre-registered Map schema and the payload
	// is a struct: covered by wrong-map-target above.
}

// TestEPLInsertIntoInvalidTransposeBuild covers the build-time rejections of
// EPLInsertIntoInvalidTranspose.
func TestEPLInsertIntoInvalidTransposeBuild(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[epSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}

	// invalid unwrap-properties: create schema E1/E2/EnrichedE2 then insert
	// into EnrichedE2 select e2.* as event, ... . Java rejects the e2.*
	// unwrap with "The 'e2.* as event' syntax is not allowed when inserting
	// into an existing bean event type, use the 'e2 as event' syntax
	// instead". The Go typed chain API has no e2.* unwrap form (an event is
	// projected whole via SelectSourceEvent/EventValue under a name), so
	// there is no Go statement shape that reaches this Java diagnostic;
	// the Java rule is documented as an approved API-surface difference.
	// The valid e2-as-event form this error points to is exercised by the
	// exec 7 join-POJO parity test (MyStream2Bean projects the source events
	// under a and b).
}
