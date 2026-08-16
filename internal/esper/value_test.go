package esper

import "testing"

type valueAliasInt int
type valueAliasString string

func TestValueDistinguishesMissingNullAndPresent(t *testing.T) {
	missing := Missing()
	null := Null()
	present := Present(42)

	if !missing.IsMissing() || null.IsMissing() || present.IsMissing() {
		t.Fatalf("unexpected missing states: %#v %#v %#v", missing, null, present)
	}
	if !null.IsNull() || !present.IsPresent() {
		t.Fatalf("unexpected null/present states: %#v %#v", null, present)
	}
	if got, err := As[int](present); err != nil || got != 42 {
		t.Fatalf("As[int] = %v, %v", got, err)
	}
	if got, err := As[int](null); err != nil || got != 0 {
		t.Fatalf("As[int](null) = %v, %v", got, err)
	}
	if !EqualValues(missing, Present(1)).IsNull() || !EqualValues(null, Present(1)).IsNull() {
		t.Fatal("comparison with missing/null must be null")
	}
}

func TestThreeValuedLogic(t *testing.T) {
	if got := andValues(Present(false), Null()); !got.Equal(Present(false)) {
		t.Fatalf("false AND null = %v", got)
	}
	if got := andValues(Present(true), Null()); !got.IsNull() {
		t.Fatalf("true AND null = %v", got)
	}
	if got := orValues(Present(true), Null()); !got.Equal(Present(true)) {
		t.Fatalf("true OR null = %v", got)
	}
	if got := orValues(Present(false), Null()); !got.IsNull() {
		t.Fatalf("false OR null = %v", got)
	}
}

func TestCompareValuesSupportsOrderedAliases(t *testing.T) {
	if comparison, ok := compareValues(Present(valueAliasInt(1)), Present(valueAliasInt(2))); !ok || comparison >= 0 {
		t.Fatalf("ordered integer alias comparison = %d, %v", comparison, ok)
	}
	if comparison, ok := compareValues(Present(valueAliasString("a")), Present(valueAliasString("b"))); !ok || comparison >= 0 {
		t.Fatalf("ordered string alias comparison = %d, %v", comparison, ok)
	}
}

func TestEqualValuesDereferencesNullablePointers(t *testing.T) {
	// A nullable struct field (P00 *string) reads through Event.Get as a
	// pointer while a plain field or literal surfaces as a value; Esper
	// compares the boxed contents. Both directions must be equal and
	// relational comparisons must work across the pointer boundary.
	value := "E1"
	ptr := &value
	if equal, ok := boolValue(EqualValues(Present("E1"), Present(ptr))); !ok || !equal {
		t.Fatalf("string vs *string equality = %v, %v", equal, ok)
	}
	if equal, ok := boolValue(EqualValues(Present(ptr), Present("E1"))); !ok || !equal {
		t.Fatalf("*string vs string equality = %v, %v", equal, ok)
	}
	if equal, ok := boolValue(EqualValues(Present(ptr), Present(ptr))); !ok || !equal {
		t.Fatalf("*string vs *string equality = %v, %v", equal, ok)
	}
	if comparison, ok := compareValues(Present("a"), Present(&[]string{"b"}[0])); !ok || comparison >= 0 {
		t.Fatalf("string vs *string comparison = %d, %v", comparison, ok)
	}
	if equal, ok := boolValue(EqualValues(Present("E1"), Present(&[]string{"E2"}[0]))); !ok || equal {
		t.Fatalf("string vs different *string equality = %v, %v", equal, ok)
	}
}
