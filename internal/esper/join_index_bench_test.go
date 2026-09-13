package esper

import (
	"context"
	"fmt"
	"testing"
)

// joinIndexBenchEvent is the row shape for the composition benchmarks. The
// string key models the dominant equi-join shape; the two int fields model
// composite equi keys and the tag field models IN-set membership.
type joinIndexBenchEvent struct {
	Key   string `esper:"key"`
	PartA int    `esper:"partA"`
	PartB int    `esper:"partB"`
	Tag   string `esper:"tag"`
}

// BenchmarkJoinEquiIndexComposition measures per-event join tuple composition
// at steady-state window sizes. Every Send runs the before/after keyed-tuple
// diff over the full join state, so ns/op is dominated by the composition
// scan (legacy nested loop) or the equi/IN index probe (indexed path).
func BenchmarkJoinEquiIndexComposition(b *testing.B) {
	b.Run("equi-string-500x500", func(b *testing.B) {
		benchmarkJoinIndexComposition(b, joinIndexBenchShape{
			sides:    2,
			rows:     500,
			join:     joinIndexJoinEquiString,
			matchMod: 25,
		})
	})
	b.Run("equi-composite-200x200", func(b *testing.B) {
		benchmarkJoinIndexComposition(b, joinIndexBenchShape{
			sides:    2,
			rows:     200,
			join:     joinIndexJoinEquiComposite,
			matchMod: 25,
		})
	})
	b.Run("in-any-join-200x200", func(b *testing.B) {
		benchmarkJoinIndexComposition(b, joinIndexBenchShape{
			sides:    2,
			rows:     200,
			join:     joinIndexJoinAnyTags,
			matchMod: 10,
		})
	})
	b.Run("in-any-selective-200x200", func(b *testing.B) {
		benchmarkJoinIndexComposition(b, joinIndexBenchShape{
			sides:    2,
			rows:     200,
			join:     joinIndexJoinAnySelective,
			matchMod: 25,
		})
	})
	b.Run("three-stream-equi-100x100x100", func(b *testing.B) {
		benchmarkJoinIndexComposition(b, joinIndexBenchShape{
			sides:    3,
			rows:     100,
			join:     joinIndexJoinThreeStream,
			matchMod: 10,
		})
	})
	b.Run("chained-inner-100x100", func(b *testing.B) {
		benchmarkJoinIndexComposition(b, joinIndexBenchShape{
			sides:    2,
			rows:     100,
			join:     joinIndexJoinChained,
			matchMod: 10,
		})
	})
}

type joinIndexBenchShape struct {
	sides    int
	rows     int
	join     func(env *Environment, names []string, rows int) (*Query, error)
	matchMod int
}

func benchmarkJoinIndexComposition(b *testing.B, shape joinIndexBenchShape) {
	b.Helper()
	env := NewEnvironment()
	names := make([]string, shape.sides)
	for index := range names {
		names[index] = fmt.Sprintf("JoinIdxBenchS%d", index)
		if _, err := RegisterStruct[joinIndexBenchEvent](env, names[index]); err != nil {
			b.Fatal(err)
		}
	}
	query, err := shape.join(env, names, shape.rows)
	if err != nil {
		b.Fatal(err)
	}
	plan, err := env.Build(*query)
	if err != nil {
		b.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		b.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		return nil
	}); err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	send := func(source string, event joinIndexBenchEvent) {
		if err := engine.Send(ctx, source, event); err != nil {
			b.Fatal(err)
		}
	}
	// Fill both (all) sides so every measured update composes at the
	// steady-state window size.
	for i := 0; i < shape.rows; i++ {
		for index, name := range names {
			send(name, joinIndexBenchRow(index, i, shape.matchMod))
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for index, name := range names {
			send(name, joinIndexBenchRow(index, shape.rows+(i%shape.rows), shape.matchMod))
		}
	}
}

func joinIndexBenchRow(index, i, matchMod int) joinIndexBenchEvent {
	// Every matchMod-th row shares the same key so the probe has a bounded,
	// non-empty candidate set; the rest are unique and match nothing.
	if i%matchMod == 0 {
		return joinIndexBenchEvent{Key: "shared", PartA: 7, PartB: 11, Tag: "t0"}
	}
	return joinIndexBenchEvent{
		Key:   fmt.Sprintf("k-%d-%d", index, i),
		PartA: index*1000 + i,
		PartB: i,
		Tag:   fmt.Sprintf("t-%d", i%4),
	}
}

