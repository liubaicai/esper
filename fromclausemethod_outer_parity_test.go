package esper

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// Parity coverage for Java's EPLFromClauseMethodOuterNStream regression suite:
// method sources under inner/left/right/full outer N-stream joins, including
// deployment-time evaluate-once seeding for constant-argument method streams.
// Java runtime IDs mapped here:
//
//	java-runtime-5e4e5ccb4cfd44bb9383  1Stream2HistStarSubordinateLeftRight
//	java-runtime-fef68112ee74271db7c7  1Stream2HistStarSubordinateInner
//	java-runtime-ea3e68f3641325942d20  2Stream1HistStarSubordinateLeftRight
//	java-runtime-e13d949b611999c2af33  1Stream2HistStarNoSubordinateLeftRight
//	java-runtime-fcf85edd19810058de22  Invalid (outer dependency diagnostics)

// makeFCMOuterMultiRowProvider mirrors Java SupportJoinMethods.fetchValMultiRow:
// number*rowsPerIndex rows with val = prefix+i+"_"+j and index = i. The s0-row
// arguments are read from the named dependency event so a keepall join
// re-evaluates the method per retained s0 row, like Esper's per-tuple
// historical evaluation.
func makeFCMOuterMultiRowProvider(histSchema Schema, prefix, depSource, numberField, rowsField string) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		dep, ok := request.Dependency(depSource)
		if !ok {
			return nil, fmt.Errorf("missing dependency %s", depSource)
		}
		number, _ := dep.Get(numberField).Any().(int)
		rowsPerIndex, _ := dep.Get(rowsField).Any().(int)
		if number <= 0 || rowsPerIndex <= 0 {
			return nil, nil
		}
		rows := make([]map[string]any, 0, number*rowsPerIndex)
		for i := 1; i <= number; i++ {
			for j := 0; j < rowsPerIndex; j++ {
				rows = append(rows, map[string]any{"val": fmt.Sprintf("%s%d_%d", prefix, i, j), "index": i})
			}
		}
		return newEventsFrom(histSchema, rows, request.Now)
	})
}

// makeFCMOuterIDProvider mirrors fetchVal(s0.id || 'H0', s0.p00): the val
// prefix is the dependency event id concatenated with suffix and the row
// count is read from the dependency event.
func makeFCMOuterIDProvider(histSchema Schema, depSource, suffix, countField string) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		dep, ok := request.Dependency(depSource)
		if !ok {
			return nil, fmt.Errorf("missing dependency %s", depSource)
		}
		count, _ := dep.Get(countField).Any().(int)
		if count <= 0 {
			return nil, nil
		}
		prefix, _ := dep.Get("id").Any().(string)
		prefix += suffix
		rows := make([]map[string]any, 0, count)
		for i := 1; i <= count; i++ {
			rows = append(rows, map[string]any{"val": fmt.Sprintf("%s%d", prefix, i), "index": i})
		}
		return newEventsFrom(histSchema, rows, request.Now)
	})
}

// makeFCMConstHistProvider mirrors fetchVal(prefix, count) with constant
// arguments: Esper classifies such method streams as evaluate-once and polls
// them a single time when the statement starts.
func makeFCMConstHistProvider(histSchema Schema, prefix string, count int) MethodProvider {
	return MethodProviderFunc(func(_ context.Context, request MethodRequest) ([]Event, error) {
		rows := make([]map[string]any, 0, count)
		for i := 1; i <= count; i++ {
			rows = append(rows, map[string]any{"val": fmt.Sprintf("%s%d", prefix, i), "index": i})
		}
		return newEventsFrom(histSchema, rows, request.Now)
	})
}

// fcmOuterListener captures the last new-rows batch without filtering by
// event id: outer join rows may carry null placeholders for the anchor stream.
type fcmOuterListener struct {
	lastNew []Row
	invoked bool
}

func fcmOuterSubscribe(t *testing.T, stmt *Statement) *fcmOuterListener {
	t.Helper()
	listener := &fcmOuterListener{}
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		listener.lastNew = lastNewRows(batch)
		listener.invoked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return listener
}

