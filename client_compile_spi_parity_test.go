package esper

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestClientCompileSPIExpressionMatchesEsper(t *testing.T) {
	env := NewEnvironment()

	multiply, err := CompileExpression[int](env, Multiply[int](Literal(1), Literal(1)))
	if err != nil {
		t.Fatal(err)
	}
	if value, state, err := multiply.Evaluate(); err != nil || state != ValuePresent || value != 1 {
		t.Fatalf("compiled multiply = %v state=%v err=%v", value, state, err)
	}

	concat, err := CompileExpression[string](env, Concat(Literal("a"), Literal("y")))
	if err != nil {
		t.Fatal(err)
	}
	if value, state, err := concat.Evaluate(); err != nil || state != ValuePresent || value != "ay" {
		t.Fatalf("compiled concat = %q state=%v err=%v", value, state, err)
	}

	list, err := CompileExpression[[]string](env, ArrayOf[string](Literal("a")))
	if err != nil {
		t.Fatal(err)
	}
	if value, state, err := list.Evaluate(); err != nil || state != ValuePresent || !reflect.DeepEqual(value, []string{"a"}) {
		t.Fatalf("compiled list = %#v state=%v err=%v", value, state, err)
	}

	first, err := CompileExpression[string](env, EnumFirstOf[string](ArrayOf[string](Literal("a"), Literal("b"))))
	if err != nil {
		t.Fatal(err)
	}
	if value, state, err := first.Evaluate(); err != nil || state != ValuePresent || value != "a" {
		t.Fatalf("compiled first-of = %q state=%v err=%v", value, state, err)
	}

	period, err := CompileExpression[time.Duration](env, Literal(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	value, state, err := period.Evaluate()
	if err != nil || state != ValuePresent || value.Seconds() != 5 {
		t.Fatalf("compiled period = %s state=%v err=%v", value, state, err)
	}
	if period.Type() != reflect.TypeOf(time.Duration(0)) || period.Description() == "" || period.Plan().Hash() == "" {
		t.Fatalf("compiled period metadata type=%v description=%q hash=%q", period.Type(), period.Description(), period.Plan().Hash())
	}
}

func TestCompileExpressionUsesTypedEnvironmentAndRejectsInvalidAST(t *testing.T) {
	env := NewEnvironment()
	if err := env.RegisterVariable("offset", 2); err != nil {
		t.Fatal(err)
	}
	compiled, err := CompileExpression[int](env, Add[int](VariableRef[int]("offset"), Literal(3)))
	if err != nil {
		t.Fatal(err)
	}
	value, state, err := compiled.EvaluateWith(CompiledExpressionContext{Variables: map[string]Value{"offset": Present(4)}})
	if err != nil || state != ValuePresent || value != 7 {
		t.Fatalf("compiled variable expression = %d state=%v err=%v", value, state, err)
	}

	var missing Expression[int]
	if _, err := CompileExpression[int](env, missing); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("nil compiled expression error = %v", err)
	}
	if _, err := CompileExpression[string](env, Field[any, string]("symbol")); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("field compiled expression error = %v", err)
	}
	if _, err := CompileExpression[[]int](env, ArrayOf[int](Literal("wrong"))); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("invalid array compiled expression error = %v", err)
	}
	var zero CompiledExpression[int]
	if _, state, err := zero.Evaluate(); err == nil || state != ValueMissing || !errors.Is(err, ErrorState) {
		t.Fatalf("zero compiled expression state=%v err=%v", state, err)
	}
}
