package esper

import (
	"errors"
	"testing"
)

type havingContainmentEvent struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

// TestHavingContainmentValidation pins the Java
// ResultSetProcessorFactoryFactory.validateHaving parity: a grouped having
// clause may reference non-aggregated properties only when they occur in the
// group-by clause. A declared-expression call that reads longPrimitive
// (outside the group) is rejected with Java's exact sentence, while the
// literal-argument form builds.
func TestHavingContainmentValidation(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[havingContainmentEvent](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if err := DefineExpression[int64](env, "F", ExpressionParam[int64]("v")); err != nil {
		t.Fatal(err)
	}
	theString := Field[havingContainmentEvent, string]("theString")
	intPrimitive := Field[havingContainmentEvent, int]("intPrimitive")

	validRef := Greater[int64](CountAll(), ExpressionRef[int64](env, "F", Literal(int64(1))))
	valid := From[havingContainmentEvent](env, "SupportBean").
		Window(LengthWindow(10)).
		GroupBy(theString).
		Select(Alias("sum(intPrimitive)", Sum[int](intPrimitive))).
		Having(validRef).
		Query(StatementName("s0"))
	if _, err := env.Build(valid); err != nil {
		t.Fatalf("valid declared-expression having rejected: %v", err)
	}

	invalidRef := Greater[int64](CountAll(), ExpressionRef[int64](env, "F", Field[havingContainmentEvent, int64]("longPrimitive")))
	invalid := From[havingContainmentEvent](env, "SupportBean").
		Window(LengthWindow(10)).
		GroupBy(theString).
		Select(Alias("sum(intPrimitive)", Sum[int](intPrimitive))).
		Having(invalidRef).
		Query(StatementName("s0-invalid"))
	_, err := env.Build(invalid)
	if err == nil {
		t.Fatal("non-aggregated having property outside the group-by accepted")
	}
	const want = "Non-aggregated property 'longPrimitive' in the HAVING clause must occur in the group-by clause"
	var espErr *Error
	if !errors.As(err, &espErr) || espErr.Message != want {
		t.Fatalf("error = %v, want message %q", err, want)
	}
}
