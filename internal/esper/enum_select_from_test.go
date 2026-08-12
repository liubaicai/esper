package esper

import (
	"context"
	"reflect"
	"testing"
)

func TestEnumerableSelectFromProjectsNamedRowsWithIndexAndSize(t *testing.T) {
	items := Literal([]enumExpressionItem{
		{ID: "E1", Score: 12},
		{ID: "E2", Score: 11},
		{ID: "E3", Score: 2},
	})
	rows := EnumSelectMap[enumExpressionItem](
		items,
		Alias("v0", EnumField[enumExpressionItem, string]("id")),
		Alias("v1", EnumIndex()),
		Alias("v2", Add[int64](EnumIndex(), Multiply[int64](EnumSize(), Literal(int64(100))))),
	)
	value, err := As[[]map[string]any](rows.eval(EvalContext{}))
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]any{
		{"v0": "E1", "v1": int64(0), "v2": int64(300)},
		{"v0": "E2", "v1": int64(1), "v2": int64(301)},
		{"v0": "E3", "v1": int64(2), "v2": int64(302)},
	}
	if !reflect.DeepEqual(value, want) {
		t.Fatalf("select-from rows = %#v, want %#v", value, want)
	}
	if got := EnumSelectMap[enumExpressionItem](NullLiteral[[]enumExpressionItem](), Alias("id", EnumField[enumExpressionItem, string]("id"))).eval(EvalContext{}); !got.IsNull() {
		t.Fatalf("select-from null = %v", got)
	}
	if got := EnumSelectMap[enumExpressionItem](Literal([]enumExpressionItem{}), Alias("id", EnumField[enumExpressionItem, string]("id"))).eval(EvalContext{}); !got.Equal(Present([]map[string]any{})) {
		t.Fatalf("select-from empty = %v", got)
	}
}

func TestEnumerableSelectFromBuildAndRuntimeProjection(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[enumExpressionContainer](env, "EnumSelectFromContainer"); err != nil {
		t.Fatal(err)
	}
	items := Field[enumExpressionContainer, []enumExpressionItem]("items")
	plan, err := env.Build(Select(
		From[enumExpressionContainer](env, "EnumSelectFromContainer"),
		Alias("rows", EnumSelectMap[enumExpressionItem](
			items,
			Alias("id", EnumField[enumExpressionItem, string]("id")),
			Alias("position", EnumIndex()),
		))).Query(StatementName("enum-select-from-runtime")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var row Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) > 0 {
			row, _ = batch.New[len(batch.New)-1].Row()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), enumExpressionContainer{Items: []enumExpressionItem{{ID: "E1"}, {ID: "E2"}}}); err != nil {
		t.Fatal(err)
	}
	actual, err := As[[]map[string]any](row.Get("rows"))
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]any{{"id": "E1", "position": int64(0)}, {"id": "E2", "position": int64(1)}}
	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("runtime select-from rows = %#v, want %#v", actual, want)
	}
}

func TestEnumerableSelectFromRejectsInvalidColumns(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[enumExpressionContainer](env, "EnumSelectFromInvalidContainer"); err != nil {
		t.Fatal(err)
	}
	stream := From[enumExpressionContainer](env, "EnumSelectFromInvalidContainer")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{
			name: "empty",
			expr: EnumSelectMap[enumExpressionItem](Literal([]enumExpressionItem{})),
			want: "enumeration select-from requires at least one column",
		},
		{
			name: "duplicate",
			expr: EnumSelectMap[enumExpressionItem](Literal([]enumExpressionItem{{ID: "E1"}}),
				Alias("id", EnumField[enumExpressionItem, string]("id")),
				Alias("id", EnumIndex()),
			),
			want: "enumeration select-from duplicates column alias",
		},
		{
			name: "nil-expression",
			expr: EnumSelectMap[enumExpressionItem](Literal([]enumExpressionItem{{ID: "E1"}}), Alias("id", nil)),
			want: "enumeration select-from column \"id\" has a nil expression",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(stream, Alias("rows", testCase.expr)).Query(StatementName("enum-select-from-invalid-" + testCase.name)))
			if err == nil || !containsString(err.Error(), testCase.want) {
				t.Fatalf("Build error = %v, want %q", err, testCase.want)
			}
		})
	}
}
