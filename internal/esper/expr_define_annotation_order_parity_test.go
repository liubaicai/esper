package esper

import (
	"context"
	"reflect"
	"testing"
)

type exprDefineAnnotationOrderEvent struct {
	ID  string `esper:"id"`
	P00 int    `esper:"p00"`
}

// TestExprDefineAnnotationOrderParity covers Java's ExprDefineAnnotationOrder.
// Java accepts @Name before or after an inline expression declaration. Go's
// typed builder has no textual annotation position, so the equivalent
// contract is that statement options are independent of declaration/source
// construction order.
func TestExprDefineAnnotationOrderParity(t *testing.T) {
	tests := []struct {
		name              string
		defineBeforeInput bool
	}{
		{name: "expressionBeforeStatementOption", defineBeforeInput: true},
		{name: "statementSourceBeforeExpression", defineBeforeInput: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[exprDefineAnnotationOrderEvent](env, "SupportBean_ST0"); err != nil {
				t.Fatal(err)
			}

			var input Stream[exprDefineAnnotationOrderEvent]
			if test.defineBeforeInput {
				if err := env.DefineExpression("scalar", Literal(1)); err != nil {
					t.Fatal(err)
				}
				input = From[exprDefineAnnotationOrderEvent](env, "SupportBean_ST0")
			} else {
				input = From[exprDefineAnnotationOrderEvent](env, "SupportBean_ST0")
				if err := env.DefineExpression("scalar", Literal(1)); err != nil {
					t.Fatal(err)
				}
			}

			plan, err := env.Build(Select(input,
				Alias("scalar()", ExpressionRef[int](env, "scalar")),
			).Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			resultSchema, ok := plan.ResultSchema()
			if !ok {
				t.Fatal("annotation-order result schema is missing")
			}
			field, ok := resultSchema.Field("scalar()")
			if !ok || field.Type != reflect.TypeOf(int(0)) {
				t.Fatalf("annotation-order result field = %#v, want int", field)
			}

			engine := NewEngine(env)
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = engine.Close(context.Background()) }()

			statement, ok := deployment.Statement("s0")
			if !ok || statement.Name() != "s0" {
				t.Fatalf("annotation-order statement = %#v, want name s0", statement)
			}
			metadata := statement.Metadata()
			if metadata.Name != "s0" {
				t.Fatalf("annotation-order metadata name = %q, want s0", metadata.Name)
			}

			rows := collectDotRows(t, deployment)
			if err := engine.SendEvent(context.Background(), exprDefineAnnotationOrderEvent{ID: "E1", P00: 1}); err != nil {
				t.Fatal(err)
			}
			if len(*rows) != 1 || (*rows)[0].Get("scalar()").Any() != 1 {
				t.Fatalf("annotation-order rows = %#v, want scalar()=1", *rows)
			}
		})
	}
}
