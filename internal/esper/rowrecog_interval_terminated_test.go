package esper

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type rowRecogIntervalTerminatedDefine struct {
	name string
	expr Expr
}

type rowRecogIntervalTerminatedStep struct {
	event    *rowRecogTestEvent
	at       time.Time
	listener []string
	snapshot []string
}

type rowRecogIntervalTerminatedHarness struct {
	t         *testing.T
	engine    *Engine
	rows      *[]Row
	statement *Statement
	fields    []string
}

func newRowRecogIntervalTerminatedHarness(t *testing.T, name string, pattern RowPattern, defines []rowRecogIntervalTerminatedDefine, partition Expr, allMatches bool, fields []string, measures ...Selection) *rowRecogIntervalTerminatedHarness {
	t.Helper()
	env, _ := newRowRecogTest(t)
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	query := stream.MatchRecognize(pattern)
	for _, define := range defines {
		query = query.Define(define.name, define.expr)
	}
	if partition != nil {
		query = query.PartitionBy(partition)
	}
	if allMatches {
		query = query.AllMatches()
	} else {
		query = query.FirstMatch()
	}
	query = query.IntervalOrTerminated(10 * time.Second)
	plan, err := env.Build(query.Measures(measures...).Query(StatementName(name)))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	return &rowRecogIntervalTerminatedHarness{
		t:         t,
		engine:    engine,
		rows:      collectRowRecogRows(t, deployment),
		statement: deployment.Statements()[0],
		fields:    fields,
	}
}

func (h *rowRecogIntervalTerminatedHarness) run(steps ...rowRecogIntervalTerminatedStep) {
	h.t.Helper()
	for _, step := range steps {
		before := len(*h.rows)
		if step.event != nil {
			if err := h.engine.SendEvent(context.Background(), *step.event); err != nil {
				h.t.Fatal(err)
			}
		} else if err := h.engine.AdvanceTime(context.Background(), step.at); err != nil {
			h.t.Fatal(err)
		}
		gotListener := rowRecogAfterKeys((*h.rows)[before:], h.fields...)
		wantListener := sortedStrings(step.listener)
		if fmt.Sprint(gotListener) != fmt.Sprint(wantListener) {
			h.t.Fatalf("row-recognize listener batch = %#v, want %#v", gotListener, wantListener)
		}
		if step.snapshot != nil {
			result, err := h.statement.Snapshot(context.Background())
			if err != nil {
				h.t.Fatal(err)
			}
			gotSnapshot := rowRecogAfterResultKeys(h.t, result.Results(), h.fields...)
			wantSnapshot := sortedStrings(step.snapshot)
			if fmt.Sprint(gotSnapshot) != fmt.Sprint(wantSnapshot) {
				h.t.Fatalf("row-recognize snapshot = %#v, want %#v", gotSnapshot, wantSnapshot)
			}
		}
	}
}

func rowRecogIntervalTerminatedEvent(symbol string, listener, snapshot []string) rowRecogIntervalTerminatedStep {
	return rowRecogIntervalTerminatedStep{event: &rowRecogTestEvent{Symbol: symbol}, listener: listener, snapshot: snapshot}
}

func rowRecogIntervalTerminatedEventAt(event rowRecogTestEvent, listener, snapshot []string) rowRecogIntervalTerminatedStep {
	return rowRecogIntervalTerminatedStep{event: &event, listener: listener, snapshot: snapshot}
}

func rowRecogIntervalTerminatedTime(milliseconds int64, listener, snapshot []string) rowRecogIntervalTerminatedStep {
	return rowRecogIntervalTerminatedStep{at: time.Unix(0, milliseconds*int64(time.Millisecond)).UTC(), listener: listener, snapshot: snapshot}
}

func rowRecogIntervalTerminatedStartsWith(symbol Expression[string], prefix string) Expr {
	return StartsWith(symbol, Literal(prefix))
}

