package esper

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type joinFourOuterEvent struct {
	ID  int    `esper:"id"`
	Key string `esper:"key"`
}

func TestFourStreamMixedOuterVariantsMatchesEsper(t *testing.T) {
	type variant struct {
		name  string
		order []int
		kinds []JoinKind
		edges [][2]int
	}
	variants := []variant{
		{
			name:  "full-middle-forward",
			order: []int{0, 1, 2, 3},
			kinds: []JoinKind{JoinInner, JoinFullOuter, JoinInner},
			edges: [][2]int{{0, 1}, {1, 2}, {2, 3}},
		},
		{
			name:  "full-middle-reverse",
			order: []int{3, 2, 1, 0},
			kinds: []JoinKind{JoinInner, JoinFullOuter, JoinInner},
			edges: [][2]int{{3, 2}, {2, 1}, {1, 0}},
		},
		{
			name:  "full-sided-forward",
			order: []int{0, 1, 2, 3},
			kinds: []JoinKind{JoinInner, JoinFullOuter, JoinFullOuter},
			edges: [][2]int{{0, 1}, {1, 2}, {2, 3}},
		},
		{
			name:  "full-sided-reverse",
			order: []int{3, 2, 1, 0},
			kinds: []JoinKind{JoinInner, JoinFullOuter, JoinFullOuter},
			edges: [][2]int{{3, 2}, {2, 1}, {1, 0}},
		},
		{
			name:  "star-forward",
			order: []int{0, 1, 2, 3},
			kinds: []JoinKind{JoinLeftOuter, JoinFullOuter, JoinInner},
			edges: [][2]int{{0, 1}, {0, 2}, {0, 3}},
		},
		{
			name:  "star-reverse",
			order: []int{3, 0, 2, 1},
			kinds: []JoinKind{JoinInner, JoinFullOuter, JoinLeftOuter},
			edges: [][2]int{{3, 0}, {0, 2}, {0, 1}},
		},
	}

	for _, current := range variants {
		current := current
		t.Run(current.name, func(t *testing.T) {
			env := NewEnvironment()
			for logical := 0; logical < 4; logical++ {
				if _, err := RegisterStruct[joinFourOuterEvent](env, fmt.Sprintf("JoinFourOuterS%d", logical)); err != nil {
					t.Fatal(err)
				}
			}
			streams := make([]Stream[joinFourOuterEvent], 4)
			for logical := range streams {
				streams[logical] = From[joinFourOuterEvent](env, fmt.Sprintf("JoinFourOuterS%d", logical))
			}
			chainIndex := make(map[int]int, len(current.order))
			for index, logical := range current.order {
				chainIndex[logical] = index
			}
			field := Field[joinFourOuterEvent, string]("key")
			chain := JoinChain(JoinSource(streams[current.order[0]]))
			for edgeIndex, kind := range current.kinds {
				logicalLeft, logicalRight := current.edges[edgeIndex][0], current.edges[edgeIndex][1]
				condition := OnSourcesEqual(
					chainIndex[logicalLeft], field,
					chainIndex[logicalRight], field,
				)
				input := JoinSource(streams[current.order[edgeIndex+1]])
				switch kind {
				case JoinInner:
					chain = chain.InnerJoin(input, condition)
				case JoinLeftOuter:
					chain = chain.LeftOuterJoin(input, condition)
				case JoinRightOuter:
					chain = chain.RightOuterJoin(input, condition)
				case JoinFullOuter:
					chain = chain.FullOuterJoin(input, condition)
				default:
					t.Fatalf("unexpected join kind %d", kind)
				}
			}

			selections := make([]JoinSelection, 0, 4)
			for logical := 0; logical < 4; logical++ {
				selections = append(selections, SelectFrom(
					chainIndex[logical],
					fmt.Sprintf("s%d", logical),
					JoinField[string](chainIndex[logical], "key"),
				))
			}
			plan, err := env.Build(chain.Select(selections...).Query(
				StatementName("join-four-outer-" + current.name),
			))
			if err != nil {
				t.Fatal(err)
			}
			engine := env.NewEngine()
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			defer deployment.Undeploy(context.Background())

			for logical := 0; logical < 4; logical++ {
				if err := engine.Send(context.Background(), fmt.Sprintf("JoinFourOuterS%d", logical), joinFourOuterEvent{
					ID:  logical,
					Key: "A",
				}); err != nil {
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
					t.Fatalf("four-stream outer result = %#v", result)
				}
				values := make([]string, 4)
				for logical := 0; logical < 4; logical++ {
					value := row.Get(fmt.Sprintf("s%d", logical))
					if value.IsNull() {
						values[logical] = "null"
					} else {
						values[logical] = value.String()
					}
				}
				seen[strings.Join(values, "/")] = true
			}
			if !seen["A/A/A/A"] {
				t.Fatalf("four-stream %s rows = %#v", current.name, seen)
			}
		})
	}
}
