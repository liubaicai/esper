package esper

import (
	"context"
	"reflect"
	"testing"
)

type exprDefineSubqueryMultiBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type exprDefineSubqueryMultiTrigger struct {
	ID  string `esper:"id"`
	P00 int    `esper:"p00"`
}

// TestExprDefineSubqueryMultiresultParity covers Java's
// ExprDefineSubqueryMultiresult. It exercises both separate scalar aggregate
// declarations and one row-valued declaration whose aggregate columns are
// consumed through typed nested properties.
func TestExprDefineSubqueryMultiresultParity(t *testing.T) {
	tests := []struct {
		name  string
		build func(*testing.T, *Environment, RecordStream) (Expression[int], Expression[int])
	}{
		{
			name: "separateScalarDeclarations",
			build: func(t *testing.T, env *Environment, inner RecordStream) (Expression[int], Expression[int]) {
				t.Helper()
				value := Field[any, int]("intPrimitive")
				if err := env.DefineExpression("maxi", SubqueryValue[int](inner, Max[int](value))); err != nil {
					t.Fatal(err)
				}
				if err := env.DefineExpression("mini", SubqueryValue[int](inner, Min[int](value))); err != nil {
					t.Fatal(err)
				}
				return ExpressionRef[int](env, "maxi"), ExpressionRef[int](env, "mini")
			},
		},
		{
			name: "multiColumnRowDeclaration",
			build: func(t *testing.T, env *Environment, inner RecordStream) (Expression[int], Expression[int]) {
				t.Helper()
				value := Field[any, int]("intPrimitive")
				row := SubqueryRow(inner,
					Alias("maxi", Max[int](value)),
					Alias("mini", Min[int](value)),
				)
				if err := env.DefineExpression("subq", row); err != nil {
					t.Fatal(err)
				}
				result := ExpressionRef[map[string]any](env, "subq")
				return Property[int](result, "maxi"), Property[int](result, "mini")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[exprDefineSubqueryMultiBean](env, "SupportBean"); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterStruct[exprDefineSubqueryMultiTrigger](env, "SupportBean_ST0"); err != nil {
				t.Fatal(err)
			}

			inner := From[exprDefineSubqueryMultiBean](env, "SupportBean").Window(KeepAll()).AsRecord()
			maximum, minimum := test.build(t, env, inner)
			p00 := Field[exprDefineSubqueryMultiTrigger, int]("p00")
			plan, err := env.Build(Select(
				From[exprDefineSubqueryMultiTrigger](env, "SupportBean_ST0").Window(LastEvent()),
				Alias("val0", DivideFloat(p00, maximum)),
				Alias("val1", DivideFloat(p00, minimum)),
			).Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			resultSchema, ok := plan.ResultSchema()
			if !ok {
				t.Fatal("subquery multiresult schema is missing")
			}
			for _, name := range []string{"val0", "val1"} {
				field, ok := resultSchema.Field(name)
				if !ok || field.Type != reflect.TypeOf(float64(0)) {
					t.Fatalf("subquery multiresult field %s = %#v, want float64", name, field)
				}
			}

			engine := NewEngine(env)
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = engine.Close(context.Background()) }()
			rows := collectDotRows(t, deployment)

			for _, event := range []exprDefineSubqueryMultiBean{
				{TheString: "E1", IntPrimitive: 10},
				{TheString: "E2", IntPrimitive: 5},
			} {
				if err := engine.SendEvent(context.Background(), event); err != nil {
					t.Fatal(err)
				}
			}
			if err := engine.SendEvent(context.Background(), exprDefineSubqueryMultiTrigger{ID: "ST0", P00: 2}); err != nil {
				t.Fatal(err)
			}
			for _, event := range []exprDefineSubqueryMultiBean{
				{TheString: "E3", IntPrimitive: 20},
				{TheString: "E4", IntPrimitive: 2},
			} {
				if err := engine.SendEvent(context.Background(), event); err != nil {
					t.Fatal(err)
				}
			}
			if err := engine.SendEvent(context.Background(), exprDefineSubqueryMultiTrigger{ID: "ST0", P00: 4}); err != nil {
				t.Fatal(err)
			}

			want := [][2]float64{{2.0 / 10.0, 2.0 / 5.0}, {4.0 / 20.0, 4.0 / 2.0}}
			if len(*rows) != len(want) {
				t.Fatalf("subquery multiresult rows = %#v, want %d", *rows, len(want))
			}
			for index, expected := range want {
				if (*rows)[index].Get("val0").Any() != expected[0] || (*rows)[index].Get("val1").Any() != expected[1] {
					t.Fatalf("subquery multiresult row %d = %#v, want %v", index, (*rows)[index], expected)
				}
			}
		})
	}
}
