package esper

import (
	"context"
	"reflect"
	"testing"
)

func collectDefineRows(t *testing.T, deployment *Deployment) *[]Row {
	t.Helper()
	rows := make([]Row, 0)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("declared-expression aggregate result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &rows
}

func TestExprDefineAggregationNoAccessParity(t *testing.T) {
	env, engine := defineBasicEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	eventParam := ExpressionParam[Event]("x")
	if err := env.DefineExpression("sumA", Sum[int](Property[int](eventParam, "intPrimitive"))); err != nil {
		t.Fatal(err)
	}
	boxed := Cast[*int, int](Property[*int](eventParam, "intBoxed"))
	if err := env.DefineExpression("sumB", Sum[int](boxed)); err != nil {
		t.Fatal(err)
	}
	if err := env.DefineExpression("countC", CountAll()); err != nil {
		t.Fatal(err)
	}

	source := From[defineBasicBean](env, "SupportBean")
	sumA := ExpressionRef[int](env, "sumA", EventValue[Event]())
	sumB := ExpressionRef[int](env, "sumB", EventValue[Event]())
	plan, err := env.Build(source.Aggregate(
		Alias("val1", sumA),
		Alias("val2", sumB),
		Alias("val3", Divide[float64](Cast[int, float64](sumA), Cast[int, float64](sumB))),
		Alias("val4", ExpressionRef[int64](env, "countC")),
	).Query(StatementName("expr-define-aggregation-no-access")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	rows := collectDefineRows(t, deployment)

	boxedSix, boxedTen := 6, 10
	for _, event := range []defineBasicBean{
		{IntPrimitive: 5, IntBoxed: &boxedSix},
		{IntPrimitive: 8, IntBoxed: &boxedTen},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 2 {
		t.Fatalf("declared aggregate rows = %#v, want 2", *rows)
	}
	want := [][4]any{{5, 6, 5.0 / 6.0, int64(1)}, {13, 16, 13.0 / 16.0, int64(2)}}
	for index, expected := range want {
		row := (*rows)[index]
		for column, name := range []string{"val1", "val2", "val3", "val4"} {
			if got := row.Get(name).Any(); !reflect.DeepEqual(got, expected[column]) {
				t.Fatalf("row %d %s = %#v, want %#v", index, name, got, expected[column])
			}
		}
	}
}

func TestExprDefineAggregatedResultParity(t *testing.T) {
	env, engine := defineBasicEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	eventParam := ExpressionParam[Event]("o")
	if err := env.DefineExpression("lambda1", Property[int](eventParam, "intPrimitive")); err != nil {
		t.Fatal(err)
	}
	if err := env.DefineExpression("lambda2", Multiply[int](Literal(3), Property[int](eventParam, "intPrimitive"))); err != nil {
		t.Fatal(err)
	}
	event := EventValue[Event]()
	source := From[defineBasicBean](env, "SupportBean")
	plan, err := env.Build(source.Aggregate(
		Alias("c0", Sum[int](ExpressionRef[int](env, "lambda1", event))),
		Alias("c1", Sum[int](ExpressionRef[int](env, "lambda2", event))),
	).Query(StatementName("expr-define-aggregated-result")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	rows := collectDefineRows(t, deployment)
	for _, event := range []defineBasicBean{{TheString: "E1", IntPrimitive: 10}, {TheString: "E2", IntPrimitive: 5}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 2 {
		t.Fatalf("declared aggregate result rows = %#v, want 2", *rows)
	}
	for index, expected := range [][2]int{{10, 30}, {15, 45}} {
		if got := (*rows)[index].Get("c0").Any(); got != expected[0] {
			t.Fatalf("row %d c0 = %#v, want %d", index, got, expected[0])
		}
		if got := (*rows)[index].Get("c1").Any(); got != expected[1] {
			t.Fatalf("row %d c1 = %#v, want %d", index, got, expected[1])
		}
	}
}

func TestExprDefineAggregationAccessParity(t *testing.T) {
	env, engine := defineBasicEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	window := WindowAccessBy[defineBasicBean](EventValue[defineBasicBean]())
	predicate := Greater[int](Property[int](EventValue[defineBasicBean](), "intPrimitive"), Literal(2))
	filtered := FilterAggregate[[]defineBasicBean](window.Values(), predicate)
	if err := env.DefineExpression("wb", filtered); err != nil {
		t.Fatal(err)
	}
	source := From[defineBasicBean](env, "SupportBean").Window(KeepAll())
	plan, err := env.Build(source.Aggregate(
		Alias("val1", ExpressionRef[[]defineBasicBean](env, "wb")),
	).Query(StatementName("expr-define-aggregation-access")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	rows := collectDefineRows(t, deployment)
	for _, event := range []defineBasicBean{{TheString: "E1", IntPrimitive: 2}, {TheString: "E2", IntPrimitive: 3}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 2 {
		t.Fatalf("declared access rows = %#v, want 2", *rows)
	}
	first, ok := (*rows)[0].Get("val1").Any().([]defineBasicBean)
	if !ok || len(first) != 0 {
		t.Fatalf("first access result = %#v state=%v row=%#v, want empty []Event", (*rows)[0].Get("val1").Any(), (*rows)[0].Get("val1").State(), (*rows)[0].AsMap())
	}
	second, ok := (*rows)[1].Get("val1").Any().([]defineBasicBean)
	if !ok || len(second) != 1 || second[0].TheString != "E2" {
		t.Fatalf("second access result = %#v, want [E2]", (*rows)[1].Get("val1").Any())
	}
}
