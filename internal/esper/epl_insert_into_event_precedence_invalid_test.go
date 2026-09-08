package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Implemented-only coverage for EPLInsertIntoEventPrecInvalid
// (java-runtime-eb5b556ae1a0a4b15822), closing
// EPLInsertIntoEventPrecedence at 11/11 dispositioned (ords 0-9
// differential-verified). Java's compile-rejection messages are
// engine-internal diagnostics with no Go rejection-message surface; per the
// established invalidity policy these sub-cases are pinned as Go Build
// rejections instead of trace rows. The validation mirrors Java's
// validateEventPrecedence: an event-precedence expression must return an
// integer and is validated "considering only the result event itself and
// not incoming streams".
//
// Approved differences (documented, not fabricated as rejections):
// - sub-case 5 (FAF insert with event-precedence): already rejected with the
//   Go message "fire-and-forget routes do not allow event-precedence"
//   (route_test.go) — Java's message differs ("Fire-and-forget
//   insert-queries do not allow event-precedence") but the rejection exists.
// - sub-case 6 (insert into a table with event-precedence): Go rejects
//   inserting into an unregistered table target for a different reason
//   (route targets must be pre-registered); the precedence-specific table
//   rule has no separate Go surface.
// - sub-case 7 (reserved-keyword syntax error in on-merge): the typed fluent
//   API has no EPL text/parse phase; the malformed grammar is
//   unrepresentable.
// - Narrow Build-time leniencies: a nil-typed precedence expression passes
//   the integer check (the runtime coerces or yields 0), and
//   NullLiteral[int]() (an explicitly typed null of int type) passes where
//   Java rejects every null literal. Neither is reachable from the pinned
//   sub-cases.

// TestEPLInsertIntoEventPrecInvalidStringPrecedenceRejected covers invalid
// sub-case 1: a string-valued precedence expression is rejected at Build.
func TestEPLInsertIntoEventPrecInvalidStringPrecedenceRejected(t *testing.T) {
	env := NewEnvironment()
	epRegisterSupportBean(t, env)
	epRegisterOut(t, env)
	_, err := env.Build(Select(From[epSupportBean](env, "SupportBean"),
		Alias("id", Field[epSupportBean, string]("theString")),
	).InsertInto("Out", StatementName("i1"),
		EventPrecedence(Literal("a"))))
	if err == nil {
		t.Fatal("string-valued event-precedence was accepted at Build")
	}
	if !strings.Contains(err.Error(), "event-precedence expects an expression returning int") {
		t.Fatalf("error = %v, want the integer-type rejection", err)
	}
}

// TestEPLInsertIntoEventPrecInvalidForeignPropertyRejected covers invalid
// sub-case 2: a precedence property is validated considering only the
// output event type — a property of the incoming stream that does not exist
// on the routed target is rejected at Build.
func TestEPLInsertIntoEventPrecInvalidForeignPropertyRejected(t *testing.T) {
	env := NewEnvironment()
	epRegisterSupportBean(t, env)
	epRegisterOut(t, env)
	_, err := env.Build(Select(From[epSupportBean](env, "SupportBean"),
		Alias("id", Field[epSupportBean, string]("theString")),
	).InsertInto("Out", StatementName("i1"),
		EventPrecedence(Field[epSupportBean, int]("intPrimitive"))))
	if err == nil {
		t.Fatal("incoming-stream property in event-precedence was accepted at Build")
	}
	if !strings.Contains(err.Error(), `"intPrimitive" is not a property of the output event type "Out"`) {
		t.Fatalf("error = %v, want the output-event property rejection", err)
	}
}

// TestEPLInsertIntoEventPrecInvalidNullPrecedenceRejected covers invalid
// sub-case 3: a null-typed precedence expression is rejected at Build (the
// Go null literal's pointer type is not the integer type Java requires).
func TestEPLInsertIntoEventPrecInvalidNullPrecedenceRejected(t *testing.T) {
	env := NewEnvironment()
	epRegisterSupportBean(t, env)
	epRegisterOut(t, env)
	_, err := env.Build(OnEvent(From[epSupportBean](env, "SupportBean")).SplitAll(
		SplitIntoWithPrecedence(NullLiteral[*int](), "Out", Alias("id", Literal("a"))),
	).Query(StatementName("i1")))
	if err == nil {
		t.Fatal("null-typed event-precedence was accepted at Build")
	}
	if !strings.Contains(err.Error(), "event-precedence expects an expression returning int") {
		t.Fatalf("error = %v, want the integer-type rejection", err)
	}
}

