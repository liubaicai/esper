package esper

import (
	"context"
	"testing"
)

type rowRecogDeleteEvent struct {
	Name  string `esper:"name"`
	Value int    `esper:"value"`
}

func newRowRecogDeleteTest(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[rowRecogDeleteEvent](env, "RowRecogDeleteEvent"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env)
}

func newRowRecogDeleteWindow(t *testing.T, env *Environment, name string) {
	t.Helper()
	schema, ok := env.Schema("RowRecogDeleteEvent")
	if !ok {
		t.Fatal("row-recognize delete schema is missing")
	}
	if _, err := CreateNamedWindow(env, name, schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
}

func deployRowRecogDeleteTrigger(t *testing.T, env *Environment, engine *Engine, window, key string) {
	t.Helper()
	trigger := From[rowRecogDeleteEvent](env, "RowRecogDeleteEvent")
	var predicate Expression[bool]
	switch key {
	case "name":
		predicate = Equal[string](NamedWindowField[string]("name"), Field[rowRecogDeleteEvent, string]("name"))
	case "value":
		predicate = Equal[int](NamedWindowField[int]("value"), Field[rowRecogDeleteEvent, int]("value"))
	default:
		t.Fatalf("unsupported row-recognize delete key %q", key)
	}
	deletePlan, err := env.Build(OnEvent(trigger).DeleteFromNamedWindow(
		window,
		predicate,
	).Query(StatementName(window + "-delete-trigger")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), deletePlan); err != nil {
		t.Fatal(err)
	}
}

func sendRowRecogDeleteWindow(t *testing.T, engine *Engine, window string, events ...rowRecogDeleteEvent) {
	t.Helper()
	for _, event := range events {
		if err := engine.InsertNamedWindow(context.Background(), window, event); err != nil {
			t.Fatal(err)
		}
	}
}

func deleteRowRecogDeleteValue(t *testing.T, engine *Engine, value int) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), rowRecogDeleteEvent{Value: value}); err != nil {
		t.Fatal(err)
	}
}

func deleteRowRecogDeleteName(t *testing.T, engine *Engine, name string) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), rowRecogDeleteEvent{Name: name}); err != nil {
		t.Fatal(err)
	}
}

func rowRecogDeleteSnapshot(t *testing.T, statement *Statement) []Row {
	t.Helper()
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, len(snapshot.Results()))
	for _, result := range snapshot.Results() {
		row, ok := result.Row()
		if !ok {
			t.Fatalf("row-recognize delete snapshot result is not a row: %#v", result)
		}
		rows = append(rows, row)
	}
	return rows
}

func rowRecogDeleteMeasure(row Row, name string) string {
	value := row.Get(name)
	if !value.IsPresent() {
		return ""
	}
	text, _ := value.Any().(string)
	return text
}

func assertRowRecogDeleteRows(t *testing.T, rows []Row, expected ...[2]string) {
	t.Helper()
	if len(rows) != len(expected) {
		t.Fatalf("row-recognize delete rows = %#v, want %d rows", rows, len(expected))
	}
	for index, want := range expected {
		got := [2]string{rowRecogDeleteMeasure(rows[index], "a"), rowRecogDeleteMeasure(rows[index], "b")}
		if got != want {
			t.Fatalf("row-recognize delete row %d = %#v, want %#v", index, got, want)
		}
	}
}