func joinIndexJoinEquiString(env *Environment, names []string, rows int) (*Query, error) {
	query := Join(
		From[joinIndexBenchEvent](env, names[0]).Window(LengthWindow(rows)),
		From[joinIndexBenchEvent](env, names[1]).Window(LengthWindow(rows)),
		OnEqual(
			Field[joinIndexBenchEvent, string]("key"),
			Field[joinIndexBenchEvent, string]("key"),
		),
	).Select(SelectLeft("key", Field[joinIndexBenchEvent, string]("key"))).Query(StatementName("bench-join-equi-index"))
	return &query, nil
}

func joinIndexJoinEquiComposite(env *Environment, names []string, rows int) (*Query, error) {
	query := Join(
		From[joinIndexBenchEvent](env, names[0]).Window(LengthWindow(rows)),
		From[joinIndexBenchEvent](env, names[1]).Window(LengthWindow(rows)),
		OnEqual(
			Field[joinIndexBenchEvent, int]("partA"),
			Field[joinIndexBenchEvent, int]("partA"),
		),
		OnEqual(
			Field[joinIndexBenchEvent, int]("partB"),
			Field[joinIndexBenchEvent, int]("partB"),
		),
	).Select(SelectLeft("key", Field[joinIndexBenchEvent, string]("key"))).Query(StatementName("bench-join-equi-index-composite"))
	return &query, nil
}

func joinIndexJoinAnyTags(env *Environment, names []string, rows int) (*Query, error) {
	left := Field[joinIndexBenchEvent, string]("tag")
	right := Field[joinIndexBenchEvent, string]("tag")
	query := Join(
		From[joinIndexBenchEvent](env, names[0]).Window(LengthWindow(rows)),
		From[joinIndexBenchEvent](env, names[1]).Window(LengthWindow(rows)),
		AnyJoin(
			OnEqual(left, right),
			OnEqual(left, Literal("t-1")),
		),
	).Select(SelectLeft("key", Field[joinIndexBenchEvent, string]("key"))).Query(StatementName("bench-join-in-index"))
	return &query, nil
}

// joinIndexJoinAnySelective is the selective IN/OR shape: both branches are
// selective (key equality, and partA equality whose unique rows never
// cross-match), and the "shared" rows match through BOTH branches so the
// multi-alternative dedupe is exercised at steady state.
func joinIndexJoinAnySelective(env *Environment, names []string, rows int) (*Query, error) {
	query := Join(
		From[joinIndexBenchEvent](env, names[0]).Window(LengthWindow(rows)),
		From[joinIndexBenchEvent](env, names[1]).Window(LengthWindow(rows)),
		AnyJoin(
			OnEqual(
				Field[joinIndexBenchEvent, string]("key"),
				Field[joinIndexBenchEvent, string]("key"),
			),
			OnEqual(
				Field[joinIndexBenchEvent, int]("partA"),
				Field[joinIndexBenchEvent, int]("partA"),
			),
		),
	).Select(SelectLeft("key", Field[joinIndexBenchEvent, string]("key"))).Query(StatementName("bench-join-in-index-selective"))
	return &query, nil
}

func joinIndexJoinThreeStream(env *Environment, names []string, rows int) (*Query, error) {
	inputs := make([]JoinInput, 3)
	for index := range inputs {
		inputs[index] = JoinSource(From[joinIndexBenchEvent](env, names[index]).Window(LengthWindow(rows)))
	}
	query := JoinMany(inputs...).On(
		OnSourcesEqual(0, Field[joinIndexBenchEvent, string]("key"), 1, Field[joinIndexBenchEvent, string]("key")),
		OnSourcesEqual(1, Field[joinIndexBenchEvent, string]("key"), 2, Field[joinIndexBenchEvent, string]("key")),
	).Select(
		SelectFrom(0, "key", JoinField[string](0, "key")),
	).Query(StatementName("bench-join-equi-index-three"))
	return &query, nil
}

func joinIndexJoinChained(env *Environment, names []string, rows int) (*Query, error) {
	key := Field[joinIndexBenchEvent, string]("key")
	query := JoinChain(
		JoinSource(From[joinIndexBenchEvent](env, names[0]).Window(LengthWindow(rows))),
	).InnerJoin(
		JoinSource(From[joinIndexBenchEvent](env, names[1]).Window(LengthWindow(rows))),
		OnSourcesEqual(0, key, 1, key),
	).Select(
		SelectFrom(0, "key", JoinField[string](0, "key")),
	).Query(StatementName("bench-join-equi-index-chained"))
	return &query, nil
}
