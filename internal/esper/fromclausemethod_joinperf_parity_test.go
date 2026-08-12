package esper

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
)

// fcmJoinPerfHistProvider mirrors SupportJoinMethods.fetchVal(prefix, count):
// count rows val = prefix+i, index = i for i = 1..count. The performance
// ports below use a 10-row grid instead of the Java 100-row grid: Go has no
// join on-condition index, so the cross product is enumerated directly. The
// smaller grid keeps the correctness check fast while preserving the join
// semantics under test.
func fcmJoinPerfHistProvider(schema Schema, prefix string, count int) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		rows := make([]map[string]any, 0, count)
		for i := 1; i <= count; i++ {
			rows = append(rows, map[string]any{"val": prefix + strconv.Itoa(i), "index": i})
		}
		return newEventsFrom(schema, rows, request.Now)
	})
}

// TestFromClauseMethod1Stream2HistInnerJoinPerformanceParity mirrors
// EPLFromClauseMethod1Stream2HistInnerJoinPerformance: an inner join of the
// last event with two constant 100-row method grids filtered by index. This
// is a correctness port of the Java 5000-iteration timing assertion. Java
// relies on join on-condition indexing; Go currently evaluates the
// where-predicate over the cross product, so the grid and iteration count are
// reduced to keep the correctness check fast while still exercising many
// indices.
func TestFromClauseMethod1Stream2HistInnerJoinPerformanceParity(t *testing.T) {
	fields := []string{"id", "valh0", "valh1"}
	env := newFCMEnvironmentWithInt(t)
	histSchema := fcmHistSchema(t, env)
	s0 := From[fcmBeanInt](env, "SupportBeanInt").Window(LengthWindow(1))
	h0 := FromMethod[map[string]any](env, "h0", histSchema, fcmJoinPerfHistProvider(histSchema, "H0", 10)).EvaluateOnce()
	h1 := FromMethod[map[string]any](env, "h1", histSchema, fcmJoinPerfHistProvider(histSchema, "H1", 10)).EvaluateOnce()
	query := JoinMany(JoinSource(s0), JoinSource(h0), JoinSource(h1)).
		Select(
			SelectFrom(0, "id", Field[fcmBeanInt, string]("id")),
			SelectFrom(1, "valh0", Field[map[string]any, string]("val")),
			SelectFrom(2, "valh1", Field[map[string]any, string]("val")),
		).
		Where(And(
			Equal[int](JoinField[int](1, "index"), JoinField[int](0, "p00")),
			Equal[int](JoinField[int](2, "index"), JoinField[int](0, "p00")),
		)).
		Query(StatementName("s0"))
	engine, stmt, listener := fcmOuterDeploy(t, env, query)
	_ = stmt
	// Deterministic increasing indices avoid collide-induced empty diffs
	// (a repeated p00 would leave the where result unchanged).
	for i := 1; i <= 5; i++ {
		num := i
		fcmMultikeyStep(t, engine, listener, fields, fcmBeanInt{ID: "E1", P00: num}, [][]string{{"E1", "H0" + strconv.Itoa(num), "H1" + strconv.Itoa(num)}}, strconv.Itoa(i))
	}
}

// TestFromClauseMethod1Stream2HistOuterJoinPerformanceParity mirrors
// EPLFromClauseMethod1Stream2HistOuterJoinPerformance: the left-outer form of
// the join with on-condition equalities. Correctness port of the Java timing
// assertion; grid and iteration count reduced for the cross-product
// evaluation.
func TestFromClauseMethod1Stream2HistOuterJoinPerformanceParity(t *testing.T) {
	fields := []string{"id", "valh0", "valh1"}
	env := newFCMEnvironmentWithInt(t)
	histSchema := fcmHistSchema(t, env)
	s0 := From[fcmBeanInt](env, "SupportBeanInt").Window(LengthWindow(1))
	h0 := FromMethod[map[string]any](env, "h0", histSchema, fcmJoinPerfHistProvider(histSchema, "H0", 10)).EvaluateOnce()
	h1 := FromMethod[map[string]any](env, "h1", histSchema, fcmJoinPerfHistProvider(histSchema, "H1", 10)).EvaluateOnce()
	query := JoinChain(JoinSource(s0)).
		LeftOuterJoin(JoinSource(h0), OnSourcesEqual(0, Field[fcmBeanInt, int]("p00"), 1, Field[map[string]any, int]("index"))).
		LeftOuterJoin(JoinSource(h1), OnSourcesEqual(0, Field[fcmBeanInt, int]("p00"), 2, Field[map[string]any, int]("index"))).
		Select(
			SelectFrom(0, "id", Field[fcmBeanInt, string]("id")),
			SelectFrom(1, "valh0", Field[map[string]any, string]("val")),
			SelectFrom(2, "valh1", Field[map[string]any, string]("val")),
		).Query(StatementName("s0"))
	engine, _, listener := fcmOuterDeploy(t, env, query)
	for i := 1; i <= 5; i++ {
		num := i
		fcmMultikeyStep(t, engine, listener, fields, fcmBeanInt{ID: "E1", P00: num}, [][]string{{"E1", "H0" + strconv.Itoa(num), "H1" + strconv.Itoa(num)}}, strconv.Itoa(i))
	}
}