// TestEPLInsertIntoEventPrecInvalidMergeShortPrecedenceRejected covers
// invalid sub-case 4: a merge-action precedence whose expression returns a
// non-int numeric type (Java's cast(1, short) returning Short) is rejected
// at Build with the clause-scoped wrapper.
func TestEPLInsertIntoEventPrecInvalidMergeShortPrecedenceRejected(t *testing.T) {
	env := NewEnvironment()
	epRegisterSupportBean(t, env)
	epRegisterOut(t, env)
	windowSchema, err := NewMapSchema("MyWindow", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindow", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	_, err = env.Build(OnEvent(From[epSupportBean](env, "SupportBean")).
		MergeIntoNamedWindowWhen("MyWindow", nil,
			WhenNotMatchedActions(
				ThenInsertIntoWithPrecedence(Literal(int16(1)), "Out", Alias("id", Literal("a"))),
			)).Query())
	if err == nil {
		t.Fatal("short-typed merge precedence was accepted at Build")
	}
	if !strings.Contains(err.Error(), "named-window merge clause 0 action 0") ||
		!strings.Contains(err.Error(), "event-precedence expects an expression returning int") {
		t.Fatalf("error = %v, want the clause-scoped integer-type rejection", err)
	}
}

// TestEPLInsertIntoEventPrecInvalidTableMergePrecedenceRejected covers the
// table-merge application of the same Java rule (InfraOnMergeHelperForge
// validates insert actions for tables and named windows alike): a
// string-typed precedence on a table-merge not-matched insert is rejected
// at Build with the clause wrapper.
func TestEPLInsertIntoEventPrecInvalidTableMergePrecedenceRejected(t *testing.T) {
	env := NewEnvironment()
	epRegisterSupportBean(t, env)
	epRegisterOut(t, env)
	if _, err := CreateTable(env, "MyTable", []TableColumn{
		PrimaryKeyColumn[string]("id"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, buildErr := env.Build(OnRecord(FromAny(env, "Out")).
		MergeIntoTableWhen("MyTable", []Expr{Field[map[string]any, string]("id")},
			WhenNotMatchedActions(
				ThenInsertIntoWithPrecedence(Literal("x"), "Out", Alias("id", Literal("a"))),
			)).Query()); buildErr == nil {
		t.Fatal("string-typed table-merge precedence was accepted at Build")
	} else if !strings.Contains(buildErr.Error(), "table merge clause 0 action 0") ||
		!strings.Contains(buildErr.Error(), "event-precedence expects an expression returning int") {
		t.Fatalf("error = %v, want the clause-scoped integer-type rejection", buildErr)
	}
}

// TestEPLInsertIntoEventPrecInvalidFAFRejected pins invalid sub-case 5 in
// this class's context: an FAF insert carrying event-precedence into a
// named window is rejected (the rejection itself is covered by
// route_test; asserted here against the parity scenario's shape).
func TestEPLInsertIntoEventPrecInvalidFAFRejected(t *testing.T) {
	env := NewEnvironment()
	epRegisterOut(t, env)
	windowSchema, err := NewMapSchema("MyWindow", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindow", windowSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	plan, err := env.Build(FromAny(env, "Out").InsertInto("MyWindow", StatementName("faf"),
		EventPrecedence(Literal(10))))
	if err != nil {
		t.Fatal(err)
	}
	result := QueryResult{Batch: ResultBatch{New: []Result{
		resultRow(newRow(plan.resultSchema, []Value{Present("A")})),
	}}}
	if err := engine.RouteFireAndForget(context.Background(), plan, result); err == nil ||
		!strings.Contains(err.Error(), "fire-and-forget routes do not allow event-precedence") {
		t.Fatalf("FAF error = %v, want the event-precedence rejection", err)
	}
}
