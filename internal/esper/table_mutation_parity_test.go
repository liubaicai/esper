package esper

import (
	"context"
	"testing"
)

type tableMutationBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int64  `esper:"intPrimitive"`
}

type tableMutationS0 struct {
	ID  int64  `esper:"id"`
	P00 string `esper:"p00"`
}

type tableMutationTwoKey struct {
	K1       string `esper:"k1"`
	K2       int64  `esper:"k2"`
	NewValue int64  `esper:"newValue"`
}

// TestTableMutationTwoKeyParity mirrors the shared table-mutation parity
// scenario (Java InfraTableOnUpdateTwoKey): merge-insert into a two-key
// table, keyed reads from SupportBean_S0, and on-trigger updates with
// new/old rows.
func TestTableMutationTwoKeyParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[tableMutationBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[tableMutationS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[tableMutationTwoKey](env, "SupportTwoKeyEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "varagg", []TableColumn{
		PrimaryKeyColumn[string]("keyOne"),
		PrimaryKeyColumn[int64]("keyTwo"),
		TableColumnOf[int64]("p0"),
	}); err != nil {
		t.Fatal(err)
	}
	beanSource := From[tableMutationBean](env, "SupportBean")
	mergePlan, err := env.Build(OnEvent(beanSource).MergeIntoTableWhen("varagg", []Expr{
		Field[tableMutationBean, string]("theString"),
		Field[tableMutationBean, int64]("intPrimitive"),
	}, WhenNotMatchedAny(
		SetColumn("keyOne", Field[tableMutationBean, string]("theString")),
		SetColumn("keyTwo", Field[tableMutationBean, int64]("intPrimitive")),
		SetColumn("p0", Literal[int64](1)),
	)).Query(StatementName("merge")))
	if err != nil {
		t.Fatal(err)
	}
	readPlan, err := env.Build(OnEvent(From[tableMutationS0](env, "SupportBean_S0")).SelectFromTable(
		"varagg",
		[]Expr{
			Field[tableMutationS0, string]("p00"),
			Field[tableMutationS0, int64]("id"),
		},
		Alias("value", TableField[int64]("p0")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(OnEvent(From[tableMutationTwoKey](env, "SupportTwoKeyEvent")).UpdateTable(
		"varagg",
		[]Expr{
			Field[tableMutationTwoKey, string]("k1"),
			Field[tableMutationTwoKey, int64]("k2"),
		},
		SetColumn("p0", Field[tableMutationTwoKey, int64]("newValue")),
	).Query(StatementName("update")))
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
	deploy(mergePlan)
	read := deploy(readPlan)
	update := deploy(updatePlan)

	var values []any
	if _, err := read.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("read result is not a row: %#v", result)
			}
			value := row.Get("value")
			if value.IsNull() {
				values = append(values, nil)
			} else {
				values = append(values, value.Any())
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var updateBatches []ResultBatch
	if _, err := update.Subscribe(func(_ context.Context, batch ResultBatch) error {
		updateBatches = append(updateBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendBean := func(theString string, intPrimitive int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), tableMutationBean{TheString: theString, IntPrimitive: intPrimitive}); err != nil {
			t.Fatal(err)
		}
	}
	sendS0 := func(id int64, p00 string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), tableMutationS0{ID: id, P00: p00}); err != nil {
			t.Fatal(err)
		}
	}
	sendTwoKey := func(k1 string, k2, newValue int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), tableMutationTwoKey{K1: k1, K2: k2, NewValue: newValue}); err != nil {
			t.Fatal(err)
		}
	}
	sendS0(20, "G2")
	sendBean("G1", 10)
	sendS0(10, "G1")
	sendTwoKey("G1", 10, 2)
	sendS0(10, "G1")
	sendBean("G2", 20)
	sendS0(20, "G2")
	sendTwoKey("G2", 20, 7)
	sendS0(20, "G2")
	sendTwoKey("G3", 30, 9)

	want := []any{nil, int64(1), int64(2), int64(1), int64(7)}
	if len(values) != len(want) {
		t.Fatalf("values = %#v, want %#v", values, want)
	}
	for index := range want {
		if values[index] != want[index] {
			t.Fatalf("values[%d] = %#v, want %#v", index, values[index], want[index])
		}
	}
	if len(updateBatches) != 2 {
		t.Fatalf("update batches = %d, want 2: %#v", len(updateBatches), updateBatches)
	}
	assertUpdate := func(index int, keyOne string, keyTwo, oldValue, newValue int64) {
		t.Helper()
		batch := updateBatches[index]
		if len(batch.New) != 1 || len(batch.Old) != 1 {
			t.Fatalf("update batch %d = %#v", index, batch)
		}
		get := func(result Result, name string) any {
			if row, ok := result.Row(); ok {
				return row.Get(name).Any()
			}
			event, ok := result.Event()
			if !ok {
				t.Fatalf("update result is neither row nor event: %#v", result)
			}
			return event.Get(name).Any()
		}
		check := func(result Result, wantKey string, wantKeyTwo, wantValue int64) {
			if get(result, "keyOne") != wantKey || get(result, "keyTwo") != wantKeyTwo || get(result, "p0") != wantValue {
				t.Fatalf("update result = %#v, want %s/%d/%d", result, wantKey, wantKeyTwo, wantValue)
			}
		}
		check(batch.New[0], keyOne, keyTwo, newValue)
		check(batch.Old[0], keyOne, keyTwo, oldValue)
	}
	assertUpdate(0, "G1", 10, 1, 2)
	assertUpdate(1, "G2", 20, 1, 7)
}
