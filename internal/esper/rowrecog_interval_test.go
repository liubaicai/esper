package esper

import (
	"context"
	"testing"
	"time"
)

func TestRowRecogIntervalSimpleTrace(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	symbol := Field[rowRecogTestEvent, string]("symbol")
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B").ZeroOrMore())).
		Define("A", StartsWith(symbol, Literal("A"))).
		Define("B", StartsWith(symbol, Literal("B"))).
		Interval(10*time.Second).
		Measures(
			Alias("a", TagField[string]("A", "symbol")),
			Alias("b0", TagFieldAt[string]("B", 0, "symbol")),
			Alias("b1", TagFieldAt[string]("B", 1, "symbol")),
			Alias("lastb", TagLast[string]("B", symbol)),
		).
		Query(
			StatementName("rowrecog-interval-simple-trace"),
			OrderBy(
				Ascending(ResultField[string]("a")),
				Ascending(ResultField[string]("b0")),
				Ascending(ResultField[string]("b1")),
				Ascending(ResultField[string]("lastb")),
			),
		)
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	statement := deployment.Statements()[0]
	send := func(event rowRecogTestEvent, listener, snapshot []string) {
		t.Helper()
		before := len(*rows)
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		assertRowRecogAfterState(t, rows, statement, before, listener, snapshot, "a", "b0", "b1", "lastb")
	}
	advance := func(milliseconds int64, listener, snapshot []string) {
		t.Helper()
		before := len(*rows)
		if err := engine.AdvanceTime(context.Background(), time.Unix(0, milliseconds*int64(time.Millisecond)).UTC()); err != nil {
			t.Fatal(err)
		}
		assertRowRecogAfterState(t, rows, statement, before, listener, snapshot, "a", "b0", "b1", "lastb")
	}

	advance(1000, nil, nil)
	send(rowRecogTestEvent{Symbol: "A1"}, nil, []string{"A1///"})
	advance(10999, nil, []string{"A1///"})
	advance(11000, []string{"A1///"}, []string{"A1///"})
	advance(13000, nil, []string{"A1///"})
	send(rowRecogTestEvent{Symbol: "A2"}, nil, []string{"A1///", "A2///"})
	advance(15000, nil, []string{"A1///", "A2///"})
	send(rowRecogTestEvent{Symbol: "B1"}, nil, []string{"A1///", "A2/B1//B1"})
	advance(22999, nil, []string{"A1///", "A2/B1//B1"})
	advance(23000, []string{"A2/B1//B1"}, []string{"A1///", "A2/B1//B1"})
	advance(25000, nil, []string{"A1///", "A2/B1//B1"})
	send(rowRecogTestEvent{Symbol: "A3"}, nil, []string{"A1///", "A2/B1//B1", "A3///"})
	advance(26000, nil, []string{"A1///", "A2/B1//B1", "A3///"})
	send(rowRecogTestEvent{Symbol: "B2"}, nil, []string{"A1///", "A2/B1//B1", "A3/B2//B2"})
	advance(29000, nil, []string{"A1///", "A2/B1//B1", "A3/B2//B2"})
	send(rowRecogTestEvent{Symbol: "B3"}, nil, []string{"A1///", "A2/B1//B1", "A3/B2/B3/B3"})
	advance(34999, nil, []string{"A1///", "A2/B1//B1", "A3/B2/B3/B3"})
	send(rowRecogTestEvent{Symbol: "B4"}, nil, []string{"A1///", "A2/B1//B1", "A3/B2/B3/B4"})
	advance(35000, []string{"A3/B2/B3/B4"}, []string{"A1///", "A2/B1//B1", "A3/B2/B3/B4"})
}

