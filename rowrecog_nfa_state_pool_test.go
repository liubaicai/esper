package esper

import (
	"context"
	"testing"
)

func newRowRecogRepeatedStatePoolPlan(t *testing.T, env *Environment, name string) Plan {
	t.Helper()
	stream := From[rowRecogStatePoolEvent](env, "RowRecogStatePoolEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(
		RowVar("A").ZeroOrMore(),
		RowVar("B"),
	)).
		Define("A", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(1))).
		Define("B", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(2))).
		Measures(Alias("id", TagField[int64]("A", "id"))).
		Query(StatementName(name))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func newRowRecogAlternatingStatePoolPlan(t *testing.T, env *Environment, name string) Plan {
	t.Helper()
	stream := From[rowRecogStatePoolEvent](env, "RowRecogStatePoolEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(
		RowAlternation(RowVar("A"), RowVar("B")).ZeroOrMore(),
		RowVar("C"),
	)).
		Define("A", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(1))).
		Define("B", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(1))).
		Define("C", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(2))).
		Measures(Alias("id", TagField[int64]("A", "id"))).
		Query(StatementName(name))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func newRowRecogOptionalStatePoolPlan(t *testing.T, env *Environment, name string) Plan {
	t.Helper()
	stream := From[rowRecogStatePoolEvent](env, "RowRecogStatePoolEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(
		RowVar("A").Optional(),
		RowVar("B").Optional(),
		RowVar("C"),
	)).
		Define("A", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(1))).
		Define("B", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(2))).
		Define("C", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(3))).
		Measures(Alias("id", TagField[int64]("C", "id"))).
		Query(StatementName(name))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func newRowRecogPermutationStatePoolPlan(t *testing.T, env *Environment, name string) Plan {
	t.Helper()
	stream := From[rowRecogStatePoolEvent](env, "RowRecogStatePoolEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(
		RowPermute(RowVar("A"), RowVar("B")).ZeroOrMore(),
		RowVar("C"),
	)).
		Define("A", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(1))).
		Define("B", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(2))).
		Define("C", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(3))).
		Measures(Alias("id", TagField[int64]("C", "id"))).
		Query(StatementName(name))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func newRowRecogNestedRepeatStatePoolPlan(t *testing.T, env *Environment, name string) Plan {
	t.Helper()
	stream := From[rowRecogStatePoolEvent](env, "RowRecogStatePoolEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(
		RowSequence(RowVar("A"), RowVar("B")).ZeroOrMore(),
		RowVar("C"),
	)).
		Define("A", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(1))).
		Define("B", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(2))).
		Define("C", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(3))).
		Measures(Alias("id", TagField[int64]("C", "id"))).
		Query(StatementName(name))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func newRowRecogFiniteRangeStatePoolPlan(t *testing.T, env *Environment, name string) Plan {
	t.Helper()
	stream := From[rowRecogStatePoolEvent](env, "RowRecogStatePoolEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(
		RowVar("A").Repeat(2, 3),
		RowVar("B"),
	)).
		Define("A", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(1))).
		Define("B", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(2))).
		Measures(Alias("id", TagField[int64]("B", "id"))).
		Query(StatementName(name))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func rowRecogStatePoolActiveTotal(statement *Statement) int64 {
	if statement == nil || statement.runtime.rowRecogState == nil {
		return 0
	}
	partition := statement.runtime.rowRecogState.partitions["<all>"]
	if partition == nil {
		return 0
	}
	var total int64
	for _, count := range partition.activeStateCounts {
		total += count
	}
	return total
}

func TestRowRecogStatePoolCountsRepeatedNFAStates(t *testing.T) {
	env, engine := newRowRecogStatePoolTest(t, WithMatchRecognizeStateLimit(3, false))
	plan := newRowRecogRepeatedStatePoolPlan(t, env, "rowrecog-nfa-repeat")
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var limits []MatchRecognizeStateLimitEvent
	if err := engine.AddMatchRecognizeStateLimitListener(MatchRecognizeStateLimitListenerFunc(func(event MatchRecognizeStateLimitEvent) {
		limits = append(limits, event)
	})); err != nil {
		t.Fatal(err)
	}

	sendRowRecogStatePoolEvent(t, engine, "A", 1, 1)
	if len(limits) != 0 {
		t.Fatalf("first repeated event overflow = %#v", limits)
	}
	sendRowRecogStatePoolEvent(t, engine, "A", 1, 2)
	if len(limits) != 1 || limits[0].Counts[deployment.Statements()[0].ID()] != 3 {
		t.Fatalf("repeated NFA overflow = %#v, want one event at count 3", limits)
	}

	// B consumes all A* successor states and releases the full fan-out. A
	// later A therefore starts without another pool overflow.
	sendRowRecogStatePoolEvent(t, engine, "A", 2, 3)
	sendRowRecogStatePoolEvent(t, engine, "A", 1, 4)
	if len(limits) != 1 {
		t.Fatalf("repeated NFA state release did not free pool = %#v", limits)
	}
}

