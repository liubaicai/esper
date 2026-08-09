package esper

import (
	"fmt"
	"testing"
)

type clientCompileLargeEvent struct {
	Value int `esper:"value"`
}

func TestClientCompileLargeConstantPoolDueToMethodsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[clientCompileLargeEvent](env, "ClientCompileLargeEvent"); err != nil {
		t.Fatal(err)
	}

	const (
		columnCount = 1000
		addendCount = 26
	)
	selections := make([]Selection, columnCount)
	for column := range selections {
		var expression Expression[int] = Literal(1)
		for addend := 1; addend < addendCount; addend++ {
			expression = Add[int](expression, Literal(1))
		}
		selections[column] = Alias(fmt.Sprintf("z%d", column), expression)
	}

	model := Select(
		From[clientCompileLargeEvent](env, "ClientCompileLargeEvent"),
		selections...,
	).Query(StatementName("large-constant-pool"))
	plan, err := env.Build(model)
	if err != nil {
		t.Fatal(err)
	}
	schema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("large plan has no result schema")
	}
	fields := schema.Fields()
	if len(fields) != columnCount || fields[0].Name != "z0" || fields[columnCount-1].Name != "z999" {
		t.Fatalf("large result schema = %d fields, first=%q last=%q", len(fields), fields[0].Name, fields[len(fields)-1].Name)
	}
	for index, field := range fields {
		if field.Name != fmt.Sprintf("z%d", index) || field.Type != typeOf[int]() {
			t.Fatalf("large result field %d = %#v", index, field)
		}
	}
	if len(plan.Canonical()) == 0 || plan.Hash() == "" {
		t.Fatalf("large plan identity = hash %q canonical bytes %d", plan.Hash(), len(plan.Canonical()))
	}

	rebuilt, err := env.Build(plan.Query())
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt.Hash() != plan.Hash() || string(rebuilt.Canonical()) != string(plan.Canonical()) {
		t.Fatal("large typed model did not rebuild deterministically")
	}
}
