package esper

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type infraTableUpdateTwoKey struct {
	K1       string `esper:"k1"`
	K2       int64  `esper:"k2"`
	NewValue int64  `esper:"newValue"`
}

type infraTableUpdateArrayEvent struct {
	ID     string `esper:"id"`
	IntOne []int  `esper:"intOne"`
	IntTwo []int  `esper:"intTwo"`
	Value  int64  `esper:"value"`
}

type infraTableUpdateSupportBean struct {
	LongPrimitive int64 `esper:"longPrimitive"`
}

func infraTableUpdateSetLong(values []Value) (infraTableUpdateSupportBean, error) {
	bean, err := As[infraTableUpdateSupportBean](values[0])
	if err != nil {
		return infraTableUpdateSupportBean{}, err
	}
	value, err := As[int64](values[1])
	if err != nil {
		return infraTableUpdateSupportBean{}, err
	}
	bean.LongPrimitive = value
	return bean, nil
}

func infraTableUpdateSnapshot(t *testing.T, engine *Engine, tableName string) []TableRow {
	t.Helper()
	table, ok := engine.Table(tableName)
	if !ok {
		t.Fatalf("table %q is missing", tableName)
	}
	rows, err := table.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func infraTableUpdateAssertRows(t *testing.T, engine *Engine, tableName string, wants []map[string]any) {
	t.Helper()
	rows := infraTableUpdateSnapshot(t, engine, tableName)
	if len(rows) != len(wants) {
		t.Fatalf("table %s rows = %d, want %d: %#v", tableName, len(rows), len(wants), rows)
	}
	used := make([]bool, len(wants))
	for _, row := range rows {
		matched := false
		for index, want := range wants {
			if used[index] {
				continue
			}
			equal := true
			for name, value := range want {
				if !reflect.DeepEqual(row.Get(name).Any(), value) {
					equal = false
					break
				}
			}
			if equal {
				used[index] = true
				matched = true
				break
			}
		}
		if !matched {
			t.Fatalf("table %s unexpected row: %#v", tableName, row.Values())
		}
	}
}

// TestInfraTableOnUpdateTwoKeyParity mirrors Java InfraTableOnUpdateTwoKey.
func TestInfraTableOnUpdateTwoKeyParity(t *testing.T) {
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
	if _, err := RegisterStruct[infraTableUpdateTwoKey](env, "SupportTwoKeyEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "varagg", []TableColumn{
		PrimaryKeyColumn[string]("keyOne"),
		PrimaryKeyColumn[int64]("keyTwo"),
		TableColumnOf[int64]("p0"),
	}); err != nil {
		t.Fatal(err)
	}
	mergePlan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean")).MergeIntoTableWhen("varagg", []Expr{
			Field[any, string]("theString"), Field[any, int64]("intPrimitive"),
		}, WhenNotMatchedAny(
			SetColumn("keyOne", Field[any, string]("theString")),
			SetColumn("keyTwo", Field[any, int64]("intPrimitive")),
			SetColumn("p0", Literal[int64](1)),
		)).Query(StatementName("infra-table-on-update-two-key-merge")),
	)
	if err != nil {
		t.Fatal(err)
	}
	readPlan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean_S0")).SelectFromTable("varagg", []Expr{
			Field[any, string]("p00"), Field[any, int64]("id"),
		}, Alias("value", TableField[int64]("p0"))).Query(StatementName("infra-table-on-update-two-key-read")),
	)
	if err != nil {
		t.Fatal(err)
	}
	updateSource := From[infraTableUpdateTwoKey](env, "SupportTwoKeyEvent")
	updatePlan, err := env.Build(
		OnEvent(updateSource).UpdateTable("varagg", []Expr{
			Field[infraTableUpdateTwoKey, string]("k1"),
			Field[infraTableUpdateTwoKey, int64]("k2"),
		}, SetColumn("p0", Field[infraTableUpdateTwoKey, int64]("newValue"))).
			Query(StatementName("infra-table-on-update-two-key")),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "MyTableSuppBean", []TableColumn{
		OptionalTableColumnOf[infraTableUpdateSupportBean]("sb"),
	}); err != nil {
		t.Fatal(err)
	}
	beanUpdatePlan, err := env.Build(
		OnRecord(FromAny(env, "SupportBean_S0")).UpdateTableWhere("MyTableSuppBean", Literal(true),
			SetColumn("sb", Construct[infraTableUpdateSupportBean](
				"set-long-primitive",
				infraTableUpdateSetLong,
				TableField[infraTableUpdateSupportBean]("sb"),
				Literal[int64](10),
			)),
		).Query(StatementName("infra-table-on-update-composite-column")),
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
	updateDeployment, err := engine.Deploy(context.Background(), updatePlan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(context.Background()) }()

	var values []Value
	if _, err := readDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, _ := result.Row()
			values = append(values, row.Get("value"))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var updateBatches []ResultBatch
	if _, err := updateDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		updateBatches = append(updateBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	lookup := func(want int64) {
		t.Helper()
		before := len(values)
		if err := engine.SendRecord(context.Background(), "SupportBean_S0", map[string]any{"id": int64(10), "p00": "G1"}); err != nil {
			t.Fatal(err)
		}
		if len(values) != before+1 || values[before].Any() != want {
			t.Fatalf("two-key lookup = %#v, want %d", values[before:], want)
		}
	}
	if err := engine.SendRecord(context.Background(), "SupportBean", map[string]any{"theString": "G1", "intPrimitive": int64(10)}); err != nil {
		t.Fatal(err)
	}
	lookup(1)
	if err := engine.SendEvent(context.Background(), infraTableUpdateTwoKey{K1: "G1", K2: 10, NewValue: 2}); err != nil {
		t.Fatal(err)
	}
	lookup(2)
	if len(updateBatches) != 1 || len(updateBatches[0].New) != 1 || len(updateBatches[0].Old) != 1 ||
		updateBatches[0].New[0].Get("p0").Any() != int64(2) || updateBatches[0].Old[0].Get("p0").Any() != int64(1) {
		t.Fatalf("two-key update batch = %#v", updateBatches)
	}

	if _, err := engine.Deploy(context.Background(), beanUpdatePlan); err != nil {
		t.Fatal(err)
	}
}

func infraTableUpdateDeployArrayPlans(t *testing.T, columns []TableColumn, keyExpressions []Expr, updateSource RecordStream) (*Engine, string) {
	t.Helper()
	env := updateSource.env
	const tableName = "MyTable"
	if _, err := CreateTable(env, tableName, columns); err != nil {
		t.Fatal(err)
	}
	insertSource := From[infraTableUpdateArrayEvent](env, "SupportEventWithManyArray").Filter(
		LikeOf(Field[infraTableUpdateArrayEvent, string]("id"), Literal("I%")),
	)
	assignments := []TableAssignment{SetColumn("k1", Field[infraTableUpdateArrayEvent, []int]("intOne"))}
	if len(columns) == 3 {
		assignments = append(assignments, SetColumn("k2", Field[infraTableUpdateArrayEvent, []int]("intTwo")))
	}
	assignments = append(assignments, SetColumn("v", Field[infraTableUpdateArrayEvent, int64]("value")))
	insertPlan, err := env.Build(OnEvent(insertSource).InsertIntoTable(tableName, assignments...).Query(StatementName("infra-table-array-insert")))
	if err != nil {
		t.Fatal(err)
	}
	updatePlan, err := env.Build(OnRecord(updateSource).UpdateTable(tableName, keyExpressions,
		SetColumn("v", Field[any, int64]("value")),
	).Query(StatementName("infra-table-array-update")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), updatePlan); err != nil {
		t.Fatal(err)
	}
	return engine, tableName
}

func infraTableUpdateSendArray(t *testing.T, engine *Engine, id string, one, two []int, value int64) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), infraTableUpdateArrayEvent{ID: id, IntOne: one, IntTwo: two, Value: value}); err != nil {
		t.Fatal(err)
	}
}