func fcmOuterSnapshot(t *testing.T, stmt *Statement) []Row {
	t.Helper()
	snapshot, err := stmt.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, len(snapshot.Results()))
	for _, result := range snapshot.Results() {
		if row, ok := result.Row(); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

// fcmOuterAssertRows compares rows against expected values in any order,
// rendering absent outer-join placeholders as <null>; an empty string in an
// expected row stands for Java null.
func fcmOuterAssertRows(t *testing.T, rows []Row, fields []string, expected [][]string, msg string) {
	t.Helper()
	if len(rows) != len(expected) {
		t.Fatalf("%s: got %d rows, want %d: %#v", msg, len(rows), len(expected), rows)
	}
	actual := make([]string, len(rows))
	for i, row := range rows {
		parts := make([]string, len(fields))
		for j, field := range fields {
			v := row.Get(field)
			if v.Any() == nil {
				parts[j] = "<null>"
			} else {
				parts[j] = fmt.Sprintf("%v", v.Any())
			}
		}
		actual[i] = strings.Join(parts, "|")
	}
	want := make([]string, len(expected))
	for i, exp := range expected {
		parts := make([]string, len(exp))
		for j, value := range exp {
			if value == "" {
				parts[j] = "<null>"
			} else {
				parts[j] = value
			}
		}
		want[i] = strings.Join(parts, "|")
	}
	sort.Strings(actual)
	sort.Strings(want)
	for i := range actual {
		if actual[i] != want[i] {
			t.Fatalf("%s: got %v, want %v", msg, actual, want)
		}
	}
}

// fcmOuterStep asserts one send step of the Java tryAssertion matrices: the
// lastNew rows (or that the listener was not invoked when expectedNew is nil)
// plus the cumulative iterator contents, mirroring assertPropsPerRowLastNew
// and assertPropsPerRowIteratorAnyOrder.
func fcmOuterStep(t *testing.T, engine *Engine, stmt *Statement, listener *fcmOuterListener, fields []string, event any, expectedNew, expectedSnapshot [][]string, label string) {
	t.Helper()
	listener.lastNew = nil
	listener.invoked = false
	if err := engine.SendEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if expectedNew == nil {
		if listener.invoked {
			t.Fatalf("%s: listener invoked unexpectedly with %#v", label, listener.lastNew)
		}
	} else {
		if !listener.invoked {
			t.Fatalf("%s: listener not invoked, want %#v", label, expectedNew)
		}
		fcmOuterAssertRows(t, listener.lastNew, fields, expectedNew, label+" lastNew")
	}
	fcmOuterAssertRows(t, fcmOuterSnapshot(t, stmt), fields, expectedSnapshot, label+" iterator")
}

func fcmOuterConcat(sets ...[][]string) [][]string {
	var result [][]string
	for _, set := range sets {
		result = append(result, set...)
	}
	return result
}

func fcmOuterBean6(id string, p00, p01, p02, p03, p04, p05 int) fcmBeanInt {
	return fcmBeanInt{ID: id, P00: p00, P01: p01, P02: p02, P03: p03, P04: p04, P05: p05}
}

func fcmOuterBean2(id string, p00, p01 int) fcmBeanInt {
	return fcmBeanInt{ID: id, P00: p00, P01: p01}
}

func fcmOuterBean1(id string, p00 int) fcmBeanInt {
	return fcmBeanInt{ID: id, P00: p00}
}

func fcmOuterDeploy(t *testing.T, env *Environment, query Query) (*Engine, *Statement, *fcmOuterListener) {
	t.Helper()
	plan, err := env.Build(query)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close(context.Background()) })
	stmt := deployment.Statements()[0]
	return engine, stmt, fcmOuterSubscribe(t, stmt)
}

