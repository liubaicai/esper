package esper

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type clientCompileBatchBean struct {
	Value string `esper:"value"`
}

func clientCompileInvalidBatchQueries(env *Environment) (Query, Query) {
	return FromAny(env, " ").Query(), SelectOnce(env)
}

func requireCompileBatchError(t *testing.T, err error, count int) []CompileErrorItem {
	t.Helper()
	var batch *CompileBatchError
	if err == nil || !errors.As(err, &batch) {
		t.Fatalf("compile batch error = %T %v", err, err)
	}
	items := batch.Items()
	if len(items) != count {
		t.Fatalf("compile batch items = %d, want %d: %v", len(items), count, err)
	}
	return items
}

func TestClientCompileExceptionTwoItemsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	first, second := clientCompileInvalidBatchQueries(env)
	plans, err := CompileBatch(env,
		NewCompileItem(first).AtLine(1).WithDiagnosticText("create schema MySchemaOne (col1 Wrong)"),
		NewCompileItem(second).AtLine(2).WithDiagnosticText("create schema MySchemaTwo (col1 WrongTwo)"),
	)
	if plans != nil {
		t.Fatalf("failed compile batch returned partial plans: %#v", plans)
	}
	items := requireCompileBatchError(t, err, 2)
	if items[0].Index != 0 || items[0].Line != 1 || items[0].Expression != "create schema MySchemaOne (col1 Wrong)" {
		t.Fatalf("first compile item = %#v", items[0])
	}
	if items[1].Index != 1 || items[1].Line != 2 || items[1].Expression != "create schema MySchemaTwo (col1 WrongTwo)" {
		t.Fatalf("second compile item = %#v", items[1])
	}
	for _, item := range items {
		if item.Cause == nil || !errors.Is(item.Cause, ErrorInvalidRule) {
			t.Fatalf("compile item cause = %v", item.Cause)
		}
	}
}

func TestClientCompileExceptionMultiLineMultiItemMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	first, second := clientCompileInvalidBatchQueries(env)
	_, err := CompileBatch(env,
		NewCompileItem(first).AtLine(1).WithDiagnosticText("create schema\nMySchemaOne\n(\n  col1 Wrong\n)"),
		NewCompileItem(second).AtLine(6).WithDiagnosticText("create schema\nMySchemaTwo\n(\n  col1 WrongTwo\n)"),
	)
	items := requireCompileBatchError(t, err, 2)
	if items[0].Line != 1 || items[0].Expression != "create schema MySchemaOne ( col1 Wrong )" {
		t.Fatalf("multiline first item = %#v", items[0])
	}
	if items[1].Line != 6 || items[1].Expression != "create schema MySchemaTwo ( col1 WrongTwo )" {
		t.Fatalf("multiline second item = %#v", items[1])
	}
}

func TestClientCompileExceptionEPLWithNewlineMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	_, err := CompileBatch(env,
		NewCompileItem(Query{env: env}).AtLine(1).WithDiagnosticText("XX\nX"),
	)
	items := requireCompileBatchError(t, err, 1)
	if items[0].Index != 0 || items[0].Line != 1 || items[0].Expression != "XX X" || !strings.Contains(items[0].Cause.Error(), "no source") {
		t.Fatalf("newline compile item = %#v", items[0])
	}
}

func TestCompileBatchIsAllOrNothingAndValidatesItemMetadata(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[clientCompileBatchBean](env, "BatchBean"); err != nil {
		t.Fatal(err)
	}
	validOne := From[clientCompileBatchBean](env, "BatchBean").Query(StatementName("one"))
	validTwo := Select(From[clientCompileBatchBean](env, "BatchBean"),
		Alias("value", Field[clientCompileBatchBean, string]("value")),
	).Query(StatementName("two"))

	plans, err := CompileBatch(env, NewCompileItem(validOne), NewCompileItem(validTwo))
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 || plans[0].Query().Name() != "one" || plans[1].Query().Name() != "two" || plans[0].Hash() == plans[1].Hash() {
		t.Fatalf("successful compile batch plans = %#v", plans)
	}
	empty, err := CompileBatch(env)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty compile batch = %#v, %v", empty, err)
	}

	items := []CompileItem{
		NewCompileItem(validOne).AtLine(0),
		NewCompileItem(validTwo).WithDiagnosticText(" \n "),
	}
	partial, err := CompileBatch(env, items...)
	if partial != nil {
		t.Fatalf("invalid item metadata returned partial plans: %#v", partial)
	}
	failures := requireCompileBatchError(t, err, 2)
	if failures[0].Line != 0 || !strings.Contains(failures[0].Cause.Error(), "positive") {
		t.Fatalf("invalid line failure = %#v", failures[0])
	}
	if failures[1].Line != 2 || !strings.Contains(failures[1].Cause.Error(), "cannot be blank") {
		t.Fatalf("blank diagnostic failure = %#v", failures[1])
	}

	detached := (&CompileBatchError{items: failures}).Items()
	detached[0].Line = 99
	if reflect.DeepEqual(detached, failures) {
		t.Fatal("CompileBatchError.Items did not return a detached slice")
	}
	if _, err := CompileBatch(nil, NewCompileItem(validOne)); err == nil || !errors.Is(err, ErrorDependency) {
		t.Fatalf("nil environment batch error = %v", err)
	}
}