// TestInfraTableOnUpdateArrayKeysParity mirrors the single-array and
// two-array Java executions. Slice primary keys compare by content.
func TestInfraTableOnUpdateArrayKeysParity(t *testing.T) {
	t.Run("single-array", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[infraTableUpdateArrayEvent](env, "SupportEventWithManyArray"); err != nil {
			t.Fatal(err)
		}
		update := From[infraTableUpdateArrayEvent](env, "SupportEventWithManyArray").Filter(
			LikeOf(Field[infraTableUpdateArrayEvent, string]("id"), Literal("U%")),
		).AsRecord()
		engine, table := infraTableUpdateDeployArrayPlans(t, []TableColumn{
			PrimaryKeyColumn[[]int]("k1"), TableColumnOf[int64]("v"),
		}, []Expr{Field[any, []int]("intOne")}, update)
		defer func() { _ = engine.Close(context.Background()) }()
		infraTableUpdateSendArray(t, engine, "I1", []int{1, 2}, nil, 10)
		infraTableUpdateSendArray(t, engine, "I2", []int{1, 2, 3}, nil, 20)
		infraTableUpdateSendArray(t, engine, "I3", []int{1}, nil, 30)
		infraTableUpdateSendArray(t, engine, "U2", []int{1, 2, 3}, nil, 21)
		infraTableUpdateSendArray(t, engine, "U1", []int{1, 2}, nil, 11)
		infraTableUpdateSendArray(t, engine, "U3", []int{1}, nil, 31)
		infraTableUpdateSendArray(t, engine, "U4", []int{}, nil, 99)
		infraTableUpdateSendArray(t, engine, "U5", []int{1, 2, 4}, nil, 99)
		infraTableUpdateAssertRows(t, engine, table, []map[string]any{
			{"k1": []int{1, 2}, "v": int64(11)},
			{"k1": []int{1, 2, 3}, "v": int64(21)},
			{"k1": []int{1}, "v": int64(31)},
		})
	})

	t.Run("two-array", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[infraTableUpdateArrayEvent](env, "SupportEventWithManyArray"); err != nil {
			t.Fatal(err)
		}
		update := From[infraTableUpdateArrayEvent](env, "SupportEventWithManyArray").Filter(
			LikeOf(Field[infraTableUpdateArrayEvent, string]("id"), Literal("U%")),
		).AsRecord()
		engine, table := infraTableUpdateDeployArrayPlans(t, []TableColumn{
			PrimaryKeyColumn[[]int]("k1"), PrimaryKeyColumn[[]int]("k2"), TableColumnOf[int64]("v"),
		}, []Expr{Field[any, []int]("intOne"), Field[any, []int]("intTwo")}, update)
		defer func() { _ = engine.Close(context.Background()) }()
		infraTableUpdateSendArray(t, engine, "I1", []int{1}, []int{1, 2}, 10)
		infraTableUpdateSendArray(t, engine, "I2", []int{1}, []int{1, 2, 3}, 20)
		infraTableUpdateSendArray(t, engine, "I3", []int{2}, []int{1}, 30)
		infraTableUpdateSendArray(t, engine, "U2", []int{1}, []int{1, 2, 3}, 21)
		infraTableUpdateSendArray(t, engine, "U1", []int{1}, []int{1, 2}, 11)
		infraTableUpdateSendArray(t, engine, "U3", []int{2}, []int{1}, 31)
		infraTableUpdateSendArray(t, engine, "U4", []int{1}, []int{1}, 99)
		infraTableUpdateSendArray(t, engine, "U5", []int{2}, []int{1, 2}, 99)
		infraTableUpdateAssertRows(t, engine, table, []map[string]any{
			{"k1": []int{1}, "k2": []int{1, 2}, "v": int64(11)},
			{"k1": []int{1}, "k2": []int{1, 2, 3}, "v": int64(21)},
			{"k1": []int{2}, "k2": []int{1}, "v": int64(31)},
		})
	})
}

