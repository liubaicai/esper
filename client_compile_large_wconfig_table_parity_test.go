package esper

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

type clientCompileLargeTableEvent struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type clientCompileLargeTableResetColumns struct {
	ID int `esper:"id"`
}

type clientCompileLargeTableResetAll struct {
	ID int `esper:"id"`
}

func newClientCompileLargeTableEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientCompileLargeTableEvent](env, "ClientCompileLargeTableEvent"); err != nil {
		t.Fatal(err)
	}
	return env
}

func clientCompileLargeSumTableDefinition(columnCount int) ([]TableColumn, []Selection, []string) {
	columns := make([]TableColumn, columnCount)
	selections := make([]Selection, columnCount)
	names := make([]string, columnCount)
	value := Field[clientCompileLargeTableEvent, int]("intPrimitive")
	for index := 0; index < columnCount; index++ {
		name := fmt.Sprintf("c%d", index)
		columns[index] = OptionalTableColumnOf[int](name)
		selections[index] = Alias(name, Sum[int](Add[int](value, Literal(index))))
		names[index] = name
	}
	return columns, selections, names
}

func clientCompileLargeOnlyTableRow(t *testing.T, engine *Engine, tableName string, columnCount int) TableRow {
	t.Helper()
	table, ok := engine.Table(tableName)
	if !ok {
		t.Fatalf("large table %q is missing", tableName)
	}
	if got := len(table.Definition().Columns()); got != columnCount {
		t.Fatalf("large table %q columns = %d, want %d", tableName, got, columnCount)
	}
	rows, err := table.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("large table %q rows = %d, want 1", tableName, len(rows))
	}
	return rows[0]
}

func assertClientCompileLargeTableSum(t *testing.T, engine *Engine, tableName string, columnCount int, input int) {
	t.Helper()
	row := clientCompileLargeOnlyTableRow(t, engine, tableName, columnCount)
	for index := 0; index < columnCount; index++ {
		name := fmt.Sprintf("c%d", index)
		if got := row.Get(name).Any(); got != input+index {
			t.Fatalf("large table %s = %#v, want %d", name, got, input+index)
		}
	}
}

func assertClientCompileLargeTableReset(t *testing.T, engine *Engine, tableName string, columnCount int) {
	t.Helper()
	row := clientCompileLargeOnlyTableRow(t, engine, tableName, columnCount)
	for index := 0; index < columnCount; index++ {
		name := fmt.Sprintf("c%d", index)
		if got := row.Get(name).Any(); got != nil {
			t.Fatalf("large table reset %s = %#v, want nil", name, got)
		}
	}
}

