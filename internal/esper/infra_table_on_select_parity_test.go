package esper

import (
	"context"
	"reflect"
	"testing"
)

// TestInfraTableOnSelectParity mirrors Java InfraTableOnSelect. Predicate
// table selection emits no listener row for a missing key and emits the live
// aggregate row once the key exists.
func TestInfraTableOnSelectParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "SupportBean", []FieldSpec{
		FieldDef("theString", reflect.TypeOf("")),
		FieldDef("intPrimitive", reflect.TypeOf(int64(0))),
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
	if _, err := CreateTable(env, "varagg", []TableColumn{
		PrimaryKeyColumn[string]("key"),
		OptionalTableColumnOf[int64]("total"),
	}); err != nil {
		t.Fatal(err)
	}
	aggregatePlan, err := env.Build(
		FromAny(env, "SupportBean").
			GroupBy(Field[any, string]("theString")).
			Select(Alias("total", Sum[int64](Field[any, int64]("intPrimitive")))).
			IntoTable("varagg", StatementName("infra-table-on-select-aggregate")),
	)
	if err != nil {
		t.Fatal(err)
	}
	readS0Plan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean_S0")).SelectFromTableWhere("varagg",
			Equal[string](TableField[string]("key"), Field[any, string]("p00")),
			Alias("value", TableField[int64]("total")),
		).Query(StatementName("infra-table-on-select-s0")),
	)
	if err != nil {
		t.Fatal(err)
	}
	readS1Plan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean_S1")).SelectFromTableWhere("varagg",
			Equal[string](TableField[string]("key"), Field[any, string]("p10")),
			Alias("total", TableField[int64]("total")),
		).Query(StatementName("infra-table-on-select-s1")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	s0Deployment, err := engine.Deploy(context.Background(), readS0Plan)
	if err != nil {
		t.Fatal(err)
	}
	s1Deployment, err := engine.Deploy(context.Background(), readS1Plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()

	var s0Rows, s1Rows []Row
	subscribe := func(deployment *Deployment, target *[]Row) {
		t.Helper()
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				row, ok := result.Row()
				if !ok {
					t.Fatalf("table-on-select result is not a row: %#v", result)
				}
				*target = append(*target, row)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	subscribe(s0Deployment, &s0Rows)
	subscribe(s1Deployment, &s1Rows)
	send := func(name string, values map[string]any) {
		t.Helper()
		if err := engine.SendRecord(context.Background(), name, values); err != nil {
			t.Fatal(err)
		}
	}
	assertS0 := func(key string, want any) {
		t.Helper()
		before := len(s0Rows)
		send("SupportBean_S0", map[string]any{"id": int64(0), "p00": key})
		if want == nil {
			if len(s0Rows) != before {
				t.Fatalf("missing key %s emitted rows: %#v", key, s0Rows[before:])
			}
			return
		}
		if len(s0Rows) != before+1 || s0Rows[before].Get("value").Any() != want {
			t.Fatalf("key %s rows = %#v, want %v", key, s0Rows[before:], want)
		}
	}

	assertS0("G1", nil)
	assertS0("G2", nil)
	send("SupportBean", map[string]any{"theString": "G1", "intPrimitive": int64(100)})
	assertS0("G1", int64(100))
	assertS0("G2", nil)
	send("SupportBean", map[string]any{"theString": "G2", "intPrimitive": int64(200)})
	assertS0("G1", int64(100))
	assertS0("G2", int64(200))
	send("SupportBean", map[string]any{"theString": "G2", "intPrimitive": int64(300)})

	before := len(s1Rows)
	send("SupportBean_S1", map[string]any{"id": int64(0), "p10": "G2"})
	if len(s1Rows) != before+1 || s1Rows[before].Get("total").Any() != int64(500) {
		t.Fatalf("S1 G2 rows = %#v, want total 500", s1Rows[before:])
	}
}