func TestRowRecogStatePoolCountsAlternatingNFAStates(t *testing.T) {
	env, engine := newRowRecogStatePoolTest(t, WithMatchRecognizeStateLimit(5, false))
	plan := newRowRecogAlternatingStatePoolPlan(t, env, "rowrecog-nfa-alternation")
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var limits []MatchRecognizeStateLimitEvent
	if err := engine.AddMatchRecognizeStateLimitListener(MatchRecognizeStateLimitListenerFunc(func(event MatchRecognizeStateLimitEvent) {
		limits = append(limits, event)
	})); err != nil {
		t.Fatal(err)
	}

	sendRowRecogStatePoolEvent(t, engine, "A", 1, 1)
	if len(limits) != 1 || limits[0].Counts[deployment.Statements()[0].ID()] != 5 {
		t.Fatalf("first alternating NFA overflow = %#v, want one event at count 5", limits)
	}
	sendRowRecogStatePoolEvent(t, engine, "A", 1, 2)
	statement := deployment.Statements()[0]
	partition := statement.runtime.rowRecogState.partitions["<all>"]
	if len(limits) != 19 {
		t.Fatalf("alternating NFA overflow events = %d, want 19 Java-style successor overflows: %#v", len(limits), limits)
	}
	total := int64(0)
	for _, count := range partition.activeStateCounts {
		total += count
	}
	if total != 18 {
		t.Fatalf("alternating NFA active state total = %d, want 18", total)
	}
	for index, event := range limits {
		if event.MaxStates != 5 || event.Counts[deployment.Statements()[0].ID()] < 5 {
			t.Fatalf("alternating NFA overflow[%d] = %#v, want max 5 and an over-limit count", index, event)
		}
	}

	sendRowRecogStatePoolEvent(t, engine, "A", 2, 3)
	if len(limits) != 19 {
		t.Fatalf("alternating NFA state release produced extra overflow = %#v", limits)
	}
}