func TestClientCompileLargeTableAggregationResetMatchesEsper(t *testing.T) {
	const columnCount = 1000
	const tableName = "ClientCompileLargeResetTable"
	env := newClientCompileLargeTableEnvironment(t)
	if _, err := RegisterStruct[clientCompileLargeTableResetColumns](env, "ClientCompileLargeTableResetColumns"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[clientCompileLargeTableResetAll](env, "ClientCompileLargeTableResetAll"); err != nil {
		t.Fatal(err)
	}
	columns, selections, names := clientCompileLargeSumTableDefinition(columnCount)
	if _, err := CreateTable(env, tableName, columns); err != nil {
		t.Fatal(err)
	}
	aggregatePlan, err := env.Build(
		From[clientCompileLargeTableEvent](env, "ClientCompileLargeTableEvent").
			Aggregate(selections...).
			IntoTable(tableName, StatementName("table-aggregate-reset")),
	)
	if err != nil {
		t.Fatal(err)
	}
	resetColumnsPlan, err := env.Build(
		OnEvent(From[clientCompileLargeTableResetColumns](env, "ClientCompileLargeTableResetColumns")).
			ResetTableAggregates(tableName, names...).
			Query(StatementName("reset-columns")),
	)
	if err != nil {
		t.Fatal(err)
	}
	resetAllPlan, err := env.Build(
		OnEvent(From[clientCompileLargeTableResetAll](env, "ClientCompileLargeTableResetAll")).
			ResetTableAggregates(tableName).
			Query(StatementName("reset-all")),
	)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, plan := range []Plan{aggregatePlan, resetColumnsPlan, resetAllPlan} {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.Send(context.Background(), "ClientCompileLargeTableEvent", clientCompileLargeTableEvent{TheString: "E0", IntPrimitive: 2}); err != nil {
		t.Fatal(err)
	}
	assertClientCompileLargeTableSum(t, engine, tableName, columnCount, 2)
	if err := engine.Send(context.Background(), "ClientCompileLargeTableResetColumns", clientCompileLargeTableResetColumns{}); err != nil {
		t.Fatal(err)
	}
	assertClientCompileLargeTableReset(t, engine, tableName, columnCount)
	if err := engine.Send(context.Background(), "ClientCompileLargeTableEvent", clientCompileLargeTableEvent{TheString: "E1", IntPrimitive: 3}); err != nil {
		t.Fatal(err)
	}
	assertClientCompileLargeTableSum(t, engine, tableName, columnCount, 3)
	if err := engine.Send(context.Background(), "ClientCompileLargeTableResetAll", clientCompileLargeTableResetAll{}); err != nil {
		t.Fatal(err)
	}
	assertClientCompileLargeTableReset(t, engine, tableName, columnCount)
}

func TestClientCompileLargeTableAggregationEnterLeaveMethodMatchesEsper(t *testing.T) {
	const columnCount = 1000
	const tableName = "ClientCompileLargeEnterLeaveTable"
	env := newClientCompileLargeTableEnvironment(t)
	columns, selections, _ := clientCompileLargeSumTableDefinition(columnCount)
	if _, err := CreateTable(env, tableName, columns); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(
		From[clientCompileLargeTableEvent](env, "ClientCompileLargeTableEvent").
			Window(LengthWindow(1)).
			Aggregate(selections...).
			IntoTable(tableName, StatementName("table-aggregate-enter-leave")),
	)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	for _, input := range []int{20, 30} {
		if err := engine.Send(context.Background(), "ClientCompileLargeTableEvent", clientCompileLargeTableEvent{TheString: "E", IntPrimitive: input}); err != nil {
			t.Fatal(err)
		}
		assertClientCompileLargeTableSum(t, engine, tableName, columnCount, input)
	}
}

func TestClientCompileLargeTableAggregationEnterLeaveAccessAggMatchesEsper(t *testing.T) {
	const columnCount = 1000
	const tableName = "ClientCompileLargeAccessTable"
	env := newClientCompileLargeTableEnvironment(t)
	columns := make([]TableColumn, columnCount)
	selections := make([]Selection, columnCount)
	for index := 0; index < columnCount; index++ {
		name := fmt.Sprintf("c%d", index)
		columns[index] = TableColumnOf[WindowAccessValue[clientCompileLargeTableEvent]](name)
		selections[index] = Alias(name, WindowAccessBy[clientCompileLargeTableEvent](EventValue[clientCompileLargeTableEvent]()))
	}
	if _, err := CreateTable(env, tableName, columns); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(
		From[clientCompileLargeTableEvent](env, "ClientCompileLargeTableEvent").
			Window(LengthWindow(1)).
			Aggregate(selections...).
			IntoTable(tableName, StatementName("table-access-enter-leave")),
	)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	for _, event := range []clientCompileLargeTableEvent{
		{TheString: "E0", IntPrimitive: 0},
		{TheString: "E1", IntPrimitive: 1},
	} {
		if err := engine.Send(context.Background(), "ClientCompileLargeTableEvent", event); err != nil {
			t.Fatal(err)
		}
		row := clientCompileLargeOnlyTableRow(t, engine, tableName, columnCount)
		for index := 0; index < columnCount; index++ {
			name := fmt.Sprintf("c%d", index)
			access, ok := row.Get(name).Any().(WindowAccessValue[clientCompileLargeTableEvent])
			if !ok {
				t.Fatalf("large table access %s = %#v", name, row.Get(name).Any())
			}
			if got := access.Values(); !reflect.DeepEqual(got, []clientCompileLargeTableEvent{event}) {
				t.Fatalf("large table access %s values = %#v, want %#v", name, got, event)
			}
		}
	}
}

func TestResetTableAggregatesValidatesCoherentTarget(t *testing.T) {
	env := newClientCompileLargeTableEnvironment(t)
	if _, err := RegisterStruct[clientCompileLargeTableResetAll](env, "ClientCompileLargeTableResetAll"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "ResetValidationTable", []TableColumn{
		OptionalTableColumnOf[int]("c0"),
		OptionalTableColumnOf[int]("c1"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "ResetValidationKeyed", []TableColumn{
		PrimaryKeyColumn[string]("key"),
		OptionalTableColumnOf[int]("c0"),
	}); err != nil {
		t.Fatal(err)
	}
	source := From[clientCompileLargeTableResetAll](env, "ClientCompileLargeTableResetAll")
	invalid := []struct {
		name    string
		table   string
		columns []string
		code    error
	}{
		{name: "partial", table: "ResetValidationTable", columns: []string{"c0"}, code: ErrorInvalidRule},
		{name: "duplicate", table: "ResetValidationTable", columns: []string{"c0", "c0"}, code: ErrorInvalidRule},
		{name: "blank", table: "ResetValidationTable", columns: []string{"c0", " "}, code: ErrorInvalidRule},
		{name: "unknown", table: "ResetValidationTable", columns: []string{"c0", "missing"}, code: ErrorUnknownName},
		{name: "keyed", table: "ResetValidationKeyed", code: ErrorInvalidRule},
	}
	for _, testCase := range invalid {
		t.Run(testCase.name, func(t *testing.T) {
			query := OnEvent(source).ResetTableAggregates(testCase.table, testCase.columns...).Query()
			if _, err := env.Build(query); err == nil || !errors.Is(err, testCase.code) {
				t.Fatalf("reset validation error = %v, want %v", err, testCase.code)
			}
		})
	}
	plan, err := env.Build(OnEvent(source).ResetTableAggregates("ResetValidationTable").Query())
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "ClientCompileLargeTableResetAll", clientCompileLargeTableResetAll{}); err == nil || !errors.Is(err, ErrorState) {
		t.Fatalf("reset without active into-table aggregate error = %v", err)
	}
}
