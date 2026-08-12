package esper

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type joinCartesianOuterEvent struct {
	ID  int    `esper:"id"`
	Key string `esper:"key"`
}

func TestCartesianOuterHubVariantsMatchEsper(t *testing.T) {
	type variant struct {
		name  string
		order []int
		kinds []JoinKind
		edges [][2]int
	}
	four := []variant{
		{name: "cart4-root-s0", order: []int{0, 1, 2, 3}, kinds: []JoinKind{JoinLeftOuter, JoinLeftOuter, JoinLeftOuter}, edges: [][2]int{{0, 1}, {0, 2}, {0, 3}}},
		{name: "cart4-root-s1", order: []int{1, 0, 2, 3}, kinds: []JoinKind{JoinRightOuter, JoinLeftOuter, JoinLeftOuter}, edges: [][2]int{{1, 0}, {0, 2}, {0, 3}}},
		{name: "cart4-root-s2", order: []int{2, 0, 1, 3}, kinds: []JoinKind{JoinRightOuter, JoinLeftOuter, JoinLeftOuter}, edges: [][2]int{{2, 0}, {0, 1}, {0, 3}}},
		{name: "cart4-root-s3", order: []int{3, 0, 1, 2}, kinds: []JoinKind{JoinRightOuter, JoinLeftOuter, JoinLeftOuter}, edges: [][2]int{{3, 0}, {0, 1}, {0, 2}}},
	}
	five := []variant{
		{name: "cart5-root-s0", order: []int{0, 1, 2, 3, 4}, kinds: []JoinKind{JoinRightOuter, JoinLeftOuter, JoinLeftOuter, JoinLeftOuter}, edges: [][2]int{{0, 1}, {1, 2}, {1, 3}, {1, 4}}},
		{name: "cart5-root-s1", order: []int{1, 2, 3, 4, 0}, kinds: []JoinKind{JoinLeftOuter, JoinLeftOuter, JoinLeftOuter, JoinLeftOuter}, edges: [][2]int{{1, 2}, {1, 3}, {1, 4}, {1, 0}}},
		{name: "cart5-root-s1-order2", order: []int{1, 2, 0, 4, 3}, kinds: []JoinKind{JoinLeftOuter, JoinLeftOuter, JoinLeftOuter, JoinLeftOuter}, edges: [][2]int{{1, 2}, {1, 0}, {1, 4}, {1, 3}}},
		{name: "cart5-root-s2", order: []int{2, 1, 3, 4, 0}, kinds: []JoinKind{JoinRightOuter, JoinLeftOuter, JoinLeftOuter, JoinLeftOuter}, edges: [][2]int{{2, 1}, {1, 3}, {1, 4}, {1, 0}}},
		{name: "cart5-root-s2-order2", order: []int{2, 1, 4, 0, 3}, kinds: []JoinKind{JoinRightOuter, JoinLeftOuter, JoinLeftOuter, JoinLeftOuter}, edges: [][2]int{{2, 1}, {1, 4}, {1, 0}, {1, 3}}},
		{name: "cart5-root-s3", order: []int{3, 1, 2, 4, 0}, kinds: []JoinKind{JoinRightOuter, JoinLeftOuter, JoinLeftOuter, JoinLeftOuter}, edges: [][2]int{{3, 1}, {1, 2}, {1, 4}, {1, 0}}},
		{name: "cart5-root-s3-order2", order: []int{3, 1, 4, 0, 2}, kinds: []JoinKind{JoinRightOuter, JoinLeftOuter, JoinLeftOuter, JoinLeftOuter}, edges: [][2]int{{3, 1}, {1, 4}, {1, 0}, {1, 2}}},
		{name: "cart5-root-s4", order: []int{4, 1, 3, 0, 2}, kinds: []JoinKind{JoinRightOuter, JoinLeftOuter, JoinLeftOuter, JoinLeftOuter}, edges: [][2]int{{4, 1}, {1, 3}, {1, 0}, {1, 2}}},
		{name: "cart5-root-s4-order2", order: []int{4, 1, 0, 2, 3}, kinds: []JoinKind{JoinRightOuter, JoinLeftOuter, JoinLeftOuter, JoinLeftOuter}, edges: [][2]int{{4, 1}, {1, 0}, {1, 2}, {1, 3}}},
	}

	for _, current := range append(four, five...) {
		current := current
		t.Run(current.name, func(t *testing.T) {
			env := NewEnvironment()
			count := len(current.order)
			for logical := 0; logical < count; logical++ {
				if _, err := RegisterStruct[joinCartesianOuterEvent](env, fmt.Sprintf("JoinCartesianOuterS%d", logical)); err != nil {
					t.Fatal(err)
				}
			}
			streams := make([]Stream[joinCartesianOuterEvent], count)
			chainIndex := make(map[int]int, count)
			for index, logical := range current.order {
				streams[logical] = From[joinCartesianOuterEvent](env, fmt.Sprintf("JoinCartesianOuterS%d", logical))
				chainIndex[logical] = index
			}
			field := Field[joinCartesianOuterEvent, string]("key")
			chain := JoinChain(JoinSource(streams[current.order[0]]))
			for edgeIndex, kind := range current.kinds {
				leftLogical, rightLogical := current.edges[edgeIndex][0], current.edges[edgeIndex][1]
				condition := OnSourcesEqual(chainIndex[leftLogical], field, chainIndex[rightLogical], field)
				input := JoinSource(streams[current.order[edgeIndex+1]])
				switch kind {
				case JoinLeftOuter:
					chain = chain.LeftOuterJoin(input, condition)
				case JoinRightOuter:
					chain = chain.RightOuterJoin(input, condition)
				default:
					t.Fatalf("unexpected cartesian edge kind %d", kind)
				}
			}
			selections := make([]JoinSelection, 0, count)
			for logical := 0; logical < count; logical++ {
				selections = append(selections, SelectFrom(
					chainIndex[logical], fmt.Sprintf("s%d", logical), JoinField[string](chainIndex[logical], "key"),
				))
			}
			plan, err := env.Build(chain.Select(selections...).Query(StatementName("join-cartesian-" + current.name)))
			if err != nil {
				t.Fatal(err)
			}
			engine := env.NewEngine()
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			defer deployment.Undeploy(context.Background())
			for logical := 0; logical < count; logical++ {
				if err := engine.Send(context.Background(), fmt.Sprintf("JoinCartesianOuterS%d", logical), joinCartesianOuterEvent{ID: logical, Key: "A"}); err != nil {
					t.Fatal(err)
				}
			}

			snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			full := strings.Repeat("A/", count-1) + "A"
			seen := make(map[string]bool)
			for _, result := range snapshot.Results() {
				row, ok := result.Row()
				if !ok {
					t.Fatalf("cartesian outer result = %#v", result)
				}
				values := make([]string, count)
				for logical := 0; logical < count; logical++ {
					value := row.Get(fmt.Sprintf("s%d", logical))
					if value.IsNull() {
						values[logical] = "null"
					} else {
						values[logical] = value.String()
					}
				}
				seen[strings.Join(values, "/")] = true
			}
			if !seen[full] {
				t.Fatalf("cartesian outer %s rows = %#v", current.name, seen)
			}
		})
	}
}
