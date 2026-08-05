package esper

import (
	"context"
	"errors"
	"testing"
)

type rowRecogArrayAccessEvent struct {
	Name     string  `esper:"name"`
	Category string  `esper:"category"`
	Value    int     `esper:"value"`
	Double   float64 `esper:"double"`
}

func newRowRecogArrayAccessTest(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[rowRecogArrayAccessEvent](env, "RowRecogArrayAccessEvent"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env)
}

func sendRowRecogArrayAccess(t *testing.T, engine *Engine, event rowRecogArrayAccessEvent) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
}

func TestRowRecogArrayAccessSingleMultiMix(t *testing.T) {
	env, engine := newRowRecogArrayAccessTest(t)
	stream := From[rowRecogArrayAccessEvent](env, "RowRecogArrayAccessEvent")
	name := Field[rowRecogArrayAccessEvent, string]("name")
	value := Field[rowRecogArrayAccessEvent, int]("value")
	query := stream.MatchRecognize(RowSequence(
		RowVar("A"),
		RowVar("B").OneOrMore(),
		RowVar("C"),
		RowVar("D").OneOrMore(),
		RowVar("E"),
	)).
		Define("A", StartsWith(name, Literal("A"))).
		Define("B", StartsWith(name, Literal("B"))).
		Define("C", And(
			StartsWith(name, Literal("C")),
			Equal[int](value, TagFieldAt[int]("B", 1, "value")),
		)).
		Define("D", StartsWith(name, Literal("D"))).
		Define("E", And(
			StartsWith(name, Literal("E")),
			And(
				Equal[int](value, TagFieldAt[int]("D", 1, "value")),
				Equal[int](value, TagFieldAt[int]("D", 0, "value")),
			),
		)).
		Measures(
			Alias("a", TagField[string]("A", "name")),
			Alias("b0", TagFieldAt[string]("B", 0, "name")),
			Alias("c", TagField[string]("C", "name")),
			Alias("d0", TagFieldAt[string]("D", 0, "name")),
			Alias("e", TagField[string]("E", "name")),
		).
		Query(StatementName("rowrecog-array-single-multi"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)

	for _, event := range []rowRecogArrayAccessEvent{
		{Name: "A1", Value: 100},
		{Name: "B1", Value: 50},
		{Name: "B2", Value: 49},
		{Name: "C1", Value: 49},
		{Name: "D1", Value: 2},
		{Name: "D2", Value: 2},
		{Name: "E1", Value: 2},
	} {
		sendRowRecogArrayAccess(t, engine, event)
	}
	if len(*rows) != 1 || (*rows)[0].Get("a").Any() != "A1" || (*rows)[0].Get("b0").Any() != "B1" ||
		(*rows)[0].Get("c").Any() != "C1" || (*rows)[0].Get("d0").Any() != "D1" || (*rows)[0].Get("e").Any() != "E1" {
		t.Fatalf("single/multi mix first row = %#v", *rows)
	}

	before := len(*rows)
	for _, event := range []rowRecogArrayAccessEvent{
		{Name: "A2", Value: 100},
		{Name: "B3", Value: 50},
		{Name: "C2", Value: 49},
		{Name: "D4", Value: 2},
		{Name: "D5", Value: 2},
		{Name: "E2", Value: 2},
	} {
		sendRowRecogArrayAccess(t, engine, event)
	}
	if len(*rows) != before {
		t.Fatalf("single/multi mix missing B[1] unexpectedly emitted: %#v", *rows)
	}

	for _, event := range []rowRecogArrayAccessEvent{
		{Name: "A3", Value: 100},
		{Name: "B4", Value: 50},
		{Name: "B5", Value: 49},
		{Name: "C3", Value: 49},
		{Name: "D6", Value: 2},
		{Name: "D7", Value: 3},
		{Name: "E3", Value: 2},
	} {
		sendRowRecogArrayAccess(t, engine, event)
	}
	if len(*rows) != before {
		t.Fatalf("single/multi mix mismatched D[1] unexpectedly emitted: %#v", *rows)
	}

	for _, event := range []rowRecogArrayAccessEvent{
		{Name: "A4", Value: 100},
		{Name: "B6", Value: 50},
		{Name: "B7", Value: 49},
		{Name: "C4", Value: 49},
		{Name: "D8", Value: 2},
		{Name: "D9", Value: 2},
		{Name: "D10", Value: 99},
		{Name: "E4", Value: 2},
	} {
		sendRowRecogArrayAccess(t, engine, event)
	}
	if len(*rows) != before+1 || (*rows)[len(*rows)-1].Get("a").Any() != "A4" || (*rows)[len(*rows)-1].Get("b0").Any() != "B6" {
		t.Fatalf("single/multi mix repeated D row = %#v", *rows)
	}
}

func TestRowRecogArrayAccessMultiDepends(t *testing.T) {
	patterns := map[string]RowPattern{
		"sequence": RowSequence(RowVar("A"), RowVar("B"), RowVar("A"), RowVar("B"), RowVar("C")),
		"repeated": RowSequence(RowSequence(RowVar("A"), RowVar("B")).ZeroOrMore(), RowVar("C")),
	}
	for name, pattern := range patterns {
		t.Run(name, func(t *testing.T) {
			env, engine := newRowRecogArrayAccessTest(t)
			stream := From[rowRecogArrayAccessEvent](env, "RowRecogArrayAccessEvent")
			eventName := Field[rowRecogArrayAccessEvent, string]("name")
			value := Field[rowRecogArrayAccessEvent, int]("value")
			equalTo := func(tag string, index int) Expression[bool] {
				return Equal[int](value, TagFieldAt[int](tag, index, "value"))
			}
			cPredicate := And(
				StartsWith(eventName, Literal("C")),
				And(equalTo("A", 0), And(equalTo("B", 0), And(equalTo("A", 1), equalTo("B", 1)))),
			)
			query := stream.MatchRecognize(pattern).
				Define("A", StartsWith(eventName, Literal("A"))).
				Define("B", StartsWith(eventName, Literal("B"))).
				Define("C", cPredicate).
				Measures(
					Alias("a0", TagFieldAt[string]("A", 0, "name")),
					Alias("a1", TagFieldAt[string]("A", 1, "name")),
					Alias("b0", TagFieldAt[string]("B", 0, "name")),
					Alias("b1", TagFieldAt[string]("B", 1, "name")),
					Alias("c", TagField[string]("C", "name")),
				).
				Query(StatementName("rowrecog-array-multi-depends-" + name))
			plan, err := env.Build(query)
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			rows := collectRowRecogRows(t, deployment)
			for _, event := range []rowRecogArrayAccessEvent{
				{Name: "A1", Value: 1},
				{Name: "B1", Value: 1},
				{Name: "A2", Value: 1},
				{Name: "B2", Value: 1},
				{Name: "C1", Value: 1},
			} {
				sendRowRecogArrayAccess(t, engine, event)
			}
			if len(*rows) != 1 || (*rows)[0].Get("a0").Any() != "A1" || (*rows)[0].Get("a1").Any() != "A2" ||
				(*rows)[0].Get("b0").Any() != "B1" || (*rows)[0].Get("b1").Any() != "B2" || (*rows)[0].Get("c").Any() != "C1" {
				t.Fatalf("multi-depends row = %#v", *rows)
			}
			before := len(*rows)
			for _, event := range []rowRecogArrayAccessEvent{
				{Name: "A10", Value: 1},
				{Name: "B10", Value: 1},
				{Name: "A11", Value: 1},
				{Name: "B11", Value: 2},
				{Name: "C2", Value: 2},
			} {
				sendRowRecogArrayAccess(t, engine, event)
			}
			if len(*rows) != before {
				t.Fatalf("multi-depends invalid row = %#v", *rows)
			}
		})
	}
}

func TestRowRecogArrayAccessMeasuresClausePresence(t *testing.T) {
	cases := []struct {
		name     string
		measures []Selection
	}{
		{
			name: "array-and-single",
			measures: []Selection{
				Alias("a_array", TagEvents("A")),
				Alias("b", TagField[string]("B", "name")),
			},
		},
		{name: "single", measures: []Selection{Alias("b", TagField[string]("B", "name"))}},
		{name: "array", measures: []Selection{Alias("a_array", TagEvents("A"))}},
		{name: "constant", measures: []Selection{Alias("one", Literal(1))}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env, engine := newRowRecogArrayAccessTest(t)
			stream := From[rowRecogArrayAccessEvent](env, "RowRecogArrayAccessEvent")
			name := Field[rowRecogArrayAccessEvent, string]("name")
			value := Field[rowRecogArrayAccessEvent, int]("value")
			query := stream.MatchRecognize(RowSequence(RowVar("A").OneOrMore(), RowVar("B"))).
				PartitionBy(name).
				Define("B", Equal[int](value, TagFieldAt[int]("A", 0, "value"))).
				Measures(testCase.measures...).
				Query(StatementName("rowrecog-array-measures-" + testCase.name))
			plan, err := env.Build(query)
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			rows := collectRowRecogRows(t, deployment)
			for _, event := range []rowRecogArrayAccessEvent{
				{Name: "A", Value: 1},
				{Name: "A", Value: 0},
				{Name: "B", Value: 1},
				{Name: "B", Value: 1},
				{Name: "A", Value: 2},
				{Name: "A", Value: 3},
				{Name: "B", Value: 2},
				{Name: "B", Value: 2},
			} {
				sendRowRecogArrayAccess(t, engine, event)
			}
			if len(*rows) != 2 {
				t.Fatalf("measure clause rows = %#v", *rows)
			}
			for _, row := range *rows {
				switch testCase.name {
				case "array-and-single", "array":
					events, ok := row.Get("a_array").Any().([]Event)
					if !ok || len(events) != 1 {
						t.Fatalf("array measure = %#v", row.Get("a_array").Any())
					}
				}
				if testCase.name == "array-and-single" || testCase.name == "single" {
					if !row.Get("b").IsPresent() {
						t.Fatalf("single measure missing = %#v", row)
					}
				}
				if testCase.name == "constant" && row.Get("one").Any() != 1 {
					t.Fatalf("constant measure = %#v", row.Get("one").Any())
				}
			}
		})
	}
}

