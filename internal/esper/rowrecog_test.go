package esper

import (
	"context"
	"errors"
	"testing"
	"time"
)

type rowRecogTestEvent struct {
	Symbol string  `esper:"symbol"`
	Group  string  `esper:"group"`
	Price  float64 `esper:"price"`
}

func newRowRecogTest(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[rowRecogTestEvent](env, "RowRecogEvent"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env)
}

func collectRowRecogRows(t *testing.T, deployment *Deployment) *[]Row {
	t.Helper()
	rows := new([]Row)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("match-recognize result is not a row: %#v", result)
			}
			*rows = append(*rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestRowRecogBasicSequenceAndJavaStyleMeasures(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		Define("B", Greater[float64](Field[rowRecogTestEvent, float64]("price"), TagField[float64]("A", "price"))).
		Measures(
			Alias("first", TagField[string]("A", "symbol")),
			Alias("second", TagField[string]("B", "symbol")),
		).
		Query(StatementName("rowrecog-basic"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogTestEvent{
		{Symbol: "E1", Group: "g1", Price: 5},
		{Symbol: "E2", Group: "g1", Price: 3},
		{Symbol: "E3", Group: "g1", Price: 6},
		{Symbol: "E4", Group: "g1", Price: 4},
		{Symbol: "E5", Group: "g1", Price: 6},
		{Symbol: "E6", Group: "g1", Price: 10},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 2 {
		t.Fatalf("rows = %#v, want two non-overlapping matches", *rows)
	}
	if (*rows)[0].Get("first").Any() != "E2" || (*rows)[0].Get("second").Any() != "E3" ||
		(*rows)[1].Get("first").Any() != "E4" || (*rows)[1].Get("second").Any() != "E5" {
		t.Fatalf("basic measures = %#v", *rows)
	}
}

func TestRowRecogOptionalAndRepeatedMeasures(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B").ZeroOrMore(), RowVar("C"))).
		Define("A", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(10.0))).
		Define("B", Greater[float64](Field[rowRecogTestEvent, float64]("price"), Literal(10.0))).
		Define("C", Less[float64](Field[rowRecogTestEvent, float64]("price"), Literal(10.0))).
		Measures(
			Alias("a", TagField[float64]("A", "price")),
			Alias("bCount", TagCount("B")),
			Alias("firstB", TagFieldAt[float64]("B", 0, "price")),
			Alias("c", TagField[float64]("C", "price")),
		).
		Query(StatementName("rowrecog-repeat"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogTestEvent{
		{Symbol: "A0", Price: 10},
		{Symbol: "C0", Price: 8},
		{Symbol: "A1", Price: 10},
		{Symbol: "B1", Price: 12},
		{Symbol: "B2", Price: 13},
		{Symbol: "C1", Price: 9},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 2 {
		t.Fatalf("rows = %#v, want optional and repeated matches", *rows)
	}
	if got := (*rows)[0].Get("bCount").Any(); got != int64(0) {
		t.Fatalf("optional B count = %#v", got)
	}
	if (*rows)[0].Get("firstB").IsPresent() {
		t.Fatalf("optional B should have a missing first element: %#v", (*rows)[0].Get("firstB"))
	}
	if got := (*rows)[1].Get("bCount").Any(); got != int64(2) || (*rows)[1].Get("firstB").Any() != 12.0 {
		t.Fatalf("repeated B measures = %#v", *rows)
	}
}

func TestRowRecogAlternationAndRepeatedVariable(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	query := stream.MatchRecognize(
		RowAlternation(
			RowSequence(RowVar("A"), RowVar("B")),
			RowSequence(RowVar("C"), RowVar("D")),
		),
	).
		Define("A", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(1.0))).
		Define("B", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(2.0))).
		Define("C", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(3.0))).
		Define("D", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(4.0))).
		Measures(
			Alias("a", TagField[string]("A", "symbol")),
			Alias("b", TagField[string]("B", "symbol")),
			Alias("c", TagField[string]("C", "symbol")),
			Alias("d", TagField[string]("D", "symbol")),
		).
		Query(StatementName("rowrecog-alternation"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogTestEvent{
		{Symbol: "x3", Price: 3},
		{Symbol: "x5", Price: 5},
		{Symbol: "c", Price: 3},
		{Symbol: "d", Price: 4},
		{Symbol: "a", Price: 1},
		{Symbol: "b", Price: 2},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 2 || (*rows)[0].Get("c").Any() != "c" || (*rows)[0].Get("d").Any() != "d" ||
		(*rows)[1].Get("a").Any() != "a" || (*rows)[1].Get("b").Any() != "b" {
		t.Fatalf("alternation rows = %#v", *rows)
	}

	repeated := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"), RowVar("A"))).
		Define("A", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(1.0))).
		Define("B", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(2.0))).
		Measures(
			Alias("firstA", TagFieldAt[string]("A", 0, "symbol")),
			Alias("b", TagField[string]("B", "symbol")),
			Alias("lastA", TagFieldAt[string]("A", 1, "symbol")),
		).
		Query(StatementName("rowrecog-reused-variable"))
	repeatedPlan, err := env.Build(repeated)
	if err != nil {
		t.Fatal(err)
	}
	repeatedDeployment, err := engine.Deploy(context.Background(), repeatedPlan)
	if err != nil {
		t.Fatal(err)
	}
	repeatedRows := collectRowRecogRows(t, repeatedDeployment)
	for _, event := range []rowRecogTestEvent{
		{Symbol: "ra", Price: 1},
		{Symbol: "rb", Price: 2},
		{Symbol: "rc", Price: 1},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*repeatedRows) != 1 || (*repeatedRows)[0].Get("firstA").Any() != "ra" ||
		(*repeatedRows)[0].Get("b").Any() != "rb" || (*repeatedRows)[0].Get("lastA").Any() != "rc" {
		t.Fatalf("reused variable rows = %#v", *repeatedRows)
	}
}

func TestRowRecogOneOrMoreAndOptional(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	define := func(query RowRecogQuery) RowRecogQuery {
		return query.
			Define("A", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(10.0))).
			Define("B", Greater[float64](Field[rowRecogTestEvent, float64]("price"), Literal(10.0))).
			Define("C", Less[float64](Field[rowRecogTestEvent, float64]("price"), Literal(10.0))).
			Measures(Alias("a", TagField[string]("A", "symbol")), Alias("c", TagField[string]("C", "symbol")))
	}
	oneQuery := define(stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B").OneOrMore(), RowVar("C"))))
	oneQuery = oneQuery.Measures(
		Alias("count", CountAll()),
		Alias("sum", Sum[float64](Field[rowRecogTestEvent, float64]("price"))),
	)
	onePlan, err := env.Build(oneQuery.Query(StatementName("rowrecog-one-more")))
	if err != nil {
		t.Fatal(err)
	}
	optionalQuery := define(stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B").Optional(), RowVar("C"))))
	optionalPlan, err := env.Build(optionalQuery.Query(StatementName("rowrecog-optional")))
	if err != nil {
		t.Fatal(err)
	}
	oneDeployment, err := engine.Deploy(context.Background(), onePlan)
	if err != nil {
		t.Fatal(err)
	}
	optionalDeployment, err := engine.Deploy(context.Background(), optionalPlan)
	if err != nil {
		t.Fatal(err)
	}
	oneRows := collectRowRecogRows(t, oneDeployment)
	optionalRows := collectRowRecogRows(t, optionalDeployment)
	for _, event := range []rowRecogTestEvent{
		{Symbol: "oa", Price: 10},
		{Symbol: "oc", Price: 8},
		{Symbol: "ra", Price: 10},
		{Symbol: "rb", Price: 12},
		{Symbol: "rc", Price: 8},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*oneRows) != 1 || (*oneRows)[0].Get("a").Any() != "ra" || (*oneRows)[0].Get("c").Any() != "rc" ||
		(*oneRows)[0].Get("count").Any() != int64(3) || (*oneRows)[0].Get("sum").Any() != 30.0 {
		t.Fatalf("one-or-more rows = %#v", *oneRows)
	}
	if len(*optionalRows) != 2 || (*optionalRows)[0].Get("a").Any() != "oa" || (*optionalRows)[1].Get("a").Any() != "ra" {
		t.Fatalf("optional rows = %#v", *optionalRows)
	}
}

func TestRowRecogReluctantQuantifiers(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(
		RowVar("A").ZeroOrMore().Reluctant(),
		RowVar("B").Optional(),
		RowVar("C"),
	)).
		Define("A", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(1.0))).
		Define("B", Or(
			Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(1.0)),
			Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(2.0)),
		)).
		Define("C", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(3.0))).
		Measures(
			Alias("a0", TagFieldAt[string]("A", 0, "symbol")),
			Alias("a1", TagFieldAt[string]("A", 1, "symbol")),
			Alias("b", TagField[string]("B", "symbol")),
			Alias("c", TagField[string]("C", "symbol")),
		).
		Query(StatementName("rowrecog-reluctant"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogTestEvent{
		{Symbol: "a0", Price: 1},
		{Symbol: "a1", Price: 1},
		{Symbol: "b", Price: 1},
		{Symbol: "c", Price: 3},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 1 || (*rows)[0].Get("a0").Any() != "a0" || (*rows)[0].Get("a1").Any() != "a1" ||
		(*rows)[0].Get("b").Any() != "b" || (*rows)[0].Get("c").Any() != "c" {
		t.Fatalf("reluctant quantifier row = %#v", *rows)
	}
}

func TestRowRecogPermutationExpandsFiniteAlternatives(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowPermute(RowVar("A"), RowVar("B"), RowVar("C"))).
		Define("A", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(1.0))).
		Define("B", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(2.0))).
		Define("C", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(3.0))).
		Measures(
			Alias("a", TagField[string]("A", "symbol")),
			Alias("b", TagField[string]("B", "symbol")),
			Alias("c", TagField[string]("C", "symbol")),
		).
		Query(StatementName("rowrecog-permute"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogTestEvent{
		{Symbol: "b", Price: 2},
		{Symbol: "a", Price: 1},
		{Symbol: "c", Price: 3},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 1 || (*rows)[0].Get("a").Any() != "a" || (*rows)[0].Get("b").Any() != "b" || (*rows)[0].Get("c").Any() != "c" {
		t.Fatalf("permutation rows = %#v", *rows)
	}
}

func TestRowRecogBoundedCompositeRepeat(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(
		RowVar("A"),
		RowSequence(RowVar("B"), RowVar("C")).Repeat(2, 2),
		RowVar("D"),
	)).
		Define("A", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(1.0))).
		Define("B", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(2.0))).
		Define("C", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(3.0))).
		Define("D", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(4.0))).
		Measures(
			Alias("a", TagField[string]("A", "symbol")),
			Alias("firstB", TagFieldAt[string]("B", 0, "symbol")),
			Alias("lastB", TagFieldAt[string]("B", 1, "symbol")),
			Alias("cCount", TagCount("C")),
			Alias("d", TagField[string]("D", "symbol")),
		).
		Query(StatementName("rowrecog-composite-repeat"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogTestEvent{
		{Symbol: "a", Price: 1},
		{Symbol: "b1", Price: 2},
		{Symbol: "c1", Price: 3},
		{Symbol: "b2", Price: 2},
		{Symbol: "c2", Price: 3},
		{Symbol: "d", Price: 4},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 1 {
		t.Fatalf("composite repeat rows = %#v, want one match", *rows)
	}
	row := (*rows)[0]
	if row.Get("a").Any() != "a" || row.Get("firstB").Any() != "b1" ||
		row.Get("lastB").Any() != "b2" || row.Get("cCount").Any() != int64(2) || row.Get("d").Any() != "d" {
		t.Fatalf("composite repeat measures = %#v", row)
	}
}

func TestRowRecogUnboundedCompositeRepeat(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(
		RowVar("A"),
		RowSequence(RowVar("B"), RowVar("C")).ZeroOrMore(),
		RowVar("D"),
	)).
		Define("A", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(1.0))).
		Define("B", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(2.0))).
		Define("C", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(3.0))).
		Define("D", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(4.0))).
		Measures(
			Alias("a", TagField[string]("A", "symbol")),
			Alias("bCount", TagCount("B")),
			Alias("cCount", TagCount("C")),
			Alias("d", TagField[string]("D", "symbol")),
		).
		Query(StatementName("rowrecog-unbounded-composite"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogTestEvent{
		{Symbol: "a", Price: 1},
		{Symbol: "b1", Price: 2},
		{Symbol: "c1", Price: 3},
		{Symbol: "b2", Price: 2},
		{Symbol: "c2", Price: 3},
		{Symbol: "d", Price: 4},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 1 || (*rows)[0].Get("a").Any() != "a" || (*rows)[0].Get("bCount").Any() != int64(2) ||
		(*rows)[0].Get("cCount").Any() != int64(2) || (*rows)[0].Get("d").Any() != "d" {
		t.Fatalf("unbounded composite repeat row = %#v", *rows)
	}
}

func TestRowRecogPartitionAndSkipStrategy(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		Define("A", Equal[string](Field[rowRecogTestEvent, string]("symbol"), Literal("A"))).
		Define("B", Greater[float64](Field[rowRecogTestEvent, float64]("price"), TagField[float64]("A", "price"))).
		PartitionBy(Field[rowRecogTestEvent, string]("group")).
		SkipToNextRow().
		Measures(Alias("group", TagField[string]("A", "group")), Alias("price", TagField[float64]("B", "price"))).
		Query(StatementName("rowrecog-partition"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogTestEvent{
		{Symbol: "A", Group: "x", Price: 1},
		{Symbol: "A", Group: "y", Price: 10},
		{Symbol: "A", Group: "x", Price: 2},
		{Symbol: "B", Group: "x", Price: 3},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 2 {
		t.Fatalf("partition rows = %#v, want two x-partition matches", *rows)
	}
	for _, row := range *rows {
		if row.Get("group").Any() != "x" {
			t.Fatalf("cross-partition match = %#v", row)
		}
	}
}

func TestRowRecogIntervalAndSnapshot(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B").ZeroOrMore())).
		Define("A", Equal[string](Field[rowRecogTestEvent, string]("symbol"), Literal("A"))).
		Define("B", Equal[string](Field[rowRecogTestEvent, string]("symbol"), Literal("B"))).
		Interval(10*time.Second).
		Measures(
			Alias("a", TagField[string]("A", "symbol")),
			Alias("bCount", TagCount("B")),
			Alias("lastB", TagField[string]("B", "symbol")),
		).
		Query(StatementName("rowrecog-interval"))
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
	base := time.Unix(0, 0).UTC()
	if err := engine.AdvanceTime(context.Background(), base.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 0 {
		t.Fatalf("interval emitted before deadline: %#v", *rows)
	}
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 1 || snapshot.Results()[0].Get("a").Any() != "A" || snapshot.Results()[0].Get("bCount").Any() != int64(0) {
		t.Fatalf("pending interval snapshot = %#v", snapshot.Results())
	}
	if err := engine.AdvanceTime(context.Background(), base.Add(11*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 || (*rows)[0].Get("a").Any() != "A" || (*rows)[0].Get("bCount").Any() != int64(0) {
		t.Fatalf("interval completion = %#v", *rows)
	}

	if err := engine.AdvanceTime(context.Background(), base.Add(13*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), base.Add(15*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: "B"}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 {
		t.Fatalf("interval emitted before second deadline: %#v", *rows)
	}
	if err := engine.AdvanceTime(context.Background(), base.Add(23*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 2 || (*rows)[1].Get("a").Any() != "A" || (*rows)[1].Get("bCount").Any() != int64(1) || (*rows)[1].Get("lastB").Any() != "B" {
		t.Fatalf("interval completed match = %#v", *rows)
	}
	finalSnapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(finalSnapshot.Results()) != 2 {
		t.Fatalf("interval iterator snapshot = %#v", finalSnapshot.Results())
	}
}

func TestRowRecogCalendarIntervalUsesMonthBoundary(t *testing.T) {
	env, _ := newRowRecogTest(t)
	base := time.Date(2026, time.February, 1, 9, 0, 0, 0, time.UTC)
	engine := NewEngine(env, WithStartTime(base))
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B").ZeroOrMore())).
		Define("A", Equal[string](Field[rowRecogTestEvent, string]("symbol"), Literal("A"))).
		Define("B", Equal[string](Field[rowRecogTestEvent, string]("symbol"), Literal("B"))).
		IntervalCalendar(0, 1, 0).
		Measures(Alias("a", TagFieldAt[string]("A", 0, "symbol")), Alias("b", TagField[string]("B", "symbol"))).
		Query(StatementName("rowrecog-calendar-interval"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogTestEvent{{Symbol: "A"}, {Symbol: "B"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), base.AddDate(0, 1, 0).Add(-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 0 {
		t.Fatalf("calendar interval emitted before boundary: %#v", *rows)
	}
	if err := engine.AdvanceTime(context.Background(), base.AddDate(0, 1, 0)); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 || (*rows)[0].Get("a").Any() != "A" || (*rows)[0].Get("b").Any() != "B" {
		t.Fatalf("calendar interval result = %#v", *rows)
	}
}

func TestRowRecogIntervalOrTerminated(t *testing.T) {
	env, _ := newRowRecogTest(t)
	base := time.Unix(0, 0).UTC()
	engine := NewEngine(env, WithStartTime(base))
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B").ZeroOrMore())).
		Define("A", Equal[string](Field[rowRecogTestEvent, string]("symbol"), Literal("A"))).
		Define("B", Equal[string](Field[rowRecogTestEvent, string]("symbol"), Literal("B"))).
		FirstMatch().
		IntervalOrTerminated(10*time.Second).
		Measures(Alias("a", TagField[string]("A", "symbol")), Alias("bCount", TagCount("B"))).
		Query(StatementName("rowrecog-interval-terminated"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogTestEvent{{Symbol: "A"}, {Symbol: "B"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 0 {
		t.Fatalf("interval-or-terminated emitted before termination: %#v", *rows)
	}
	if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: "X"}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 || (*rows)[0].Get("a").Any() != "A" || (*rows)[0].Get("bCount").Any() != int64(1) {
		t.Fatalf("interval-or-terminated mismatch completion = %#v", *rows)
	}
	for _, event := range []rowRecogTestEvent{{Symbol: "A"}, {Symbol: "B"}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), base.Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 2 || (*rows)[1].Get("a").Any() != "A" || (*rows)[1].Get("bCount").Any() != int64(1) {
		t.Fatalf("interval-or-terminated timer completion = %#v", *rows)
	}

	exactEnv, _ := newRowRecogTest(t)
	exactEngine := NewEngine(exactEnv, WithStartTime(base))
	exactStream := From[rowRecogTestEvent](exactEnv, "RowRecogEvent").Window(KeepAll())
	exactQuery := exactStream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		Define("A", Equal[string](Field[rowRecogTestEvent, string]("symbol"), Literal("A"))).
		Define("B", Equal[string](Field[rowRecogTestEvent, string]("symbol"), Literal("B"))).
		IntervalOrTerminated(10*time.Second).
		Measures(Alias("a", TagField[string]("A", "symbol")), Alias("b", TagField[string]("B", "symbol"))).
		Query(StatementName("rowrecog-fixed-interval-terminated"))
	exactPlan, err := exactEnv.Build(exactQuery)
	if err != nil {
		t.Fatal(err)
	}
	exactDeployment, err := exactEngine.Deploy(context.Background(), exactPlan)
	if err != nil {
		t.Fatal(err)
	}
	exactRows := collectRowRecogRows(t, exactDeployment)
	for _, event := range []rowRecogTestEvent{{Symbol: "A"}, {Symbol: "B"}} {
		if err := exactEngine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*exactRows) != 1 || (*exactRows)[0].Get("a").Any() != "A" || (*exactRows)[0].Get("b").Any() != "B" {
		t.Fatalf("fixed pattern did not terminate immediately = %#v", *exactRows)
	}
	if err := exactEngine.AdvanceTime(context.Background(), base.Add(20*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(*exactRows) != 1 {
		t.Fatalf("fixed pattern emitted again at interval deadline = %#v", *exactRows)
	}
}

func TestRowRecogOrderByMeasureAlias(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(RowVar("A").OneOrMore(), RowVar("B"))).
		Define("A", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(1.0))).
		Define("B", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(2.0))).
		AllMatches().
		SkipToNextRow().
		Measures(Alias("a", TagFieldAt[string]("A", 0, "symbol")), Alias("b", TagField[string]("B", "symbol"))).
		Query(StatementName("rowrecog-order-by-measure"), OrderBy(Descending(ResultField[string]("a"))))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogTestEvent{{Symbol: "A1", Price: 1}, {Symbol: "A2", Price: 1}, {Symbol: "B1", Price: 2}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 2 || (*rows)[0].Get("a").Any() != "A2" || (*rows)[1].Get("a").Any() != "A1" {
		t.Fatalf("row-recognize order by measure = %#v", *rows)
	}
	snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 2 || snapshot.Results()[0].Get("a").Any() != "A2" || snapshot.Results()[1].Get("a").Any() != "A1" {
		t.Fatalf("row-recognize ordered snapshot = %#v", snapshot.Results())
	}
}

func TestRowRecogPreviousExpressionInDefine(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	price := Field[rowRecogTestEvent, float64]("price")
	query := stream.MatchRecognize(RowSequence(RowVar("A"))).
		Define("A", Equal[float64](Prev[float64](2, price), price)).
		Measures(Alias("a", TagField[string]("A", "symbol"))).
		Query(StatementName("rowrecog-prev-define"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogTestEvent{
		{Symbol: "E1", Price: 1},
		{Symbol: "E2", Price: 2},
		{Symbol: "E3", Price: 1},
		{Symbol: "E4", Price: 2},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 2 || (*rows)[0].Get("a").Any() != "E3" || (*rows)[1].Get("a").Any() != "E4" {
		t.Fatalf("row-recognize prev define = %#v", *rows)
	}
}

func TestRowRecogSkipKeepsRecognitionStateForSnapshot(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B").ZeroOrMore())).
		Define("A", Equal[string](Field[rowRecogTestEvent, string]("symbol"), Literal("A"))).
		Define("B", Equal[string](Field[rowRecogTestEvent, string]("symbol"), Literal("B"))).
		SkipToNextRow().
		Measures(Alias("a", TagField[string]("A", "symbol")), Alias("bCount", TagCount("B"))).
		Query(StatementName("rowrecog-skip-snapshot"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 {
		t.Fatalf("initial skip match = %#v", *rows)
	}
	if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: "B"}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 {
		t.Fatalf("skip-to-next-row emitted skipped update: %#v", *rows)
	}
	snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 1 || snapshot.Results()[0].Get("a").Any() != "A" || snapshot.Results()[0].Get("bCount").Any() != int64(1) {
		t.Fatalf("skip state snapshot = %#v", snapshot.Results())
	}
}

func TestRowRecogLengthWindowEvictionRecomputesSnapshot(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(LengthWindow(2))
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		Define("A", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(1.0))).
		Define("B", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(2.0))).
		Measures(
			Alias("a", TagField[string]("A", "symbol")),
			Alias("b", TagField[string]("B", "symbol")),
		).
		Query(StatementName("rowrecog-length-eviction"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches []ResultBatch
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: "A1", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: "B1", Price: 2}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 1 || snapshot.Results()[0].Get("a").Any() != "A1" || snapshot.Results()[0].Get("b").Any() != "B1" {
		t.Fatalf("length-window match before eviction = %#v", snapshot.Results())
	}
	if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: "C0", Price: 9}); err != nil {
		t.Fatal(err)
	}
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 0 {
		t.Fatalf("length-window eviction retained stale match = %#v", snapshot.Results())
	}
	if len(batches) != 1 || len(batches[0].New) != 1 || len(batches[0].Old) != 0 {
		t.Fatalf("length-window eviction listener batches = %#v", batches)
	}
}

func TestRowRecogNamedWindowDeleteRecomputesSnapshot(t *testing.T) {
	env, _ := newRowRecogTest(t)
	schema, ok := env.Schema("RowRecogEvent")
	if !ok {
		t.Fatal("row-recognize schema is missing")
	}
	if _, err := env.RegisterNamedWindow("rowrecog-delete", schema); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	stream := FromNamedWindow(env, "rowrecog-delete")
	query := stream.MatchRecognize(RowSequence(RowVar("A").OneOrMore(), RowVar("B"))).
		Define("A", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(1.0))).
		Define("B", Equal[float64](Field[rowRecogTestEvent, float64]("price"), Literal(3.0))).
		Measures(
			Alias("firstA", TagFieldAt[string]("A", 0, "symbol")),
			Alias("secondA", TagFieldAt[string]("A", 1, "symbol")),
			Alias("b", TagField[string]("B", "symbol")),
		).
		Query(StatementName("rowrecog-named-delete"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches []ResultBatch
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	triggerSource := From[rowRecogTestEvent](env, "RowRecogEvent")
	deletePlan, err := env.Build(OnEvent(triggerSource).DeleteFromNamedWindow(
		"rowrecog-delete",
		Equal[string](NamedWindowField[string]("symbol"), Field[rowRecogTestEvent, string]("symbol")),
	).Query(StatementName("rowrecog-named-delete-trigger")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), deletePlan); err != nil {
		t.Fatal(err)
	}
	for _, event := range []rowRecogTestEvent{
		{Symbol: "A1", Price: 1},
		{Symbol: "A2", Price: 1},
		{Symbol: "B1", Price: 3},
	} {
		if err := engine.InsertNamedWindow(context.Background(), "rowrecog-delete", event); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 2 || snapshot.Results()[0].Get("firstA").Any() != "A1" || snapshot.Results()[0].Get("secondA").Any() != "A2" || snapshot.Results()[0].Get("b").Any() != "B1" || snapshot.Results()[1].Get("firstA").Any() != "A2" || snapshot.Results()[1].Get("secondA").IsPresent() || snapshot.Results()[1].Get("b").Any() != "B1" {
		t.Fatalf("named-window match before delete = %#v", snapshot.Results())
	}
	if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: "A2"}); err != nil {
		t.Fatal(err)
	}
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 1 || snapshot.Results()[0].Get("firstA").Any() != "A1" || snapshot.Results()[0].Get("secondA").IsPresent() || snapshot.Results()[0].Get("b").Any() != "B1" {
		stateEvents := []Event(nil)
		if statement.runtime.rowRecogState != nil {
			for _, partition := range statement.runtime.rowRecogState.partitions {
				stateEvents = append(stateEvents, partition.events...)
			}
		}
		t.Fatalf("named-window out-of-sequence delete did not recompute = %#v, state=%#v, batches=%#v", snapshot.Results(), stateEvents, batches)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("named-window delete unexpectedly dispatched row-recognize batch = %#v", batches)
	}
	if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: "A1"}); err != nil {
		t.Fatal(err)
	}
	snapshot, err = statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 0 {
		t.Fatalf("named-window delete retained stale partial match = %#v", snapshot.Results())
	}
}

func TestRowRecogCanonicalAndValidation(t *testing.T) {
	env, _ := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	makeQuery := func(first, second string) Query {
		return stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
			Define(first, Equal[string](Field[rowRecogTestEvent, string]("symbol"), Literal(first))).
			Define(second, Equal[string](Field[rowRecogTestEvent, string]("symbol"), Literal(second))).
			Measures(Alias("a", TagField[string]("A", "symbol"))).
			Query(StatementName("canonical"))
	}
	first, err := env.Build(makeQuery("B", "A"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := env.Build(makeQuery("A", "B"))
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash() != second.Hash() {
		t.Fatalf("definition order changed canonical hash: %s != %s", first.Hash(), second.Hash())
	}

	invalid := []Query{
		stream.MatchRecognize(RowSequence()).Measures(Alias("a", TagField[string]("A", "symbol"))).Query(),
		stream.MatchRecognize(RowSequence(RowVar("A"))).Define("unknown", Literal(true)).Measures(Alias("a", TagField[string]("A", "symbol"))).Query(),
		stream.MatchRecognize(RowSequence(RowVar("A"))).MaxStates(-1).Measures(Alias("a", TagField[string]("A", "symbol"))).Query(),
		stream.MatchRecognize(RowSequence(RowVar("A"))).Measures(Alias("a", TagField[string]("A", "symbol")), Alias("a", TagField[string]("A", "symbol"))).Query(),
		stream.MatchRecognize(RowSequence(RowVar("A"))).Measures(Alias("a", TagField[string]("A", "symbol"))).Query(WithOutput(OutputSnapshot())),
		stream.MatchRecognize(RowSequence(RowVar("A"))).Measures(Alias("a", TagField[string]("Unknown", "symbol"))).Query(),
		stream.MatchRecognize(RowSequence(RowVar("A"))).Measures(Alias("a", TagField[string]("A", "symbol"))).Query(WithOldStream()),
		stream.MatchRecognize(RowSequence(RowVar("A"))).Measures(Alias("a", TagField[string]("A", "symbol"))).Query(OrderBy(Ascending(Field[rowRecogTestEvent, string]("symbol")))),
		stream.MatchRecognize(RowSequence(RowVar("A"))).Interval(-time.Second).Measures(Alias("a", TagField[string]("A", "symbol"))).Query(),
		stream.MatchRecognize(RowSequence(RowVar("A"))).IntervalOrTerminated(-time.Second).Measures(Alias("a", TagField[string]("A", "symbol"))).Query(),
		stream.MatchRecognize(RowSequence(RowVar("A"))).IntervalCalendar(-1, 0, 0).Measures(Alias("a", TagField[string]("A", "symbol"))).Query(),
	}
	for index, query := range invalid {
		if _, err := env.Build(query); err == nil {
			t.Errorf("invalid row-recognize query %d was accepted", index)
		} else if !errors.Is(err, ErrorInvalidRule) && !errors.Is(err, ErrorUnknownName) {
			t.Errorf("invalid row-recognize query %d error = %v", index, err)
		}
	}
}

func TestRowRecogTagAggregatesAndEnumeration(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	price := Field[rowRecogTestEvent, float64]("price")
	symbol := Field[rowRecogTestEvent, string]("symbol")
	tagPrice := TagSum[float64]("A", price)
	query := stream.MatchRecognize(RowSequence(RowVar("A").ZeroOrMore(), RowVar("B"))).
		Define("A", Like(symbol, Literal("A%"))).
		Define("B", And(
			Like(symbol, Literal("B%")),
			Greater[float64](Add[float64](Coalesce[float64](tagPrice, Literal(0.0)), price), Literal(100.0)),
		)).
		Measures(
			Alias("a0", TagFieldAt[string]("A", 0, "symbol")),
			Alias("a1", TagFieldAt[string]("A", 1, "symbol")),
			Alias("b", TagField[string]("B", "symbol")),
			Alias("size", TagSize("A")),
			Alias("sum", tagPrice),
			Alias("avg", TagAvg[float64]("A", price)),
			Alias("min", TagMin[float64]("A", price)),
			Alias("max", TagMax[float64]("A", price)),
			Alias("first", TagFirst[float64]("A", price)),
			Alias("last", TagLast[float64]("A", price)),
			Alias("hasA2", TagAny("A", Equal[string](symbol, Literal("A2")))),
			Alias("allAtLeast49", TagAll("A", GreaterOrEqual[float64](price, Literal(49.0)))),
			Alias("events", TagEvents("A")),
		).
		Query(StatementName("rowrecog-tag-aggregate"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for _, event := range []rowRecogTestEvent{
		{Symbol: "A1", Price: 50},
		{Symbol: "A2", Price: 49},
		{Symbol: "B1", Price: 2},
		{Symbol: "B2", Price: 101},
		{Symbol: "A3", Price: 50},
		{Symbol: "B3", Price: 51},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rows) != 3 {
		t.Fatalf("tag aggregate rows = %#v", *rows)
	}
	first := (*rows)[0]
	if first.Get("a0").Any() != "A1" || first.Get("a1").Any() != "A2" || first.Get("b").Any() != "B1" ||
		first.Get("size").Any() != int64(2) || first.Get("sum").Any() != float64(99) ||
		first.Get("avg").Any() != 49.5 || first.Get("min").Any() != float64(49) || first.Get("max").Any() != float64(50) ||
		first.Get("first").Any() != float64(50) || first.Get("last").Any() != float64(49) ||
		first.Get("hasA2").Any() != true || first.Get("allAtLeast49").Any() != true {
		t.Fatalf("tag aggregate first row = %#v", first)
	}
	events, ok := first.Get("events").Any().([]Event)
	if !ok || len(events) != 2 || events[0].Underlying().(rowRecogTestEvent).Symbol != "A1" || events[1].Underlying().(rowRecogTestEvent).Symbol != "A2" {
		t.Fatalf("tag events = %#v", first.Get("events").Any())
	}
	if (*rows)[1].Get("a0").IsPresent() || (*rows)[1].Get("a1").IsPresent() || (*rows)[1].Get("b").Any() != "B2" ||
		(*rows)[1].Get("size").Any() != int64(0) || (*rows)[1].Get("sum").IsPresent() || (*rows)[1].Get("hasA2").Any() != false ||
		(*rows)[1].Get("allAtLeast49").Any() != true {
		t.Fatalf("optional tag aggregate row = %#v", (*rows)[1])
	}
	if (*rows)[2].Get("a0").Any() != "A3" || (*rows)[2].Get("a1").IsPresent() || (*rows)[2].Get("b").Any() != "B3" ||
		(*rows)[2].Get("sum").Any() != float64(50) || (*rows)[2].Get("hasA2").Any() != false {
		t.Fatalf("second repeated tag aggregate row = %#v", (*rows)[2])
	}

	invalid := stream.MatchRecognize(RowSequence(RowVar("A"))).
		Measures(Alias("sum", TagSum[float64]("Unknown", price))).
		Query(StatementName("rowrecog-unknown-tag-aggregate"))
	if _, err := env.Build(invalid); err == nil || !errors.Is(err, ErrorUnknownName) {
		t.Fatalf("unknown tag aggregate error = %v", err)
	}
}
