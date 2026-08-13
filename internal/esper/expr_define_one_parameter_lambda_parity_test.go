package esper

import (
	"context"
	"reflect"
	"testing"
)

// TestExprDefineOneParameterLambdaReturnParity covers Java's
// ExprDefineOneParameterLambdaReturn. A declared expression returns a typed
// collection from an event property, and a second declaration composes it
// with another enumeration filter.
func TestExprDefineOneParameterLambdaReturnParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[nestedST0Container](env, "SupportBean_ST0_Container"); err != nil {
		t.Fatal(err)
	}

	container := ExpressionParam[nestedST0Container]("container")
	contained := Property[[]nestedST0](container, "contained")
	underTen := EnumWhere[nestedST0](contained,
		Less[int64](EnumField[nestedST0, int64]("p00"), Literal(int64(10))))
	if err := env.DefineExpression("one", underTen); err != nil {
		t.Fatal(err)
	}

	containerForTwo := ExpressionParam[nestedST0Container]("container")
	oneCall := ExpressionRef[[]nestedST0](env, "one", containerForTwo)
	overOne := EnumWhere[nestedST0](oneCall,
		Greater[int64](EnumField[nestedST0, int64]("p00"), Literal(int64(1))))
	if err := env.DefineExpression("two", overOne); err != nil {
		t.Fatal(err)
	}

	plan, err := env.Build(Select(From[nestedST0Container](env, "SupportBean_ST0_Container"),
		Alias("val1", ExpressionRef[[]nestedST0](env, "one", EventValue[nestedST0Container]())),
		Alias("val2", ExpressionRef[[]nestedST0](env, "two", EventValue[nestedST0Container]())),
	).Query(StatementName("expr-define-one-parameter-lambda")))
	if err != nil {
		t.Fatal(err)
	}

	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("declared collection result schema is missing")
	}
	wantType := reflect.TypeOf([]nestedST0{})
	for _, name := range []string{"val1", "val2"} {
		field, exists := resultSchema.Field(name)
		if !exists || field.Type != wantType {
			t.Fatalf("result field %q = %#v, want %v", name, field, wantType)
		}
	}

	engine := NewEngine(env)
	deployed, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectDotRows(t, deployed)

	input := nestedST0Container{Contained: []nestedST0{
		{ID: "E1", P00: 1},
		{ID: "E2", P00: 2},
		{ID: "E20", P00: 20},
	}}
	if err := engine.SendEvent(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 {
		t.Fatalf("declared collection rows = %d, want 1", len(*rows))
	}
	val1, err := As[[]nestedST0]((*rows)[0].Get("val1"))
	if err != nil {
		t.Fatal(err)
	}
	val2, err := As[[]nestedST0]((*rows)[0].Get("val2"))
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{val1[0].ID, val1[1].ID}; !reflect.DeepEqual(got, []string{"E1", "E2"}) {
		t.Fatalf("one() = %v, want [E1 E2]", got)
	}
	if got := []string{val2[0].ID}; !reflect.DeepEqual(got, []string{"E2"}) {
		t.Fatalf("two() = %v, want [E2]", got)
	}

	if err := engine.SendEvent(context.Background(), nestedST0Container{}); err != nil {
		t.Fatal(err)
	}
	empty := (*rows)[1]
	if got := empty.Get("val1"); !got.Equal(Present([]nestedST0{})) {
		t.Fatalf("one() empty = %v, want present empty collection", got)
	}
	if got := empty.Get("val2"); !got.Equal(Present([]nestedST0{})) {
		t.Fatalf("two() empty = %v, want present empty collection", got)
	}
}
