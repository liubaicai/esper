package esper

import (
	"context"
	"testing"
	"time"
)

func TestRowRecogClausePresenceMeasures(t *testing.T) {
	cases := []struct {
		name     string
		measure  Selection
		expected any
	}{
		{
			name:     "size",
			measure:  Alias("val", TagSize("B")),
			expected: int64(1),
		},
		{
			name:     "size-arithmetic",
			measure:  Alias("val", Add[int64](Literal(int64(100)), TagSize("B"))),
			expected: int64(101),
		},
		{
			name: "any-of",
			measure: Alias("val", TagAny("B", Equal[string](
				Field[rowRecogPreviousEvent, string]("name"), Literal("E2"),
			))),
			expected: true,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env, engine := newRowRecogTimedPreviousTest(t)
			stream := From[rowRecogPreviousEvent](env, "RowRecogPreviousEvent")
			value := Field[rowRecogPreviousEvent, int]("value")
			query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B").ZeroOrMore())).
				Define("A", Equal[int](value, Literal(1))).
				Define("B", Equal[int](value, Literal(2))).
				Interval(time.Minute).
				Measures(
					Alias("a", ArrayAt[Event](TagEvents("A"), Literal(int64(0)))),
					Alias("id", TagField[string]("A", "name")),
					testCase.measure,
				).
				Query(StatementName("rowrecog-clause-presence-" + testCase.name))
			plan, err := env.Build(query)
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			rows := collectRowRecogRows(t, deployment)
			sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "E1", Value: 1})
			sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "E2", Value: 2})
			if len(*rows) != 0 {
				t.Fatalf("clause-presence emitted before interval = %#v", *rows)
			}
			advanceRowRecogPrevious(t, engine, 2*60*1000)
			if len(*rows) != 1 {
				t.Fatalf("clause-presence listener rows = %#v", *rows)
			}
			row := (*rows)[0]
			if row.Get("id").Any() != "E1" || row.Get("val").Any() != testCase.expected {
				t.Fatalf("clause-presence row = %#v, want id E1 and val %#v", row, testCase.expected)
			}
			a, ok := row.Get("a").Any().(Event)
			if !ok || a.Get("name").Any() != "E1" {
				t.Fatalf("clause-presence event measure = %#v", row.Get("a"))
			}
		})
	}
}

