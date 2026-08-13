package esper

import (
	"context"
	"math"
	"reflect"
	"testing"
)

type infraTableOMSupportBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int64  `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type infraTableOMS0 struct {
	ID  int64  `esper:"id"`
	P00 string `esper:"p00"`
}

type infraTableOMS1 struct {
	ID  int64  `esper:"id"`
	P10 string `esper:"p10"`
}

type infraTableOMEMAEvent struct {
	ID string  `esper:"id"`
	X  float64 `esper:"x"`
}

func infraTableOMPtrFloat64(value float64) *float64 {
	return &value
}

func infraTableOMSendRecord(t *testing.T, engine *Engine, name string, values map[string]any) {
	t.Helper()
	if err := engine.SendRecord(context.Background(), name, values); err != nil {
		t.Fatal(err)
	}
}

func infraTableOMSnapshotMap(t *testing.T, engine *Engine, name, keyColumn string) map[string]map[string]any {
	t.Helper()
	table, ok := engine.Table(name)
	if !ok {
		t.Fatalf("table %q is missing", name)
	}
	rows, err := table.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]map[string]any, len(rows))
	for _, row := range rows {
		key, _ := row.Get(keyColumn).Any().(string)
		values := make(map[string]any, len(row.Values()))
		for name, value := range row.Values() {
			values[name] = value.Any()
		}
		result[key] = values
	}
	return result
}

func infraTableOMAssertSnapshot(t *testing.T, engine *Engine, name string, want []map[string]any) {
	t.Helper()
	table, ok := engine.Table(name)
	if !ok {
		t.Fatalf("table %q is missing", name)
	}
	rows, err := table.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(want) {
		t.Fatalf("table %q snapshot has %d rows, want %d: %#v", name, len(rows), len(want), rows)
	}
	used := make([]bool, len(want))
	for _, row := range rows {
		found := false
		for index, expected := range want {
			if used[index] {
				continue
			}
			match := true
			for column, expectedValue := range expected {
				if !reflect.DeepEqual(row.Get(column).Any(), expectedValue) {
					match = false
					break
				}
			}
			if match {
				used[index] = true
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("table %q snapshot contains unexpected row: %#v", name, row.Values())
		}
	}
}

// TestInfraTableOnMergeSimpleParity mirrors Java
// InfraTableOnMerge.InfraTableOnMergeSimple.
func TestInfraTableOnMergeSimpleParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "SupportBean", []FieldSpec{
		FieldDef("theString", reflect.TypeOf("")),
		FieldDef("intPrimitive", reflect.TypeOf(int64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "varaggKV", []TableColumn{
		PrimaryKeyColumn[string]("k1"),
		TableColumnOf[int64]("v1"),
	}); err != nil {
		t.Fatal(err)
	}
	mergePlan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean")).MergeIntoTableWhen("varaggKV", []Expr{
			Field[any, string]("theString"),
		},
			WhenNotMatchedAny(
				SetColumn("k1", Field[any, string]("theString")),
				SetColumn("v1", Field[any, int64]("intPrimitive")),
			),
			WhenMatchedAny(
				SetColumn("v1", Field[any, int64]("intPrimitive")),
			),
		).Query(StatementName("infra-table-on-merge-simple")),
	)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), mergePlan); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": "E1", "intPrimitive": int64(10)})
	infraTableOMAssertSnapshot(t, engine, "varaggKV", []map[string]any{{"k1": "E1", "v1": int64(10)}})
	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": "E1", "intPrimitive": int64(11)})
	infraTableOMAssertSnapshot(t, engine, "varaggKV", []map[string]any{{"k1": "E1", "v1": int64(11)}})
	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": "E2", "intPrimitive": int64(100)})
	infraTableOMAssertSnapshot(t, engine, "varaggKV", []map[string]any{
		{"k1": "E1", "v1": int64(11)},
		{"k1": "E2", "v1": int64(100)},
	})
	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": "E2", "intPrimitive": int64(101)})
	infraTableOMAssertSnapshot(t, engine, "varaggKV", []map[string]any{
		{"k1": "E1", "v1": int64(11)},
		{"k1": "E2", "v1": int64(101)},
	})
}

func infraTableOMRegisterSchemas(t *testing.T, env *Environment) {
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
	if _, err := RegisterMap(env, "SupportBean_S2", []FieldSpec{
		FieldDef("id", reflect.TypeOf(int64(0))),
		FieldDef("p20", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}
}

func infraTableOMSubscribeRows(t *testing.T, deployment *Deployment) *[]Row {
	t.Helper()
	rows := new([]Row)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("table-on-merge result is not a row: %#v", result)
			}
			*rows = append(*rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return rows
}

func infraTableOMLatestRow(t *testing.T, rows *[]Row) Row {
	t.Helper()
	if len(*rows) == 0 {
		t.Fatal("table-on-merge read produced no row")
	}
	return (*rows)[len(*rows)-1]
}

func testInfraOnMergeSingleKey(t *testing.T) {
	env := NewEnvironment()
	infraTableOMRegisterSchemas(t, env)
	if _, err := CreateTable(env, "varaggMIU", []TableColumn{
		PrimaryKeyColumn[int64]("key"),
		OptionalTableColumnOf[string]("p0"),
		OptionalTableColumnOf[int64]("p1"),
		OptionalTableColumnOf[[]int64]("p2"),
		OptionalTableColumnOf[int64]("sumint"),
	}); err != nil {
		t.Fatal(err)
	}

	mergePlan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean")).MergeIntoTableWhen("varaggMIU", []Expr{
			Field[any, int64]("intPrimitive"),
		},
			WhenNotMatchedAny(
				SetColumn("key", Field[any, int64]("intPrimitive")),
				SetColumn("p0", Literal("v1")),
				SetColumn("p1", Literal[int64](1000)),
				SetColumn("p2", Literal([]int64{1, 2})),
			),
			WhenMatched(
				LikeOf(Field[any, string]("theString"), Literal("U%")),
				SetColumn("p0", Literal("v2")),
				SetColumn("p1", Literal[int64](2000)),
				SetColumn("p2", Literal([]int64{3, 4})),
			),
			WhenMatchedDelete(
				LikeOf(Field[any, string]("theString"), Literal("D%")),
			),
		).Query(StatementName("infra-on-merge-single-key-merge")),
	)
	if err != nil {
		t.Fatal(err)
	}
	readPlan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean_S0")).SelectFromTable("varaggMIU", []Expr{
			Field[any, int64]("id"),
		},
			Alias("c0", TableField[string]("p0")),
			Alias("c1", TableField[int64]("p1")),
			Alias("c2", TableField[[]int64]("p2")),
			Alias("c3", TableField[int64]("sumint")),
		).Query(StatementName("infra-on-merge-single-key-read")),
	)
	if err != nil {
		t.Fatal(err)
	}
	aggregatePlan, err := env.Build(
		FromAny(env, "SupportBean_S1").
			GroupBy(Field[any, int64]("id")).
			Select(
				Alias("key", Field[any, int64]("id")),
				Alias("p0", Literal("v1")),
				Alias("p1", Literal[int64](1000)),
				Alias("p2", Literal([]int64{1, 2})),
				Alias("sumint", Sum[int64](Literal(int64(50)))),
			).
			IntoTable("varaggMIU", StatementName("infra-on-merge-single-key-aggregate")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), mergePlan); err != nil {
		t.Fatal(err)
	}
	readDeployment, err := engine.Deploy(context.Background(), readPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	rows := infraTableOMSubscribeRows(t, readDeployment)
	read := func() Row {
		t.Helper()
		before := len(*rows)
		infraTableOMSendRecord(t, engine, "SupportBean_S0", map[string]any{"id": int64(10), "p00": "unused"})
		if len(*rows) != before+1 {
			t.Fatalf("single-key read emitted %d rows, want 1", len(*rows)-before)
		}
		return infraTableOMLatestRow(t, rows)
	}

	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": "E1", "intPrimitive": int64(10)})
	row := read()
	if row.Get("c0").Any() != "v1" || row.Get("c1").Any() != int64(1000) ||
		!reflect.DeepEqual(row.Get("c2").Any(), []int64{1, 2}) || !row.Get("c3").IsNull() {
		t.Fatalf("single-key initial row = %#v", row.AsMap())
	}

	infraTableOMSendRecord(t, engine, "SupportBean_S1", map[string]any{"id": int64(10), "p10": "unused"})
	row = read()
	if row.Get("c0").Any() != "v1" || row.Get("c1").Any() != int64(1000) ||
		!reflect.DeepEqual(row.Get("c2").Any(), []int64{1, 2}) || row.Get("c3").Any() != int64(50) {
		t.Fatalf("single-key aggregate row = %#v", row.AsMap())
	}

	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": "U2", "intPrimitive": int64(10)})
	row = read()
	if row.Get("c0").Any() != "v2" || row.Get("c1").Any() != int64(2000) ||
		!reflect.DeepEqual(row.Get("c2").Any(), []int64{3, 4}) || row.Get("c3").Any() != int64(50) {
		t.Fatalf("single-key updated row = %#v", row.AsMap())
	}

	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": "D3", "intPrimitive": int64(10)})
	row = read()
	if !row.Get("c0").IsNull() || !row.Get("c1").IsNull() || !row.Get("c2").IsNull() || !row.Get("c3").IsNull() {
		t.Fatalf("single-key deleted row = %#v, want null fields", row.AsMap())
	}
}

func testInfraOnMergeTwoKey(t *testing.T) {
	env := NewEnvironment()
	infraTableOMRegisterSchemas(t, env)
	if _, err := CreateTable(env, "varaggMIUD", []TableColumn{
		PrimaryKeyColumn[int64]("keyOne"),
		PrimaryKeyColumn[string]("keyTwo"),
		TableColumnOf[string]("prop"),
	}); err != nil {
		t.Fatal(err)
	}

	mergePlan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean")).MergeIntoTableWhen("varaggMIUD", []Expr{
			Field[any, int64]("intPrimitive"),
			Field[any, string]("theString"),
		},
			WhenNotMatchedAny(
				SetColumn("keyOne", Field[any, int64]("intPrimitive")),
				SetColumn("keyTwo", Field[any, string]("theString")),
				SetColumn("prop", Literal("inserted")),
			),
			WhenMatched(
				Greater[int64](Field[any, int64]("longPrimitive"), Literal[int64](0)),
				SetColumn("prop", Literal("updated")),
			),
			WhenMatchedDelete(
				Less[int64](Field[any, int64]("longPrimitive"), Literal[int64](0)),
			),
		).Query(StatementName("infra-on-merge-two-key-merge")),
	)
	if err != nil {
		t.Fatal(err)
	}
	readPlan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean_S0")).SelectFromTable("varaggMIUD", []Expr{
			Field[any, int64]("id"),
			Field[any, string]("p00"),
		},
			Alias("c0", TableField[int64]("keyOne")),
			Alias("c1", TableField[string]("keyTwo")),
			Alias("c2", TableField[string]("prop")),
		).Query(StatementName("infra-on-merge-two-key-read")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), mergePlan); err != nil {
		t.Fatal(err)
	}
	readDeployment, err := engine.Deploy(context.Background(), readPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	rows := infraTableOMSubscribeRows(t, readDeployment)
	read := func() Row {
		t.Helper()
		before := len(*rows)
		infraTableOMSendRecord(t, engine, "SupportBean_S0", map[string]any{"id": int64(10), "p00": "A"})
		if len(*rows) != before+1 {
			t.Fatalf("two-key read emitted %d rows, want 1", len(*rows)-before)
		}
		return infraTableOMLatestRow(t, rows)
	}

	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": "A", "intPrimitive": int64(10)})
	row := read()
	if row.Get("c0").Any() != int64(10) || row.Get("c1").Any() != "A" || row.Get("c2").Any() != "inserted" {
		t.Fatalf("two-key initial row = %#v", row.AsMap())
	}
	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": "A", "intPrimitive": int64(10), "longPrimitive": int64(1)})
	row = read()
	if row.Get("c2").Any() != "updated" {
		t.Fatalf("two-key updated row = %#v", row.AsMap())
	}
	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": "A", "intPrimitive": int64(10), "longPrimitive": int64(-1)})
	row = read()
	if !row.Get("c0").IsNull() || !row.Get("c1").IsNull() || !row.Get("c2").IsNull() {
		t.Fatalf("two-key deleted row = %#v, want null fields", row.AsMap())
	}
}

func testInfraOnMergeUngrouped(t *testing.T) {
	env := NewEnvironment()
	infraTableOMRegisterSchemas(t, env)
	if _, err := CreateTable(env, "varaggIUD", []TableColumn{
		OptionalTableColumnOf[string]("p0"),
		OptionalTableColumnOf[int64]("sumint"),
	}); err != nil {
		t.Fatal(err)
	}

	mergePlan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean_S2")).MergeIntoTableWhen("varaggIUD", nil,
			WhenNotMatchedAny(SetColumn("p0", Field[any, string]("p20"))),
			WhenMatched(
				LikeOf(Field[any, string]("p20"), Literal("U%")),
				SetColumn("p0", Literal("updated")),
			),
			WhenMatchedDelete(LikeOf(Field[any, string]("p20"), Literal("D%"))),
		).Query(StatementName("infra-on-merge-ungrouped-merge")),
	)
	if err != nil {
		t.Fatal(err)
	}
	aggregatePlan, err := env.Build(
		FromAny(env, "SupportBean_S1").
			Aggregate(Alias("sumint", Sum[int64](Literal(int64(50))))).
			IntoTable("varaggIUD", StatementName("infra-on-merge-ungrouped-aggregate")),
	)
	if err != nil {
		t.Fatal(err)
	}
	readPlan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean_S0")).SelectFromTable("varaggIUD", nil,
			Alias("c0", TableField[string]("p0")),
			Alias("c1", TableField[int64]("sumint")),
		).Query(StatementName("infra-on-merge-ungrouped-read")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), mergePlan); err != nil {
		t.Fatal(err)
	}
	readDeployment, err := engine.Deploy(context.Background(), readPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()
	rows := infraTableOMSubscribeRows(t, readDeployment)
	read := func() Row {
		t.Helper()
		before := len(*rows)
		infraTableOMSendRecord(t, engine, "SupportBean_S0", map[string]any{"id": int64(0), "p00": "unused"})
		if len(*rows) != before+1 {
			t.Fatalf("ungrouped read emitted %d rows, want 1", len(*rows)-before)
		}
		return infraTableOMLatestRow(t, rows)
	}

	infraTableOMSendRecord(t, engine, "SupportBean_S2", map[string]any{"id": int64(0), "p20": "E1"})
	row := read()
	if row.Get("c0").Any() != "E1" || !row.Get("c1").IsNull() {
		t.Fatalf("ungrouped initial row = %#v", row.AsMap())
	}
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	infraTableOMSendRecord(t, engine, "SupportBean_S1", map[string]any{"id": int64(0), "p10": "unused"})
	row = read()
	if row.Get("c0").Any() != "E1" || row.Get("c1").Any() != int64(50) {
		t.Fatalf("ungrouped aggregate row = %#v", row.AsMap())
	}
	infraTableOMSendRecord(t, engine, "SupportBean_S2", map[string]any{"id": int64(0), "p20": "U2"})
	row = read()
	if row.Get("c0").Any() != "updated" || row.Get("c1").Any() != int64(50) {
		t.Fatalf("ungrouped updated row = %#v", row.AsMap())
	}
	infraTableOMSendRecord(t, engine, "SupportBean_S2", map[string]any{"id": int64(0), "p20": "D3"})
	infraTableOMAssertSnapshot(t, engine, "varaggIUD", nil)
	row = read()
	if !row.Get("c0").IsNull() || !row.Get("c1").IsNull() {
		t.Fatalf("ungrouped deleted row = %#v, want null fields", row.AsMap())
	}
}

// TestInfraOnMergePlainPropsAnyKeyedParity mirrors Java
// InfraOnMergePlainPropsAnyKeyed. The three table shapes share the same
// insert/update/delete merge path in fluent form.
func TestInfraOnMergePlainPropsAnyKeyedParity(t *testing.T) {
	t.Run("single-key", testInfraOnMergeSingleKey)
	t.Run("two-key", testInfraOnMergeTwoKey)
	t.Run("ungrouped", testInfraOnMergeUngrouped)
}

// TestInfraMergeWhereWithMethodReadParity mirrors Java
// InfraMergeWhereWithMethodRead. The fluent API expresses the row-wide cnt=0
// merge-delete predicate with DeleteFromTableWhere while retaining the same
// table lookup, grouping and observed row lifecycle.
func TestInfraMergeWhereWithMethodReadParity(t *testing.T) {
	env := NewEnvironment()
	infraTableOMRegisterSchemas(t, env)
	if _, err := CreateTable(env, "varaggMMR", []TableColumn{
		PrimaryKeyColumn[string]("keyOne"),
		TableColumnOf[int64]("cnt"),
	}); err != nil {
		t.Fatal(err)
	}

	aggregatePlan, err := env.Build(
		FromAny(env, "SupportBean").Window(LastEvent()).
			GroupBy(Field[any, string]("theString")).
			Select(
				Alias("keyOne", Field[any, string]("theString")),
				Alias("cnt", CountAll()),
			).
			IntoTable("varaggMMR", StatementName("infra-merge-method-read-aggregate")),
	)
	if err != nil {
		t.Fatal(err)
	}
	readPlan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean_S0")).SelectFromTable("varaggMMR", []Expr{
			Field[any, string]("p00"),
		}, Alias("c0", TableField[string]("keyOne"))).
			Query(StatementName("infra-merge-method-read")),
	)
	if err != nil {
		t.Fatal(err)
	}
	deletePlan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean_S1")).DeleteFromTableWhere("varaggMMR",
			Equal[int64](TableField[int64]("cnt"), Literal[int64](0)),
		).Query(StatementName("infra-merge-method-delete")),
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

	var values []Value
	if _, err := readDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("merge-method-read result is not a row: %#v", result)
			}
			values = append(values, row.Get("c0"))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	assertKeys := func(want []any) {
		t.Helper()
		for _, key := range []string{"G1", "G2", "G3"} {
			before := len(values)
			infraTableOMSendRecord(t, engine, "SupportBean_S0", map[string]any{"id": int64(0), "p00": key})
			if len(values) != before+1 {
				t.Fatalf("merge-method-read key %s emitted %d values, want 1", key, len(values)-before)
			}
			got := values[len(values)-1]
			index := map[string]int{"G1": 0, "G2": 1, "G3": 2}[key]
			if want[index] == nil {
				if !got.IsNull() {
					t.Fatalf("merge-method-read key %s = %#v, want null", key, got)
				}
				continue
			}
			if got.Any() != want[index] {
				t.Fatalf("merge-method-read key %s = %#v, want %v", key, got.Any(), want[index])
			}
		}
	}

	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": "G1", "intPrimitive": int64(0)})
	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": "G2", "intPrimitive": int64(0)})
	assertKeys([]any{"G1", "G2", nil})

	infraTableOMSendRecord(t, engine, "SupportBean_S1", map[string]any{"id": int64(0), "p10": "unused"})
	infraTableOMAssertSnapshot(t, engine, "varaggMMR", []map[string]any{
		{"keyOne": "G2", "cnt": int64(1)},
	})
	assertKeys([]any{nil, "G2", nil})

	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": "G3", "intPrimitive": int64(0)})
	assertKeys([]any{nil, "G2", "G3"})

	infraTableOMSendRecord(t, engine, "SupportBean_S1", map[string]any{"id": int64(0), "p10": "unused"})
	infraTableOMAssertSnapshot(t, engine, "varaggMMR", []map[string]any{
		{"keyOne": "G3", "cnt": int64(1)},
	})
	assertKeys([]any{nil, nil, "G3"})
}

// TestInfraMergeSelectWithAggReadAndEnumParity mirrors Java
// InfraMergeSelectWithAggReadAndEnum. A length(2) window aggregate stores
// eventset/total in a table; a matched merge side-stream emits the current
// eventset, total and takeLast(1).
func TestInfraMergeSelectWithAggReadAndEnumParity(t *testing.T) {
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
	if _, err := RegisterMap(env, "ResultStream", []FieldSpec{
		FieldDef("eventset", reflect.TypeOf([]infraTableOMSupportBean{})),
		FieldDef("total", reflect.TypeOf(int64(0))),
		FieldDef("c0", reflect.TypeOf([]infraTableOMSupportBean{})),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "varaggMS", []TableColumn{
		TableColumnOf[WindowAccessValue[infraTableOMSupportBean]]("eventset"),
		TableColumnOf[int64]("total"),
	}); err != nil {
		t.Fatal(err)
	}

	aggregatePlan, err := env.Build(
		FromAny(env, "SupportBean").Window(LengthWindow(2)).
			Aggregate(
				Alias("eventset", WindowAccessBy[infraTableOMSupportBean](EventValue[infraTableOMSupportBean]())),
				Alias("total", Sum[int64](Field[any, int64]("intPrimitive"))),
			).
			IntoTable("varaggMS", StatementName("infra-merge-agg-read-aggregate")),
	)
	if err != nil {
		t.Fatal(err)
	}
	eventSet := TableField[WindowAccessValue[infraTableOMSupportBean]]("eventset")
	eventValues := Method[[]infraTableOMSupportBean](eventSet, "Values")
	takeLast := EnumTakeLast[infraTableOMSupportBean](
		eventValues,
		1,
	)
	mergePlan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean_S0")).MergeIntoTableWhen("varaggMS", nil,
			WhenMatchedActions(
				ThenInsertInto("ResultStream",
					Alias("eventset", eventValues),
					Alias("total", TableField[int64]("total")),
					Alias("c0", takeLast),
				),
			),
		).Query(StatementName("infra-merge-agg-read-trigger")),
	)
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromAny(env, "ResultStream").Query(StatementName("infra-merge-agg-read-consumer")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), aggregatePlan); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), mergePlan); err != nil {
		t.Fatal(err)
	}
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()

	var rows []Row
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				rows = append(rows, Row{
					schema: event.Schema(),
					values: []Value{
						event.Get("eventset"),
						event.Get("total"),
						event.Get("c0"),
					},
				})
				continue
			}
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			} else {
				t.Fatalf("merge aggregate side-stream result is neither a row nor event: %#v", result)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	assertRead := func(index int, wantEvents []infraTableOMSupportBean, wantTotal int64) {
		t.Helper()
		infraTableOMSendRecord(t, engine, "SupportBean_S0", map[string]any{"id": int64(0), "p00": "unused"})
		if len(rows) <= index {
			t.Fatal("merge aggregate side-stream produced too few rows")
		}
		row := rows[index]
		events, ok := row.Get("eventset").Any().([]infraTableOMSupportBean)
		if !ok || !reflect.DeepEqual(events, wantEvents) {
			t.Fatalf("merge aggregate eventset = %#v, want %#v", row.Get("eventset").Any(), wantEvents)
		}
		if row.Get("total").Any() != wantTotal {
			t.Fatalf("merge aggregate total = %#v, want %v", row.Get("total").Any(), wantTotal)
		}
		taken, ok := row.Get("c0").Any().([]infraTableOMSupportBean)
		if !ok || len(taken) != 1 || !reflect.DeepEqual(taken, []infraTableOMSupportBean{wantEvents[len(wantEvents)-1]}) {
			t.Fatalf("merge aggregate take-last = %#v, want %#v", row.Get("c0").Any(), wantEvents[len(wantEvents)-1])
		}
	}

	e1 := infraTableOMSupportBean{TheString: "E1", IntPrimitive: 15}
	e2 := infraTableOMSupportBean{TheString: "E2", IntPrimitive: 20}
	e3 := infraTableOMSupportBean{TheString: "E3", IntPrimitive: 30}
	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": e1.TheString, "intPrimitive": e1.IntPrimitive})
	assertRead(0, []infraTableOMSupportBean{e1}, 15)
	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": e2.TheString, "intPrimitive": e2.IntPrimitive})
	assertRead(1, []infraTableOMSupportBean{e1, e2}, 35)
	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": e3.TheString, "intPrimitive": e3.IntPrimitive})
	assertRead(2, []infraTableOMSupportBean{e2, e3}, 50)
}

// TestInfraMergeTwoTablesParity mirrors Java InfraMergeTwoTables. A
// not-matched merge emits a side-stream projection and then inserts the
// target row; the side stream feeds the second table.
func TestInfraMergeTwoTablesParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "SupportBean", []FieldSpec{
		FieldDef("theString", reflect.TypeOf("")),
		FieldDef("intPrimitive", reflect.TypeOf(int64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "TableOneRows", []FieldSpec{
		FieldDef("k1", reflect.TypeOf("")),
		FieldDef("v1", reflect.TypeOf(int64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "TableZero", []TableColumn{
		PrimaryKeyColumn[string]("k0"),
		TableColumnOf[int64]("v0"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "TableOne", []TableColumn{
		PrimaryKeyColumn[string]("k1"),
		TableColumnOf[int64]("v1"),
	}); err != nil {
		t.Fatal(err)
	}

	mergePlan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean")).MergeIntoTableWhen("TableZero", []Expr{
			Field[any, string]("theString"),
		},
			WhenNotMatchedActions(
				ThenInsertInto("TableOneRows",
					Alias("k1", Field[any, string]("theString")),
					Alias("v1", Field[any, int64]("intPrimitive")),
				),
				ThenInsertIntoTarget(
					SetColumn("k0", Field[any, string]("theString")),
					SetColumn("v0", Field[any, int64]("intPrimitive")),
				),
			),
		).Query(StatementName("infra-merge-two-tables-zero")),
	)
	if err != nil {
		t.Fatal(err)
	}
	tableOnePlan, err := env.Build(
		OnRecord(FromAny(env, "TableOneRows")).InsertIntoTable("TableOne",
			SetColumn("k1", Field[any, string]("k1")),
			SetColumn("v1", Field[any, int64]("v1")),
		).Query(StatementName("infra-merge-two-tables-one")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), mergePlan); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), tableOnePlan); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()

	assertTables := func(want []map[string]any) {
		t.Helper()
		zero := make([]map[string]any, len(want))
		one := make([]map[string]any, len(want))
		for index, row := range want {
			zero[index] = map[string]any{"k0": row["k0"], "v0": row["v0"]}
			one[index] = map[string]any{"k1": row["k0"], "v1": row["v0"]}
		}
		infraTableOMAssertSnapshot(t, engine, "TableZero", zero)
		infraTableOMAssertSnapshot(t, engine, "TableOne", one)
	}

	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": "E1", "intPrimitive": int64(1)})
	assertTables([]map[string]any{{"k0": "E1", "v0": int64(1)}})

	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": "E2", "intPrimitive": int64(2)})
	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": "E2", "intPrimitive": int64(3)})
	assertTables([]map[string]any{
		{"k0": "E1", "v0": int64(1)},
		{"k0": "E2", "v0": int64(2)},
	})
}

func infraTableOMEMAInitial(values []Value) (float64, error) {
	if len(values) != 2 {
		return 0, NewError(ErrorTypeMismatch, "ema-initial expects alpha and burnValues")
	}
	alpha, err := As[float64](values[0])
	if err != nil {
		return 0, err
	}
	burn, err := As[[]float64](values[1])
	if err != nil {
		return 0, err
	}
	total := 0.0
	for _, value := range burn {
		total += value
	}
	result := total / float64(len(burn))
	for _, value := range burn {
		result = alpha*value + (1-alpha)*result
	}
	return result, nil
}

func infraTableOMAssertNear(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-10 {
		t.Fatalf("%s = %.12f, want %.12f", name, got, want)
	}
}

// TestInfraTableEMAComputeParity mirrors Java InfraTableEMACompute. A
// no-key table is seeded on the first event and updated through ordered
// matched actions. The update reads the initial burnValues with
// InitialTableField and later actions read the working row with TableField.
func TestInfraTableEMAComputeParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "MyEvent", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
		FieldDef("x", reflect.TypeOf(float64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "EMA", []TableColumn{
		TableColumnOf[[]float64]("burnValues"),
		TableColumnOf[int64]("cnt"),
		OptionalTableColumnOf[float64]("value"),
	}); err != nil {
		t.Fatal(err)
	}

	emaInitial := Construct[float64](
		"ema-initial",
		infraTableOMEMAInitial,
		Literal(0.1),
		TableField[[]float64]("burnValues"),
	)
	burnArray := MakeArray[float64](Literal[int64](5))
	value := TableField[float64]("value")
	x := Field[any, float64]("x")
	seedPlan, err := env.Build(
		OnRecord(FromAny(env, "MyEvent")).MergeIntoTableWhen("EMA", nil,
			WhenNotMatchedAny(
				SetColumn("burnValues", burnArray),
				SetColumn("cnt", Literal[int64](0)),
				SetColumn("value", NullLiteral[float64]()),
			),
		).Query(StatementName("infra-table-ema-seed"), StatementPriority(2)),
	)
	if err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(
		OnRecord(FromAny(env, "MyEvent")).MergeIntoTableWhen("EMA", nil,
			WhenMatched(
				Less[int64](TableField[int64]("cnt"), Literal[int64](4)),
				SetArrayElement("burnValues", TableField[int64]("cnt"), x),
				SetColumn("cnt", Add[int64](TableField[int64]("cnt"), Literal[int64](1))),
			),
			WhenMatched(
				Equal[int64](TableField[int64]("cnt"), Literal[int64](4)),
				SetArrayElement("burnValues", TableField[int64]("cnt"), x),
				SetColumn("value", emaInitial),
				SetColumn("burnValues", NullLiteral[[]float64]()),
				SetColumn("cnt", Add[int64](TableField[int64]("cnt"), Literal[int64](1))),
			),
			WhenMatched(
				Greater[int64](TableField[int64]("cnt"), Literal[int64](4)),
				SetColumn("value", Add[float64](
					Multiply[float64](Literal(0.1), x),
					Multiply[float64](Literal(0.9), value),
				)),
			),
		).Query(StatementName("infra-table-ema-update"), StatementPriority(1)),
	)
	if err != nil {
		t.Fatal(err)
	}
	outputPlan, err := env.Build(
		OnRecord(FromAny(env, "MyEvent")).SelectFromTableWhere("EMA", Literal(true),
			Alias("burn", TableField[float64]("value")),
		).Query(StatementName("infra-table-ema-output"), StatementPriority(1)),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), seedPlan); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), updatePlan); err != nil {
		t.Fatal(err)
	}
	outputDeployment, err := engine.Deploy(context.Background(), outputPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()

	var outputs []Value
	if _, err := outputDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("EMA output result is not a row: %#v", result)
			}
			outputs = append(outputs, row.Get("burn"))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	assert := func(index int, expected any) {
		t.Helper()
		if len(outputs) <= index {
			t.Fatal("EMA output produced too few rows")
		}
		got := outputs[index]
		if expected == nil {
			if !got.IsNull() {
				t.Fatalf("EMA output %d = %#v, want null", index, got)
			}
			return
		}
		infraTableOMAssertNear(t, "EMA output", got.Any().(float64), expected.(float64))
	}

	inputs := []struct {
		id     string
		x      float64
		output any
	}{
		{id: "E1", x: 1},
		{id: "E2", x: 2},
		{id: "E3", x: 3},
		{id: "E4", x: 4},
		{id: "E5", x: 5, output: 3.08588},
		{id: "E6", x: 6, output: 3.377292},
		{id: "E7", x: 7, output: 3.7395628},
		{id: "E8", x: 8, output: 4.16560652},
	}
	for index, input := range inputs {
		infraTableOMSendRecord(t, engine, "MyEvent", map[string]any{"id": input.id, "x": input.x})
		assert(index, input.output)
	}
}

// TestInfraTableArrayAssignmentBoxedParity mirrors Java
// InfraTableArrayAssignmentBoxed. A no-key table stores a boxed float array;
// merge seeds []*float64 and an indexed update writes a pointer value.
func TestInfraTableArrayAssignmentBoxedParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "SupportBean", []FieldSpec{
		FieldDef("theString", reflect.TypeOf("")),
		FieldDef("intPrimitive", reflect.TypeOf(int64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "MyTable", []TableColumn{
		TableColumnOf[[]*float64]("dbls"),
	}); err != nil {
		t.Fatal(err)
	}

	seedPlan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean")).MergeIntoTableWhen("MyTable", nil,
			WhenNotMatchedAny(
				SetColumn("dbls", MakeArray[*float64](Literal[int64](3))),
			),
		).Query(StatementName("infra-table-array-assignment-boxed-seed"), StatementPriority(2)),
	)
	if err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean")).MergeIntoTableWhen("MyTable", nil,
			WhenMatchedAny(
				SetArrayElement(
					"dbls",
					Field[any, int64]("intPrimitive"),
					Literal(infraTableOMPtrFloat64(1)),
				),
			),
		).Query(StatementName("infra-table-array-assignment-boxed-update"), StatementPriority(1)),
	)
	if err != nil {
		t.Fatal(err)
	}
	outputPlan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean")).SelectFromTableWhere("MyTable", Literal(true),
			Alias("c0", TableField[[]*float64]("dbls")),
		).Query(StatementName("infra-table-array-assignment-boxed-output")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), seedPlan); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), updatePlan); err != nil {
		t.Fatal(err)
	}
	outputDeployment, err := engine.Deploy(context.Background(), outputPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()

	var outputs []Row
	if _, err := outputDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("boxed-array output result is not a row: %#v", result)
			}
			outputs = append(outputs, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	infraTableOMSendRecord(t, engine, "SupportBean", map[string]any{"theString": "E1", "intPrimitive": int64(1)})
	if len(outputs) != 1 {
		t.Fatalf("boxed-array output rows = %d, want 1", len(outputs))
	}
	got, ok := outputs[0].Get("c0").Any().([]*float64)
	if !ok || len(got) != 3 || !reflect.DeepEqual(got, []*float64{nil, infraTableOMPtrFloat64(1), nil}) {
		t.Fatalf("boxed-array output = %#v, want [nil 1 nil]", outputs[0].Get("c0").Any())
	}
}
