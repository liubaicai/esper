package esper

import (
	"context"
	"testing"
)

// SupportBean_ST0 equivalent for nested enum tests.
type nestedST0 struct {
	ID  string `esper:"id"`
	P00 int64  `esper:"p00"`
}

type nestedST0Container struct {
	Contained []nestedST0 `esper:"contained"`
}

// SupportContainerLevelEvent equivalent for nested anyOf tests.
type nestedLevel2 struct {
	Multivalues []string `esper:"multivalues"`
	Singlevalue string   `esper:"singlevalue"`
}

type nestedLevel1 struct {
	Level2s []nestedLevel2 `esper:"level2s"`
}

type nestedLevelEvent struct {
	Level1s []nestedLevel1 `esper:"level1s"`
}

func makeNestedLevelEvent(value string) nestedLevelEvent {
	return nestedLevelEvent{Level1s: []nestedLevel1{
		{Level2s: []nestedLevel2{{Multivalues: []string{"X1"}, Singlevalue: "X1"}}},
		{Level2s: []nestedLevel2{{Multivalues: []string{value}, Singlevalue: value}}},
		{Level2s: []nestedLevel2{{Multivalues: []string{"X2"}, Singlevalue: "X2"}}},
	}}
}

// TestExprEnumNestedMinByUncorrelatedParity covers Java
// ExprEnumEquivalentToMinByUncorrelated: an inner EnumMinOf on the same
// collection referenced inside an outer EnumWhere predicate. With p00 values
// [2, 1, 2] the minimum is 1, so only the element with p00=1 survives the
// filter.
func TestExprEnumNestedMinByUncorrelatedParity(t *testing.T) {
	contained := Literal([]nestedST0{
		{ID: "E1", P00: 2},
		{ID: "E2", P00: 1},
		{ID: "E3", P00: 2},
	})
	minP00 := EnumMinOf[nestedST0, int64](contained, EnumField[nestedST0, int64]("p00"))
	predicate := Equal[int64](EnumField[nestedST0, int64]("p00"), minP00)
	result := EnumWhere[nestedST0](contained, predicate)
	v := result.eval(EvalContext{})
	if !v.IsPresent() {
		t.Fatalf("nested min-uncorrelated = %v, want present", v)
	}
	got, err := As[[]nestedST0](v)
	if err != nil {
		t.Fatalf("nested min-uncorrelated cast error: %v", err)
	}
	want := []nestedST0{{ID: "E2", P00: 1}}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("nested min-uncorrelated = %v, want %v", got, want)
	}
}

// TestExprEnumNestedMinByWhereParity covers Java ExprEnumMinByWhere: an
// inner EnumMinBy on persons inside an outer EnumWhere predicate on sales.
// The youngest person is Jim (age 19); only the sale whose buyer is Jim
// survives the filter.
func TestExprEnumNestedMinByWhereParity(t *testing.T) {
	jim := chainedPerson{Name: "Jim", Age: 19}
	henry := chainedPerson{Name: "Henry", Age: 20}
	peter := chainedPerson{Name: "Peter", Age: 50}
	boris := chainedPerson{Name: "Boris", Age: 42}
	persons := Literal([]chainedPerson{jim, henry, peter, boris})
	sales := Literal([]chainedSale{
		{Buyer: jim, Seller: henry, Cost: 1000},
		{Buyer: peter, Seller: boris, Cost: 5000},
	})
	youngest := EnumMinBy[chainedPerson, int64](persons, EnumField[chainedPerson, int64]("age"))
	predicate := Equal[chainedPerson](EnumField[chainedSale, chainedPerson]("buyer"), youngest)
	result := EnumWhere[chainedSale](sales, predicate)
	v := result.eval(EvalContext{})
	if !v.IsPresent() {
		t.Fatalf("nested min-by-where = %v, want present", v)
	}
	got, err := As[[]chainedSale](v)
	if err != nil {
		t.Fatalf("nested min-by-where cast error: %v", err)
	}
	if len(got) != 1 || got[0].Buyer.Name != "Jim" {
		t.Fatalf("nested min-by-where = %+v, want sale with buyer Jim", got)
	}
}

// TestExprEnumNestedAnyOfParity covers Java ExprEnumAnyOf: nested EnumAnyOf
// where the inner collection comes from a property of the outer element. The
// filter is true when any level2 singlevalue matches the target.
func TestExprEnumNestedAnyOfParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[nestedLevelEvent](env, "SupportContainerLevelEvent"); err != nil {
		t.Fatal(err)
	}

	level1s := Field[nestedLevelEvent, []nestedLevel1]("level1s")
	innerAnyOf := EnumAnyOf[nestedLevel2](
		EnumField[nestedLevel1, []nestedLevel2]("level2s"),
		Equal[string](EnumField[nestedLevel2, string]("singlevalue"), Literal("A")),
	)
	anyOfExpr := EnumAnyOf[nestedLevel1](level1s, innerAnyOf)

	plan, err := env.Build(Select(From[nestedLevelEvent](env, "SupportContainerLevelEvent").
		Filter(anyOfExpr),
		Alias("match", Literal(true)),
	).Query(StatementName("enum-nested-anyof")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectDotRows(t, deployment)

	// "A" is present in the second level1's level2s → match.
	if err := engine.SendEvent(context.Background(), makeNestedLevelEvent("A")); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 {
		t.Fatalf("nested anyOf match: expected 1 row, got %d", len(*rows))
	}

	// "B" is not present → no row.
	if err := engine.SendEvent(context.Background(), makeNestedLevelEvent("B")); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 {
		t.Fatalf("nested anyOf no-match: expected still 1 row, got %d", len(*rows))
	}
}