func fcmOuterTwoHistSources(env *Environment, histSchema Schema) (s0 Stream[fcmBeanInt], h0, h1 Stream[map[string]any]) {
	s0 = FromAs[fcmBeanInt](env, "s0").Filter(StartsWith(Field[fcmBeanInt, string]("id"), Literal("E"))).Window(KeepAll())
	h0 = FromMethodOn[map[string]any](env, "h0", "SupportBeanInt", histSchema,
		makeFCMOuterMultiRowProvider(histSchema, "H0", "s0", "p00", "p04")).DependingOn("s0")
	h1 = FromMethodOn[map[string]any](env, "h1", "SupportBeanInt", histSchema,
		makeFCMOuterMultiRowProvider(histSchema, "H1", "s0", "p01", "p05")).DependingOn("s0")
	return s0, h0, h1
}

// fcmOuterRunAssertionOne mirrors tryAssertionOne: the shared event matrix and
// expectations of the 1Stream2HistStarSubordinateLeftRight variants.
func fcmOuterRunAssertionOne(t *testing.T, env *Environment, query Query) {
	t.Helper()
	fields := []string{"id", "valh0", "valh1"}
	engine, stmt, listener := fcmOuterDeploy(t, env, query)

	fcmOuterAssertRows(t, fcmOuterSnapshot(t, stmt), fields, nil, "deploy iterator")

	rE1 := [][]string{{"E1", "", ""}}
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean6("E1", 0, 0, 0, 0, 1, 1), rE1, rE1, "E1")
	rE2 := [][]string{{"E2", "H01_0", "H11_0"}}
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean6("E2", 1, 1, 1, 1, 1, 1), rE2, fcmOuterConcat(rE1, rE2), "E2")
	rE3 := [][]string{{"E3", "H03_0", "H14_0"}}
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean6("E3", 5, 5, 3, 4, 1, 1), rE3, fcmOuterConcat(rE1, rE2, rE3), "E3")
	rE4 := [][]string{{"E4", "", "H14_0"}}
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean6("E4", 0, 5, 3, 4, 1, 1), rE4, fcmOuterConcat(rE1, rE2, rE3, rE4), "E4")
	rE5 := [][]string{{"E5", "H02_0", ""}}
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean6("E5", 2, 0, 2, 1, 1, 1), rE5, fcmOuterConcat(rE1, rE2, rE3, rE4, rE5), "E5")
	rE6 := [][]string{{"E6", "H02_0", "H12_0"}, {"E6", "H02_1", "H12_0"}}
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean6("E6", 2, 2, 2, 2, 2, 1), rE6, fcmOuterConcat(rE1, rE2, rE3, rE4, rE5, rE6), "E6")
	rE7 := [][]string{{"E7", "H04_0", "H15_0"}, {"E7", "H04_0", "H15_1"}}
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean6("E7", 10, 10, 4, 5, 1, 2), rE7, fcmOuterConcat(rE1, rE2, rE3, rE4, rE5, rE6, rE7), "E7")
}

// fcmOuterRunAssertionTwo mirrors tryAssertionTwo: the inner-join variant of
// the same event matrix (no null-placeholder rows, unmatched events produce
// no listener invocation).
func fcmOuterRunAssertionTwo(t *testing.T, env *Environment, query Query) {
	t.Helper()
	fields := []string{"id", "valh0", "valh1"}
	engine, stmt, listener := fcmOuterDeploy(t, env, query)

	fcmOuterAssertRows(t, fcmOuterSnapshot(t, stmt), fields, nil, "deploy iterator")

	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean6("E1", 0, 0, 0, 0, 1, 1), nil, nil, "E1")
	rE2 := [][]string{{"E2", "H01_0", "H11_0"}}
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean6("E2", 1, 1, 1, 1, 1, 1), rE2, rE2, "E2")
	rE3 := [][]string{{"E3", "H03_0", "H14_0"}}
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean6("E3", 5, 5, 3, 4, 1, 1), rE3, fcmOuterConcat(rE2, rE3), "E3")
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean6("E4", 0, 5, 3, 4, 1, 1), nil, fcmOuterConcat(rE2, rE3), "E4")
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean6("E5", 2, 0, 2, 1, 1, 1), nil, fcmOuterConcat(rE2, rE3), "E5")
	rE6 := [][]string{{"E6", "H02_0", "H12_0"}, {"E6", "H02_1", "H12_0"}}
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean6("E6", 2, 2, 2, 2, 2, 1), rE6, fcmOuterConcat(rE2, rE3, rE6), "E6")
	rE7 := [][]string{{"E7", "H04_0", "H15_0"}, {"E7", "H04_0", "H15_1"}}
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean6("E7", 10, 10, 4, 5, 1, 2), rE7, fcmOuterConcat(rE2, rE3, rE6, rE7), "E7")
}