func TestRowRecogIntervalOrTerminatedAStarKeepsBranchAfterInterval(t *testing.T) {
	symbol := Field[rowRecogTestEvent, string]("symbol")
	harness := newRowRecogIntervalTerminatedHarness(
		t,
		"rowrecog-interval-terminated-astar",
		RowVar("A").ZeroOrMore(),
		[]rowRecogIntervalTerminatedDefine{{"A", rowRecogIntervalTerminatedStartsWith(symbol, "A")}},
		nil,
		false,
		[]string{"a0", "a1", "a2", "a3", "a4", "a5"},
		Alias("a0", TagFieldAt[string]("A", 0, "symbol")),
		Alias("a1", TagFieldAt[string]("A", 1, "symbol")),
		Alias("a2", TagFieldAt[string]("A", 2, "symbol")),
		Alias("a3", TagFieldAt[string]("A", 3, "symbol")),
		Alias("a4", TagFieldAt[string]("A", 4, "symbol")),
		Alias("a5", TagFieldAt[string]("A", 5, "symbol")),
	)
	harness.run(
		rowRecogIntervalTerminatedEvent("A1", nil, []string{"A1/////"}),
		rowRecogIntervalTerminatedEvent("A2", nil, []string{"A1/A2////", "A2/////"}),
		rowRecogIntervalTerminatedEvent("B1", []string{"A1/A2////"}, []string{"A1/A2////"}),
		rowRecogIntervalTerminatedTime(2000, nil, []string{"A1/A2////"}),
		rowRecogIntervalTerminatedEvent("A3", nil, []string{"A1/A2////", "A3/////"}),
		rowRecogIntervalTerminatedEvent("A4", nil, []string{"A1/A2////", "A3/A4////", "A4/////"}),
		rowRecogIntervalTerminatedEvent("A5", nil, []string{"A1/A2////", "A3/A4/A5///", "A4/A5////", "A5/////"}),
		rowRecogIntervalTerminatedTime(12000, []string{"A3/A4/A5///"}, []string{"A1/A2////", "A3/A4/A5///", "A4/A5////", "A5/////"}),
		rowRecogIntervalTerminatedEvent("A6", nil, []string{"A1/A2////", "A3/A4/A5/A6//", "A4/A5/A6///", "A5/A6////", "A6/////"}),
		rowRecogIntervalTerminatedEvent("B2", []string{"A3/A4/A5/A6//"}, []string{"A1/A2////", "A3/A4/A5/A6//"}),
	)
}

func TestRowRecogIntervalOrTerminatedAllMatchesAStarSuffix(t *testing.T) {
	symbol := Field[rowRecogTestEvent, string]("symbol")
	harness := newRowRecogIntervalTerminatedHarness(
		t,
		"rowrecog-interval-terminated-all-astar-suffix",
		RowSequence(RowVar("A"), RowVar("B").ZeroOrMore()),
		[]rowRecogIntervalTerminatedDefine{
			{"A", rowRecogIntervalTerminatedStartsWith(symbol, "A")},
			{"B", rowRecogIntervalTerminatedStartsWith(symbol, "B")},
		},
		nil,
		true,
		[]string{"a", "b0", "b1", "b2"},
		Alias("a", TagField[string]("A", "symbol")),
		Alias("b0", TagFieldAt[string]("B", 0, "symbol")),
		Alias("b1", TagFieldAt[string]("B", 1, "symbol")),
		Alias("b2", TagFieldAt[string]("B", 2, "symbol")),
	)
	harness.run(
		rowRecogIntervalTerminatedEvent("A1", nil, []string{"A1///"}),
		rowRecogIntervalTerminatedEvent("B1", nil, []string{"A1/B1//"}),
		rowRecogIntervalTerminatedEvent("X1", []string{"A1/B1//", "A1///"}, []string{"A1/B1//", "A1///"}),
		rowRecogIntervalTerminatedTime(20000, nil, []string{"A1/B1//", "A1///"}),
		rowRecogIntervalTerminatedEvent("A2", nil, []string{"A1/B1//", "A1///", "A2///"}),
		rowRecogIntervalTerminatedEvent("B2", nil, []string{"A1/B1//", "A1///", "A2/B2//"}),
		rowRecogIntervalTerminatedTime(29999, nil, []string{"A1/B1//", "A1///", "A2/B2//"}),
		rowRecogIntervalTerminatedTime(30000, []string{"A2/B2//", "A2///"}, []string{"A1/B1//", "A1///", "A2/B2//"}),
	)
}

