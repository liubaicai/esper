// Pattern-source transpose boundary tests: the transpose selection in a
// pattern select routes the tagged event's underlying into the insert-into
// target (Java's `a.*` over `from pattern [...]`). These tests pin the
// rejection boundaries around that surface.

package esper

import (
	"strings"
	"testing"
)

func patternProbe(t *testing.T, sel []Selection, route string) error {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[epSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[epSupportBean](env, "BeanTarget"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "MapTarget", []FieldSpec{FieldDef("abc", nil)}); err != nil {
		t.Fatal(err)
	}
	p := PatternFrom(From[epSupportBean](env, "SupportBean"), "a", Literal(true)).Every().Select(sel...)
	var q Query
	if route != "" {
		q = p.InsertInto(route)
	} else {
		q = p.Query()
	}
	_, err := env.Build(q)
	return err
}

func TestPatternTransposeNilRejected(t *testing.T) {
	err := patternProbe(t, []Selection{{Expr: Transpose[Event](nil)}}, "BeanTarget")
	if err == nil || !strings.Contains(err.Error(), "single parameter") {
		t.Fatalf("err = %v", err)
	}
	err = patternProbe(t, []Selection{{Expr: Transpose[Event](nil)}}, "")
	if err == nil || !strings.Contains(err.Error(), "single parameter") {
		t.Fatalf("no-route err = %v", err)
	}
}

func TestTransposeNullPayloadRejected(t *testing.T) {
	// Pattern selects reject a null transpose via the pattern-event-tag
	// requirement; the plain-select route path must reject it via the
	// restored null-type check.
	err := patternProbe(t, []Selection{{Expr: Transpose[Event](NullLiteral[Event]())}}, "BeanTarget")
	if err == nil {
		t.Fatal("pattern null transpose accepted")
	}
	env := NewEnvironment()
	if _, err := RegisterStruct[epSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[epSupportBean](env, "BeanTarget2"); err != nil {
		t.Fatal(err)
	}
	_, err = env.Build(Select(From[epSupportBean](env, "SupportBean"), Selection{Expr: Transpose[epSupportBean](NullLiteral[epSupportBean]())}).InsertInto("BeanTarget2"))
	if err == nil || !strings.Contains(err.Error(), "null-type") {
		t.Fatalf("plain-select null transpose err = %v", err)
	}
}

func TestTransposeEventPayloadPlainSelectRejected(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[epSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "MapTarget", nil); err != nil {
		t.Fatal(err)
	}
	_, err := env.Build(Select(From[epSupportBean](env, "SupportBean"), Selection{Expr: Transpose[Event](EventValue[Event]())}).InsertInto("MapTarget"))
	if err == nil {
		t.Fatal("Transpose(EventValue) in plain select accepted")
	}
	_, err = env.Build(Select(From[epSupportBean](env, "SupportBean"), Selection{Expr: Transpose[Event](PatternEvent("a"))}).InsertInto("MapTarget"))
	if err == nil {
		t.Fatal("Transpose(PatternEvent) in plain select accepted")
	}
}

func TestPatternTransposeCompanionWithoutRouteRejected(t *testing.T) {
	err := patternProbe(t, []Selection{
		{Expr: Transpose[Event](PatternEvent("a"))},
		{Name: "abc", Expr: Literal("abc")},
	}, "")
	if err == nil {
		t.Fatal("companion without route accepted")
	}
	err = patternProbe(t, []Selection{
		{Expr: Transpose[Event](PatternEvent("a"))},
		{Expr: Transpose[Event](PatternEvent("a"))},
	}, "")
	if err == nil {
		t.Fatal("two transposes without route accepted")
	}
	err = patternProbe(t, []Selection{
		{Name: "evt", Expr: Transpose[Event](PatternEvent("a"))},
	}, "BeanTarget")
	if err == nil {
		t.Fatal("named transpose accepted")
	}
}
