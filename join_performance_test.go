package esper

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type joinPerformanceEvent struct {
	Key        string  `esper:"key"`
	IntValue   int     `esper:"intValue"`
	LongValue  int64   `esper:"longValue"`
	FloatValue float64 `esper:"floatValue"`
}

type joinPerformanceRange struct {
	Key   string `esper:"key"`
	Start int64  `esper:"start"`
	End   int64  `esper:"end"`
}

func TestJoinPerformanceBaseline(t *testing.T) {
	t.Run("two-stream-selectivity", func(t *testing.T) {
		env := NewEnvironment()
		for _, name := range []string{"JoinPerfSelectivityLeft", "JoinPerfSelectivityRight"} {
			if _, err := RegisterStruct[joinPerformanceEvent](env, name); err != nil {
				t.Fatal(err)
			}
		}
		plan, err := env.Build(Join(
			From[joinPerformanceEvent](env, "JoinPerfSelectivityLeft").Window(LengthWindow(128)),
			From[joinPerformanceEvent](env, "JoinPerfSelectivityRight").Window(LengthWindow(128)),
			OnEqual(
				Field[joinPerformanceEvent, string]("key"),
				Field[joinPerformanceEvent, string]("key"),
			),
		).Select(SelectLeft("key", Field[joinPerformanceEvent, string]("key"))).Query(
			StatementName("join-performance-selectivity"),
		))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer deployment.Undeploy(context.Background())
		matches := 0
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			matches += len(batch.New)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		started := time.Now()
		for i := 0; i < 20; i++ {
			sendJoinPerformance(t, engine, "JoinPerfSelectivityLeft", joinPerformanceEvent{Key: fmt.Sprintf("left-%d", i)})
			sendJoinPerformance(t, engine, "JoinPerfSelectivityRight", joinPerformanceEvent{Key: fmt.Sprintf("right-%d", i)})
		}
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("match-%d", i)
			sendJoinPerformance(t, engine, "JoinPerfSelectivityLeft", joinPerformanceEvent{Key: key})
			sendJoinPerformance(t, engine, "JoinPerfSelectivityRight", joinPerformanceEvent{Key: key})
		}
		if matches != 10 {
			t.Fatalf("selectivity matches = %d, want 10", matches)
		}
		t.Logf("two-stream selectivity baseline: %s", time.Since(started))
	})

	t.Run("range-and-coercion", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[joinPerformanceEvent](env, "JoinPerfRangeProbe"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[joinPerformanceRange](env, "JoinPerfRangeBounds"); err != nil {
			t.Fatal(err)
		}
		query := Join(
			From[joinPerformanceEvent](env, "JoinPerfRangeProbe").Window(LastEvent()),
			From[joinPerformanceRange](env, "JoinPerfRangeBounds").Window(LastEvent()),
			OnEqual(
				Field[joinPerformanceEvent, string]("key"),
				Field[joinPerformanceRange, string]("key"),
			),
			OnGreaterOrEqual(
				Field[joinPerformanceEvent, int]("intValue"),
				Field[joinPerformanceRange, int64]("start"),
			),
			OnLessOrEqual(
				Field[joinPerformanceEvent, int]("intValue"),
				Field[joinPerformanceRange, int64]("end"),
			),
		).Select(SelectLeft("value", Field[joinPerformanceEvent, int]("intValue"))).Query(
			StatementName("join-performance-range"),
		)
		plan, err := env.Build(query)
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer deployment.Undeploy(context.Background())
		matches := 0
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			matches += len(batch.New)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		started := time.Now()
		for i := 0; i < 80; i++ {
			key := fmt.Sprintf("G-%d", i)
			sendJoinPerformance(t, engine, "JoinPerfRangeProbe", joinPerformanceEvent{Key: key, IntValue: i})
			sendJoinPerformance(t, engine, "JoinPerfRangeBounds", joinPerformanceRange{Key: key, Start: int64(i), End: int64(i + 2)})
		}
		if matches != 80 {
			t.Fatalf("range matches = %d, want 80", matches)
		}
		t.Logf("range/coercion baseline: %s", time.Since(started))
	})

	t.Run("three-stream-coercion-and-outer", func(t *testing.T) {
		env := NewEnvironment()
		streams := make([]Stream[joinPerformanceEvent], 3)
		for logical := 0; logical < 3; logical++ {
			name := fmt.Sprintf("JoinPerfThreeS%d", logical)
			if _, err := RegisterStruct[joinPerformanceEvent](env, name); err != nil {
				t.Fatal(err)
			}
			streams[logical] = From[joinPerformanceEvent](env, name).Window(LastEvent())
		}
		inner, err := env.Build(JoinMany(
			JoinSource(streams[0]), JoinSource(streams[1]), JoinSource(streams[2]),
		).On(
			OnSourcesEqual(0, Field[joinPerformanceEvent, int64]("longValue"), 1, Field[joinPerformanceEvent, int]("intValue")),
			OnSourcesEqual(1, Field[joinPerformanceEvent, int]("intValue"), 2, Field[joinPerformanceEvent, float64]("floatValue")),
		).Select(SelectFrom(1, "value", JoinField[int](1, "intValue"))).Query(
			StatementName("join-performance-three-inner"),
		))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		innerDeployment, err := engine.Deploy(context.Background(), inner)
		if err != nil {
			t.Fatal(err)
		}
		defer innerDeployment.Undeploy(context.Background())
		matches := 0
		if _, err := innerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			matches += len(batch.New)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		started := time.Now()
		for i := 0; i < 60; i++ {
			key := fmt.Sprintf("K-%d", i)
			sendJoinPerformance(t, engine, "JoinPerfThreeS1", joinPerformanceEvent{Key: key, IntValue: i})
			sendJoinPerformance(t, engine, "JoinPerfThreeS2", joinPerformanceEvent{Key: key, FloatValue: float64(i)})
			sendJoinPerformance(t, engine, "JoinPerfThreeS0", joinPerformanceEvent{Key: key, LongValue: int64(i)})
		}
		if matches != 60 {
			t.Fatalf("three-stream coercion matches = %d, want 60", matches)
		}

		outerStreams := make([]Stream[joinPerformanceEvent], 3)
		for logical := 0; logical < 3; logical++ {
			name := fmt.Sprintf("JoinPerfThreeOuterS%d", logical)
			if _, err := RegisterStruct[joinPerformanceEvent](env, name); err != nil {
				t.Fatal(err)
			}
			outerStreams[logical] = From[joinPerformanceEvent](env, name).Window(LastEvent())
		}
		outerField := Field[joinPerformanceEvent, string]("key")
		outerChain := JoinChain(JoinSource(outerStreams[0])).
			LeftOuterJoin(JoinSource(outerStreams[1]), OnSourcesEqual(0, outerField, 1, outerField)).
			LeftOuterJoin(JoinSource(outerStreams[2]), OnSourcesEqual(1, outerField, 2, outerField))
		outer, err := env.Build(outerChain.Select(
			SelectFrom(0, "s0", JoinField[string](0, "key")),
			SelectFrom(1, "s1", JoinField[string](1, "key")),
			SelectFrom(2, "s2", JoinField[string](2, "key")),
		).Query(StatementName("join-performance-three-outer")))
		if err != nil {
			t.Fatal(err)
		}
		outerDeployment, err := engine.Deploy(context.Background(), outer)
		if err != nil {
			t.Fatal(err)
		}
		defer outerDeployment.Undeploy(context.Background())
		outerMatches := 0
		if _, err := outerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				row, ok := result.Row()
				if ok && !row.Get("s0").IsNull() && !row.Get("s1").IsNull() && !row.Get("s2").IsNull() {
					outerMatches++
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 20; i++ {
			key := fmt.Sprintf("O-%d", i)
			sendJoinPerformance(t, engine, "JoinPerfThreeOuterS1", joinPerformanceEvent{Key: key})
			sendJoinPerformance(t, engine, "JoinPerfThreeOuterS2", joinPerformanceEvent{Key: key})
			sendJoinPerformance(t, engine, "JoinPerfThreeOuterS0", joinPerformanceEvent{Key: key})
		}
		if outerMatches != 20 {
			t.Fatalf("three-stream outer matches = %d, want 20", outerMatches)
		}
		t.Logf("three-stream coercion/outer baseline: %s", time.Since(started))
	})

	t.Run("five-stream-readiness", func(t *testing.T) {
		env := NewEnvironment()
		inputs := make([]JoinInput, 5)
		for logical := range inputs {
			name := fmt.Sprintf("JoinPerfFiveS%d", logical)
			if _, err := RegisterStruct[joinPerformanceEvent](env, name); err != nil {
				t.Fatal(err)
			}
			inputs[logical] = JoinSource(From[joinPerformanceEvent](env, name).Window(LastEvent()))
		}
		field := Field[joinPerformanceEvent, string]("key")
		conditions := []JoinCondition{
			OnSourcesEqual(0, field, 1, field),
			OnSourcesEqual(1, field, 2, field),
			OnSourcesEqual(2, field, 3, field),
			OnSourcesEqual(3, field, 4, field),
		}
		plan, err := env.Build(JoinMany(inputs...).On(conditions...).Select(
			SelectFrom(0, "s0", JoinField[string](0, "key")),
			SelectFrom(4, "s4", JoinField[string](4, "key")),
		).Query(StatementName("join-performance-five")))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer deployment.Undeploy(context.Background())
		matches := 0
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			matches += len(batch.New)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		started := time.Now()
		for i := 0; i < 40; i++ {
			key := fmt.Sprintf("F-%d", i)
			for logical := 0; logical < 5; logical++ {
				sendJoinPerformance(t, engine, fmt.Sprintf("JoinPerfFiveS%d", logical), joinPerformanceEvent{Key: key})
			}
		}
		if matches != 40 {
			t.Fatalf("five-stream matches = %d, want 40", matches)
		}
		t.Logf("five-stream readiness baseline: %s", time.Since(started))
	})

	t.Run("unidirectional-range", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[joinPerformanceRange](env, "JoinPerfUniRange"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[joinPerformanceEvent](env, "JoinPerfUniProbe"); err != nil {
			t.Fatal(err)
		}
		fieldRange := Field[joinPerformanceRange, string]("key")
		fieldProbe := Field[joinPerformanceEvent, string]("key")
		plan, err := env.Build(JoinMany(
			JoinSource(From[joinPerformanceRange](env, "JoinPerfUniRange")).Unidirectional(),
			JoinSource(From[joinPerformanceEvent](env, "JoinPerfUniProbe").Window(KeepAll())),
		).On(
			OnSourcesEqual(0, fieldRange, 1, fieldProbe),
			OnSourcesCompare(1, Field[joinPerformanceEvent, int]("intValue"), 0, Field[joinPerformanceRange, int64]("start"), JoinGreaterOrEqual),
			OnSourcesCompare(1, Field[joinPerformanceEvent, int]("intValue"), 0, Field[joinPerformanceRange, int64]("end"), JoinLessOrEqual),
		).Select(SelectFrom(1, "value", JoinField[int](1, "intValue"))).Query(
			StatementName("join-performance-unidirectional-range"),
		))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer deployment.Undeploy(context.Background())
		matches := 0
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			matches += len(batch.New)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 100; i++ {
			sendJoinPerformance(t, engine, "JoinPerfUniProbe", joinPerformanceEvent{Key: "G", IntValue: i})
		}
		started := time.Now()
		for i := 0; i < 40; i++ {
			sendJoinPerformance(t, engine, "JoinPerfUniRange", joinPerformanceRange{Key: "G", Start: int64(i * 2), End: int64(i*2 + 2)})
		}
		if matches != 120 {
			t.Fatalf("unidirectional range matches = %d, want 120", matches)
		}
		t.Logf("unidirectional range baseline: %s", time.Since(started))
	})
}

func sendJoinPerformance(t *testing.T, engine *Engine, source string, event any) {
	t.Helper()
	if err := engine.Send(context.Background(), source, event); err != nil {
		t.Fatal(err)
	}
}