func TestRowRecogIntervalPartitionedTrace(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	symbol := Field[rowRecogTestEvent, string]("symbol")
	group := Field[rowRecogTestEvent, string]("group")
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B").ZeroOrMore())).
		PartitionBy(group).
		Define("A", StartsWith(symbol, Literal("A"))).
		Define("B", StartsWith(symbol, Literal("B"))).
		Interval(10*time.Second).
		Measures(
			Alias("a", TagField[string]("A", "symbol")),
			Alias("b0", TagFieldAt[string]("B", 0, "symbol")),
			Alias("b1", TagFieldAt[string]("B", 1, "symbol")),
			Alias("lastb", TagLast[string]("B", symbol)),
		).
		Query(
			StatementName("rowrecog-interval-partitioned-trace"),
			OrderBy(Ascending(ResultField[string]("a"))),
		)
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	statement := deployment.Statements()[0]
	send := func(event rowRecogTestEvent, listener, snapshot []string) {
		t.Helper()
		before := len(*rows)
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		assertRowRecogAfterState(t, rows, statement, before, listener, snapshot, "a", "b0", "b1", "lastb")
	}
	advance := func(milliseconds int64, listener, snapshot []string) {
		t.Helper()
		before := len(*rows)
		if err := engine.AdvanceTime(context.Background(), time.Unix(0, milliseconds*int64(time.Millisecond)).UTC()); err != nil {
			t.Fatal(err)
		}
		assertRowRecogAfterState(t, rows, statement, before, listener, snapshot, "a", "b0", "b1", "lastb")
	}

	advance(1000, nil, nil)
	send(rowRecogTestEvent{Symbol: "A1", Group: "C1"}, nil, []string{"A1///"})
	send(rowRecogTestEvent{Symbol: "A2", Group: "C2"}, nil, []string{"A1///", "A2///"})
	advance(2000, nil, []string{"A1///", "A2///"})
	send(rowRecogTestEvent{Symbol: "A3", Group: "C3"}, nil, []string{"A1///", "A2///", "A3///"})
	advance(3000, nil, []string{"A1///", "A2///", "A3///"})
	send(rowRecogTestEvent{Symbol: "A4", Group: "C4"}, nil, []string{"A1///", "A2///", "A3///", "A4///"})
	send(rowRecogTestEvent{Symbol: "B1", Group: "C3"}, nil, []string{"A1///", "A2///", "A3/B1//B1", "A4///"})
	send(rowRecogTestEvent{Symbol: "B2", Group: "C1"}, nil, []string{"A1/B2//B2", "A2///", "A3/B1//B1", "A4///"})
	send(rowRecogTestEvent{Symbol: "B3", Group: "C1"}, nil, []string{"A1/B2/B3/B3", "A2///", "A3/B1//B1", "A4///"})
	send(rowRecogTestEvent{Symbol: "B4", Group: "C4"}, nil, []string{"A1/B2/B3/B3", "A2///", "A3/B1//B1", "A4/B4//B4"})
	advance(10999, nil, []string{"A1/B2/B3/B3", "A2///", "A3/B1//B1", "A4/B4//B4"})
	advance(11000, []string{"A1/B2/B3/B3", "A2///"}, []string{"A1/B2/B3/B3", "A2///", "A3/B1//B1", "A4/B4//B4"})
	advance(12000, []string{"A3/B1//B1"}, []string{"A1/B2/B3/B3", "A2///", "A3/B1//B1", "A4/B4//B4"})
	advance(13000, []string{"A4/B4//B4"}, []string{"A1/B2/B3/B3", "A2///", "A3/B1//B1", "A4/B4//B4"})
}

