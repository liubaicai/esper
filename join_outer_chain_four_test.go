package esper

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type joinOuterChainEvent struct {
	Key string `esper:"key"`
	ID  string `esper:"id"`
}

// TestFourStreamOuterChainCardinalityMatchesEsper mirrors the four root
// executions in Java EPLOuterJoinChain4Stream. Each root uses a different
// source declaration and left/right edge direction, while the logical join
// graph remains s0 -> s1 -> s2 -> s3. The scenarios intentionally exercise
// the multi-row Cartesian cardinalities that a one-row topology test misses.
func TestFourStreamOuterChainCardinalityMatchesEsper(t *testing.T) {
	type rootVariant struct {
		name  string
		order []int
		kinds []JoinKind
		edges [][2]int
	}
	variants := []rootVariant{
		{name: "root-s0", order: []int{0, 1, 2, 3}, kinds: []JoinKind{JoinLeftOuter, JoinLeftOuter, JoinLeftOuter}, edges: [][2]int{{0, 1}, {1, 2}, {2, 3}}},
		{name: "root-s1", order: []int{1, 0, 2, 3}, kinds: []JoinKind{JoinRightOuter, JoinLeftOuter, JoinLeftOuter}, edges: [][2]int{{1, 0}, {1, 2}, {2, 3}}},
		{name: "root-s2", order: []int{2, 1, 0, 3}, kinds: []JoinKind{JoinRightOuter, JoinRightOuter, JoinLeftOuter}, edges: [][2]int{{2, 1}, {1, 0}, {2, 3}}},
		{name: "root-s3", order: []int{3, 2, 1, 0}, kinds: []JoinKind{JoinRightOuter, JoinRightOuter, JoinRightOuter}, edges: [][2]int{{3, 2}, {2, 1}, {1, 0}}},
	}
	type scenario struct {
		name   string
		counts [4]int
	}
	scenarios := []scenario{
		{name: "one-each", counts: [4]int{1, 1, 1, 1}},
		{name: "middle-one-missing-tail", counts: [4]int{1, 2, 1, 0}},
		{name: "two-by-two-missing-tail", counts: [4]int{1, 2, 2, 0}},
		{name: "two-by-two-by-two", counts: [4]int{1, 2, 2, 2}},
		{name: "three-tail", counts: [4]int{1, 1, 1, 3}},
		{name: "two-left-root", counts: [4]int{2, 1, 1, 0}},
		{name: "two-left-two-tail", counts: [4]int{2, 1, 1, 2}},
	}

	for _, variant := range variants {
		variant := variant
		t.Run(variant.name, func(t *testing.T) {
			env := NewEnvironment()
			streams := make([]Stream[joinOuterChainEvent], 4)
			for logical := 0; logical < 4; logical++ {
				name := fmt.Sprintf("JoinOuterChainS%d", logical)
				if _, err := RegisterStruct[joinOuterChainEvent](env, name); err != nil {
					t.Fatal(err)
				}
				streams[logical] = From[joinOuterChainEvent](env, name).Window(LengthWindow(1000))
			}

			chainIndex := make(map[int]int, 4)
			for index, logical := range variant.order {
				chainIndex[logical] = index
			}
			field := Field[joinOuterChainEvent, string]("key")
			chain := JoinChain(JoinSource(streams[variant.order[0]]))
			for edgeIndex, kind := range variant.kinds {
				leftLogical, rightLogical := variant.edges[edgeIndex][0], variant.edges[edgeIndex][1]
				condition := OnSourcesEqual(chainIndex[leftLogical], field, chainIndex[rightLogical], field)
				input := JoinSource(streams[variant.order[edgeIndex+1]])
				switch kind {
				case JoinLeftOuter:
					chain = chain.LeftOuterJoin(input, condition)
				case JoinRightOuter:
					chain = chain.RightOuterJoin(input, condition)
				default:
					t.Fatalf("unexpected join kind %d", kind)
				}
			}
			selections := make([]JoinSelection, 0, 4)
			for logical := 0; logical < 4; logical++ {
				selections = append(selections, SelectFrom(
					chainIndex[logical], fmt.Sprintf("s%d", logical), JoinField[string](chainIndex[logical], "id"),
				))
			}
			plan, err := env.Build(chain.Select(selections...).Query(
				StatementName("join-outer-chain-"+variant.name), WithOldStream(),
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
			var batches []ResultBatch
			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
				batches = append(batches, batch)
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			for scenarioIndex, current := range scenarios {
				current := current
				t.Run(current.name, func(t *testing.T) {
					key := fmt.Sprintf("%s-%d", variant.name, scenarioIndex)
					batches = nil
					for logical := 1; logical < 4; logical++ {
						for eventIndex := 0; eventIndex < current.counts[logical]; eventIndex++ {
							name := fmt.Sprintf("JoinOuterChainS%d", logical)
							if err := engine.Send(context.Background(), name, joinOuterChainEvent{
								Key: key, ID: fmt.Sprintf("%s-s%d-%d", key, logical, eventIndex),
							}); err != nil {
								t.Fatal(err)
							}
						}
					}
					if len(batches) != 0 {
						t.Fatalf("downstream-only events emitted %d batches: %#v", len(batches), batches)
					}
					for eventIndex := 0; eventIndex < current.counts[0]; eventIndex++ {
						if err := engine.Send(context.Background(), "JoinOuterChainS0", joinOuterChainEvent{
							Key: key, ID: fmt.Sprintf("%s-s0-%d", key, eventIndex),
						}); err != nil {
							t.Fatal(err)
						}
					}
					want := current.counts[0] * current.counts[1] * current.counts[2]
					if current.counts[3] > 0 {
						want *= current.counts[3]
					}
					var rows []Row
					for _, batch := range batches {
						for _, result := range batch.New {
							row, ok := result.Row()
							if !ok {
								t.Fatalf("outer chain result = %#v", result)
							}
							rows = append(rows, row)
						}
					}
					if len(rows) != want {
						t.Fatalf("outer chain %s new cardinality = %d, want %d", current.name, len(rows), want)
					}
					seen := make(map[string]int, len(rows))
					for _, row := range rows {
						values := make([]string, 4)
						for logical := range values {
							value := row.Get(fmt.Sprintf("s%d", logical))
							if value.IsNull() {
								values[logical] = "null"
							} else {
								values[logical] = value.String()
							}
						}
						seen[strings.Join(values, "/")]++
					}
					if len(seen) != want {
						t.Fatalf("outer chain %s unique rows = %d, want %d: %#v", current.name, len(seen), want, seen)
					}
				})
			}
		})
	}
}