// fcmOuterRunAssertionSix mirrors tryAssertionSix: 2Stream1Hist with E%/F%
// filtered keepall streams and a subordinate method reading s0.id/s0.p00.
func fcmOuterRunAssertionSix(t *testing.T, env *Environment, query Query) {
	t.Helper()
	fields := []string{"s0id", "s1id", "valh0"}
	engine, stmt, listener := fcmOuterDeploy(t, env, query)

	fcmOuterAssertRows(t, fcmOuterSnapshot(t, stmt), fields, nil, "deploy iterator")

	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean2("E1", 1, 1), nil, nil, "E1")
	rF1 := [][]string{{"E1", "F1", "E1H01"}}
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean2("F1", 1, 1), rF1, rF1, "F1")
	rF2 := [][]string{{"", "F2", ""}}
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean2("F2", 2, 2), rF2, fcmOuterConcat(rF1, rF2), "F2")
	rE2 := [][]string{{"E2", "F2", "E2H02"}}
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean2("E2", 2, 2), rE2, fcmOuterConcat(rF1, rE2), "E2")
	rF3 := [][]string{{"", "F3", ""}}
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean2("F3", 3, 3), rF3, fcmOuterConcat(rF1, rE2, rF3), "F3")
	rE3 := [][]string{{"E3", "F3", ""}}
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean2("E3", 0, 3), rE3, fcmOuterConcat(rF1, rE2, rE3), "E3")
}

// fcmOuterRunAssertionSeven mirrors tryAssertionSeven: two constant-argument
// (evaluate-once) method streams right-outer joined against a keepall event
// stream, including the deployment-time iterator of unmatched placeholders.
func fcmOuterRunAssertionSeven(t *testing.T, env *Environment, query Query) {
	t.Helper()
	fields := []string{"s0id", "valh0", "valh1"}
	engine, stmt, listener := fcmOuterDeploy(t, env, query)

	seeded := [][]string{{"", "H01", ""}, {"", "H02", ""}, {"", "", "H11"}, {"", "", "H12"}}
	fcmOuterAssertRows(t, fcmOuterSnapshot(t, stmt), fields, seeded, "deploy iterator")

	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean1("E1", 0), nil, seeded, "E1")
	rE2 := [][]string{{"E2", "H02", "H12"}}
	itE2 := [][]string{{"", "H01", ""}, {"", "", "H11"}, {"E2", "H02", "H12"}}
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean1("E2", 2), rE2, itE2, "E2")
	rE3 := [][]string{{"E3", "H01", "H11"}}
	itE3 := [][]string{{"E3", "H01", "H11"}, {"E2", "H02", "H12"}}
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean1("E3", 1), rE3, itE3, "E3")
	rE4 := [][]string{{"E4", "H01", "H11"}}
	itE4 := [][]string{{"E3", "H01", "H11"}, {"E4", "H01", "H11"}, {"E2", "H02", "H12"}}
	fcmOuterStep(t, engine, stmt, listener, fields, fcmOuterBean1("E4", 1), rE4, itE4, "E4")
}