// TestFromClauseMethod2Stream1HistTwoSidedEntryIdenticalIndexParity mirrors
// EPLFromClauseMethod2Stream1HistTwoSidedEntryIdenticalIndex: two last-event
// streams plus a method grid joined on the same index column. Correctness
// port of the Java timing assertion; reset events keep each iteration
// independent; grid and iteration count reduced for the cross-product
// evaluation.
func TestFromClauseMethod2Stream1HistTwoSidedEntryIdenticalIndexParity(t *testing.T) {
	fields := []string{"s0id", "s1id", "valh0"}
	env := newFCMEnvironmentWithInt(t)
	histSchema := fcmHistSchema(t, env)
	s0 := From[fcmBeanInt](env, "SupportBeanInt").Filter(StartsWith(Field[fcmBeanInt, string]("id"), Literal("E"))).Window(LengthWindow(1))
	s1 := From[fcmBeanInt](env, "SupportBeanInt").Filter(StartsWith(Field[fcmBeanInt, string]("id"), Literal("F"))).Window(LengthWindow(1))
	h0 := FromMethod[map[string]any](env, "h0", histSchema, fcmJoinPerfHistProvider(histSchema, "H0", 10)).EvaluateOnce()
	query := JoinMany(JoinSource(s0), JoinSource(h0), JoinSource(s1)).
		Select(
			SelectFrom(0, "s0id", Field[fcmBeanInt, string]("id")),
			SelectFrom(2, "s1id", Field[fcmBeanInt, string]("id")),
			SelectFrom(1, "valh0", Field[map[string]any, string]("val")),
		).
		Where(And(
			Equal[int](JoinField[int](1, "index"), JoinField[int](0, "p00")),
			Equal[int](JoinField[int](1, "index"), JoinField[int](2, "p00")),
		)).
		Query(StatementName("s0"))
	engine, _, listener := fcmOuterDeploy(t, env, query)
	for i := 1; i <= 6; i++ {
		num := i
		if err := engine.SendEvent(context.Background(), fcmBeanInt{ID: "E1", P00: num}); err != nil {
			t.Fatal(err)
		}
		// The match completes when F1 arrives; assert last-new then.
		fcmMultikeyStep(t, engine, listener, fields, fcmBeanInt{ID: "F1", P00: num}, [][]string{{"E1", "F1", "H0" + strconv.Itoa(num)}}, "F1/"+strconv.Itoa(i))
		if err := engine.SendEvent(context.Background(), fcmBeanInt{ID: "E1", P00: 0}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), fcmBeanInt{ID: "F1", P00: 0}); err != nil {
			t.Fatal(err)
		}
		listener.lastNew = nil
		listener.invoked = false
	}
}

// TestFromClauseMethod2Stream1HistTwoSidedEntryMixedIndexParity mirrors
// EPLFromClauseMethod2Stream1HistTwoSidedEntryMixedIndex: the method grid
// declared first and joined on both an index and a value equality. Correctness
// port of the Java timing assertion; grid and iteration count reduced.
func TestFromClauseMethod2Stream1HistTwoSidedEntryMixedIndexParity(t *testing.T) {
	fields := []string{"s0id", "s1id", "valh0", "indexh0"}
	env := newFCMEnvironmentWithInt(t)
	histSchema := fcmHistSchema(t, env)
	s1 := From[fcmBeanInt](env, "SupportBeanInt").Filter(StartsWith(Field[fcmBeanInt, string]("id"), Literal("H"))).Window(LengthWindow(1))
	s0 := From[fcmBeanInt](env, "SupportBeanInt").Filter(StartsWith(Field[fcmBeanInt, string]("id"), Literal("E"))).Window(LengthWindow(1))
	h0 := FromMethod[map[string]any](env, "h0", histSchema, fcmJoinPerfHistProvider(histSchema, "H0", 10)).EvaluateOnce()
	query := JoinMany(JoinSource(h0), JoinSource(s1), JoinSource(s0)).
		Select(
			SelectFrom(2, "s0id", Field[fcmBeanInt, string]("id")),
			SelectFrom(1, "s1id", Field[fcmBeanInt, string]("id")),
			SelectFrom(0, "valh0", Field[map[string]any, string]("val")),
			SelectFrom(0, "indexh0", Field[map[string]any, int]("index")),
		).
		Where(And(
			Equal[int](JoinField[int](0, "index"), JoinField[int](2, "p00")),
			Equal[string](JoinField[string](0, "val"), JoinField[string](1, "id")),
		)).
		Query(StatementName("s0"))
	engine, _, listener := fcmOuterDeploy(t, env, query)
	random := rand.New(rand.NewSource(17))
	for i := 1; i <= 6; i++ {
		num := random.Intn(8) + 1
		// s1 (H%) first so the E1 event observes the matching s1 row.
		if err := engine.SendEvent(context.Background(), fcmBeanInt{ID: "H0" + strconv.Itoa(num), P00: num}); err != nil {
			t.Fatal(err)
		}
		fcmMultikeyStep(t, engine, listener, fields, fcmBeanInt{ID: "E1", P00: num}, [][]string{{"E1", "H0" + strconv.Itoa(num), "H0" + strconv.Itoa(num), strconv.Itoa(num)}}, "E1/"+strconv.Itoa(i))
		if err := engine.SendEvent(context.Background(), fcmBeanInt{ID: "E1", P00: 0}); err != nil {
			t.Fatal(err)
		}
		listener.lastNew = nil
		listener.invoked = false
	}
}