func TestRowRecogStatePoolAllowsAcceptedTerminalWhenContinuationIsBlocked(t *testing.T) {
	env, engine := newRowRecogStatePoolTest(t, WithMatchRecognizeStateLimit(1, true))
	stream := From[rowRecogStatePoolEvent](env, "RowRecogStatePoolEvent").Window(KeepAll())
	query := stream.MatchRecognize(RowVar("A").ZeroOrMore()).
		Define("A", Equal[int](Field[rowRecogStatePoolEvent, int]("phase"), Literal(1))).
		Measures(
			Alias("id", TagField[int64]("A", "id")),
			Alias("size", TagSize("A")),
		).
		SkipToCurrentRow().
		Query(StatementName("rowrecog-nfa-terminal-accepted"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	var limits []MatchRecognizeStateLimitEvent
	if err := engine.AddMatchRecognizeStateLimitListener(MatchRecognizeStateLimitListenerFunc(func(event MatchRecognizeStateLimitEvent) {
		limits = append(limits, event)
	})); err != nil {
		t.Fatal(err)
	}

	sendRowRecogStatePoolEvent(t, engine, "A", 1, 1)
	sendRowRecogStatePoolEvent(t, engine, "A", 1, 2)
	if len(limits) != 1 {
		t.Fatalf("terminal-plus-continuation overflow = %#v, want one rejected continuation", limits)
	}
	if len(*rows) != 3 {
		t.Fatalf("accepted terminal rows = %#v, want the first match plus both second-event terminals", *rows)
	}
	last := (*rows)[2]
	if last.Get("id").Any() != int64(2) || last.Get("size").Any() != int64(1) {
		t.Fatalf("new blocked start terminal row = %#v, want id=2,size=1", last)
	}
}

func TestRowRecogStatePoolCountsFiniteOptionalPermutationAndNestedBranches(t *testing.T) {
	t.Run("optional", func(t *testing.T) {
		env, engine := newRowRecogStatePoolTest(t, WithMatchRecognizeStateLimit(1, false))
		plan := newRowRecogOptionalStatePoolPlan(t, env, "rowrecog-nfa-optional")
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		var limits []MatchRecognizeStateLimitEvent
		if err := engine.AddMatchRecognizeStateLimitListener(MatchRecognizeStateLimitListenerFunc(func(event MatchRecognizeStateLimitEvent) {
			limits = append(limits, event)
		})); err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		sendRowRecogStatePoolEvent(t, engine, "A", 1, 1)
		if len(limits) != 1 || rowRecogStatePoolActiveTotal(statement) != 2 {
			t.Fatalf("optional state count after A limits=%#v active=%d, want one overflow and two successors", limits, rowRecogStatePoolActiveTotal(statement))
		}
		sendRowRecogStatePoolEvent(t, engine, "B", 2, 2)
		if len(limits) != 2 || rowRecogStatePoolActiveTotal(statement) != 2 {
			t.Fatalf("optional successor fan-out limits=%#v active=%d, want two overflows and two states", limits, rowRecogStatePoolActiveTotal(statement))
		}
		sendRowRecogStatePoolEvent(t, engine, "C", 3, 3)
		if got := rowRecogStatePoolActiveTotal(statement); got != 0 {
			t.Fatalf("optional terminal release active=%d, want 0", got)
		}
	})

	t.Run("permutation", func(t *testing.T) {
		env, engine := newRowRecogStatePoolTest(t, WithMatchRecognizeStateLimit(2, false))
		plan := newRowRecogPermutationStatePoolPlan(t, env, "rowrecog-nfa-permutation")
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		var limits []MatchRecognizeStateLimitEvent
		if err := engine.AddMatchRecognizeStateLimitListener(MatchRecognizeStateLimitListenerFunc(func(event MatchRecognizeStateLimitEvent) {
			limits = append(limits, event)
		})); err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		sendRowRecogStatePoolEvent(t, engine, "A", 1, 1)
		sendRowRecogStatePoolEvent(t, engine, "B", 2, 2)
		if len(limits) != 2 || rowRecogStatePoolActiveTotal(statement) != 4 {
			t.Fatalf("permutation successor fan-out limits=%#v active=%d, want two overflows and four states", limits, rowRecogStatePoolActiveTotal(statement))
		}
		sendRowRecogStatePoolEvent(t, engine, "C", 3, 3)
		if got := rowRecogStatePoolActiveTotal(statement); got != 0 {
			t.Fatalf("permutation terminal release active=%d, want 0", got)
		}
	})

	t.Run("nested-repeat", func(t *testing.T) {
		env, engine := newRowRecogStatePoolTest(t, WithMatchRecognizeStateLimit(1, false))
		plan := newRowRecogNestedRepeatStatePoolPlan(t, env, "rowrecog-nfa-nested-repeat")
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		var limits []MatchRecognizeStateLimitEvent
		if err := engine.AddMatchRecognizeStateLimitListener(MatchRecognizeStateLimitListenerFunc(func(event MatchRecognizeStateLimitEvent) {
			limits = append(limits, event)
		})); err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		sendRowRecogStatePoolEvent(t, engine, "A", 1, 1)
		sendRowRecogStatePoolEvent(t, engine, "B", 2, 2)
		if len(limits) != 1 || rowRecogStatePoolActiveTotal(statement) != 2 {
			t.Fatalf("nested repeat successor fan-out limits=%#v active=%d, want one overflow and two states", limits, rowRecogStatePoolActiveTotal(statement))
		}
		sendRowRecogStatePoolEvent(t, engine, "C", 3, 3)
		if got := rowRecogStatePoolActiveTotal(statement); got != 0 {
			t.Fatalf("nested repeat terminal release active=%d, want 0", got)
		}
	})

	t.Run("finite-range", func(t *testing.T) {
		env, engine := newRowRecogStatePoolTest(t, WithMatchRecognizeStateLimit(2, false))
		plan := newRowRecogFiniteRangeStatePoolPlan(t, env, "rowrecog-nfa-finite-range")
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		var limits []MatchRecognizeStateLimitEvent
		if err := engine.AddMatchRecognizeStateLimitListener(MatchRecognizeStateLimitListenerFunc(func(event MatchRecognizeStateLimitEvent) {
			limits = append(limits, event)
		})); err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		sendRowRecogStatePoolEvent(t, engine, "A", 1, 1)
		if got := rowRecogStatePoolActiveTotal(statement); got != 1 {
			t.Fatalf("finite range first mandatory row active=%d, want 1 successor", got)
		}
		sendRowRecogStatePoolEvent(t, engine, "A", 1, 2)
		if len(limits) != 1 || rowRecogStatePoolActiveTotal(statement) != 3 {
			t.Fatalf("finite range second row limits=%#v active=%d, want one overflow and three states", limits, rowRecogStatePoolActiveTotal(statement))
		}
		sendRowRecogStatePoolEvent(t, engine, "B", 2, 3)
		if got := rowRecogStatePoolActiveTotal(statement); got != 0 {
			t.Fatalf("finite range terminal release active=%d, want 0", got)
		}
	})
}