func TestRowRecogIntervalMultipleCompletedTrace(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	symbol := Field[rowRecogTestEvent, string]("symbol")
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B").ZeroOrMore())).
		Define("A", StartsWith(symbol, Literal("A"))).
		Define("B", StartsWith(symbol, Literal("B"))).
		Interval(10*time.Second).
		Measures(
			Alias("a", TagField[string]("A", "symbol")),
			Alias("b0", TagFieldAt[string]("B", 0, "symbol")),
			Alias("b1", TagFieldAt[string]("B", 1, "symbol")),
			Alias("lastb", TagLast[string]("B", symbol)),
		).
		Query(StatementName("rowrecog-interval-multiple-completed"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	statement := deployment.Statements()[0]
	send := func(event rowRecogTestEvent, listener, snapshot []string) {
		t.Helper()
		before := len(*rows)
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		assertRowRecogAfterState(t, rows, statement, before, listener, snapshot, "a", "b0", "b1", "lastb")
	}
	advance := func(milliseconds int64, listener, snapshot []string) {
		t.Helper()
		before := len(*rows)
		if err := engine.AdvanceTime(context.Background(), time.Unix(0, milliseconds*int64(time.Millisecond)).UTC()); err != nil {
			t.Fatal(err)
		}
		assertRowRecogAfterState(t, rows, statement, before, listener, snapshot, "a", "b0", "b1", "lastb")
	}

	advance(1000, nil, nil)
	send(rowRecogTestEvent{Symbol: "A1"}, nil, []string{"A1///"})
	advance(5000, nil, []string{"A1///"})
	send(rowRecogTestEvent{Symbol: "A2"}, nil, []string{"A1///", "A2///"})
	advance(10999, nil, []string{"A1///", "A2///"})
	advance(11000, []string{"A1///"}, []string{"A1///", "A2///"})
	advance(15000, []string{"A2///"}, []string{"A1///", "A2///"})
	advance(21000, nil, []string{"A1///", "A2///"})
	send(rowRecogTestEvent{Symbol: "A3"}, nil, []string{"A1///", "A2///", "A3///"})
	advance(22000, nil, []string{"A1///", "A2///", "A3///"})
	send(rowRecogTestEvent{Symbol: "A4"}, nil, []string{"A1///", "A2///", "A3///", "A4///"})
	advance(23000, nil, []string{"A1///", "A2///", "A3///", "A4///"})
	send(rowRecogTestEvent{Symbol: "B1"}, nil, []string{"A1///", "A2///", "A3///", "A4/B1//B1"})
	send(rowRecogTestEvent{Symbol: "B2"}, nil, []string{"A1///", "A2///", "A3///", "A4/B1/B2/B2"})
	send(rowRecogTestEvent{Symbol: "B3"}, nil, []string{"A1///", "A2///", "A3///", "A4/B1/B2/B3"})
	send(rowRecogTestEvent{Symbol: "B4"}, nil, []string{"A1///", "A2///", "A3///", "A4/B1/B2/B4"})
	advance(31000, []string{"A3///"}, []string{"A1///", "A2///", "A3///", "A4/B1/B2/B4"})
	advance(32000, []string{"A4/B1/B2/B4"}, []string{"A1///", "A2///", "A3///", "A4/B1/B2/B4"})
}

func TestRowRecogIntervalMonthScopedTrace(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[rowRecogTestEvent](env, "RowRecogEvent"); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2002, time.February, 1, 9, 0, 0, 0, time.UTC)
	engine := NewEngine(env, WithStartTime(start))
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	symbol := Field[rowRecogTestEvent, string]("symbol")
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B").ZeroOrMore())).
		Define("A", StartsWith(symbol, Literal("A"))).
		Define("B", StartsWith(symbol, Literal("B"))).
		IntervalCalendar(0, 1, 0).
		Measures(
			Alias("a", TagField[string]("A", "symbol")),
			Alias("b0", TagFieldAt[string]("B", 0, "symbol")),
			Alias("b1", TagFieldAt[string]("B", 1, "symbol")),
		).
		Query(StatementName("rowrecog-interval-month-scoped"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	statement := deployment.Statements()[0]
	send := func(event rowRecogTestEvent, listener, snapshot []string) {
		t.Helper()
		before := len(*rows)
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		assertRowRecogAfterState(t, rows, statement, before, listener, snapshot, "a", "b0", "b1")
	}
	advance := func(at time.Time, listener, snapshot []string) {
		t.Helper()
		before := len(*rows)
		if err := engine.AdvanceTime(context.Background(), at); err != nil {
			t.Fatal(err)
		}
		assertRowRecogAfterState(t, rows, statement, before, listener, snapshot, "a", "b0", "b1")
	}

	send(rowRecogTestEvent{Symbol: "A1"}, nil, []string{"A1//"})
	send(rowRecogTestEvent{Symbol: "B1"}, nil, []string{"A1/B1/"})
	deadline := start.AddDate(0, 1, 0)
	advance(deadline.Add(-time.Millisecond), nil, []string{"A1/B1/"})
	advance(deadline, []string{"A1/B1/"}, []string{"A1/B1/"})
}