func TestFromClauseMethodOuterOneStreamTwoHistStarSubordinateInnerParity(t *testing.T) {
	t.Run("s0-first-inner", func(t *testing.T) {
		env := newFCMEnvironmentWithInt(t)
		histSchema := fcmHistSchema(t, env)
		s0, h0, h1 := fcmOuterTwoHistSources(env, histSchema)
		query := JoinMany(JoinSource(s0), JoinSource(h0), JoinSource(h1)).On(
			OnSourcesEqual(0, Field[fcmBeanInt, int]("p02"), 1, Field[map[string]any, int]("index")),
			OnSourcesEqual(0, Field[fcmBeanInt, int]("p03"), 2, Field[map[string]any, int]("index")),
		).Select(
			SelectFrom(0, "id", Field[fcmBeanInt, string]("id")),
			SelectFrom(1, "valh0", Field[map[string]any, string]("val")),
			SelectFrom(2, "valh1", Field[map[string]any, string]("val")),
		).Query(StatementName("fcm-outer-1s2h-inner-s0first"))
		fcmOuterRunAssertionTwo(t, env, query)
	})

	t.Run("h0-first-inner", func(t *testing.T) {
		env := newFCMEnvironmentWithInt(t)
		histSchema := fcmHistSchema(t, env)
		s0, h0, h1 := fcmOuterTwoHistSources(env, histSchema)
		query := JoinMany(JoinSource(h0), JoinSource(s0), JoinSource(h1)).On(
			OnSourcesEqual(1, Field[fcmBeanInt, int]("p02"), 0, Field[map[string]any, int]("index")),
			OnSourcesEqual(1, Field[fcmBeanInt, int]("p03"), 2, Field[map[string]any, int]("index")),
		).Select(
			SelectFrom(1, "id", Field[fcmBeanInt, string]("id")),
			SelectFrom(0, "valh0", Field[map[string]any, string]("val")),
			SelectFrom(2, "valh1", Field[map[string]any, string]("val")),
		).Query(StatementName("fcm-outer-1s2h-inner-h0first"))
		fcmOuterRunAssertionTwo(t, env, query)
	})
}