func TestRowRecogNamedWindowDeleteOutOfSequencePrev(t *testing.T) {
	env, _ := newRowRecogDeleteTest(t)
	window := "rowrecog-delete-prev-oosd"
	newRowRecogDeleteWindow(t, env, window)
	engine := NewEngine(env)
	name := Field[rowRecogDeleteEvent, string]("name")
	value := Field[rowRecogDeleteEvent, int]("value")
	query := FromNamedWindow(env, window).
		MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		AllMatches().
		Define("A", And(
			Equal[string](Prev[string](3, name), Literal("P3")),
			And(
				Equal[string](Prev[string](2, name), Literal("P2")),
				Equal[string](Prev[string](4, name), Literal("P4")),
			),
		)).
		Define("B", In[int](value, Prev[int](4, value), Prev[int](2, value))).
		Measures(
			Alias("a", TagField[string]("A", "name")),
			Alias("b", TagField[string]("B", "name")),
		).
		Query(StatementName("rowrecog-delete-prev-oosd"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	deployRowRecogDeleteTrigger(t, env, engine, window, "value")

	sendRowRecogDeleteWindow(t, engine, window,
		rowRecogDeleteEvent{Name: "P2", Value: 1},
		rowRecogDeleteEvent{Name: "P1", Value: 2},
		rowRecogDeleteEvent{Name: "P3", Value: 3},
		rowRecogDeleteEvent{Name: "P4", Value: 4},
		rowRecogDeleteEvent{Name: "P2", Value: 1},
		rowRecogDeleteEvent{Name: "E1", Value: 3},
	)
	assertRowRecogDeleteRows(t, rowRecogDeleteSnapshot(t, statement))

	sendRowRecogDeleteWindow(t, engine, window,
		rowRecogDeleteEvent{Name: "P4", Value: 11},
		rowRecogDeleteEvent{Name: "P3", Value: 12},
		rowRecogDeleteEvent{Name: "P2", Value: 13},
		rowRecogDeleteEvent{Name: "xx", Value: 4},
		rowRecogDeleteEvent{Name: "E2", Value: -4},
		rowRecogDeleteEvent{Name: "E3", Value: 12},
	)
	assertRowRecogDeleteRows(t, rowRecogDeleteSnapshot(t, statement), [2]string{"E2", "E3"})

	sendRowRecogDeleteWindow(t, engine, window,
		rowRecogDeleteEvent{Name: "P4", Value: 21},
		rowRecogDeleteEvent{Name: "P3", Value: 22},
		rowRecogDeleteEvent{Name: "P2", Value: 23},
		rowRecogDeleteEvent{Name: "xx", Value: -2},
		rowRecogDeleteEvent{Name: "E5", Value: -1},
		rowRecogDeleteEvent{Name: "E6", Value: -2},
	)
	assertRowRecogDeleteRows(t, rowRecogDeleteSnapshot(t, statement), [2]string{"E2", "E3"}, [2]string{"E5", "E6"})

	deleteRowRecogDeleteValue(t, engine, 21)
	assertRowRecogDeleteRows(t, rowRecogDeleteSnapshot(t, statement), [2]string{"E2", "E3"}, [2]string{"E5", "E6"})
	deleteRowRecogDeleteValue(t, engine, -1)
	assertRowRecogDeleteRows(t, rowRecogDeleteSnapshot(t, statement), [2]string{"E2", "E3"})
	deleteRowRecogDeleteValue(t, engine, 12)
	assertRowRecogDeleteRows(t, rowRecogDeleteSnapshot(t, statement))
}

func TestRowRecogNamedWindowDeleteOutOfSequence(t *testing.T) {
	env, _ := newRowRecogDeleteTest(t)
	window := "rowrecog-delete-oosd"
	newRowRecogDeleteWindow(t, env, window)
	engine := NewEngine(env)
	value := Field[rowRecogDeleteEvent, int]("value")
	query := FromNamedWindow(env, window).
		MatchRecognize(RowSequence(RowVar("A").OneOrMore(), RowVar("B").ZeroOrMore(), RowVar("C"))).
		FirstMatch().
		Define("A", Equal[int](value, Literal(1))).
		Define("B", Equal[int](value, Literal(2))).
		Define("C", Equal[int](value, Literal(3))).
		Measures(
			Alias("a", TagFieldAt[string]("A", 0, "name")),
			Alias("b", TagFieldAt[string]("A", 1, "name")),
			Alias("c", TagFieldAt[string]("B", 0, "name")),
			Alias("d", TagFieldAt[string]("B", 1, "name")),
			Alias("e", TagField[string]("C", "name")),
		).
		Query(StatementName("rowrecog-delete-oosd"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	deployRowRecogDeleteTrigger(t, env, engine, window, "name")

	sendRowRecogDeleteWindow(t, engine, window,
		rowRecogDeleteEvent{Name: "E1", Value: 1},
		rowRecogDeleteEvent{Name: "E2", Value: 1},
	)
	deleteRowRecogDeleteName(t, engine, "E2")
	sendRowRecogDeleteWindow(t, engine, window, rowRecogDeleteEvent{Name: "E3", Value: 3})
	if len(*rows) != 1 || rowRecogDeleteMeasure((*rows)[0], "a") != "E1" || rowRecogDeleteMeasure((*rows)[0], "e") != "E3" {
		t.Fatalf("row-recognize OOSD first match = %#v", *rows)
	}

	deleteRowRecogDeleteName(t, engine, "E1")
	sendRowRecogDeleteWindow(t, engine, window,
		rowRecogDeleteEvent{Name: "E4", Value: 1},
		rowRecogDeleteEvent{Name: "E5", Value: 1},
	)
	deleteRowRecogDeleteName(t, engine, "E4")
	sendRowRecogDeleteWindow(t, engine, window, rowRecogDeleteEvent{Name: "E6", Value: 3})
	if len(*rows) != 2 || rowRecogDeleteMeasure((*rows)[1], "a") != "E5" || rowRecogDeleteMeasure((*rows)[1], "e") != "E6" {
		t.Fatalf("row-recognize OOSD second match = %#v", *rows)
	}

	sendRowRecogDeleteWindow(t, engine, window,
		rowRecogDeleteEvent{Name: "E7", Value: 1},
		rowRecogDeleteEvent{Name: "E8", Value: 1},
		rowRecogDeleteEvent{Name: "E9", Value: 2},
		rowRecogDeleteEvent{Name: "E10", Value: 2},
		rowRecogDeleteEvent{Name: "E11", Value: 2},
	)
	deleteRowRecogDeleteName(t, engine, "E9")
	sendRowRecogDeleteWindow(t, engine, window, rowRecogDeleteEvent{Name: "E12", Value: 3})
	if len(*rows) != 3 || rowRecogDeleteMeasure((*rows)[2], "a") != "E7" || rowRecogDeleteMeasure((*rows)[2], "b") != "E8" ||
		rowRecogDeleteMeasure((*rows)[2], "c") != "E10" || rowRecogDeleteMeasure((*rows)[2], "d") != "E11" || rowRecogDeleteMeasure((*rows)[2], "e") != "E12" {
		t.Fatalf("row-recognize OOSD third match = %#v", *rows)
	}

	sendRowRecogDeleteWindow(t, engine, window,
		rowRecogDeleteEvent{Name: "E13", Value: 1},
		rowRecogDeleteEvent{Name: "E14", Value: 1},
		rowRecogDeleteEvent{Name: "E15", Value: 2},
		rowRecogDeleteEvent{Name: "E16", Value: 2},
	)
	deleteRowRecogDeleteName(t, engine, "E14")
	deleteRowRecogDeleteName(t, engine, "E15")
	deleteRowRecogDeleteName(t, engine, "E16")
	deleteRowRecogDeleteName(t, engine, "E13")
	sendRowRecogDeleteWindow(t, engine, window, rowRecogDeleteEvent{Name: "E18", Value: 3})
	if len(*rows) != 3 {
		t.Fatalf("row-recognize OOSD invalidated tail emitted a match = %#v", *rows)
	}
}

func TestRowRecogNamedWindowDeleteInSequence(t *testing.T) {
	env, _ := newRowRecogDeleteTest(t)
	window := "rowrecog-delete-in-sequence"
	newRowRecogDeleteWindow(t, env, window)
	engine := NewEngine(env)
	value := Field[rowRecogDeleteEvent, int]("value")
	query := FromNamedWindow(env, window).
		MatchRecognize(RowSequence(RowVar("A").ZeroOrMore(), RowVar("B"))).
		FirstMatch().
		Define("A", Equal[int](value, Literal(1))).
		Define("B", Equal[int](value, Literal(2))).
		Measures(
			Alias("a0", TagFieldAt[string]("A", 0, "name")),
			Alias("a1", TagFieldAt[string]("A", 1, "name")),
			Alias("b", TagField[string]("B", "name")),
		).
		Query(StatementName("rowrecog-delete-in-sequence"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	deployRowRecogDeleteTrigger(t, env, engine, window, "name")

	sendRowRecogDeleteWindow(t, engine, window,
		rowRecogDeleteEvent{Name: "E1", Value: 1},
		rowRecogDeleteEvent{Name: "E2", Value: 1},
	)
	deleteRowRecogDeleteName(t, engine, "E1")
	deleteRowRecogDeleteName(t, engine, "E2")
	sendRowRecogDeleteWindow(t, engine, window, rowRecogDeleteEvent{Name: "E3", Value: 3})
	if len(*rows) != 0 {
		t.Fatalf("row-recognize in-sequence initial delete emitted = %#v", *rows)
	}

	sendRowRecogDeleteWindow(t, engine, window,
		rowRecogDeleteEvent{Name: "E4", Value: 1},
		rowRecogDeleteEvent{Name: "E5", Value: 1},
	)
	deleteRowRecogDeleteName(t, engine, "E4")
	sendRowRecogDeleteWindow(t, engine, window,
		rowRecogDeleteEvent{Name: "E6", Value: 1},
		rowRecogDeleteEvent{Name: "E7", Value: 2},
	)
	if len(*rows) != 1 || rowRecogDeleteMeasure((*rows)[0], "a0") != "E5" || rowRecogDeleteMeasure((*rows)[0], "a1") != "E6" || rowRecogDeleteMeasure((*rows)[0], "b") != "E7" {
		t.Fatalf("row-recognize in-sequence match = %#v", *rows)
	}

	snapshotRows := rowRecogDeleteSnapshot(t, deployment.Statements()[0])
	if len(snapshotRows) != 1 || rowRecogDeleteMeasure(snapshotRows[0], "a0") != "E5" ||
		rowRecogDeleteMeasure(snapshotRows[0], "a1") != "E6" || rowRecogDeleteMeasure(snapshotRows[0], "b") != "E7" {
		t.Fatalf("row-recognize in-sequence snapshot = %#v", snapshotRows)
	}
}