func infraTableUpdateParseIntArray(values []Value) ([]int, error) {
	text, err := As[string](values[0])
	if err != nil {
		return nil, err
	}
	parts := strings.Split(text, ",")
	result := make([]int, len(parts))
	for index, part := range parts {
		value, parseErr := strconv.Atoi(strings.TrimSpace(part))
		if parseErr != nil {
			return nil, parseErr
		}
		result[index] = value
	}
	return result, nil
}

// TestInfraTableOnUpdateArrayKeysConstructedParity mirrors the non-getter
// execution: both array keys are constructed from string trigger properties
// inside the analyzable expression tree.
func TestInfraTableOnUpdateArrayKeysConstructedParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[infraTableUpdateArrayEvent](env, "SupportEventWithManyArray"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "SupportBean_S0", []FieldSpec{
		FieldDef("id", reflect.TypeOf(int64(0))),
		FieldDef("p00", reflect.TypeOf("")),
		FieldDef("p01", reflect.TypeOf("")),
		FieldDef("value", reflect.TypeOf(int64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	update := FromAny(env, "SupportBean_S0")
	first := Construct[[]int]("parse-int-array", infraTableUpdateParseIntArray, Field[any, string]("p00"))
	second := Construct[[]int]("parse-int-array", infraTableUpdateParseIntArray, Field[any, string]("p01"))
	engine, table := infraTableUpdateDeployArrayPlans(t, []TableColumn{
		PrimaryKeyColumn[[]int]("k1"), PrimaryKeyColumn[[]int]("k2"), TableColumnOf[int64]("v"),
	}, []Expr{first, second}, update)
	defer func() { _ = engine.Close(context.Background()) }()
	infraTableUpdateSendArray(t, engine, "I1", []int{1}, []int{1, 2}, 10)
	infraTableUpdateSendArray(t, engine, "I2", []int{1}, []int{1, 2, 3}, 20)
	infraTableUpdateSendArray(t, engine, "I3", []int{2}, []int{1}, 30)
	send := func(first, second string, value int64) {
		t.Helper()
		if err := engine.SendRecord(context.Background(), "SupportBean_S0", map[string]any{
			"id": value, "p00": first, "p01": second, "value": value,
		}); err != nil {
			t.Fatal(err)
		}
	}
	send("1", "1, 2, 3", 21)
	send("1", "1,2", 11)
	send("2", "1", 31)
	send("1", "1", 99)
	send("2", "1, 2", 99)
	infraTableUpdateAssertRows(t, engine, table, []map[string]any{
		{"k1": []int{1}, "k2": []int{1, 2}, "v": int64(11)},
		{"k1": []int{1}, "k2": []int{1, 2, 3}, "v": int64(21)},
		{"k1": []int{2}, "k2": []int{1}, "v": int64(31)},
	})
}
