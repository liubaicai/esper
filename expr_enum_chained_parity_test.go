package esper

import (
	"context"
	"reflect"
	"testing"
)

type chainedPerson struct {
	Name string `esper:"name"`
	Age  int64  `esper:"age"`
}

type chainedSale struct {
	Buyer  chainedPerson `esper:"buyer"`
	Seller chainedPerson `esper:"seller"`
	Cost   float64       `esper:"cost"`
}

type chainedPersonSales struct {
	Persons []chainedPerson `esper:"persons"`
	Sales   []chainedSale   `esper:"sales"`
}

func makeChainedPersonSales() chainedPersonSales {
	jim := chainedPerson{Name: "Jim", Age: 19}
	henry := chainedPerson{Name: "Henry", Age: 20}
	peter := chainedPerson{Name: "Peter", Age: 50}
	boris := chainedPerson{Name: "Boris", Age: 42}
	return chainedPersonSales{
		Persons: []chainedPerson{jim, henry, peter, boris},
		Sales: []chainedSale{
			{Buyer: jim, Seller: henry, Cost: 1000},
			{Buyer: peter, Seller: boris, Cost: 5000},
		},
	}
}

// TestExprEnumChainedScalarParity covers the Java ExprEnumChained
// expression sales.where(x => x.cost > 1000).min(y => y.buyer.age) using
// direct evaluation. Only the sale with cost 5000 survives the filter;
// buyer.age is 50, so the chain result is 50.
func TestExprEnumChainedScalarParity(t *testing.T) {
	bean := makeChainedPersonSales()
	predicate := Greater[float64](EnumField[chainedSale, float64]("cost"), Literal(float64(1000)))
	filtered := EnumWhere[chainedSale](Literal(bean.Sales), predicate)
	result := EnumMinOf[chainedSale, int64](filtered, EnumField[chainedSale, int64]("buyer.age"))
	assertEnumEval(t, result, int64(50), "chained where+min")

	// When the filter removes every element min returns null.
	allFiltered := EnumWhere[chainedSale](Literal(bean.Sales),
		Greater[float64](EnumField[chainedSale, float64]("cost"), Literal(float64(9999))))
	if v := EnumMinOf[chainedSale, int64](allFiltered, EnumField[chainedSale, int64]("buyer.age")).eval(EvalContext{}); !v.IsNull() {
		t.Fatalf("chained empty min = %v, want null", v)
	}
}

// TestExprEnumChainedEventParity covers the chained where+min expression
// through Build/Deploy with typed schema metadata and live event evaluation.
func TestExprEnumChainedEventParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[chainedPersonSales](env, "PersonSales"); err != nil {
		t.Fatal(err)
	}

	sales := Field[chainedPersonSales, []chainedSale]("sales")
	filtered := EnumWhere[chainedSale](sales,
		Greater[float64](EnumField[chainedSale, float64]("cost"), Literal(float64(1000))))
	result := EnumMinOf[chainedSale, int64](filtered, EnumField[chainedSale, int64]("buyer.age"))

	plan, err := env.Build(Select(From[chainedPersonSales](env, "PersonSales"),
		Alias("val", result),
	).Query(StatementName("enum-chained")))
	if err != nil {
		t.Fatal(err)
	}

	schema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("chained result schema is missing")
	}
	field, exists := schema.Field("val")
	if !exists || field.Type != reflect.TypeOf(int64(0)) {
		t.Fatalf("chained val field = %#v, want int64", field)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectDotRows(t, deployment)
	if err := engine.SendEvent(context.Background(), makeChainedPersonSales()); err != nil {
		t.Fatal(err)
	}
	row := (*rows)[0]
	if !row.Get("val").Equal(Present(int64(50))) {
		t.Fatalf("chained event val = %v, want 50", row.Get("val"))
	}
}