func TestFromClauseMethodOuterOneStreamTwoHistStarSubordinateLeftRightParity(t *testing.T) {
	t.Run("s0-left-h0-left-h1", func(t *testing.T) {
		env := newFCMEnvironmentWithInt(t)
		histSchema := fcmHistSchema(t, env)
		s0, h0, h1 := fcmOuterTwoHistSources(env, histSchema)
		query := JoinChain(JoinSource(s0)).
			LeftOuterJoin(JoinSource(h0), OnSourcesEqual(0, Field[fcmBeanInt, int]("p02"), 1, Field[map[string]any, int]("index"))).
			LeftOuterJoin(JoinSource(h1), OnSourcesEqual(0, Field[fcmBeanInt, int]("p03"), 2, Field[map[string]any, int]("index"))).
			Select(
				SelectFrom(0, "id", Field[fcmBeanInt, string]("id")),
				SelectFrom(1, "valh0", Field[map[string]any, string]("val")),
				SelectFrom(2, "valh1", Field[map[string]any, string]("val")),
			).Query(StatementName("fcm-outer-1s2h-left-left"))
		fcmOuterRunAssertionOne(t, env, query)
	})

	t.Run("h1-right-s0-left-h0", func(t *testing.T) {
		env := newFCMEnvironmentWithInt(t)
		histSchema := fcmHistSchema(t, env)
		s0, h0, h1 := fcmOuterTwoHistSources(env, histSchema)
		query := JoinChain(JoinSource(h1)).
			RightOuterJoin(JoinSource(s0), OnSourcesEqual(1, Field[fcmBeanInt, int]("p03"), 0, Field[map[string]any, int]("index"))).
			LeftOuterJoin(JoinSource(h0), OnSourcesEqual(1, Field[fcmBeanInt, int]("p02"), 2, Field[map[string]any, int]("index"))).
			Select(
				SelectFrom(1, "id", Field[fcmBeanInt, string]("id")),
				SelectFrom(2, "valh0", Field[map[string]any, string]("val")),
				SelectFrom(0, "valh1", Field[map[string]any, string]("val")),
			).Query(StatementName("fcm-outer-1s2h-right-left"))
		fcmOuterRunAssertionOne(t, env, query)
	})

	t.Run("h0-right-s0-left-h1", func(t *testing.T) {
		env := newFCMEnvironmentWithInt(t)
		histSchema := fcmHistSchema(t, env)
		s0, h0, h1 := fcmOuterTwoHistSources(env, histSchema)
		query := JoinChain(JoinSource(h0)).
			RightOuterJoin(JoinSource(s0), OnSourcesEqual(1, Field[fcmBeanInt, int]("p02"), 0, Field[map[string]any, int]("index"))).
			LeftOuterJoin(JoinSource(h1), OnSourcesEqual(1, Field[fcmBeanInt, int]("p03"), 2, Field[map[string]any, int]("index"))).
			Select(
				SelectFrom(1, "id", Field[fcmBeanInt, string]("id")),
				SelectFrom(0, "valh0", Field[map[string]any, string]("val")),
				SelectFrom(2, "valh1", Field[map[string]any, string]("val")),
			).Query(StatementName("fcm-outer-1s2h-right-left-h1"))
		fcmOuterRunAssertionOne(t, env, query)
	})

	// Java's fourth variant declares full outer joins on both edges. Esper
	// still anchors every output tuple on s0 because the subordinate method
	// rows only exist inside their dependency tuple's context; dependency-
	// bound rows that do not join simply vanish (joinStoredTupleHasAnchor).
	t.Run("h0-full-s0-full-h1", func(t *testing.T) {
		env := newFCMEnvironmentWithInt(t)
		histSchema := fcmHistSchema(t, env)
		s0, h0, h1 := fcmOuterTwoHistSources(env, histSchema)
		query := JoinChain(JoinSource(h0)).
			FullOuterJoin(JoinSource(s0), OnSourcesEqual(1, Field[fcmBeanInt, int]("p02"), 0, Field[map[string]any, int]("index"))).
			FullOuterJoin(JoinSource(h1), OnSourcesEqual(1, Field[fcmBeanInt, int]("p03"), 2, Field[map[string]any, int]("index"))).
			Select(
				SelectFrom(1, "id", Field[fcmBeanInt, string]("id")),
				SelectFrom(0, "valh0", Field[map[string]any, string]("val")),
				SelectFrom(2, "valh1", Field[map[string]any, string]("val")),
			).Query(StatementName("fcm-outer-1s2h-full-full"))
		fcmOuterRunAssertionOne(t, env, query)
	})
}

