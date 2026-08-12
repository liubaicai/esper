package esper

import (
	"context"
	"testing"
)

func TestRowRecogGreedynessReluctantZeroToOne(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(
		RowVar("A").Optional().Reluctant(),
		RowVar("B").Optional(),
	)).
		Define("A", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(1.0))).
		Define("B", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(1.0))).
		Measures(
			Alias("a", TagField[string]("A", "symbol")),
			Alias("b", TagField[string]("B", "symbol")),
		).
		Query(StatementName("rowrecog-reluctant-zero-to-one"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: "E1", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 || (*rows)[0].Get("a").IsPresent() || (*rows)[0].Get("b").Any() != "E1" {
		t.Fatalf("reluctant zero-to-one row = %#v, want null/E1", *rows)
	}
}

func TestRowRecogGreedynessReluctantZeroToMany(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	price := Field[rowRecogTestEvent, float64]("price")
	query := stream.MatchRecognize(RowSequence(
		RowVar("A").ZeroOrMore().Reluctant(),
		RowVar("B").Optional(),
		RowVar("C"),
	)).
		Define("A", Equal[float64](price, Literal(1.0))).
		Define("B", Or(Equal[float64](price, Literal(1.0)), Equal[float64](price, Literal(2.0)))).
		Define("C", Equal[float64](price, Literal(3.0))).
		Measures(
			Alias("a0", TagFieldAt[string]("A", 0, "symbol")),
			Alias("a1", TagFieldAt[string]("A", 1, "symbol")),
			Alias("a2", TagFieldAt[string]("A", 2, "symbol")),
			Alias("b", TagField[string]("B", "symbol")),
			Alias("c", TagField[string]("C", "symbol")),
		).
		Query(StatementName("rowrecog-reluctant-zero-to-many"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	send := func(events ...rowRecogTestEvent) {
		t.Helper()
		for _, event := range events {
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
	}
	assertRow := func(index int, a0, a1, a2, b, c string) {
		t.Helper()
		if len(*rows) <= index {
			t.Fatalf("reluctant zero-to-many rows = %#v, want row %d", *rows, index)
		}
		row := (*rows)[index]
		values := map[string]string{"a0": a0, "a1": a1, "a2": a2, "b": b, "c": c}
		for name, expected := range values {
			value := row.Get(name)
			if expected == "" {
				if value.IsPresent() {
					t.Fatalf("reluctant zero-to-many row %d %s = %#v, want null", index, name, value)
				}
			} else if value.Any() != expected {
				t.Fatalf("reluctant zero-to-many row %d %s = %#v, want %s", index, name, value, expected)
			}
		}
	}

	send(
		rowRecogTestEvent{Symbol: "E1", Price: 1},
		rowRecogTestEvent{Symbol: "E2", Price: 1},
		rowRecogTestEvent{Symbol: "E3", Price: 1},
		rowRecogTestEvent{Symbol: "E4", Price: 3},
	)
	assertRow(0, "E1", "E2", "", "E3", "E4")

	send(
		rowRecogTestEvent{Symbol: "E11", Price: 1},
		rowRecogTestEvent{Symbol: "E12", Price: 1},
		rowRecogTestEvent{Symbol: "E13", Price: 1},
		rowRecogTestEvent{Symbol: "E14", Price: 1},
		rowRecogTestEvent{Symbol: "E15", Price: 3},
	)
	assertRow(1, "E11", "E12", "E13", "E14", "E15")

	send(rowRecogTestEvent{Symbol: "E16", Price: 1}, rowRecogTestEvent{Symbol: "E17", Price: 3})
	assertRow(2, "", "", "", "E16", "E17")

	send(rowRecogTestEvent{Symbol: "E18", Price: 3})
	if len(*rows) != 4 {
		t.Fatalf("reluctant zero-to-many final rows = %#v, want four", *rows)
	}
	assertRow(3, "", "", "", "", "E18")
}

func TestRowRecogGreedynessReluctantOneToMany(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	price := Field[rowRecogTestEvent, float64]("price")
	query := stream.MatchRecognize(RowSequence(
		RowVar("A").OneOrMore().Reluctant(),
		RowVar("B").Optional(),
		RowVar("C"),
	)).
		Define("A", Equal[float64](price, Literal(1.0))).
		Define("B", Or(Equal[float64](price, Literal(1.0)), Equal[float64](price, Literal(2.0)))).
		Define("C", Equal[float64](price, Literal(3.0))).
		Measures(
			Alias("a0", TagFieldAt[string]("A", 0, "symbol")),
			Alias("a1", TagFieldAt[string]("A", 1, "symbol")),
			Alias("a2", TagFieldAt[string]("A", 2, "symbol")),
			Alias("b", TagField[string]("B", "symbol")),
			Alias("c", TagField[string]("C", "symbol")),
		).
		Query(StatementName("rowrecog-reluctant-one-to-many"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	send := func(events ...rowRecogTestEvent) {
		t.Helper()
		for _, event := range events {
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
	}
	assertRow := func(index int, a0, a1, a2, b, c string) {
		t.Helper()
		if len(*rows) <= index {
			t.Fatalf("reluctant one-to-many rows = %#v, want row %d", *rows, index)
		}
		row := (*rows)[index]
		values := map[string]string{"a0": a0, "a1": a1, "a2": a2, "b": b, "c": c}
		for name, expected := range values {
			value := row.Get(name)
			if expected == "" {
				if value.IsPresent() {
					t.Fatalf("reluctant one-to-many row %d %s = %#v, want null", index, name, value)
				}
			} else if value.Any() != expected {
				t.Fatalf("reluctant one-to-many row %d %s = %#v, want %s", index, name, value, expected)
			}
		}
	}

	send(
		rowRecogTestEvent{Symbol: "E1", Price: 1},
		rowRecogTestEvent{Symbol: "E2", Price: 1},
		rowRecogTestEvent{Symbol: "E3", Price: 1},
		rowRecogTestEvent{Symbol: "E4", Price: 3},
	)
	assertRow(0, "E1", "E2", "", "E3", "E4")

	send(
		rowRecogTestEvent{Symbol: "E11", Price: 1},
		rowRecogTestEvent{Symbol: "E12", Price: 1},
		rowRecogTestEvent{Symbol: "E13", Price: 1},
		rowRecogTestEvent{Symbol: "E14", Price: 1},
		rowRecogTestEvent{Symbol: "E15", Price: 3},
	)
	assertRow(1, "E11", "E12", "E13", "E14", "E15")

	send(rowRecogTestEvent{Symbol: "E16", Price: 1}, rowRecogTestEvent{Symbol: "E17", Price: 3})
	assertRow(2, "E16", "", "", "", "E17")

	send(rowRecogTestEvent{Symbol: "E18", Price: 3})
	if len(*rows) != 3 {
		t.Fatalf("reluctant one-to-many final rows = %#v, want three", *rows)
	}
}
