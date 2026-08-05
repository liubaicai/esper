package esper

import (
	"context"
	"errors"
	"testing"
)

type rowRecogRepetitionEvent struct {
	Name   string  `esper:"name"`
	Device int     `esper:"device"`
	Value  int     `esper:"value"`
	Temp   float64 `esper:"temp"`
}

func newRowRecogRepetitionTest(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[rowRecogRepetitionEvent](env, "RowRecogRepetitionEvent"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env)
}

func sendRowRecogRepetition(t *testing.T, engine *Engine, event rowRecogRepetitionEvent) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
}

func TestRowRecogRepetitionDocSamples(t *testing.T) {
	tests := []struct {
		name         string
		pattern      RowPattern
		awant        []string
		withB        bool
		expectedRows int
	}{
		{name: "exact-two", pattern: RowVar("A").Repeat(2, 2), awant: []string{"E4", "E5"}, expectedRows: 2},
		{name: "at-least-two", pattern: RowSequence(RowVar("A").Repeat(2, 0), RowVar("B")), awant: []string{"E2", "E3", "E4"}, withB: true, expectedRows: 1},
		{name: "between-two-three", pattern: RowSequence(RowVar("A").Repeat(2, 3), RowVar("B")), awant: []string{"E2", "E3", "E4"}, withB: true, expectedRows: 1},
		{name: "up-to-two", pattern: RowSequence(RowVar("A").Repeat(0, 2), RowVar("B")), awant: []string{"E3", "E4"}, withB: true, expectedRows: 1},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			env, engine := newRowRecogRepetitionTest(t)
			stream := From[rowRecogRepetitionEvent](env, "RowRecogRepetitionEvent").Window(KeepAll())
			device := Field[rowRecogRepetitionEvent, int]("device")
			temp := Field[rowRecogRepetitionEvent, float64]("temp")
			rowQuery := stream.MatchRecognize(testCase.pattern).
				PartitionBy(device).
				Define("A", GreaterOrEqual[float64](temp, Literal(100.0))).
				Measures(
					Alias("a0", TagFieldAt[string]("A", 0, "name")),
					Alias("a1", TagFieldAt[string]("A", 1, "name")),
					Alias("a2", TagFieldAt[string]("A", 2, "name")),
					Alias("aCount", TagCount("A")),
				)
			if testCase.withB {
				rowQuery = rowQuery.Define("B", GreaterOrEqual[float64](temp, Literal(102.0))).Measures(Alias("b", TagField[string]("B", "name")))
			}
			query := rowQuery.Query(StatementName("rowrecog-repetition-doc-" + testCase.name))
			plan, err := env.Build(query)
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			rows := collectRowRecogRows(t, deployment)
			for _, event := range []rowRecogRepetitionEvent{
				{Name: "E1", Device: 1, Temp: 99},
				{Name: "E2", Device: 1, Temp: 100},
				{Name: "E3", Device: 1, Temp: 100},
				{Name: "E4", Device: 1, Temp: 101},
				{Name: "E5", Device: 1, Temp: 102},
			} {
				sendRowRecogRepetition(t, engine, event)
			}
			if len(*rows) != testCase.expectedRows {
				t.Fatalf("repetition doc rows = %#v, want %d rows", *rows, testCase.expectedRows)
			}
			row := (*rows)[len(*rows)-1]
			if row.Get("a0").Any() != testCase.awant[0] || row.Get("aCount").Any() != int64(len(testCase.awant)) {
				t.Fatalf("repetition doc row = %#v, want A=%v", *rows, testCase.awant)
			}
			if testCase.withB && row.Get("b").Any() != "E5" {
				t.Fatalf("repetition doc B row = %#v, want E5", *rows)
			}
			for index, name := range testCase.awant[1:] {
				if index >= 2 {
					break
				}
				field := "a" + string(rune('0'+index+1))
				if row.Get(field).Any() != name {
					t.Fatalf("repetition doc %s = %#v, want %s=%s", field, *rows, field, name)
				}
			}
			if testCase.name == "exact-two" && ((*rows)[0].Get("a0").Any() != "E2" || (*rows)[0].Get("a1").Any() != "E3") {
				t.Fatalf("exact repetition first batch = %#v", *rows)
			}
		})
	}
}