func TestRowRecogIntervalOrTerminatedDocSample(t *testing.T) {
	group := Field[rowRecogTestEvent, string]("group")
	price := Field[rowRecogTestEvent, float64]("price")
	harness := newRowRecogIntervalTerminatedHarness(
		t,
		"rowrecog-interval-terminated-doc-sample",
		RowSequence(RowVar("A"), RowVar("B").ZeroOrMore()),
		[]rowRecogIntervalTerminatedDefine{
			{"A", Greater[float64](price, Literal(100.0))},
			{"B", Greater[float64](price, Literal(100.0))},
		},
		group,
		false,
		[]string{"aID", "bCount", "firstB", "lastB"},
		Alias("aID", TagField[string]("A", "symbol")),
		Alias("bCount", TagCount("B")),
		Alias("firstB", TagFieldAt[string]("B", 0, "symbol")),
		Alias("lastB", TagLast[string]("B", Field[rowRecogTestEvent, string]("symbol"))),
	)
	harness.run(
		rowRecogIntervalTerminatedEventAt(rowRecogTestEvent{Symbol: "E1", Group: "device-1", Price: 98}, nil, nil),
		rowRecogIntervalTerminatedEventAt(rowRecogTestEvent{Symbol: "E2", Group: "device-1", Price: 101}, nil, []string{"E2/0//"}),
		rowRecogIntervalTerminatedEventAt(rowRecogTestEvent{Symbol: "E3", Group: "device-1", Price: 102}, nil, nil),
		rowRecogIntervalTerminatedEventAt(rowRecogTestEvent{Symbol: "E4", Group: "device-1", Price: 101}, nil, nil),
		rowRecogIntervalTerminatedEventAt(rowRecogTestEvent{Symbol: "E5", Group: "device-1", Price: 100}, []string{"E2/2/E3/E4"}, []string{"E2/2/E3/E4"}),
		rowRecogIntervalTerminatedTime(2147483647, nil, nil),
	)
}

func TestRowRecogIntervalOrTerminatedParenthesizedBStar(t *testing.T) {
	symbol := Field[rowRecogTestEvent, string]("symbol")
	harness := newRowRecogIntervalTerminatedHarness(
		t,
		"rowrecog-interval-terminated-parenthesized-bstar",
		RowSequence(RowVar("A"), RowSequence(RowVar("B")).ZeroOrMore()),
		[]rowRecogIntervalTerminatedDefine{
			{"A", rowRecogIntervalTerminatedStartsWith(symbol, "A")},
			{"B", rowRecogIntervalTerminatedStartsWith(symbol, "B")},
		},
		nil,
		false,
		[]string{"a", "b0", "b1", "b2"},
		Alias("a", TagField[string]("A", "symbol")),
		Alias("b0", TagFieldAt[string]("B", 0, "symbol")),
		Alias("b1", TagFieldAt[string]("B", 1, "symbol")),
		Alias("b2", TagFieldAt[string]("B", 2, "symbol")),
	)
	harness.run(
		rowRecogIntervalTerminatedEvent("A1", nil, nil),
		rowRecogIntervalTerminatedEvent("B1", nil, nil),
		rowRecogIntervalTerminatedEvent("X1", []string{"A1/B1//"}, []string{"A1/B1//"}),
		rowRecogIntervalTerminatedTime(20000, nil, nil),
		rowRecogIntervalTerminatedEvent("A2", nil, nil),
		rowRecogIntervalTerminatedEvent("B2", nil, nil),
		rowRecogIntervalTerminatedTime(29999, nil, nil),
		rowRecogIntervalTerminatedTime(30000, []string{"A2/B2//"}, []string{"A1/B1//", "A2/B2//"}),
	)
}

func TestRowRecogIntervalOrTerminatedAStarBPlus(t *testing.T) {
	symbol := Field[rowRecogTestEvent, string]("symbol")
	harness := newRowRecogIntervalTerminatedHarness(
		t,
		"rowrecog-interval-terminated-astar-bplus",
		RowSequence(RowVar("A"), RowVar("B").OneOrMore()),
		[]rowRecogIntervalTerminatedDefine{
			{"A", rowRecogIntervalTerminatedStartsWith(symbol, "A")},
			{"B", rowRecogIntervalTerminatedStartsWith(symbol, "B")},
		},
		nil,
		false,
		[]string{"a", "b0", "b1", "b2"},
		Alias("a", TagField[string]("A", "symbol")),
		Alias("b0", TagFieldAt[string]("B", 0, "symbol")),
		Alias("b1", TagFieldAt[string]("B", 1, "symbol")),
		Alias("b2", TagFieldAt[string]("B", 2, "symbol")),
	)
	harness.run(
		rowRecogIntervalTerminatedEvent("A1", nil, nil),
		rowRecogIntervalTerminatedEvent("X1", nil, nil),
		rowRecogIntervalTerminatedEvent("A2", nil, nil),
		rowRecogIntervalTerminatedEvent("B2", nil, nil),
		rowRecogIntervalTerminatedEvent("X2", []string{"A2/B2//"}, nil),
		rowRecogIntervalTerminatedEvent("A3", nil, nil),
		rowRecogIntervalTerminatedEvent("A4", nil, nil),
		rowRecogIntervalTerminatedEvent("B3", nil, nil),
		rowRecogIntervalTerminatedEvent("B4", nil, nil),
		rowRecogIntervalTerminatedEvent("X3", []string{"A4/B3/B4/"}, nil),
	)
}

