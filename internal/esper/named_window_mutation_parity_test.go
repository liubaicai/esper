package esper

import (
	"context"
	"reflect"
	"testing"
)

type namedWindowMutationBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type namedWindowMutationRow struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type namedWindowMutationS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// TestNamedWindowMutationFirstUniqueParity mirrors the shared
// named-window-mutation parity scenario (Java InfraFirstUnique): a
// firstunique(theString) named window keeps the first row per key, deletes
// by SupportBean_S0.p00, and a count consumer follows every mutation.
func TestNamedWindowMutationFirstUniqueParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[namedWindowMutationBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[namedWindowMutationS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	rowSchema, err := RegisterStruct[namedWindowMutationRow](env, "MyWindowRow")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindow", rowSchema,
		NamedWindowRetention(FirstUnique(Field[namedWindowMutationRow, string]("theString")))); err != nil {
		t.Fatal(err)
	}
	insertPlan, err := env.Build(OnEvent(From[namedWindowMutationBean](env, "SupportBean")).InsertIntoNamedWindow(
		"MyWindow",
		SetColumn("theString", Field[namedWindowMutationBean, string]("theString")),
		SetColumn("intPrimitive", Field[namedWindowMutationBean, int]("intPrimitive")),
	).Query(StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(OnEvent(From[namedWindowMutationS0](env, "SupportBean_S0")).DeleteFromNamedWindow(
		"MyWindow",
		Equal[string](NamedWindowField[string]("theString"), Field[namedWindowMutationS0, string]("p00")),
	).Query(StatementName("delete")))
	if err != nil {
		t.Fatal(err)
	}
	createPlan, err := env.Build(FromNamedWindow(env, "MyWindow").Select(
		Alias("theString", Field[any, string]("theString")),
		Alias("intPrimitive", Field[any, int]("intPrimitive")),
	).Query(StatementName("create"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	countPlan, err := env.Build(FromNamedWindow(env, "MyWindow").Aggregate(
		Alias("cnt", CountAll()),
	).Query(StatementName("count")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deploy := func(plan Plan) *Statement {
		t.Helper()
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		return deployment.Statements()[0]
	}
	deploy(insertPlan)
	deploy(deletePlan)
	create := deploy(createPlan)
	count := deploy(countPlan)

	type mutationBatch struct {
		statement string
		newRows   []map[string]any
		oldRows   []map[string]any
	}
	var batches []mutationBatch
	rowMap := func(rows []Result) []map[string]any {
		if len(rows) == 0 {
			return nil
		}
		out := make([]map[string]any, 0, len(rows))
		for _, result := range rows {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("result is not a row: %#v", result)
			}
			out = append(out, map[string]any{
				"theString":    row.Get("theString").Any(),
				"intPrimitive": row.Get("intPrimitive").Any(),
			})
		}
		return out
	}
	countRow := func(rows []Result) map[string]any {
		if len(rows) != 1 {
			t.Fatalf("count rows = %#v", rows)
		}
		row, ok := rows[0].Row()
		if !ok {
			t.Fatalf("count result is not a row: %#v", rows[0])
		}
		return map[string]any{"cnt": row.Get("cnt").Any()}
	}
	if _, err := create.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, mutationBatch{statement: "create", newRows: rowMap(batch.New), oldRows: rowMap(batch.Old)})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := count.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, mutationBatch{statement: "count", newRows: []map[string]any{countRow(batch.New)}})
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sendBean := func(theString string, intPrimitive int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), namedWindowMutationBean{TheString: theString, IntPrimitive: intPrimitive}); err != nil {
			t.Fatal(err)
		}
	}
	sendS0 := func(id int, p00 string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), namedWindowMutationS0{ID: id, P00: p00}); err != nil {
			t.Fatal(err)
		}
	}
	row := func(theString string, intPrimitive int) map[string]any {
		return map[string]any{"theString": theString, "intPrimitive": intPrimitive}
	}
	sendBean("A", 1)
	sendBean("A", 2) // duplicate ignored
	sendS0(0, "A")
	sendBean("A", 3)
	sendS0(0, "A")
	sendBean("B", 4)
	sendS0(0, "X") // no match

	want := []mutationBatch{
		{statement: "create", newRows: []map[string]any{row("A", 1)}},
		{statement: "count", newRows: []map[string]any{{"cnt": int64(1)}}},
		{statement: "create", oldRows: []map[string]any{row("A", 1)}},
		{statement: "count", newRows: []map[string]any{{"cnt": int64(0)}}},
		{statement: "create", newRows: []map[string]any{row("A", 3)}},
		{statement: "count", newRows: []map[string]any{{"cnt": int64(1)}}},
		{statement: "create", oldRows: []map[string]any{row("A", 3)}},
		{statement: "count", newRows: []map[string]any{{"cnt": int64(0)}}},
		{statement: "create", newRows: []map[string]any{row("B", 4)}},
		{statement: "count", newRows: []map[string]any{{"cnt": int64(1)}}},
	}
	if len(batches) != len(want) {
		t.Fatalf("batches = %#v, want %d batches", batches, len(want))
	}
	for index, expected := range want {
		got := batches[index]
		if got.statement != expected.statement || !reflect.DeepEqual(got.newRows, expected.newRows) || !reflect.DeepEqual(got.oldRows, expected.oldRows) {
			t.Fatalf("batches[%d] = %#v, want %#v", index, got, expected)
		}
	}
}
