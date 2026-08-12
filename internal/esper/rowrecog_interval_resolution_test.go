package esper

import (
	"context"
	"testing"
	"time"
)

func TestRowRecogIntervalResolutionExactBoundary(t *testing.T) {
	start := time.Unix(0, 0).UTC()
	runRowRecogIntervalResolutionTrace(t, "rowrecog-interval-resolution-exact", start, start.Add(10*time.Second-time.Nanosecond), start.Add(10*time.Second))
}

func TestRowRecogIntervalResolutionMicrosecondBoundary(t *testing.T) {
	start := time.Unix(0, 0).UTC()
	flipTimeMicros := int64(10_000_000)
	before := start.Add(time.Duration(flipTimeMicros-1) * time.Microsecond)
	exact := start.Add(time.Duration(flipTimeMicros) * time.Microsecond)
	runRowRecogIntervalResolutionTrace(t, "rowrecog-interval-resolution-microsecond", start, before, exact)
}

func runRowRecogIntervalResolutionTrace(t *testing.T, statementName string, start, before, exact time.Time) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[rowRecogTestEvent](env, "RowRecogEvent"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(start))
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowVar("A").ZeroOrMore()).
		Interval(10*time.Second).
		Measures(
			Alias("aCount", TagCount("A")),
			Alias("firstA", TagFieldAt[string]("A", 0, "symbol")),
		).
		Query(StatementName(statementName))
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
	if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: "E1"}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 0 {
		t.Fatalf("interval resolution emitted on arrival = %#v", *rows)
	}
	assertRowRecogAfterState(t, rows, statement, 0, nil, []string{"1/E1"}, "aCount", "firstA")

	if err := engine.AdvanceTime(context.Background(), before); err != nil {
		t.Fatal(err)
	}
	assertRowRecogAfterState(t, rows, statement, 0, nil, []string{"1/E1"}, "aCount", "firstA")

	if err := engine.AdvanceTime(context.Background(), exact); err != nil {
		t.Fatal(err)
	}
	assertRowRecogAfterState(t, rows, statement, 0, []string{"1/E1"}, []string{"1/E1"}, "aCount", "firstA")
}