func TestFromClauseMethodOuterTwoStreamOneHistStarSubordinateLeftRightParity(t *testing.T) {
	sources := func(env *Environment, histSchema Schema) (s0, s1 Stream[fcmBeanInt], h0 Stream[map[string]any]) {
		s0 = FromAs[fcmBeanInt](env, "s0").Filter(StartsWith(Field[fcmBeanInt, string]("id"), Literal("E"))).Window(KeepAll())
		s1 = FromAs[fcmBeanInt](env, "s1").Filter(StartsWith(Field[fcmBeanInt, string]("id"), Literal("F"))).Window(KeepAll())
		h0 = FromMethodOn[map[string]any](env, "h0", "SupportBeanInt", histSchema,
			makeFCMOuterIDProvider(histSchema, "s0", "H0", "p00")).DependingOn("s0")
		return s0, s1, h0
	}

	t.Run("s0-left-h0-right-s1", func(t *testing.T) {
		env := newFCMEnvironmentWithInt(t)
		histSchema := fcmHistSchema(t, env)
		s0, s1, h0 := sources(env, histSchema)
		query := JoinChain(JoinSource(s0)).
			LeftOuterJoin(JoinSource(h0), OnSourcesEqual(0, Field[fcmBeanInt, int]("p01"), 1, Field[map[string]any, int]("index"))).
			RightOuterJoin(JoinSource(s1), OnSourcesEqual(2, Field[fcmBeanInt, int]("p01"), 0, Field[fcmBeanInt, int]("p01"))).
			Select(
				SelectFrom(0, "s0id", Field[fcmBeanInt, string]("id")),
				SelectFrom(2, "s1id", Field[fcmBeanInt, string]("id")),
				SelectFrom(1, "valh0", Field[map[string]any, string]("val")),
			).Query(StatementName("fcm-outer-2s1h-left-right"))
		fcmOuterRunAssertionSix(t, env, query)
	})

	t.Run("s1-left-s0-left-h0", func(t *testing.T) {
		env := newFCMEnvironmentWithInt(t)
		histSchema := fcmHistSchema(t, env)
		s0, s1, h0 := sources(env, histSchema)
		query := JoinChain(JoinSource(s1)).
			LeftOuterJoin(JoinSource(s0), OnSourcesEqual(0, Field[fcmBeanInt, int]("p01"), 1, Field[fcmBeanInt, int]("p01"))).
			LeftOuterJoin(JoinSource(h0), OnSourcesEqual(1, Field[fcmBeanInt, int]("p01"), 2, Field[map[string]any, int]("index"))).
			Select(
				SelectFrom(1, "s0id", Field[fcmBeanInt, string]("id")),
				SelectFrom(0, "s1id", Field[fcmBeanInt, string]("id")),
				SelectFrom(2, "valh0", Field[map[string]any, string]("val")),
			).Query(StatementName("fcm-outer-2s1h-left-left"))
		fcmOuterRunAssertionSix(t, env, query)
	})

	t.Run("h0-right-s0-right-s1", func(t *testing.T) {
		env := newFCMEnvironmentWithInt(t)
		histSchema := fcmHistSchema(t, env)
		s0, s1, h0 := sources(env, histSchema)
		query := JoinChain(JoinSource(h0)).
			RightOuterJoin(JoinSource(s0), OnSourcesEqual(1, Field[fcmBeanInt, int]("p01"), 0, Field[map[string]any, int]("index"))).
			RightOuterJoin(JoinSource(s1), OnSourcesEqual(2, Field[fcmBeanInt, int]("p01"), 1, Field[fcmBeanInt, int]("p01"))).
			Select(
				SelectFrom(1, "s0id", Field[fcmBeanInt, string]("id")),
				SelectFrom(2, "s1id", Field[fcmBeanInt, string]("id")),
				SelectFrom(0, "valh0", Field[map[string]any, string]("val")),
			).Query(StatementName("fcm-outer-2s1h-right-right"))
		fcmOuterRunAssertionSix(t, env, query)
	})
}

