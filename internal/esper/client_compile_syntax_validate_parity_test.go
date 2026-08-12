package esper

import (
	"errors"
	"strings"
	"testing"
)

type clientCompileSyntaxUnknownEvent struct{}

func TestClientCompileOptionsValidateOnlyMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	unknown := From[clientCompileSyntaxUnknownEvent](env, "NoSuchEvent").Query()
	if err := env.ValidateSyntax(unknown); err != nil {
		t.Fatalf("syntax-only validation resolved an unknown event type: %v", err)
	}
	if _, err := env.Build(unknown); err == nil || !strings.Contains(err.Error(), "NoSuchEvent") {
		t.Fatalf("semantic build of unknown event type error = %v", err)
	}
	if err := env.ValidateSyntax(); err != nil {
		t.Fatalf("empty typed syntax module: %v", err)
	}

	invalid := []struct {
		name  string
		query Query
		text  string
	}{
		{name: "empty query", query: Query{env: env}, text: "no source"},
		{name: "empty object model", query: SelectOnce(env), text: "select expression"},
	}
	for _, test := range invalid {
		err := env.ValidateSyntax(test.query)
		if err == nil || !errors.Is(err, ErrorInvalidRule) || !strings.Contains(err.Error(), test.text) {
			t.Fatalf("%s error = %v", test.name, err)
		}
	}
}

func TestClientCompileSyntaxMessagesTypedBoundaryMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	base := From[clientCompileSyntaxUnknownEvent](env, "UnknownButSyntacticallyValid")
	invalid := []struct {
		name  string
		query Query
		text  string
	}{
		{name: "blank source", query: FromAny(env, " ").Query(), text: "source name"},
		{name: "nil filter", query: base.Filter(nil).Query(), text: "filter requires"},
		{name: "nil window", query: base.Window(nil).Query(), text: "window specification"},
		{name: "blank alias", query: Select(base, Alias(" ", Literal(1))).Query(), text: "non-blank alias"},
		{name: "nil select expression", query: Select(base, Alias("value", nil)).Query(), text: "is nil"},
		{name: "invalid expression constructor", query: Select(base, Alias("value", Concat())).Query(), text: "at least one operand"},
		{name: "negative limit", query: base.Query(Limit(-1)), text: "cannot be negative"},
		{name: "blank statement name", query: base.Query(StatementName(" ")), text: "statement name"},
		{name: "blank context name", query: base.Query(WithContext(" ")), text: "context name"},
		{name: "invalid output count", query: base.Query(WithOutput(OutputEvery(0))), text: "output count"},
	}
	for _, test := range invalid {
		err := env.ValidateSyntax(test.query)
		if err == nil || !errors.Is(err, ErrorInvalidRule) || !strings.Contains(err.Error(), test.text) {
			t.Fatalf("%s error = %v", test.name, err)
		}
	}

	other := NewEnvironment()
	if err := other.ValidateSyntax(base.Query()); err == nil || !errors.Is(err, ErrorInvalidRule) || !strings.Contains(err.Error(), "different or nil environment") {
		t.Fatalf("foreign query syntax error = %v", err)
	}
	var nilEnvironment *Environment
	if err := nilEnvironment.ValidateSyntax(base.Query()); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("nil environment syntax error = %v", err)
	}
}