func TestRowRecogClausePresenceAllowsOmittedDefines(t *testing.T) {
	env, engine := newRowRecogTimedPreviousTest(t)
	stream := From[rowRecogPreviousEvent](env, "RowRecogPreviousEvent")
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		Measures(
			Alias("a", ArrayAt[Event](TagEvents("A"), Literal(int64(0)))),
			Alias("b", ArrayAt[Event](TagEvents("B"), Literal(int64(0)))),
		).
		Query(StatementName("rowrecog-clause-presence-omitted-defines"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogPreviousEvent{
		{Name: "E0", Value: 0},
		{Name: "E1", Value: 1},
		{Name: "E2", Value: 2},
		{Name: "E3", Value: 3},
	} {
		sendRowRecogPrevious(t, engine, event)
	}
	if len(*rows) != 2 {
		t.Fatalf("omitted DEFINE rows = %#v", *rows)
	}
	for index, names := range [][2]string{{"E0", "E1"}, {"E2", "E3"}} {
		first, firstOK := (*rows)[index].Get("a").Any().(Event)
		second, secondOK := (*rows)[index].Get("b").Any().(Event)
		if !firstOK || !secondOK || first.Get("name").Any() != names[0] || second.Get("name").Any() != names[1] {
			t.Fatalf("omitted DEFINE row %d = %#v", index, (*rows)[index])
		}
	}
}

func TestRowRecogEmptyPartitionLifecycle(t *testing.T) {
	env, engine := newRowRecogTimedPreviousTest(t)
	stream := From[rowRecogPreviousEvent](env, "RowRecogPreviousEvent").Window(LengthWindow(10))
	name := Field[rowRecogPreviousEvent, string]("name")
	value := Field[rowRecogPreviousEvent, int]("value")
	query := stream.MatchRecognize(RowAlternation(
		RowSequence(RowVar("E1"), RowVar("E2")),
		RowSequence(RowVar("E2"), RowVar("E1")),
	)).
		PartitionBy(value).
		Define("E1", Equal[string](name, Literal("A"))).
		Define("E2", Equal[string](name, Literal("B"))).
		Measures(Alias("value", TagField[int]("E1", "value"))).
		Query(StatementName("rowrecog-empty-partition"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogPreviousEvent{
		{Name: "A", Value: 1},
		{Name: "B", Value: 1},
		{Name: "B", Value: 2},
		{Name: "A", Value: 2},
		{Name: "B", Value: 3},
		{Name: "A", Value: 4},
		{Name: "A", Value: 3},
		{Name: "B", Value: 4},
		{Name: "A", Value: 6},
		{Name: "B", Value: 7},
		{Name: "B", Value: 8},
		{Name: "A", Value: 7},
	} {
		sendRowRecogPrevious(t, engine, event)
	}
	want := []int{1, 2, 3, 4, 7}
	if len(*rows) != len(want) {
		t.Fatalf("empty-partition listener rows = %#v", *rows)
	}
	for index, expected := range want {
		if (*rows)[index].Get("value").Any() != expected {
			t.Fatalf("empty-partition row %d = %#v, want %d", index, (*rows)[index], expected)
		}
	}
	for value := 0; value < 10000; value++ {
		sendRowRecogPrevious(t, engine, rowRecogPreviousEvent{Name: "A", Value: value + 100})
	}
	if len(*rows) != len(want) {
		t.Fatalf("empty-partition churn changed listener rows = %#v", *rows)
	}
}

func TestRowRecogArrayAccessLambdaAPlusB(t *testing.T) {
	env, engine := newRowRecogArrayAccessTest(t)
	stream := From[rowRecogArrayAccessEvent](env, "RowRecogArrayAccessEvent")
	name := Field[rowRecogArrayAccessEvent, string]("name")
	value := Field[rowRecogArrayAccessEvent, int]("value")
	query := stream.MatchRecognize(RowSequence(RowVar("A").OneOrMore(), RowVar("B"))).
		Define("A", StartsWith(name, Literal("A"))).
		Define("B", And(
			StartsWith(name, Literal("B")),
			Greater[int](value, TagSum[int]("A", value)),
		)).
		Measures(
			Alias("a0", TagFieldAt[string]("A", 0, "name")),
			Alias("a1", TagFieldAt[string]("A", 1, "name")),
			Alias("b", TagField[string]("B", "name")),
		).
		Query(StatementName("rowrecog-array-lambda-a-plus-b"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	send := func(events ...rowRecogArrayAccessEvent) {
		t.Helper()
		for _, event := range events {
			sendRowRecogArrayAccess(t, engine, event)
		}
	}
	assert := func(index int, a0, a1, b string) {
		t.Helper()
		if len(*rows) <= index || (*rows)[index].Get("a0").Any() != a0 || (*rows)[index].Get("b").Any() != b {
			t.Fatalf("A+ B row %d = %#v, want %s/%s", index, *rows, a0, b)
		}
		if a1 == "" {
			if (*rows)[index].Get("a1").IsPresent() {
				t.Fatalf("A+ B row %d a1 = %#v, want null", index, (*rows)[index].Get("a1"))
			}
		} else if (*rows)[index].Get("a1").Any() != a1 {
			t.Fatalf("A+ B row %d a1 = %#v, want %s", index, (*rows)[index].Get("a1"), a1)
		}
	}
	send(rowRecogArrayAccessEvent{Name: "A1", Value: 1}, rowRecogArrayAccessEvent{Name: "A2", Value: 2}, rowRecogArrayAccessEvent{Name: "B1", Value: 3})
	assert(0, "A2", "", "B1")
	send(rowRecogArrayAccessEvent{Name: "A3", Value: 1}, rowRecogArrayAccessEvent{Name: "A4", Value: 2}, rowRecogArrayAccessEvent{Name: "B2", Value: 4})
	assert(1, "A3", "A4", "B2")
	send(rowRecogArrayAccessEvent{Name: "A5", Value: -1}, rowRecogArrayAccessEvent{Name: "B3", Value: 0})
	assert(2, "A5", "", "B3")
	send(rowRecogArrayAccessEvent{Name: "A6", Value: 10}, rowRecogArrayAccessEvent{Name: "B3", Value: 9}, rowRecogArrayAccessEvent{Name: "B4", Value: 11})
	if len(*rows) != 3 {
		t.Fatalf("A+ B row after A6/B4 = %#v", *rows)
	}
	send(rowRecogArrayAccessEvent{Name: "A7", Value: 10}, rowRecogArrayAccessEvent{Name: "A8", Value: 9}, rowRecogArrayAccessEvent{Name: "A9", Value: 8})
	if len(*rows) != 3 {
		t.Fatalf("A+ B emitted while waiting for B5 = %#v", *rows)
	}
	send(rowRecogArrayAccessEvent{Name: "B5", Value: 18})
	if len(*rows) != 4 {
		t.Fatalf("A+ B row after B5 = %#v", *rows)
	}
	assert(3, "A8", "A9", "B5")
	send(rowRecogArrayAccessEvent{Name: "A0", Value: 10}, rowRecogArrayAccessEvent{Name: "A11", Value: 9}, rowRecogArrayAccessEvent{Name: "A12", Value: 8}, rowRecogArrayAccessEvent{Name: "B6", Value: 8})
	if len(*rows) != 4 {
		t.Fatalf("A+ B invalid B6 emitted = %#v", *rows)
	}
	send(rowRecogArrayAccessEvent{Name: "A13", Value: 1}, rowRecogArrayAccessEvent{Name: "A14", Value: 1}, rowRecogArrayAccessEvent{Name: "A15", Value: 1}, rowRecogArrayAccessEvent{Name: "A16", Value: 1}, rowRecogArrayAccessEvent{Name: "B7", Value: 5})
	if len(*rows) != 5 {
		t.Fatalf("A+ B row after B7 = %#v", *rows)
	}
	assert(4, "A13", "A14", "B7")
	send(rowRecogArrayAccessEvent{Name: "A17", Value: 1}, rowRecogArrayAccessEvent{Name: "A18", Value: 1}, rowRecogArrayAccessEvent{Name: "B8", Value: 1})
	if len(*rows) != 5 {
		t.Fatalf("A+ B invalid B8 emitted = %#v", *rows)
	}
}

func newRowRecogTimedPreviousTest(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[rowRecogPreviousEvent](env, "RowRecogPreviousEvent"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
}
