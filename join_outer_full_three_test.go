package esper

import (
	"context"
	"fmt"
	"testing"
)

type joinFullOuterEvent struct {
	Key    string `esper:"key"`
	Subkey string `esper:"subkey"`
	ID     string `esper:"id"`
}

// TestThreeStreamFullOuterStarCardinalityMatchesEsper covers the two
// EPLOuterFullJoin3Stream executions: a full outer hub with multiple rows per
// source and the same hub with a composite key. The final snapshot is checked
// after unmatched source rows have been replaced by complete combinations.
func TestThreeStreamFullOuterStarCardinalityMatchesEsper(t *testing.T) {
	for _, composite := range []bool{false, true} {
		composite := composite
		name := "single-key"
		if composite {
			name = "composite-key"
		}
		t.Run(name, func(t *testing.T) {
			env := NewEnvironment()
			streams := make([]Stream[joinFullOuterEvent], 3)
			for logical := range streams {
				source := fmt.Sprintf("JoinFullOuterS%d", logical)
				if _, err := RegisterStruct[joinFullOuterEvent](env, source); err != nil {
					t.Fatal(err)
				}
				streams[logical] = From[joinFullOuterEvent](env, source).Window(KeepAll())
			}
			key := func(source int) Expr {
				return JoinField[string](source, "key")
			}
			conditions := []JoinCondition{
				OnSourcesEqual(0, key(0), 1, key(1)),
				OnSourcesEqual(0, key(0), 2, key(2)),
			}
			if composite {
				conditions = append(conditions,
					OnSourcesEqual(0, JoinField[string](0, "subkey"), 1, JoinField[string](1, "subkey")),
					OnSourcesEqual(0, JoinField[string](0, "subkey"), 2, JoinField[string](2, "subkey")),
				)
			}
			plan, err := env.Build(JoinMany(
				JoinSource(streams[0]), JoinSource(streams[1]), JoinSource(streams[2]),
			).On(conditions...).FullOuter().Select(
				SelectFrom(0, "s0", JoinField[string](0, "id")),
				SelectFrom(0, "s0Key", key(0)),
				SelectFrom(1, "s1", JoinField[string](1, "id")),
				SelectFrom(1, "s1Key", key(1)),
				SelectFrom(2, "s2", JoinField[string](2, "id")),
				SelectFrom(2, "s2Key", key(2)),
			).Query(StatementName("full-outer-star-" + name)))
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			defer deployment.Undeploy(context.Background())

			send := func(source int, event joinFullOuterEvent) {
				t.Helper()
				if err := engine.Send(context.Background(), fmt.Sprintf("JoinFullOuterS%d", source), event); err != nil {
					t.Fatal(err)
				}
			}
			// Downstream rows arrive first, just as in Esper's full-outer
			// execution. They must remain visible and later be replaced when the
			// hub row arrives.
			send(1, joinFullOuterEvent{Key: "A", Subkey: "x", ID: "A-s1-1"})
			send(1, joinFullOuterEvent{Key: "A", Subkey: "x", ID: "A-s1-2"})
			send(2, joinFullOuterEvent{Key: "A", Subkey: "x", ID: "A-s2-1"})
			send(2, joinFullOuterEvent{Key: "A", Subkey: "x", ID: "A-s2-2"})
			send(1, joinFullOuterEvent{Key: "B", Subkey: "y", ID: "B-s1-1"})
			send(2, joinFullOuterEvent{Key: "C", Subkey: "z", ID: "C-s2-1"})
			send(2, joinFullOuterEvent{Key: "C", Subkey: "z", ID: "C-s2-2"})

			send(0, joinFullOuterEvent{Key: "A", Subkey: "x", ID: "A-s0-1"})
			send(0, joinFullOuterEvent{Key: "B", Subkey: "y", ID: "B-s0-1"})
			send(0, joinFullOuterEvent{Key: "C", Subkey: "z", ID: "C-s0-1"})
			// Two rows on both sides of the hub produce four rows. In the
			// composite case, the following same-key/different-subkey row must
			// remain unmatched instead of joining on only the first column.
			send(0, joinFullOuterEvent{Key: "D", Subkey: "q", ID: "D-s0-1"})
			send(1, joinFullOuterEvent{Key: "D", Subkey: "q", ID: "D-s1-1"})
			send(1, joinFullOuterEvent{Key: "D", Subkey: "q", ID: "D-s1-2"})
			send(2, joinFullOuterEvent{Key: "D", Subkey: "q", ID: "D-s2-1"})
			send(2, joinFullOuterEvent{Key: "D", Subkey: "q", ID: "D-s2-2"})
			if composite {
				send(1, joinFullOuterEvent{Key: "D", Subkey: "different", ID: "D-s1-mismatch"})
			}

			snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			counts := make(map[string]int)
			for _, result := range snapshot.Results() {
				row, ok := result.Row()
				if !ok {
					t.Fatalf("full outer result = %#v", result)
				}
				for _, field := range []string{"s0Key", "s1Key", "s2Key"} {
					value := row.Get(field)
					if !value.IsNull() {
						counts[value.String()]++
					}
				}
			}
			wantD := 12
			if composite {
				wantD = 13
			}
			if counts["A"] != 12 || counts["B"] != 2 || counts["C"] != 3 || counts["D"] != wantD {
				t.Fatalf("full outer star key cardinalities = %#v, want D=%d", counts, wantD)
			}
		})
	}
}
