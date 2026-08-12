package esper

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type joinVarOuterEvent struct {
	ID  int    `esper:"id"`
	Key string `esper:"key"`
}

func TestThreeStreamOuterVarRootVariantsMatchEsper(t *testing.T) {
	type variant struct {
		name  string
		order []int
		kinds []JoinKind
		edges [][2]int
	}
	variants := []variant{
		{name: "var-a-root-s0", order: []int{0, 1, 2}, kinds: []JoinKind{JoinLeftOuter, JoinLeftOuter}, edges: [][2]int{{0, 1}, {0, 2}}},
		{name: "var-a-root-s1", order: []int{1, 0, 2}, kinds: []JoinKind{JoinRightOuter, JoinLeftOuter}, edges: [][2]int{{1, 0}, {0, 2}}},
		{name: "var-a-root-s2", order: []int{2, 0, 1}, kinds: []JoinKind{JoinRightOuter, JoinLeftOuter}, edges: [][2]int{{2, 0}, {0, 1}}},
		{name: "var-b-root-s0", order: []int{0, 1, 2}, kinds: []JoinKind{JoinLeftOuter, JoinRightOuter}, edges: [][2]int{{0, 1}, {0, 2}}},
		{name: "var-b-root-s1", order: []int{1, 0, 2}, kinds: []JoinKind{JoinRightOuter, JoinRightOuter}, edges: [][2]int{{1, 0}, {0, 2}}},
		{name: "var-b-root-s2", order: []int{2, 0, 1}, kinds: []JoinKind{JoinLeftOuter, JoinLeftOuter}, edges: [][2]int{{2, 0}, {0, 1}}},
		{name: "var-c-root-s0", order: []int{0, 1, 2}, kinds: []JoinKind{JoinRightOuter, JoinRightOuter}, edges: [][2]int{{0, 1}, {0, 2}}},
		{name: "var-c-root-s1", order: []int{1, 0, 2}, kinds: []JoinKind{JoinLeftOuter, JoinRightOuter}, edges: [][2]int{{1, 0}, {0, 2}}},
		{name: "var-c-root-s2", order: []int{2, 0, 1}, kinds: []JoinKind{JoinLeftOuter, JoinRightOuter}, edges: [][2]int{{2, 0}, {0, 1}}},
	}

	for _, current := range variants {
		current := current
		t.Run(current.name, func(t *testing.T) {
			env := NewEnvironment()
			streams := make([]Stream[joinVarOuterEvent], 3)
			chainIndex := make(map[int]int, 3)
			for logical := 0; logical < 3; logical++ {
				name := fmt.Sprintf("JoinVarOuterS%d", logical)
				if _, err := RegisterStruct[joinVarOuterEvent](env, name); err != nil {
					t.Fatal(err)
				}
				streams[logical] = From[joinVarOuterEvent](env, name)
			}
			for index, logical := range current.order {
				chainIndex[logical] = index
			}
			field := Field[joinVarOuterEvent, string]("key")
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
					t.Fatalf("unexpected three-stream edge kind %d", kind)
				}
			}
			selections := make([]JoinSelection, 0, 3)
			for logical := 0; logical < 3; logical++ {
				selections = append(selections, SelectFrom(
					chainIndex[logical], fmt.Sprintf("s%d", logical), JoinField[string](chainIndex[logical], "key"),
				))
			}
			plan, err := env.Build(chain.Select(selections...).Query(StatementName("join-var-outer-" + current.name)))
			if err != nil {
				t.Fatal(err)
			}
			engine := env.NewEngine()
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			defer deployment.Undeploy(context.Background())
			for logical := 0; logical < 3; logical++ {
				for id := 0; id < 2; id++ {
					if err := engine.Send(context.Background(), fmt.Sprintf("JoinVarOuterS%d", logical), joinVarOuterEvent{ID: id, Key: "A"}); err != nil {
						t.Fatal(err)
					}
				}
			}
			snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(snapshot.Results()) != 8 {
				t.Fatalf("three-stream outer %s cardinality = %d, want 8", current.name, len(snapshot.Results()))
			}
			seen := make(map[string]bool)
			for _, result := range snapshot.Results() {
				row, ok := result.Row()
				if !ok {
					t.Fatalf("three-stream outer result = %#v", result)
				}
				values := make([]string, 3)
				for logical := 0; logical < 3; logical++ {
					value := row.Get(fmt.Sprintf("s%d", logical))
					if value.IsNull() {
						values[logical] = "null"
					} else {
						values[logical] = value.String()
					}
				}
				seen[strings.Join(values, "/")] = true
			}
			if len(seen) != 1 || !seen["A/A/A"] {
				t.Fatalf("three-stream outer %s rows = %#v", current.name, seen)
			}
		})
	}
}
