package esper

import (
	"context"
	"testing"
)

// TestExprEnumDataSourcesPropertyParity covers the Java ExprEnumProperty
// execution pattern: enum methods applied to event properties through
// Build/Deploy. Mirrors contained.allOf(x => x.p00 < 5) from
// SupportBean_ST0_Container, validating that property-derived collections
// feed enum methods in the deployed engine.
func TestExprEnumDataSourcesPropertyParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[nestedST0Container](env, "SupportBean_ST0_Container"); err != nil {
		t.Fatal(err)
	}

	contained := Field[nestedST0Container, []nestedST0]("contained")
	allOfExpr := EnumAllOf[nestedST0](contained, Less[int64](EnumField[nestedST0, int64]("p00"), Literal(int64(5))))

	plan, err := env.Build(Select(From[nestedST0Container](env, "SupportBean_ST0_Container"),
		Alias("allOfX", allOfExpr),
	).Query(StatementName("enum-datasources-property")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectDotRows(t, deployment)

	// p00=1 < 5 → allOf is true
	if err := engine.SendEvent(context.Background(), nestedST0Container{Contained: []nestedST0{{ID: "ID1", P00: 1}}}); err != nil {
		t.Fatal(err)
	}
	if v := (*rows)[0].Get("allOfX"); !v.Equal(Present(true)) {
		t.Fatalf("datasources allOf p00=1 = %v, want true", v)
	}

	// p00=10 >= 5 → allOf is false
	if err := engine.SendEvent(context.Background(), nestedST0Container{Contained: []nestedST0{{ID: "ID1", P00: 10}}}); err != nil {
		t.Fatal(err)
	}
	if v := (*rows)[1].Get("allOfX"); !v.Equal(Present(false)) {
		t.Fatalf("datasources allOf p00=10 = %v, want false", v)
	}
}

// TestExprEnumDataSourcesSumOfArrayParity covers the Java ExprEnumProperty
// array/iterable pattern: intarray.sumof() from SupportCollection. Validates
// that array-typed event properties feed EnumSum through Build/Deploy.
func TestExprEnumDataSourcesSumOfArrayParity(t *testing.T) {
	type supportCollection struct {
		IntArray []int64 `esper:"intarray"`
	}

	env := NewEnvironment()
	if _, err := RegisterStruct[supportCollection](env, "SupportCollection"); err != nil {
		t.Fatal(err)
	}

	intArray := Field[supportCollection, []int64]("intarray")
	sumExpr := EnumSum[int64](intArray)

	plan, err := env.Build(Select(From[supportCollection](env, "SupportCollection"),
		Alias("val0", sumExpr),
	).Query(StatementName("enum-datasources-sum")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectDotRows(t, deployment)

	if err := engine.SendEvent(context.Background(), supportCollection{IntArray: []int64{5, 6, 7}}); err != nil {
		t.Fatal(err)
	}
	if v := (*rows)[0].Get("val0"); !v.Equal(Present(int64(18))) {
		t.Fatalf("datasources sum [5,6,7] = %v, want 18", v)
	}
}