func TestRowRecogIntervalOrTerminatedABStar(t *testing.T) {
	symbol := Field[rowRecogTestEvent, string]("symbol")
	harness := newRowRecogIntervalTerminatedHarness(
		t,
		"rowrecog-interval-terminated-abstar",
		RowSequence(RowVar("A"), RowVar("B").ZeroOrMore()),
		[]rowRecogIntervalTerminatedDefine{
			{"A", rowRecogIntervalTerminatedStartsWith(symbol, "A")},
			{"B", rowRecogIntervalTerminatedStartsWith(symbol, "B")},
		},
		nil,
		false,
		[]string{"a", "b0", "b1", "b2"},
		Alias("a", TagField[string]("A", "symbol")),
		Alias("b0", TagFieldAt[string]("B", 0, "symbol")),
		Alias("b1", TagFieldAt[string]("B", 1, "symbol")),
		Alias("b2", TagFieldAt[string]("B", 2, "symbol")),
	)
	harness.run(
		rowRecogIntervalTerminatedEvent("A1", nil, nil),
		rowRecogIntervalTerminatedEvent("B1", nil, nil),
		rowRecogIntervalTerminatedEvent("X1", []string{"A1/B1//"}, []string{"A1/B1//"}),
		rowRecogIntervalTerminatedTime(20000, nil, nil),
		rowRecogIntervalTerminatedEvent("A2", nil, nil),
		rowRecogIntervalTerminatedEvent("B2", nil, nil),
		rowRecogIntervalTerminatedTime(29999, nil, nil),
		rowRecogIntervalTerminatedTime(30000, []string{"A2/B2//"}, []string{"A1/B1//", "A2/B2//"}),
	)
}

func TestRowRecogIntervalOrTerminatedABCStar(t *testing.T) {
	symbol := Field[rowRecogTestEvent, string]("symbol")
	harness := newRowRecogIntervalTerminatedHarness(
		t,
		"rowrecog-interval-terminated-abcstar",
		RowSequence(RowVar("A"), RowVar("B"), RowVar("C").ZeroOrMore()),
		[]rowRecogIntervalTerminatedDefine{
			{"A", rowRecogIntervalTerminatedStartsWith(symbol, "A")},
			{"B", rowRecogIntervalTerminatedStartsWith(symbol, "B")},
			{"C", rowRecogIntervalTerminatedStartsWith(symbol, "C")},
		},
		nil,
		false,
		[]string{"a", "b", "c0", "c1", "c2"},
		Alias("a", TagField[string]("A", "symbol")),
		Alias("b", TagField[string]("B", "symbol")),
		Alias("c0", TagFieldAt[string]("C", 0, "symbol")),
		Alias("c1", TagFieldAt[string]("C", 1, "symbol")),
		Alias("c2", TagFieldAt[string]("C", 2, "symbol")),
	)
	harness.run(
		rowRecogIntervalTerminatedEvent("A1", nil, nil),
		rowRecogIntervalTerminatedEvent("B1", nil, nil),
		rowRecogIntervalTerminatedEvent("C1", nil, nil),
		rowRecogIntervalTerminatedEvent("C2", nil, nil),
		rowRecogIntervalTerminatedEvent("B2", []string{"A1/B1/C1/C2/"}, []string{"A1/B1/C1/C2/"}),
		rowRecogIntervalTerminatedEvent("A2", nil, nil),
		rowRecogIntervalTerminatedEvent("X1", nil, nil),
		rowRecogIntervalTerminatedEvent("B3", nil, nil),
		rowRecogIntervalTerminatedEvent("X2", nil, nil),
		rowRecogIntervalTerminatedEvent("A3", nil, nil),
		rowRecogIntervalTerminatedEvent("B4", nil, nil),
		rowRecogIntervalTerminatedEvent("X3", []string{"A3/B4///"}, nil),
		rowRecogIntervalTerminatedTime(20000, nil, nil),
		rowRecogIntervalTerminatedEvent("A4", nil, nil),
		rowRecogIntervalTerminatedEvent("B5", nil, nil),
		rowRecogIntervalTerminatedEvent("C3", nil, nil),
		rowRecogIntervalTerminatedTime(29999, nil, nil),
		rowRecogIntervalTerminatedTime(30000, []string{"A4/B5/C3//"}, nil),
	)
}

