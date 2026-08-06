package esper

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type joinSevenOuterEvent struct {
	ID  int    `esper:"id"`
	Key string `esper:"key"`
}

func TestSevenStreamOuterRootVariantsMatchesEsper(t *testing.T) {
	type variant struct {
		name  string
		order []int
		kinds []JoinKind
		edges [][2]int
	}
	variants := []variant{
		{name: "root-s0", order: []int{0, 1, 2, 3, 4, 5, 6}, kinds: []JoinKind{JoinLeftOuter, JoinRightOuter, JoinRightOuter, JoinLeftOuter, JoinLeftOuter, JoinRightOuter}, edges: [][2]int{{0, 1}, {0, 2}, {1, 3}, {1, 4}, {2, 5}, {2, 6}}},
		{name: "root-s1", order: []int{1, 3, 4, 0, 2, 6, 5}, kinds: []JoinKind{JoinRightOuter, JoinLeftOuter, JoinRightOuter, JoinRightOuter, JoinRightOuter, JoinLeftOuter}, edges: [][2]int{{1, 3}, {1, 4}, {1, 0}, {0, 2}, {2, 6}, {2, 5}}},
		{name: "root-s2", order: []int{2, 6, 5, 0, 1, 3, 4}, kinds: []JoinKind{JoinRightOuter, JoinLeftOuter, JoinLeftOuter, JoinLeftOuter, JoinRightOuter, JoinLeftOuter}, edges: [][2]int{{2, 6}, {2, 5}, {2, 0}, {0, 1}, {1, 3}, {1, 4}}},
		{name: "root-s3", order: []int{3, 1, 4, 0, 2, 5, 6}, kinds: []JoinKind{JoinLeftOuter, JoinLeftOuter, JoinRightOuter, JoinRightOuter, JoinLeftOuter, JoinRightOuter}, edges: [][2]int{{3, 1}, {1, 4}, {1, 0}, {0, 2}, {2, 5}, {2, 6}}},
		{name: "root-s4", order: []int{4, 1, 3, 0, 2, 5, 6}, kinds: []JoinKind{JoinRightOuter, JoinRightOuter, JoinRightOuter, JoinRightOuter, JoinLeftOuter, JoinRightOuter}, edges: [][2]int{{4, 1}, {1, 3}, {1, 0}, {0, 2}, {2, 5}, {2, 6}}},
		{name: "root-s5", order: []int{5, 2, 6, 0, 1, 3, 4}, kinds: []JoinKind{JoinRightOuter, JoinRightOuter, JoinLeftOuter, JoinLeftOuter, JoinRightOuter, JoinLeftOuter}, edges: [][2]int{{5, 2}, {2, 6}, {2, 0}, {0, 1}, {1, 3}, {1, 4}}},
		{name: "root-s6", order: []int{6, 2, 5, 0, 1, 3, 4}, kinds: []JoinKind{JoinLeftOuter, JoinLeftOuter, JoinLeftOuter, JoinLeftOuter, JoinRightOuter, JoinLeftOuter}, edges: [][2]int{{6, 2}, {2, 5}, {2, 0}, {0, 1}, {1, 3}, {1, 4}}},
	}

	for _, current := range variants {
		current := current
		t.Run(current.name, func(t *testing.T) {
			env := NewEnvironment()
			for logical := 0; logical < 7; logical++ {
				if _, err := RegisterStruct[joinSevenOuterEvent](env, fmt.Sprintf("JoinSevenOuterS%d", logical)); err != nil {
					t.Fatal(err)
				}
			}
			streams := make([]Stream[joinSevenOuterEvent], 7)
			chainIndex := make(map[int]int, len(current.order))
			for logical := range streams {
				streams[logical] = From[joinSevenOuterEvent](env, fmt.Sprintf("JoinSevenOuterS%d", logical))
			}
			for index, logical := range current.order {
				chainIndex[logical] = index
			}
			field := Field[joinSevenOuterEvent, string]("key")
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
					t.Fatalf("unexpected seven-stream edge kind %d", kind)
				}
			}

			selections := make([]JoinSelection, 0, 7)
			for logical := 0; logical < 7; logical++ {
				selections = append(selections, SelectFrom(
					chainIndex[logical], fmt.Sprintf("s%d", logical), JoinField[string](chainIndex[logical], "key"),
				))
			}
			plan, err := env.Build(chain.Select(selections...).Query(StatementName("join-seven-outer-" + current.name)))
			if err != nil {
				t.Fatal(err)
			}
			engine := env.NewEngine()
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			defer deployment.Undeploy(context.Background())
			for logical := 0; logical < 7; logical++ {
				if err := engine.Send(context.Background(), fmt.Sprintf("JoinSevenOuterS%d", logical), joinSevenOuterEvent{ID: logical, Key: "A"}); err != nil {
					t.Fatal(err)
				}
			}

			snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			seen := make(map[string]bool)
			for _, result := range snapshot.Results() {
				row, ok := result.Row()
				if !ok {
					t.Fatalf("seven-stream outer result = %#v", result)
				}
				values := make([]string, 7)
				for logical := 0; logical < 7; logical++ {
					value := row.Get(fmt.Sprintf("s%d", logical))
					if value.IsNull() {
						values[logical] = "null"
					} else {
						values[logical] = value.String()
					}
				}
				seen[strings.Join(values, "/")] = true
			}
			if len(seen) != 1 || !seen["A/A/A/A/A/A/A"] {
				t.Fatalf("seven-stream outer %s rows = %#v", current.name, seen)
			}
		})
	}
}
