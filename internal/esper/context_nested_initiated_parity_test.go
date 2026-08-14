package esper

import "testing"

// TestContextNestedInvalidParity mirrors ContextNestedInvalid: a nested
// context cannot declare the same child context name twice.
func TestContextNestedInvalidParity(t *testing.T) {
	env, _ := newContextNestedParityEnvironment(t)
	outer, err := NewKeyContext("Outer", Field[contextNestedParityBean, string]("theString"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.RegisterContext(outer.Name(), outer.Keys()...); err != nil {
		t.Fatal(err)
	}
	inner, err := NewKeyContext("Inner", Field[contextNestedParityBean, int]("intPrimitive"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNestedContext(env, "Nested", "Outer", inner); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNestedContext(env, "Nested", "Outer", inner); err == nil {
		t.Fatal("duplicate nested context name was accepted")
	}
}