func TestRowRecogIntervalOrTerminatedAB(t *testing.T) {
	symbol := Field[rowRecogTestEvent, string]("symbol")
	harness := newRowRecogIntervalTerminatedHarness(
		t,
		"rowrecog-interval-terminated-ab",
		RowSequence(RowVar("A"), RowVar("B")),
		[]rowRecogIntervalTerminatedDefine{
			{"A", rowRecogIntervalTerminatedStartsWith(symbol, "A")},
			{"B", rowRecogIntervalTerminatedStartsWith(symbol, "B")},
		},
		nil,
		false,
		[]string{"a", "b"},
		Alias("a", TagField[string]("A", "symbol")),
		Alias("b", TagField[string]("B", "symbol")),
	)
	harness.run(
		rowRecogIntervalTerminatedEvent("A1", nil, nil),
		rowRecogIntervalTerminatedEvent("B1", []string{"A1/B1"}, nil),
		rowRecogIntervalTerminatedEvent("A2", nil, nil),
		rowRecogIntervalTerminatedEvent("A3", nil, nil),
		rowRecogIntervalTerminatedEvent("B2", []string{"A3/B2"}, nil),
	)
}

func TestRowRecogIntervalOrTerminatedABStarOrC(t *testing.T) {
	symbol := Field[rowRecogTestEvent, string]("symbol")
	harness := newRowRecogIntervalTerminatedHarness(
		t,
		"rowrecog-interval-terminated-abstar-or-c",
		RowSequence(RowVar("A"), RowAlternation(RowVar("B").ZeroOrMore(), RowVar("C"))),
		[]rowRecogIntervalTerminatedDefine{
			{"A", rowRecogIntervalTerminatedStartsWith(symbol, "A")},
			{"B", rowRecogIntervalTerminatedStartsWith(symbol, "B")},
			{"C", rowRecogIntervalTerminatedStartsWith(symbol, "C")},
		},
		nil,
		false,
		[]string{"a", "b0", "b1", "b2", "c"},
		Alias("a", TagField[string]("A", "symbol")),
		Alias("b0", TagFieldAt[string]("B", 0, "symbol")),
		Alias("b1", TagFieldAt[string]("B", 1, "symbol")),
		Alias("b2", TagFieldAt[string]("B", 2, "symbol")),
		Alias("c", TagField[string]("C", "symbol")),
	)
	harness.run(
		rowRecogIntervalTerminatedEvent("A1", nil, nil),
		rowRecogIntervalTerminatedEvent("C1", []string{"A1////C1"}, nil),
		rowRecogIntervalTerminatedEvent("A2", nil, nil),
		rowRecogIntervalTerminatedEvent("B1", []string{"A2////"}, nil),
		rowRecogIntervalTerminatedEvent("B2", nil, nil),
		rowRecogIntervalTerminatedEvent("X1", []string{"A2/B1/B2//"}, nil),
		rowRecogIntervalTerminatedEvent("A3", nil, nil),
		rowRecogIntervalTerminatedTime(10000, []string{"A3////"}, nil),
	)
}

func TestRowRecogIntervalOrTerminatedABStarOrCStar(t *testing.T) {
	symbol := Field[rowRecogTestEvent, string]("symbol")
	harness := newRowRecogIntervalTerminatedHarness(
		t,
		"rowrecog-interval-terminated-abstar-or-cstar",
		RowSequence(RowVar("A"), RowAlternation(RowVar("B").ZeroOrMore(), RowVar("C").ZeroOrMore())),
		[]rowRecogIntervalTerminatedDefine{
			{"A", rowRecogIntervalTerminatedStartsWith(symbol, "A")},
			{"B", rowRecogIntervalTerminatedStartsWith(symbol, "B")},
			{"C", rowRecogIntervalTerminatedStartsWith(symbol, "C")},
		},
		nil,
		false,
		[]string{"a", "b0", "b1", "c0", "c1"},
		Alias("a", TagField[string]("A", "symbol")),
		Alias("b0", TagFieldAt[string]("B", 0, "symbol")),
		Alias("b1", TagFieldAt[string]("B", 1, "symbol")),
		Alias("c0", TagFieldAt[string]("C", 0, "symbol")),
		Alias("c1", TagFieldAt[string]("C", 1, "symbol")),
	)
	harness.run(
		rowRecogIntervalTerminatedEvent("A1", nil, nil),
		rowRecogIntervalTerminatedEvent("X1", []string{"A1////"}, nil),
		rowRecogIntervalTerminatedEvent("A2", nil, nil),
		rowRecogIntervalTerminatedEvent("C1", []string{"A2////"}, nil),
		rowRecogIntervalTerminatedEvent("B1", []string{"A2///C1/"}, nil),
		rowRecogIntervalTerminatedEvent("C2", nil, nil),
	)
}