func TestFromClauseMethodOuterOneStreamTwoHistStarNoSubordinateLeftRightParity(t *testing.T) {
	build := func(env *Environment, histSchema Schema, name string) Query {
		s0 := FromAs[fcmBeanInt](env, "s0").Filter(StartsWith(Field[fcmBeanInt, string]("id"), Literal("E"))).Window(KeepAll())
		h0 := FromMethod[map[string]any](env, "h0", histSchema, makeFCMConstHistProvider(histSchema, "H0", 2)).EvaluateOnce()
		h1 := FromMethod[map[string]any](env, "h1", histSchema, makeFCMConstHistProvider(histSchema, "H1", 2)).EvaluateOnce()
		return JoinChain(JoinSource(s0)).
			RightOuterJoin(JoinSource(h0), OnSourcesEqual(0, Field[fcmBeanInt, int]("p00"), 1, Field[map[string]any, int]("index"))).
			RightOuterJoin(JoinSource(h1), OnSourcesEqual(0, Field[fcmBeanInt, int]("p00"), 2, Field[map[string]any, int]("index"))).
			Select(
				SelectFrom(0, "s0id", Field[fcmBeanInt, string]("id")),
				SelectFrom(1, "valh0", Field[map[string]any, string]("val")),
				SelectFrom(2, "valh1", Field[map[string]any, string]("val")),
			).Query(StatementName(name))
	}

	t.Run("s0-right-h0-right-h1", func(t *testing.T) {
		env := newFCMEnvironmentWithInt(t)
		histSchema := fcmHistSchema(t, env)
		fcmOuterRunAssertionSeven(t, env, build(env, histSchema, "fcm-outer-1s2h-nosub-right-right"))
	})

	// Java's second textual variant (h1 left outer join s0 right outer join
	// h0) asserts the exact same tryAssertionSeven matrix: Esper evaluates
	// N-way outer joins over constant method streams per anchor stream, so a
	// left-preserved h1 placeholder survives the later right edge. A
	// left-deep evaluation of that declared shape would drop the unmatched
	// h1 placeholder once s0 joins, hence the fluent port models the variant
	// through the observably equivalent s0-anchored right-outer chain (see
	// the capability manifest difference note).
	t.Run("h1-left-s0-right-h0-anchored-equivalent", func(t *testing.T) {
		env := newFCMEnvironmentWithInt(t)
		histSchema := fcmHistSchema(t, env)
		fcmOuterRunAssertionSeven(t, env, build(env, histSchema, "fcm-outer-1s2h-nosub-left-right"))
	})
}

// TestFromClauseMethodOuterInvalidParity mirrors EPLFromClauseMethodInvalid:
// a method stream may not depend on its own outer-join child/descendant, and
// a required stream may not depend on an optional (full-outer) stream.
func TestFromClauseMethodOuterInvalidParity(t *testing.T) {
	env := newFCMEnvironmentWithInt(t)
	histSchema := fcmHistSchema(t, env)
	provider := MethodProviderFunc(func(context.Context, MethodRequest) ([]Event, error) { return nil, nil })
	s0 := func() JoinInput {
		return JoinSource(FromAs[fcmBeanInt](env, "s0").Window(LastEvent()))
	}
	method := func(name string, dependencies ...string) JoinInput {
		stream := FromMethodOn[map[string]any](env, name, "SupportBeanInt", histSchema, provider)
		if len(dependencies) > 0 {
			stream = stream.DependingOn(dependencies...)
		}
		return JoinSource(stream)
	}
	selection := []JoinSelection{SelectFrom(0, "id", Field[fcmBeanInt, string]("id"))}

	// S0 left-outer H0 (H0 depends on H1) left-outer H1: H0 depends on its
	// own outer join descendant.
	dependentChild := JoinChain(s0()).
		LeftOuterJoin(method("h0", "h1"), OnSourcesEqual(0, Field[fcmBeanInt, int]("p00"), 1, Field[map[string]any, int]("index"))).
		LeftOuterJoin(method("h1"), OnSourcesEqual(1, Field[map[string]any, int]("index"), 2, Field[map[string]any, int]("index"))).
		Select(selection...).Query(StatementName("fcm-outer-invalid-dependent-child"))
	if _, err := env.Build(dependentChild); err == nil || !strings.Contains(err.Error(), "cannot or may not be satisfied") {
		t.Fatalf("dependent-child outer dependency error = %v", err)
	}

	// S0 full-outer H0 left-outer H1 (H1 depends on H0): H1 requires the
	// optional full-outer stream H0.
	optionalDependency := JoinChain(s0()).
		FullOuterJoin(method("h0"), OnSourcesEqual(0, Field[fcmBeanInt, int]("p00"), 1, Field[map[string]any, int]("index"))).
		LeftOuterJoin(method("h1", "h0"), OnSourcesEqual(0, Field[fcmBeanInt, int]("p00"), 2, Field[map[string]any, int]("index"))).
		Select(selection...).Query(StatementName("fcm-outer-invalid-optional-dependency"))
	if _, err := env.Build(optionalDependency); err == nil || !strings.Contains(err.Error(), "cannot or may not be satisfied") {
		t.Fatalf("optional-dependency outer dependency error = %v", err)
	}
}