func TestRowRecogArrayAccessLambda(t *testing.T) {
	env, engine := newRowRecogArrayAccessTest(t)
	stream := From[rowRecogArrayAccessEvent](env, "RowRecogArrayAccessEvent")
	value := Field[rowRecogArrayAccessEvent, int]("value")
	query := stream.MatchRecognize(RowSequence(RowVar("A").ZeroOrMore(), RowVar("B"))).
		Define("B", Greater[int](Add[int](Coalesce[int](TagSum[int]("A", value), Literal(0)), value), Literal(100))).
		Measures(
			Alias("a0", TagFieldAt[string]("A", 0, "name")),
			Alias("a1", TagFieldAt[string]("A", 1, "name")),
			Alias("a2", TagFieldAt[string]("A", 2, "name")),
			Alias("b", TagField[string]("B", "name")),
		).
		Query(StatementName("rowrecog-array-lambda"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogArrayAccessEvent{
		{Name: "E1", Value: 50},
		{Name: "E2", Value: 49},
		{Name: "E3", Value: 2},
	} {
		sendRowRecogArrayAccess(t, engine, event)
	}
	if len(*rows) != 1 || (*rows)[0].Get("a0").Any() != "E1" || (*rows)[0].Get("a1").Any() != "E2" ||
		(*rows)[0].Get("a2").IsPresent() || (*rows)[0].Get("b").Any() != "E3" {
		t.Fatalf("lambda first row = %#v", *rows)
	}

	for _, event := range []rowRecogArrayAccessEvent{
		{Name: "E4", Value: 101},
		{Name: "E5", Value: 50},
		{Name: "E6", Value: 51},
	} {
		sendRowRecogArrayAccess(t, engine, event)
	}
	if len(*rows) != 3 || (*rows)[1].Get("a0").IsPresent() || (*rows)[1].Get("b").Any() != "E4" ||
		(*rows)[2].Get("a0").Any() != "E5" || (*rows)[2].Get("a1").IsPresent() || (*rows)[2].Get("b").Any() != "E6" {
		t.Fatalf("lambda subsequent rows = %#v", *rows)
	}

	invalid := stream.MatchRecognize(RowSequence(RowVar("A"))).
		Measures(Alias("sum", TagSum[int]("Unknown", value))).
		Query(StatementName("rowrecog-array-lambda-invalid"))
	if _, err := env.Build(invalid); err == nil || !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("lambda unknown tag error = %v", err)
	}
}

func TestRowRecogEnumMethodFirstOfEquivalent(t *testing.T) {
	env, engine := newRowRecogArrayAccessTest(t)
	stream := From[rowRecogArrayAccessEvent](env, "RowRecogArrayAccessEvent")
	name := Field[rowRecogArrayAccessEvent, string]("name")
	value := Field[rowRecogArrayAccessEvent, int]("value")
	double := Field[rowRecogArrayAccessEvent, float64]("double")
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B").OneOrMore(), RowVar("C"))).
		PartitionBy(name).
		Define("B", Greater[int](value, TagField[int]("A", "value"))).
		Define("C", Greater[float64](double, TagFieldAt[int]("B", 0, "value"))).
		Measures(
			Alias("c0", TagField[string]("A", "name")),
			Alias("c1", TagField[int]("C", "value")),
		).
		Query(StatementName("rowrecog-enum-first-of"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogArrayAccessEvent{
		{Name: "E1", Value: 10, Double: 0},
		{Name: "E1", Value: 11, Double: 50},
		{Name: "E1", Value: 12, Double: 11},
		{Name: "E2", Value: 10, Double: 0},
		{Name: "E2", Value: 11, Double: 50},
		{Name: "E2", Value: 12, Double: 12},
	} {
		sendRowRecogArrayAccess(t, engine, event)
	}
	if len(*rows) != 1 || (*rows)[0].Get("c0").Any() != "E2" || (*rows)[0].Get("c1").Any() != 12 {
		t.Fatalf("enum first-of equivalent rows = %#v", *rows)
	}
}