func TestRowRecogRepetitionPrev(t *testing.T) {
	env, engine := newRowRecogRepetitionTest(t)
	stream := From[rowRecogRepetitionEvent](env, "RowRecogRepetitionEvent").Window(KeepAll())
	value := Field[rowRecogRepetitionEvent, int]("value")
	query := stream.MatchRecognize(RowVar("A").Repeat(3, 3)).
		Define("A", Greater[int](value, Prev[int](1, value))).
		Measures(Alias("a", TagEvents("A"))).
		Query(StatementName("rowrecog-repetition-prev"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogRepetitionEvent{
		{Name: "A1", Value: 1},
		{Name: "A2", Value: 4},
		{Name: "A3", Value: 2},
		{Name: "A4", Value: 6},
		{Name: "A5", Value: 5},
		{Name: "A6", Value: 6},
		{Name: "A7", Value: 7},
		{Name: "A9", Value: 8},
	} {
		sendRowRecogRepetition(t, engine, event)
	}
	if len(*rows) != 1 {
		t.Fatalf("repetition PREV rows = %#v", *rows)
	}
	events, ok := (*rows)[0].Get("a").Any().([]Event)
	if !ok || len(events) != 3 || events[0].Underlying().(rowRecogRepetitionEvent).Name != "A6" ||
		events[1].Underlying().(rowRecogRepetitionEvent).Name != "A7" || events[2].Underlying().(rowRecogRepetitionEvent).Name != "A9" {
		t.Fatalf("repetition PREV capture = %#v", (*rows)[0].Get("a").Any())
	}
}

func TestRowRecogRepetitionNestedAndRanges(t *testing.T) {
	tests := []struct {
		name    string
		pattern RowPattern
		values  []rowRecogRepetitionEvent
	}{
		{
			name:    "nested-exact",
			pattern: RowSequence(RowSequence(RowVar("A"), RowVar("B")).Repeat(2, 2), RowVar("C")),
			values: []rowRecogRepetitionEvent{
				{Name: "A1"}, {Name: "B1"}, {Name: "A2"}, {Name: "B2"}, {Name: "C1"},
			},
		},
		{
			name:    "nested-range",
			pattern: RowSequence(RowSequence(RowVar("A"), RowVar("B")).Repeat(1, 2), RowVar("C")),
			values: []rowRecogRepetitionEvent{
				{Name: "A1"}, {Name: "B1"}, {Name: "A2"}, {Name: "B2"}, {Name: "C1"},
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			env, engine := newRowRecogRepetitionTest(t)
			stream := From[rowRecogRepetitionEvent](env, "RowRecogRepetitionEvent").Window(KeepAll())
			name := Field[rowRecogRepetitionEvent, string]("name")
			query := stream.MatchRecognize(testCase.pattern).
				Define("A", StartsWith(name, Literal("A"))).
				Define("B", StartsWith(name, Literal("B"))).
				Define("C", StartsWith(name, Literal("C"))).
				Measures(
					Alias("a0", TagFieldAt[string]("A", 0, "name")),
					Alias("a1", TagFieldAt[string]("A", 1, "name")),
					Alias("b0", TagFieldAt[string]("B", 0, "name")),
					Alias("b1", TagFieldAt[string]("B", 1, "name")),
					Alias("c", TagField[string]("C", "name")),
				).
				Query(StatementName("rowrecog-repetition-nested-" + testCase.name))
			plan, err := env.Build(query)
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			rows := collectRowRecogRows(t, deployment)
			for _, event := range testCase.values {
				sendRowRecogRepetition(t, engine, event)
			}
			if len(*rows) != 1 || (*rows)[0].Get("a0").Any() != "A1" || (*rows)[0].Get("a1").Any() != "A2" ||
				(*rows)[0].Get("b0").Any() != "B1" || (*rows)[0].Get("b1").Any() != "B2" || (*rows)[0].Get("c").Any() != "C1" {
				t.Fatalf("nested repetition row = %#v", *rows)
			}
		})
	}
}

func TestRowRecogRepetitionEquivalentExecution(t *testing.T) {
	env, engine := newRowRecogRepetitionTest(t)
	stream := From[rowRecogRepetitionEvent](env, "RowRecogRepetitionEvent").Window(KeepAll())
	name := Field[rowRecogRepetitionEvent, string]("name")
	makeQuery := func(pattern RowPattern, statementName string) Query {
		return stream.MatchRecognize(pattern).
			Define("A", StartsWith(name, Literal("A"))).
			Define("B", StartsWith(name, Literal("B"))).
			Measures(
				Alias("a0", TagFieldAt[string]("A", 0, "name")),
				Alias("a1", TagFieldAt[string]("A", 1, "name")),
				Alias("b", TagField[string]("B", "name")),
			).
			Query(StatementName(statementName))
	}
	repeatedPlan, err := env.Build(makeQuery(RowSequence(RowVar("A").Repeat(2, 2), RowVar("B")), "rowrecog-repeat-equivalent"))
	if err != nil {
		t.Fatal(err)
	}
	expandedPlan, err := env.Build(makeQuery(RowSequence(RowVar("A"), RowVar("A"), RowVar("B")), "rowrecog-repeat-expanded"))
	if err != nil {
		t.Fatal(err)
	}
	repeatedDeployment, err := engine.Deploy(context.Background(), repeatedPlan)
	if err != nil {
		t.Fatal(err)
	}
	expandedDeployment, err := engine.Deploy(context.Background(), expandedPlan)
	if err != nil {
		t.Fatal(err)
	}
	repeatedRows := collectRowRecogRows(t, repeatedDeployment)
	expandedRows := collectRowRecogRows(t, expandedDeployment)
	for _, event := range []rowRecogRepetitionEvent{{Name: "A1"}, {Name: "A2"}, {Name: "B1"}} {
		sendRowRecogRepetition(t, engine, event)
	}
	if len(*repeatedRows) != 1 || len(*expandedRows) != 1 || (*repeatedRows)[0].Get("a0").Any() != (*expandedRows)[0].Get("a0").Any() ||
		(*repeatedRows)[0].Get("a1").Any() != (*expandedRows)[0].Get("a1").Any() || (*repeatedRows)[0].Get("b").Any() != (*expandedRows)[0].Get("b").Any() {
		t.Fatalf("equivalent repetition rows = %#v / %#v", *repeatedRows, *expandedRows)
	}
}

func TestRowRecogRepetitionInvalidBounds(t *testing.T) {
	env, _ := newRowRecogRepetitionTest(t)
	stream := From[rowRecogRepetitionEvent](env, "RowRecogRepetitionEvent")
	makeQuery := func(pattern RowPattern) Query {
		return stream.MatchRecognize(pattern).
			Measures(Alias("a", TagField[string]("A", "name"))).
			Query()
	}
	invalid := []Query{
		makeQuery(RowVar("A").Repeat(-1, 0)),
		makeQuery(RowVar("A").Repeat(5, 3)),
		makeQuery(RowSequence(RowVar("A")).Repeat(-1, 2)),
	}
	for index, query := range invalid {
		if _, err := env.Build(query); err == nil || !errors.Is(err, ErrorInvalidRule) {
			t.Errorf("invalid repetition %d error = %v", index, err)
		}
	}
}
