package esper

import (
	"context"
	"reflect"
	"testing"
)

func infraTableODRegisterSchemas(t *testing.T, env *Environment) {
	t.Helper()
	if _, err := RegisterMap(env, "SupportBean", []FieldSpec{
		FieldDef("theString", reflect.TypeOf("")),
		FieldDef("intPrimitive", reflect.TypeOf(int64(0))),
		FieldDef("longPrimitive", reflect.TypeOf(int64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "SupportBean_S0", []FieldSpec{
		FieldDef("id", reflect.TypeOf(int64(0))),
		FieldDef("p00", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "SupportBean_S1", []FieldSpec{
		FieldDef("id", reflect.TypeOf(int64(0))),
		FieldDef("p10", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}
}

func infraTableODSend(t *testing.T, engine *Engine, name string, values map[string]any) {
	t.Helper()
	if err := engine.SendRecord(context.Background(), name, values); err != nil {
		t.Fatal(err)
	}
}

func infraTableODSubscribeRows(t *testing.T, deployment *Deployment) *[]Row {
	t.Helper()
	rows := new([]Row)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("table-on-delete projection is not a row: %#v", result)
			}
			*rows = append(*rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return rows
}

func infraTableODAssertLookup(t *testing.T, engine *Engine, rows *[]Row, key, column string, want any) {
	t.Helper()
	before := len(*rows)
	infraTableODSend(t, engine, "SupportBean_S0", map[string]any{"id": int64(0), "p00": key})
	if len(*rows) != before+1 {
		t.Fatalf("table-on-delete lookup %s emitted %d rows, want 1", key, len(*rows)-before)
	}
	got := (*rows)[before].Get(column)
	if want == nil {
		if !got.IsNull() {
			t.Fatalf("table-on-delete lookup %s = %#v, want null", key, got)
		}
		return
	}
	if got.Any() != want {
		t.Fatalf("table-on-delete lookup %s = %#v, want %#v", key, got.Any(), want)
	}
}

// TestInfraTableOnDeleteFlowParity mirrors Java
// InfraTableOnDelete.InfraDeleteFlow, including the on-delete listener's
// insert-stream delivery of removed table rows.
func TestInfraTableOnDeleteFlowParity(t *testing.T) {
	env := NewEnvironment()
	infraTableODRegisterSchemas(t, env)
	if _, err := CreateTable(env, "varagg", []TableColumn{
		PrimaryKeyColumn[string]("key"),
		OptionalTableColumnOf[int64]("thesum"),
	}); err != nil {
		t.Fatal(err)
	}
	aggregatePlan, err := env.Build(
		FromAny(env, "SupportBean").
			GroupBy(Field[any, string]("theString")).
			Select(Alias("thesum", Sum[int64](Field[any, int64]("intPrimitive")))).
			IntoTable("varagg", StatementName("infra-table-on-delete-aggregate")),
	)
	if err != nil {
		t.Fatal(err)
	}
	readPlan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean_S0")).SelectFromTable("varagg", []Expr{
			Field[any, string]("p00"),
		}, Alias("value", TableField[int64]("thesum"))).Query(StatementName("infra-table-on-delete-read")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deleteOneSource := FromAny(env, "SupportBean_S1").Filter(
		Equal[int64](Field[any, int64]("id"), Literal[int64](1)),
	)
	deleteOnePlan, err := env.Build(
		OnRecord(deleteOneSource).DeleteFromTableWhere("varagg",
			Equal[string](TableField[string]("key"), Field[any, string]("p10")),
		).Query(StatementName("infra-table-on-delete-one")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deleteAllSource := FromAny(env, "SupportBean_S1").Filter(
		Equal[int64](Field[any, int64]("id"), Literal[int64](2)),
	)
	deleteAllPlan, err := env.Build(
		OnRecord(deleteAllSource).DeleteAllFromTable("varagg").Query(StatementName("infra-table-on-delete-all")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	readDeployment, err := engine.Deploy(context.Background(), readPlan)
	if err != nil {
		t.Fatal(err)
	}
	deleteOneDeployment, err := engine.Deploy(context.Background(), deleteOnePlan)
	if err != nil {
		t.Fatal(err)
	}
	deleteAllDeployment, err := engine.Deploy(context.Background(), deleteAllPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	reads := infraTableODSubscribeRows(t, readDeployment)

	var deleteOne, deleteAll []Event
	collectDeleted := func(target *[]Event) Listener {
		return func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				event, ok := result.Event()
				if !ok {
					t.Fatalf("table-on-delete result is not an event: %#v", result)
				}
				*target = append(*target, event)
			}
			if len(batch.Old) != 0 {
				t.Fatalf("table-on-delete old data = %#v, want none", batch.Old)
			}
			return nil
		}
	}
	if _, err := deleteOneDeployment.Statements()[0].Subscribe(collectDeleted(&deleteOne)); err != nil {
		t.Fatal(err)
	}
	if _, err := deleteAllDeployment.Statements()[0].Subscribe(collectDeleted(&deleteAll)); err != nil {
		t.Fatal(err)
	}

	infraTableODSend(t, engine, "SupportBean", map[string]any{"theString": "G1", "intPrimitive": int64(10)})
	infraTableODAssertLookup(t, engine, reads, "G1", "value", int64(10))
	infraTableODAssertLookup(t, engine, reads, "G2", "value", nil)

	infraTableODSend(t, engine, "SupportBean", map[string]any{"theString": "G2", "intPrimitive": int64(20)})
	infraTableODAssertLookup(t, engine, reads, "G1", "value", int64(10))
	infraTableODAssertLookup(t, engine, reads, "G2", "value", int64(20))

	infraTableODSend(t, engine, "SupportBean_S1", map[string]any{"id": int64(1), "p10": "G1"})
	infraTableODAssertLookup(t, engine, reads, "G1", "value", nil)
	infraTableODAssertLookup(t, engine, reads, "G2", "value", int64(20))
	if len(deleteOne) != 1 || deleteOne[0].Get("key").Any() != "G1" || deleteOne[0].Get("thesum").Any() != int64(10) {
		t.Fatalf("single delete listener rows = %#v", deleteOne)
	}

	infraTableODSend(t, engine, "SupportBean_S1", map[string]any{"id": int64(2), "p10": nil})
	infraTableODAssertLookup(t, engine, reads, "G1", "value", nil)
	infraTableODAssertLookup(t, engine, reads, "G2", "value", nil)
	if len(deleteAll) != 1 || deleteAll[0].Get("key").Any() != "G2" || deleteAll[0].Get("thesum").Any() != int64(20) {
		t.Fatalf("delete-all listener rows = %#v", deleteAll)
	}
}

// TestInfraTableOnDeleteSecondaryIndexUpdateParity mirrors Java
// InfraDeleteSecondaryIndexUpd. The declared hash index is used by a
// correlated aggregate subquery before and after aggregate row updates and
// keyed deletes.
func TestInfraTableOnDeleteSecondaryIndexUpdateParity(t *testing.T) {
	env := NewEnvironment()
	infraTableODRegisterSchemas(t, env)
	if _, err := CreateTable(env, "MyTable", []TableColumn{
		PrimaryKeyColumn[string]("pkey0"),
		PrimaryKeyColumn[int64]("pkey1"),
		OptionalTableColumnOf[int64]("thesum"),
	}, SecondaryIndex("MyIdx", "pkey0")); err != nil {
		t.Fatal(err)
	}
	aggregatePlan, err := env.Build(
		FromAny(env, "SupportBean").
			GroupBy(
				Field[any, string]("theString"),
				Field[any, int64]("intPrimitive"),
			).
			Select(Alias("thesum", Sum[int64](Field[any, int64]("longPrimitive")))).
			IntoTable("MyTable", StatementName("infra-table-on-delete-index-aggregate")),
	)
	if err != nil {
		t.Fatal(err)
	}
	indexedSum := SubqueryValueWithOptions[int64](
		FromTable(env, "MyTable"),
		Sum[int64](Field[any, int64]("thesum")),
		SubqueryWhere(Equal[string](Field[any, string]("pkey0"), OuterField[string]("p00"))),
		SubqueryUseIndex("MyIdx"),
	)
	readPlan, err := env.Build(
		FromAny(env, "SupportBean_S0").Select(Alias("c0", indexedSum)).
			Query(StatementName("infra-table-on-delete-index-read")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean_S1")).DeleteFromTableWhere("MyTable",
			And(
				Equal[string](TableField[string]("pkey0"), Field[any, string]("p10")),
				Equal[int64](TableField[int64]("pkey1"), Field[any, int64]("id")),
			),
		).Query(StatementName("infra-table-on-delete-index-delete")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	readDeployment, err := engine.Deploy(context.Background(), readPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), deletePlan); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	reads := infraTableODSubscribeRows(t, readDeployment)

	sendBean := func(key string, id, value int64) {
		t.Helper()
		infraTableODSend(t, engine, "SupportBean", map[string]any{
			"theString": key, "intPrimitive": id, "longPrimitive": value,
		})
	}
	assertSums := func(wants map[string]any) {
		t.Helper()
		for _, key := range []string{"E1", "E2", "E3"} {
			infraTableODAssertLookup(t, engine, reads, key, "c0", wants[key])
		}
	}

	sendBean("E1", 10, 2)
	sendBean("E2", 20, 3)
	sendBean("E1", 11, 4)
	sendBean("E2", 21, 5)
	assertSums(map[string]any{"E1": int64(6), "E2": int64(8), "E3": nil})

	sendBean("E3", 30, 77)
	sendBean("E2", 21, 2)
	assertSums(map[string]any{"E1": int64(6), "E2": int64(10), "E3": int64(77)})

	infraTableODSend(t, engine, "SupportBean_S1", map[string]any{"id": int64(11), "p10": "E1"})
	assertSums(map[string]any{"E1": int64(2), "E2": int64(10), "E3": int64(77)})

	infraTableODSend(t, engine, "SupportBean_S1", map[string]any{"id": int64(20), "p10": "E2"})
	assertSums(map[string]any{"E1": int64(2), "E2": int64(7), "E3": int64(77)})
}
