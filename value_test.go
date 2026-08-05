package esper

import "testing"

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
