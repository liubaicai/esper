package esper

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
)

type stressSyntheticEvent struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

type stressJoinLeftEvent struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

type stressJoinRightEvent struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

// TestStressSyntheticMediumLoad is the periodic medium/stress synthetic load.
// It uses a fixed seed, exercises filter/window/aggregate, high-cardinality
// keyed contexts and join deploy/undeploy lifecycle, and asserts semantic
// invariants without timing thresholds.
func TestStressSyntheticMediumLoad(t *testing.T) {
	if os.Getenv("ESPER_STRESS") != "1" {
		t.Skip("set ESPER_STRESS=1 to run the periodic medium/stress synthetic load")
	}
	t.Run("filter-window-aggregate", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[stressSyntheticEvent](env, "StressAggregate"); err != nil {
			t.Fatal(err)
		}
		value := Field[stressSyntheticEvent, int64]("value")
		plan, err := env.Build(From[stressSyntheticEvent](env, "StressAggregate").
			Window(LengthWindow(200)).Aggregate(
			Alias("total", Sum[int64](value)),
		).Query(
			StatementName("stress-aggregate"),
		))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		rows := 0
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			rows += len(batch.New)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		rng := rand.New(rand.NewSource(42))
		ring := make([]int64, 200)
		var ringSum int64
		next := 0
		const total = 5000
		for i := 0; i < total; i++ {
			value := int64(rng.Intn(1000) + 1)
			if i >= 200 {
				ringSum -= ring[next]
			}
			ring[next] = value
			ringSum += value
			next = (next + 1) % 200
			if err := engine.SendEvent(context.Background(), stressSyntheticEvent{Key: fmt.Sprintf("K-%d", i), Value: value}); err != nil {
				t.Fatal(err)
			}
		}
		if rows == 0 {
			t.Fatal("filter-window-aggregate stress produced no listener rows")
		}
		snapshot, err := statement.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(snapshot.Batch.New) != 1 {
			t.Fatalf("aggregate snapshot rows = %d, want 1", len(snapshot.Batch.New))
		}
		row, ok := snapshot.Batch.New[0].Row()
		if !ok || row.Get("total").Any() != ringSum {
			t.Fatalf("aggregate snapshot total = %#v, want %d", row.AsMap(), ringSum)
		}
	})

	t.Run("high-cardinality-context", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[stressSyntheticEvent](env, "StressContext"); err != nil {
			t.Fatal(err)
		}
		if _, err := CreateKeyContext(env, "stress-context", Field[stressSyntheticEvent, string]("key")); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(From[stressSyntheticEvent](env, "StressContext").Aggregate(
			Alias("sum", Sum[int64](Field[stressSyntheticEvent, int64]("value"))),
		).Query(StatementName("stress-context"), WithContext("stress-context")))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		const partitions = 5000
		for i := 0; i < partitions; i++ {
			if err := engine.SendEvent(context.Background(), stressSyntheticEvent{Key: fmt.Sprintf("P%05d", i), Value: 1}); err != nil {
				t.Fatal(err)
			}
		}
		for i := 0; i < 1000; i++ {
			if err := engine.SendEvent(context.Background(), stressSyntheticEvent{Key: fmt.Sprintf("P%05d", i), Value: 2}); err != nil {
				t.Fatal(err)
			}
		}
		snapshot, err := statement.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(snapshot.Batch.New) != partitions {
			t.Fatalf("context snapshot rows = %d, want %d", len(snapshot.Batch.New), partitions)
		}
		for index, result := range snapshot.Batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("context snapshot result is not a row: %#v", result)
			}
			want := int64(1)
			if index < 1000 {
				want = 3
			}
			if got := row.Get("sum").Any().(int64); got != want {
				t.Fatalf("context sum row %d = %d, want %d", index, got, want)
			}
		}
	})

	t.Run("join-lifecycle", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[stressJoinLeftEvent](env, "StressJoinLeft"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[stressJoinRightEvent](env, "StressJoinRight"); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(Join(
			From[stressJoinLeftEvent](env, "StressJoinLeft").Window(LengthWindow(50)),
			From[stressJoinRightEvent](env, "StressJoinRight").Window(LengthWindow(50)),
			OnEqual(
				Field[stressJoinLeftEvent, string]("key"),
				Field[stressJoinRightEvent, string]("key"),
			),
		).Select(
			SelectLeft("key", Field[stressJoinLeftEvent, string]("key")),
		).Query(
			StatementName("stress-join"),
		))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		sendPairs := func(deployment *Deployment, count int) int {
			t.Helper()
			rows := 0
			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
				rows += len(batch.New)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < count; i++ {
				key := fmt.Sprintf("J-%d", i)
				if err := engine.SendEvent(context.Background(), stressJoinLeftEvent{Key: key, Value: 1}); err != nil {
					t.Fatal(err)
				}
				if err := engine.SendEvent(context.Background(), stressJoinRightEvent{Key: key, Value: 2}); err != nil {
					t.Fatal(err)
				}
			}
			return rows
		}
		first, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		if rows := sendPairs(first, 500); rows != 500 {
			t.Fatalf("first join rows = %d, want 500", rows)
		}
		if err := first.Undeploy(context.Background()); err != nil {
			t.Fatal(err)
		}
		second, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		if rows := sendPairs(second, 100); rows != 100 {
			t.Fatalf("redeployed join rows = %d, want 100 (old state must not leak)", rows)
		}
		if err := second.Undeploy(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
}
